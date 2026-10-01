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

	"github.com/MicahParks/keyfunc/v3"
	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/fx"
)

func TestOIDCRejectsInvalidClaimsAlgorithmsAndSignatures(t *testing.T) {
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	other, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	jwks, e := json.Marshal(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": "trusted", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}})
	if e != nil {
		t.Fatal(e)
	}
	kf, e := keyfunc.NewJWKSetJSON(jwks)
	if e != nil {
		t.Fatal(e)
	}
	v := &Verifier{now: time.Now, issuer: "issuer", audience: "api", kf: kf, fetched: time.Now(), attempted: time.Now(), gate: make(chan struct{}, 1)}
	for _, tc := range []struct {
		name       string
		change     func(jwt.MapClaims)
		method     jwt.SigningMethod
		signingKey any
	}{
		{"wrong issuer", func(c jwt.MapClaims) { c["iss"] = "attacker" }, jwt.SigningMethodRS256, key},
		{"missing issuer", func(c jwt.MapClaims) { delete(c, "iss") }, jwt.SigningMethodRS256, key},
		{"wrong audience", func(c jwt.MapClaims) { c["aud"] = "another-api" }, jwt.SigningMethodRS256, key},
		{"missing audience", func(c jwt.MapClaims) { delete(c, "aud") }, jwt.SigningMethodRS256, key},
		{"expired", func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Minute).Unix() }, jwt.SigningMethodRS256, key},
		{"missing expiry", func(c jwt.MapClaims) { delete(c, "exp") }, jwt.SigningMethodRS256, key},
		{"invalid expiry type", func(c jwt.MapClaims) { c["exp"] = "tomorrow" }, jwt.SigningMethodRS256, key},
		{"not yet valid", func(c jwt.MapClaims) { c["nbf"] = time.Now().Add(time.Minute).Unix() }, jwt.SigningMethodRS256, key},
		{"missing subject", func(c jwt.MapClaims) { delete(c, "sub") }, jwt.SigningMethodRS256, key},
		{"invalid subject type", func(c jwt.MapClaims) { c["sub"] = 42 }, jwt.SigningMethodRS256, key},
		{"wrong signature", func(jwt.MapClaims) {}, jwt.SigningMethodRS256, other},
		{"HMAC confusion", func(jwt.MapClaims) {}, jwt.SigningMethodHS256, []byte("public-key-as-secret")},
		{"unsigned", func(jwt.MapClaims) {}, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claims := jwt.MapClaims{"iss": "issuer", "aud": "api", "sub": "subject", "exp": time.Now().Add(time.Hour).Unix(), "providerId": "p", "role": "internal"}
			tc.change(claims)
			token := jwt.NewWithClaims(tc.method, claims)
			token.Header["kid"] = "trusted"
			raw, e := token.SignedString(tc.signingKey)
			if e != nil {
				t.Fatal(e)
			}
			p, e := v.Verify(context.Background(), raw)
			if !errors.Is(e, ErrInvalidToken) || p != (Principal{}) {
				t.Fatal("invalid token yielded authority", p, e)
			}
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("Authorization", "Bearer "+raw)
			w := httptest.NewRecorder()
			v.Authenticate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unauthorized handler reached") })).ServeHTTP(w, r)
			if w.Code != 401 || w.Body.String() != `{"code":"UNAUTHENTICATED"}` || w.Header().Get("Retry-After") != "" {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
}

func TestOIDCKeyRotationAcceptsNewSignatureAndRejectsRemovedKey(t *testing.T) {
	first, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	second, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	var rotated atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, kid := first, "first"
		if rotated.Load() {
			key, kid = second, "second"
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "kid": kid, "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}}); err != nil {
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
	sign := func(key *rsa.PrivateKey, kid string) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{"iss": "issuer", "aud": "api", "sub": "service", "providerId": "p", "exp": time.Now().Add(time.Hour).Unix()})
		tok.Header["kid"] = kid
		raw, e := tok.SignedString(key)
		if e != nil {
			t.Fatal(e)
		}
		return raw
	}
	oldToken, newToken := sign(first, "first"), sign(second, "second")
	if p, e := v.Verify(context.Background(), oldToken); e != nil || p.ProviderID != "p" {
		t.Fatal(p, e)
	}
	rotated.Store(true)
	v.mu.Lock()
	v.attempted = time.Time{}
	v.mu.Unlock()
	if p, e := v.Verify(context.Background(), newToken); e != nil || p.Subject != "service" || p.ProviderID != "p" {
		t.Fatal("rotation rejected valid new signing key", p, e)
	}
	if p, e := v.Verify(context.Background(), oldToken); !errors.Is(e, ErrInvalidToken) || p != (Principal{}) {
		t.Fatal("removed key still grants authority after refresh", p, e)
	}
}
