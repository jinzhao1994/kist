// Package syncclient implements authenticated NAS synchronization transport.
// It never edits archive or client state; library owns those transitions.
package syncclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

type Options struct {
	// AllowHTTP explicitly permits cleartext transport outside loopback.
	AllowHTTP bool
	Transport http.RoundTripper
}

type Peer struct {
	base   string
	token  string
	client *http.Client
}

var _ library.SyncPeer = (*Peer)(nil)

func New(address, token string, options Options) (*Peer, error) {
	if err := library.ValidateClientConnection(library.ClientConnection{Server: address, AllowHTTP: options.AllowHTTP}, token); err != nil {
		return nil, err
	}
	u, _ := url.Parse(address)
	transport := options.Transport
	if transport == nil {
		t := http.DefaultTransport.(*http.Transport).Clone()
		// Final content verification can legitimately take minutes for large
		// originals. The caller's context owns cancellation for that work.
		t.ResponseHeaderTimeout = 0
		transport = t
	}
	return &Peer{base: strings.TrimRight(u.String(), "/") + "/api/v1/sync/", token: token, client: &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

// HTTPError is a transport failure, never proof that a submission was rejected.
type HTTPError struct{ Status int }

func (e *HTTPError) Error() string { return fmt.Sprintf("sync HTTP status %d", e.Status) }

type remoteError struct{ problem wire.Error }

func (e *remoteError) Error() string { return e.problem.Code + ": " + e.problem.Message }

func (p *Peer) request(ctx context.Context, method, path string, body io.Reader, headers http.Header) (*http.Response, error) {
	r, err := http.NewRequestWithContext(ctx, method, p.base+path, body)
	if err != nil {
		return nil, err
	}
	for k, values := range headers {
		r.Header[k] = values
	}
	r.Header.Set("Authorization", "Bearer "+p.token)
	return p.client.Do(r)
}

func (p *Peer) json(ctx context.Context, method, path string, input, output any, limit int64) error {
	var body io.Reader
	if input != nil {
		raw, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	response, err := p.request(ctx, method, path, body, http.Header{"Content-Type": {"application/json"}})
	if err != nil {
		return err
	}
	return decodeResponse(response, output, limit)
}

func decodeResponse(response *http.Response, output any, limit int64) error {
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return &HTTPError{Status: response.StatusCode}
	}
	var reader io.Reader = response.Body
	var limited *io.LimitedReader
	if limit > 0 {
		limited = &io.LimitedReader{R: reader, N: limit + 1}
		reader = limited
	}
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(output); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("sync response contains multiple JSON values")
		}
		return err
	}
	if limited != nil && limited.N == 0 {
		return errors.New("sync response exceeds size limit")
	}
	return nil
}

func (p *Peer) Capabilities(ctx context.Context) (library.SyncCapabilities, error) {
	var out library.SyncCapabilities
	err := p.json(ctx, "GET", "capabilities", nil, &out, 1<<20)
	return out, err
}
func (p *Peer) Head(ctx context.Context) (library.SyncBoundary, error) {
	var out library.SyncBoundary
	err := p.json(ctx, "GET", "head", nil, &out, 1<<20)
	return out, err
}
func (p *Peer) Catalog(ctx context.Context, boundary library.SyncBoundary, after int64, limit, maxBytes int) (library.SyncCatalogPage, error) {
	var out library.SyncCatalogPage
	// A single legacy event can exceed the requested page byte budget. Decode
	// the authenticated catalog as a stream, without truncating that event.
	err := p.json(ctx, "POST", "catalog", struct {
		Boundary library.SyncBoundary `json:"boundary"`
		After    int64                `json:"after"`
		Limit    int                  `json:"limit"`
		MaxBytes int                  `json:"max_bytes"`
	}{boundary, after, limit, maxBytes}, &out, 0)
	return out, err
}
func (p *Peer) HasOriginal(ctx context.Context, key string) (bool, error) {
	var out library.SyncOriginalStatus
	err := p.json(ctx, "GET", "originals/"+url.PathEscape(key), nil, &out, 1<<20)
	return out.Present, err
}
func (p *Peer) Submit(ctx context.Context, proposal library.SyncProposal) (library.SyncResult, error) {
	return p.submit(ctx, "submit", proposal)
}

// SubmitAt keeps the head condition outside the immutable intent and its digest.
func (p *Peer) SubmitAt(ctx context.Context, proposal library.SyncProposal, expected library.SyncBoundary) (library.SyncResult, error) {
	return p.submit(ctx, "submit-conditional", struct {
		Proposal library.SyncProposal `json:"proposal"`
		Expected library.SyncBoundary `json:"expected"`
	}{proposal, expected})
}

func (p *Peer) submit(ctx context.Context, path string, body any) (library.SyncResult, error) {
	var out struct {
		Result   *library.SyncResult `json:"result"`
		Rejected bool                `json:"rejected"`
		Error    *wire.Error         `json:"error"`
	}
	if err := p.json(ctx, "POST", path, body, &out, 1<<20); err != nil {
		return library.SyncResult{}, err
	}
	if out.Result != nil && out.Error == nil && !out.Rejected && out.Result.OpID > 0 && !out.Result.AcceptedAt.IsZero() {
		return *out.Result, nil
	}
	if out.Error != nil && out.Result == nil && out.Error.Code != "" && out.Error.Class != "" {
		err := &remoteError{problem: *out.Error}
		if out.Rejected {
			return library.SyncResult{}, &library.SyncRejectedError{Cause: err}
		}
		return library.SyncResult{}, err
	}
	return library.SyncResult{}, errors.New("invalid sync submission response; outcome is uncertain")
}
