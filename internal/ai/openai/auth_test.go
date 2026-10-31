package openai

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/jinzhao1994/kist/internal/ai"
)

type rewriteTransport struct{ target string }

func (r rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	next := req.Clone(req.Context())
	u, _ := url.Parse(r.target)
	next.URL.Scheme = u.Scheme
	next.URL.Host = u.Host
	return http.DefaultTransport.RoundTrip(next)
}
func TestSignInVerifiesIdentityAndState(t *testing.T) {
	for _, mode := range []string{"success", "bad-nonce", "bad-signature", "bad-audience", "bad-issuer", "expired", "wrong-subject", "missing-scope"} {
		t.Run(mode, func(t *testing.T) {
			key, err := rsa.GenerateKey(rand.Reader, 2048)
			if err != nil {
				t.Fatal(err)
			}
			signingKey := key
			if mode == "bad-signature" {
				signingKey, _ = rsa.GenerateKey(rand.Reader, 2048)
			}
			signer, _ := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: signingKey}, (&jose.SignerOptions{}).WithHeader("kid", "test"))
			nonce := ""
			exchangeCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/.well-known/openid-configuration":
					json.NewEncoder(w).Encode(map[string]any{"issuer": Issuer, "authorization_endpoint": Issuer + "/authorize", "token_endpoint": Issuer + "/token", "jwks_uri": Issuer + "/jwks", "id_token_signing_alg_values_supported": []string{"RS256"}})
				case "/jwks":
					json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "test", Algorithm: "RS256", Use: "sig"}}})
				case "/api/accounts/oauth/token":
					exchangeCount++
					r.ParseForm()
					if r.Form.Get("client_id") != "issued-client" || r.Form.Get("code_verifier") == "" || r.Form.Get("resource") != Resource {
						t.Error("incorrect exchange")
					}
					claims := map[string]any{"iss": Issuer, "aud": "issued-client", "sub": "person", "nonce": nonce, "iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(), "email": "person@example.test"}
					switch mode {
					case "bad-nonce":
						claims["nonce"] = "wrong"
					case "bad-audience":
						claims["aud"] = "wrong"
					case "bad-issuer":
						claims["iss"] = "https://wrong.test"
					case "expired":
						claims["exp"] = time.Now().Add(-time.Hour).Unix()
					}
					raw, _ := json.Marshal(claims)
					signed, _ := signer.Sign(raw)
					token, _ := signed.CompactSerialize()
					scope := "openid resource.invoke chatgpt.tokens.use.direct"
					if mode == "missing-scope" {
						scope = "openid"
					}
					json.NewEncoder(w).Encode(map[string]any{"access_token": "private-access", "refresh_token": "private-refresh", "id_token": token, "token_type": "Bearer", "expires_in": 3600, "scope": scope})
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client := &http.Client{Transport: rewriteTransport{server.URL}, Timeout: time.Second * 5}
			saved := ai.Credentials{HostID: "urn:uuid:test"}
			if mode == "wrong-subject" {
				saved.ClientID = "issued-client"
				saved.Subject = "another-person"
			}
			var result ai.Credentials
			attempt, err := Begin(saved, func(_ context.Context, c ai.Credentials) error { result = c; return nil }, client)
			if err != nil {
				t.Fatal(err)
			}
			defer attempt.Cancel()
			u, _ := url.Parse(attempt.URL)
			q := u.Query()
			nonce = q.Get("nonce")
			callback := q.Get("redirect_uri")
			if q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
				t.Fatal("missing PKCE")
			}
			bad, err := http.Get(callback + "?state=wrong&code=code&client_id=issued-client")
			if err != nil {
				t.Fatal(err)
			}
			bad.Body.Close()
			if bad.StatusCode != 400 || exchangeCount != 0 {
				t.Fatal("bad state exchanged code")
			}
			query := url.Values{"state": {q.Get("state")}, "code": {"code"}, "client_id": {"issued-client"}}
			res, err := http.Get(callback + "?" + query.Encode())
			if err == nil {
				io.Copy(io.Discard, res.Body)
				res.Body.Close()
			}
			select {
			case err := <-attempt.Done:
				if (err == nil) != (mode == "success") {
					t.Fatal(err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("login did not finish")
			}
			if mode == "success" {
				if result.Subject != "person" || result.AccessToken != "private-access" {
					t.Fatal("credentials missing")
				}
			} else if result.AccessToken != "" {
				t.Fatal("invalid identity saved")
			}
		})
	}
}
func TestRefreshPreservesSessionIdentity(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("client_id") != "client" || r.Form.Has("scope") {
			t.Error("invalid refresh")
		}
		w.Write([]byte(`{"access_token":"new","refresh_token":"rotated","token_type":"Bearer","expires_in":3600}`))
	}))
	defer server.Close()
	previous := authHTTP
	authHTTP = &http.Client{Transport: rewriteTransport{server.URL}}
	defer func() { authHTTP = previous }()
	c, err := Refresh(t.Context(), ai.Credentials{ClientID: "client", Subject: "person", AccessToken: "old", RefreshToken: "old-refresh", Scope: "resource.invoke chatgpt.tokens.use.direct"})
	if err != nil || c.AccessToken != "new" || c.RefreshToken != "rotated" || c.Subject != "person" {
		t.Fatal(c.Subject, err)
	}
	if strings.Contains(c.Scope, "openid") {
		t.Fatal("scope changed")
	}
}

func TestRevocationConfirmsRemoteOutcome(t *testing.T) {
	for _, status := range []int{200, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/.well-known/openid-configuration" {
					json.NewEncoder(w).Encode(map[string]string{"revocation_endpoint": Issuer + "/revoke"})
					return
				}
				if r.URL.Path != "/revoke" {
					t.Error("unexpected revocation destination")
				}
				r.ParseForm()
				if r.Form.Get("client_id") != "client" || r.Form.Get("token") != "refresh" || r.Form.Get("token_type_hint") != "refresh_token" {
					t.Error("incorrect revocation")
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			previous := authHTTP
			authHTTP = &http.Client{Transport: rewriteTransport{server.URL}}
			defer func() { authHTTP = previous }()
			err := Revoke(t.Context(), ai.Credentials{ClientID: "client", RefreshToken: "refresh"})
			if (err == nil) != (status == 200) {
				t.Fatal(err)
			}
		})
	}
}
