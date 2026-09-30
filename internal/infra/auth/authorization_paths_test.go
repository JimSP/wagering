package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/fx"
)

func TestAuthorizationPolicyMatrix(t *testing.T) {
	for name, wrap := range map[string]func(http.Handler) http.Handler{"internal": RequireInternal, "provider": RequireProvider, "either": RequireProviderOrInternal} {
		for label, p := range map[string]*Principal{"absent": nil, "unprivileged": {}, "internal": {Subject: "svc", Internal: true}, "provider": {Subject: "svc", ProviderID: "p"}} {
			t.Run(name+"/"+label, func(t *testing.T) {
				r := httptest.NewRequest("GET", "/", nil)
				if p != nil {
					r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, *p))
				}
				called := false
				w := httptest.NewRecorder()
				wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					called = true
					got, ok := PrincipalFrom(r.Context())
					if !ok || got != *p {
						t.Fatal("identity lost")
					}
					w.WriteHeader(204)
				})).ServeHTTP(w, r)
				allowed := p != nil && ((name == "internal" && p.Internal) || (name == "provider" && p.ProviderID != "") || (name == "either" && (p.Internal || p.ProviderID != "")))
				if allowed {
					if !called || w.Code != 204 {
						t.Fatal(w.Code)
					}
				} else if called || w.Code != 403 || w.Body.String() != `{"code":"FORBIDDEN"}` {
					t.Fatal(w.Code, w.Body.String())
				}
			})
		}
	}
}

func TestAuthenticationClaimsHeadersAndCacheBounds(t *testing.T) {
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "alg": "RS256", "kid": "k", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	var v *Verifier
	app := fx.New(fx.NopLogger, fx.Invoke(func(lc fx.Lifecycle) {
		v = NewVerifier(lc, config.Config{OIDCIssuer: "issuer", OIDCAudience: "api", OIDCJWKSURL: server.URL})
	}))
	if e = app.Start(context.Background()); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	token := func(subject, kid string) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"iss": "issuer", "aud": "api", "sub": subject, "exp": time.Now().Add(time.Minute).Unix(), "role": "internal"})
		if kid != "" {
			tok.Header["kid"] = kid
		}
		s, e := tok.SignedString(key)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	for name, header := range map[string]string{"raw JWT without Bearer": token("svc", "k"), "missing": "", "empty": "Bearer ", "malformed": "Bearer bad", "no subject": "Bearer " + token("", "k"), "no kid": "Bearer " + token("svc", ""), "valid": "Bearer " + token("svc", "k")} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("Authorization", header)
			w := httptest.NewRecorder()
			called := false
			v.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				p, ok := PrincipalFrom(r.Context())
				if !ok || !p.Internal || p.Subject != "svc" {
					t.Fatal(p)
				}
				w.WriteHeader(204)
			})).ServeHTTP(w, r)
			if name == "valid" {
				if !called || w.Code != 204 {
					t.Fatal(w.Code)
				}
			} else if called || w.Code != 401 || w.Body.String() != `{"code":"UNAUTHENTICATED"}` || w.Header().Get("Content-Type") != "application/json" {
				t.Fatal(w.Code)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = v.key(ctx, &jwt.Token{Header: map[string]any{"kid": "k"}}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	v.gate <- struct{}{}
	if e = v.refresh(ctx, false); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	<-v.gate
	v.mu.Lock()
	v.fetched = time.Now().Add(-2 * time.Hour)
	v.attempted = time.Now()
	v.mu.Unlock()
	if _, e = v.key(context.Background(), &jwt.Token{Header: map[string]any{"kid": "k"}}); !errors.Is(e, ErrJWKSUnavailable) {
		t.Fatal("stale keys accepted", e)
	}
	v.jwksURL = ":"
	if e = v.refresh(context.Background(), true); !errors.Is(e, ErrJWKSUnavailable) {
		t.Fatal(e)
	}
}
