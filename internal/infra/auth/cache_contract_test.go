package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/fx"
)

type contractLifecycle struct{ hooks []fx.Hook }

func (l *contractLifecycle) Append(h fx.Hook) { l.hooks = append(l.hooks, h) }

type contractTransport struct {
	calls, closed int
	body          string
	err           error
}

func (tr *contractTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	tr.calls++
	if tr.err != nil {
		return nil, tr.err
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tr.body)), Header: make(http.Header), Request: r}, nil
}
func (tr *contractTransport) CloseIdleConnections() { tr.closed++ }

func TestJWKSCacheExpiryThrottleAndLifecycleContracts(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"keys": []any{map[string]any{"kty": "RSA", "alg": "RS256", "kid": "known", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": "AQAB"}}})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name      string
		age       time.Duration
		wantCalls int
		blocked   bool
	}{
		{"fresh cache", time.Minute, 0, false},
		{"one ns before expiry", time.Hour - time.Nanosecond, 0, false},
		{"at expiry", time.Hour, 1, false},
		{"after expiry", time.Hour + time.Nanosecond, 1, false},
		{"expired throttled", time.Hour, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lc := &contractLifecycle{}
			v := NewVerifier(lc, config.Config{OIDCJWKSURL: "http://jwks.test"})
			v.now = func() time.Time { return now }
			if v.client.Timeout != 2*time.Second {
				t.Fatalf("HTTP timeout=%v", v.client.Timeout)
			}
			tr := &contractTransport{body: string(body)}
			v.client.Transport = tr
			if err := lc.hooks[0].OnStart(context.Background()); err != nil {
				t.Fatal(err)
			}
			if tr.closed != 0 {
				t.Fatal("healthy startup closed connections")
			}
			v.fetched = now.Add(-tc.age)
			v.attempted = now.Add(-time.Second)
			if tc.blocked {
				v.attempted = now
			}
			tr.calls = 0
			got, err := v.key(context.Background(), &jwt.Token{Header: map[string]any{"kid": "known", "alg": "RS256"}})
			if tc.blocked {
				if got != nil || !errors.Is(err, ErrJWKSUnavailable) {
					t.Fatalf("stale key=%v err=%v", got, err)
				}
			} else if err != nil || !reflect.DeepEqual(got, &key.PublicKey) {
				t.Fatalf("key=%v err=%v", got, err)
			}
			if tr.calls != tc.wantCalls {
				t.Fatalf("requests=%d want=%d", tr.calls, tc.wantCalls)
			}
			if err := lc.hooks[0].OnStop(context.Background()); err != nil || tr.closed != 1 {
				t.Fatalf("cleanup=%d err=%v", tr.closed, err)
			}
		})
	}
	for _, age := range []time.Duration{time.Second - time.Nanosecond, time.Second, time.Second + time.Nanosecond} {
		lc := &contractLifecycle{}
		v := NewVerifier(lc, config.Config{OIDCJWKSURL: "http://jwks.test"})
		v.now = func() time.Time { return now }
		v.attempted = now.Add(-age)
		tr := &contractTransport{body: string(body)}
		v.client.Transport = tr
		if err := v.refresh(context.Background(), false); err != nil {
			t.Fatal(err)
		}
		want := 1
		if age < time.Second {
			want = 0
		}
		if tr.calls != want {
			t.Fatalf("age=%v requests=%d want=%d", age, tr.calls, want)
		}
	}
	lc := &contractLifecycle{}
	v := NewVerifier(lc, config.Config{OIDCJWKSURL: "http://jwks.test"})
	cause := errors.New("offline")
	tr := &contractTransport{err: cause}
	v.client.Transport = tr
	if err := lc.hooks[0].OnStart(context.Background()); !errors.Is(err, ErrJWKSUnavailable) || !errors.Is(err, cause) || tr.closed != 1 {
		t.Fatalf("failed startup cleanup=%d err=%v", tr.closed, err)
	}
}

func TestEmptyKeyIDDoesNotFetchJWKS(t *testing.T) {
	lc := &contractLifecycle{}
	v := NewVerifier(lc, config.Config{OIDCJWKSURL: "http://jwks.test"})
	tr := &contractTransport{err: errors.New("offline")}
	v.client.Transport = tr
	for _, kid := range []any{"", 42, nil} {
		got, err := v.key(context.Background(), &jwt.Token{Header: map[string]any{"kid": kid, "alg": "RS256"}})
		if got != nil || !errors.Is(err, ErrInvalidToken) || tr.calls != 0 {
			t.Fatalf("key=%v err=%v fetches=%d", got, err, tr.calls)
		}
	}
}
