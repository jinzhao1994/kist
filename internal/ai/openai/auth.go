package openai

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jinzhao1994/kist/internal/ai"
)

var authHTTP = &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func randomValue() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// Attempt owns only an expiring, single-use OAuth transaction.
type Attempt struct {
	URL    string
	Done   <-chan error
	Cancel func()
}

func Begin(saved ai.Credentials, save func(context.Context, ai.Credentials) error) (*Attempt, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	state, nonce, verifier := randomValue(), randomValue(), randomValue()
	digest := sha256.Sum256([]byte(verifier))
	callback := "http://" + listener.Addr().String() + "/auth/callback"
	clientID := saved.ClientID
	if clientID == "" {
		clientID = "dynamic_agent_client"
	}
	q := url.Values{"client_id": {clientID}, "ext_agent_host_id": {saved.HostID}, "response_type": {"code"}, "redirect_uri": {callback}, "scope": {"openid profile email offline_access resource.invoke chatgpt.tokens.use.direct"}, "resource": {Resource}, "state": {state}, "nonce": {nonce}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(digest[:])}}
	if clientID == "dynamic_agent_client" {
		q.Set("agent_name_hint", "kist")
	}
	done := make(chan error, 1)
	var once sync.Once
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second}
	finish := func(err error) { once.Do(func() { done <- err; cancel() }) }
	var callbackMu sync.Mutex
	consumed := false
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if r.Method != "GET" || r.URL.Path != "/auth/callback" || r.Host != listener.Addr().String() {
			http.Error(w, "Invalid callback", 400)
			return
		}
		values := r.URL.Query()
		if subtle.ConstantTimeCompare([]byte(values.Get("state")), []byte(state)) != 1 {
			http.Error(w, "Invalid sign-in state", 400)
			return
		}
		callbackMu.Lock()
		if consumed {
			callbackMu.Unlock()
			http.Error(w, "Sign-in already consumed", 400)
			return
		}
		consumed = true
		callbackMu.Unlock()
		fail := func() {
			http.Error(w, "ChatGPT sign-in failed. Return to kist and try again.", 400)
			finish(errors.New("ChatGPT sign-in failed; please try again"))
		}
		if values.Get("error") != "" || values.Get("code") == "" {
			fail()
			return
		}
		issued := values.Get("client_id")
		if clientID != "dynamic_agent_client" {
			if issued != "" && issued != clientID {
				fail()
				return
			}
			issued = clientID
		}
		if issued == "" || issued == "dynamic_agent_client" {
			fail()
			return
		}
		token, err := exchange(ctx, url.Values{"grant_type": {"authorization_code"}, "client_id": {issued}, "code": {values.Get("code")}, "code_verifier": {verifier}, "redirect_uri": {callback}, "resource": {Resource}})
		if err != nil {
			fail()
			return
		}
		verifyCtx := oidc.ClientContext(ctx, authHTTP)
		provider, err := oidc.NewProvider(verifyCtx, Issuer)
		if err != nil {
			fail()
			return
		}
		verified, err := provider.Verifier(&oidc.Config{ClientID: issued}).Verify(verifyCtx, token.IDToken)
		if err != nil || verified.Nonce != nonce || verified.Subject == "" || (saved.Subject != "" && saved.Subject != verified.Subject) {
			fail()
			return
		}
		var claims struct {
			Email string `json:"email"`
		}
		if verified.Claims(&claims) != nil {
			fail()
			return
		}
		if !hasScope(token.Scope, "chatgpt.tokens.use.direct") || !hasScope(token.Scope, "resource.invoke") {
			fail()
			return
		}
		next := ai.Credentials{HostID: saved.HostID, Provider: "openai", ClientID: issued, Subject: verified.Subject, Email: claims.Email, AccessToken: token.AccessToken, RefreshToken: token.RefreshToken, IDToken: token.IDToken, Scope: token.Scope, ExpiresAt: time.Now().Unix() + token.ExpiresIn}
		if err = save(ctx, next); err != nil {
			fail()
			return
		}
		fmt.Fprintln(w, "Connected to ChatGPT. You can close this window and return to kist.")
		finish(nil)
	})
	go func() {
		<-ctx.Done()
		shutdownCtx, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		server.Shutdown(shutdownCtx)
		finish(errors.New("ChatGPT sign-in expired or was cancelled"))
	}()
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			finish(errors.New("ChatGPT callback listener stopped"))
		}
	}()
	return &Attempt{URL: Issuer + "/api/accounts/authorize?" + q.Encode(), Done: done, Cancel: cancel}, nil
}

type tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	Scope        string `json:"scope"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

func exchange(ctx context.Context, form url.Values) (tokens, error) {
	var out tokens
	req, err := http.NewRequestWithContext(ctx, "POST", Issuer+"/api/accounts/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return out, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := authHTTP.Do(req)
	if err != nil {
		return out, errors.New("ChatGPT token exchange failed")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return out, fmt.Errorf("ChatGPT token exchange returned HTTP %d; reconnect your account", res.StatusCode)
	}
	if json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&out) != nil || out.AccessToken == "" || !strings.EqualFold(out.TokenType, "Bearer") || out.ExpiresIn <= 0 {
		return out, errors.New("invalid ChatGPT token response")
	}
	return out, nil
}
func hasScope(scopes, scope string) bool {
	for _, s := range strings.Fields(scopes) {
		if s == scope {
			return true
		}
	}
	return false
}
func Refresh(ctx context.Context, c ai.Credentials) (ai.Credentials, error) {
	if c.AccessToken == "" {
		return c, errors.New("connect ChatGPT first")
	}
	if c.ExpiresAt > time.Now().Unix()+60 {
		return c, nil
	}
	if c.RefreshToken == "" {
		return c, errors.New("ChatGPT session expired; reconnect your account")
	}
	t, err := exchange(ctx, url.Values{"grant_type": {"refresh_token"}, "client_id": {c.ClientID}, "refresh_token": {c.RefreshToken}, "resource": {Resource}})
	if err != nil {
		return c, err
	}
	if t.Scope != "" {
		c.Scope = t.Scope
	}
	if !hasScope(c.Scope, "chatgpt.tokens.use.direct") || !hasScope(c.Scope, "resource.invoke") {
		return c, errors.New("ChatGPT plan permission is missing")
	}
	c.AccessToken = t.AccessToken
	if t.RefreshToken != "" {
		c.RefreshToken = t.RefreshToken
	}
	c.ExpiresAt = time.Now().Unix() + t.ExpiresIn
	return c, nil
}

// Revoke attempts remote revocation; callers must clear local tokens even on failure.
func Revoke(ctx context.Context, c ai.Credentials) error {
	if c.RefreshToken == "" {
		return nil
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", Issuer+"/.well-known/openid-configuration", nil)
	res, err := authHTTP.Do(req)
	if err != nil {
		return errors.New("remote revocation was not confirmed")
	}
	defer res.Body.Close()
	var meta struct {
		Endpoint string `json:"revocation_endpoint"`
	}
	if res.StatusCode != 200 || json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&meta) != nil {
		return errors.New("remote revocation was not confirmed")
	}
	endpoint, err := url.Parse(meta.Endpoint)
	if err != nil || endpoint.Scheme != "https" || endpoint.Host != "auth.openai.com" {
		return errors.New("invalid revocation endpoint")
	}
	form := url.Values{"token": {c.RefreshToken}, "token_type_hint": {"refresh_token"}, "client_id": {c.ClientID}}
	req, _ = http.NewRequestWithContext(ctx, "POST", meta.Endpoint, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err = authHTTP.Do(req)
	if err != nil {
		return errors.New("remote revocation was not confirmed")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return errors.New("remote revocation was not confirmed")
	}
	return nil
}
