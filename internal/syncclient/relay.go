package syncclient

import (
	"context"
	"errors"

	"github.com/jinzhao1994/kist/internal/domain"
	"github.com/jinzhao1994/kist/internal/library"
	"github.com/jinzhao1994/kist/internal/wire"
)

func (p *Peer) RelayHead(ctx context.Context) (library.RelayHead, error) {
	var out library.RelayHead
	err := p.json(ctx, "GET", "relay/head", nil, &out, 1<<20)
	return out, err
}
func (p *Peer) RelayCatalog(ctx context.Context, head library.RelayHead, after, limit, maxBytes int) (library.RelayPage, error) {
	var out library.RelayPage
	err := p.json(ctx, "POST", "relay/catalog", struct {
		Head     library.RelayHead `json:"head"`
		After    int               `json:"after"`
		Limit    int               `json:"limit"`
		MaxBytes int               `json:"max_bytes"`
	}{head, after, limit, maxBytes}, &out, 0)
	return out, err
}
func (p *Peer) RelayRootCatalog(ctx context.Context, h library.SyncBoundary, a int64, l, b int) (library.SyncCatalogPage, error) {
	return p.Catalog(ctx, h, a, l, b)
}
func (p *Peer) ReceiveRelay(ctx context.Context, id domain.ID, proposal library.SyncProposal) (library.RelayOutcome, error) {
	return p.receiveRelay(ctx, id, proposal, nil)
}
func (p *Peer) ReceiveRelayAt(ctx context.Context, id domain.ID, proposal library.SyncProposal, head library.RelayHead) (library.RelayOutcome, error) {
	return p.receiveRelay(ctx, id, proposal, &head)
}
func (p *Peer) receiveRelay(ctx context.Context, id domain.ID, proposal library.SyncProposal, head *library.RelayHead) (library.RelayOutcome, error) {
	var out struct {
		Result   *library.RelayOutcome `json:"result"`
		Rejected bool                  `json:"rejected"`
		Error    *wire.Error           `json:"error"`
	}
	err := p.json(ctx, "POST", "relay/submit", struct {
		LibraryID domain.ID            `json:"library_id"`
		Proposal  library.SyncProposal `json:"proposal"`
		Expected  *library.RelayHead   `json:"expected,omitempty"`
	}{id, proposal, head}, &out, 20<<20)
	if err != nil {
		return library.RelayOutcome{}, err
	}
	if out.Result != nil && out.Error == nil && !out.Rejected {
		return *out.Result, nil
	}
	if out.Error != nil && out.Result == nil && out.Error.Code != "" && out.Error.Class != "" {
		err = &remoteError{problem: *out.Error}
		if out.Rejected {
			err = &library.SyncRejectedError{Cause: err}
		}
		return library.RelayOutcome{}, err
	}
	return library.RelayOutcome{}, errors.New("invalid relay submission response; outcome is uncertain")
}
