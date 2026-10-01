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
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/fx"
)

func TestOIDCStartupCacheRotationAndFailureClassification(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var mode atomic.Int32
	entered := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch mode.Load() {
		case 1:
			http.Error(w, "offline", http.StatusServiceUnavailable)
			return
		case 2:
			entered <- struct{}{}
			<-r.Context().Done()
			return
		case 3:
			if _, err := w.Write([]byte(`{}`)); err != nil {
				t.Error(err)
			}
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "use": "sig", "alg": "RS256", "kid": "known", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}}); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	var v *Verifier
	app := fx.New(fx.NopLogger, fx.Provide(func(lc fx.Lifecycle) *Verifier {
		return NewVerifier(lc, config.Config{OIDCIssuer: "issuer", OIDCAudience: "api", OIDCJWKSURL: server.URL})
	}), fx.Populate(&v))
	if err = app.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := app.Stop(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	token := func(kid string, expired bool) string {
		exp := time.Now().Add(time.Minute)
		if expired {
			exp = time.Now().Add(-time.Minute)
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"iss": "issuer", "aud": "api", "sub": "service", "providerId": "provider-a", "exp": exp.Unix()})
		tok.Header["kid"] = kid
		s, e := tok.SignedString(key)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	if p, e := v.Verify(context.Background(), token("known", false)); e != nil || p.ProviderID != "provider-a" {
		t.Fatalf("%+v %v", p, e)
	}
	mode.Store(1)
	if _, e := v.Verify(context.Background(), token("known", false)); e != nil {
		t.Fatalf("cached key: %v", e)
	}
	if _, e := v.Verify(context.Background(), token("known", true)); !errors.Is(e, ErrInvalidToken) {
		t.Fatalf("expired: %v", e)
	}
	allowFetch := func() { v.mu.Lock(); v.attempted = time.Time{}; v.mu.Unlock() }
	allowFetch()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token("rotated", false))
	recorder := httptest.NewRecorder()
	v.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("unauthorized handler reached") })).ServeHTTP(recorder, req)
	if recorder.Code != 503 {
		t.Fatalf("outage status %d", recorder.Code)
	}
	mode.Store(0)
	allowFetch()
	if _, e := v.Verify(context.Background(), token("missing", false)); !errors.Is(e, ErrInvalidToken) {
		t.Fatalf("unknown key with healthy IdP: %v", e)
	}
	mode.Store(2)
	allowFetch()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	raw := token("rotated", false)
	go func() { _, e := v.Verify(ctx, raw); done <- e }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("fetch not started")
	}
	cancel()
	select {
	case e := <-done:
		if !errors.Is(e, context.Canceled) || !errors.Is(e, ErrJWKSUnavailable) {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("request cancellation ignored")
	}
	for _, m := range []int32{1, 3} {
		mode.Store(m)
		other := fx.New(fx.NopLogger, fx.Invoke(func(lc fx.Lifecycle) { NewVerifier(lc, config.Config{OIDCJWKSURL: server.URL}) }))
		if e := other.Start(context.Background()); !errors.Is(e, ErrJWKSUnavailable) {
			t.Fatalf("startup must fail: %v", e)
		}
	}
}
