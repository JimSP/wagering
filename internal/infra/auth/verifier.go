// Package auth validates externally issued OIDC tokens with a bounded JWKS cache.
package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/fx"
)

var (
	ErrJWKSUnavailable = errors.New("auth: jwks unavailable")
	ErrInvalidToken    = errors.New("auth: invalid token")
)

type Principal struct {
	Subject, ProviderID string
	Internal            bool
}

// Refresh is request-owned: no background goroutine can outlive the application.
// Cached keys live for one hour; an unknown kid can trigger at most one fetch/second.
type Verifier struct {
	issuer, jwksURL, audience string
	client                    *http.Client
	gate                      chan struct{}
	mu                        sync.RWMutex
	kf                        keyfunc.Keyfunc
	fetched, attempted        time.Time
	lastErr                   error
	now                       func() time.Time // immutable source for cache-age decisions; deadlines use context
}

func NewVerifier(lc fx.Lifecycle, cfg config.Config) *Verifier {
	v := &Verifier{
		now: time.Now, issuer: cfg.OIDCIssuer, jwksURL: cfg.OIDCJWKSURL, audience: cfg.OIDCAudience,
		client: &http.Client{Timeout: 2 * time.Second, Transport: http.DefaultTransport.(*http.Transport).Clone()}, gate: make(chan struct{}, 1),
	}
	lc.Append(fx.Hook{OnStart: func(ctx context.Context) error {
		err := v.refresh(ctx, true)
		if err != nil {
			v.client.CloseIdleConnections()
		}
		return err
	}, OnStop: func(context.Context) error { v.client.CloseIdleConnections(); return nil }})
	return v
}

func (v *Verifier) refresh(ctx context.Context, initial bool) error {
	select {
	case v.gate <- struct{}{}:
	case <-ctx.Done():
		return fmt.Errorf("%w: %w", ErrJWKSUnavailable, ctx.Err())
	}
	defer func() { <-v.gate }()
	v.mu.RLock()
	recent := v.now().Sub(v.attempted) < time.Second
	last := v.lastErr
	v.mu.RUnlock()
	if recent && !initial {
		return last
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	var kf keyfunc.Keyfunc
	if err == nil {
		var res *http.Response
		res, err = v.client.Do(req)
		if err == nil {
			defer func() { _ = res.Body.Close() }()
			if res.StatusCode != http.StatusOK {
				err = fmt.Errorf("JWKS HTTP status %d", res.StatusCode)
			} else {
				var body []byte
				body, err = io.ReadAll(io.LimitReader(res.Body, 1<<20))
				if err == nil {
					parsed, parseErr := keyfunc.NewJWKSetJSON(body)
					err = parseErr
					if parseErr == nil {
						kf = parsed
						keys, readErr := parsed.Storage().KeyReadAll(ctx)
						err = readErr
						if readErr == nil && len(keys) == 0 {
							err = errors.New("empty JWKS")
						}
					}
				}
			}
		}
	}
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrJWKSUnavailable, err)
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.attempted = v.now()
	v.lastErr = err
	if err == nil {
		v.kf = kf
		v.fetched = v.attempted
	}
	return err
}

func (v *Verifier) key(ctx context.Context, t *jwt.Token) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrJWKSUnavailable, err)
	}
	kid, ok := t.Header["kid"].(string)
	if !ok || kid == "" {
		return nil, ErrInvalidToken
	}
	v.mu.RLock()
	kf, fetched := v.kf, v.fetched
	v.mu.RUnlock()
	if kf != nil && v.now().Sub(fetched) < time.Hour {
		key, err := kf.KeyfuncCtx(ctx)(t)
		if !errors.Is(err, jwkset.ErrKeyNotFound) {
			return key, err
		}
	}
	if err := v.refresh(ctx, false); err != nil {
		return nil, err
	}
	v.mu.RLock()
	kf, fetched = v.kf, v.fetched
	v.mu.RUnlock()
	if kf == nil || v.now().Sub(fetched) >= time.Hour {
		return nil, ErrJWKSUnavailable
	}
	return kf.KeyfuncCtx(ctx)(t)
}

func (v *Verifier) Verify(ctx context.Context, raw string) (Principal, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) { return v.key(ctx, t) },
		jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer(v.issuer), jwt.WithAudience(v.audience), jwt.WithExpirationRequired())
	if err != nil {
		if errors.Is(err, ErrJWKSUnavailable) {
			return Principal{}, err
		}
		return Principal{}, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	sub, _ := claims.GetSubject()
	if sub == "" {
		return Principal{}, ErrInvalidToken
	}
	p := Principal{Subject: sub}
	p.ProviderID, _ = claims["providerId"].(string)
	role, _ := claims["role"].(string)
	p.Internal = role == "internal"
	return p, nil
}

var Module = fx.Module("auth", fx.Provide(NewVerifier))
