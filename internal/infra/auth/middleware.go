package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type ctxKey struct{}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}

func deny(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"code":"` + code + `"}`))
}

// Authenticate requires a valid Bearer token (401 missing/invalid/expired; 503 if the IdP is unreachable).
func (v *Verifier) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || tok == "" {
			deny(w, http.StatusUnauthorized, "UNAUTHENTICATED")
			return
		}
		p, err := v.Verify(r.Context(), tok)
		if err != nil {
			if errors.Is(err, ErrJWKSUnavailable) {
				w.Header().Set("Retry-After", "2")
				deny(w, http.StatusServiceUnavailable, "IDP_UNAVAILABLE")
				return
			}
			deny(w, http.StatusUnauthorized, "UNAUTHENTICATED")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, p)))
	})
}

// RequireInternal restricts wallet operations to the internal service.
func RequireInternal(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := PrincipalFrom(r.Context()); !ok || !p.Internal {
			deny(w, http.StatusForbidden, "FORBIDDEN")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireProvider requires a provider identity (providerId claim).
func RequireProvider(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := PrincipalFrom(r.Context()); !ok || p.ProviderID == "" {
			deny(w, http.StatusForbidden, "FORBIDDEN")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireProviderOrInternal is used by read endpoints (scoping is enforced in the use case).
func RequireProviderOrInternal(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := PrincipalFrom(r.Context()); !ok || (p.ProviderID == "" && !p.Internal) {
			deny(w, http.StatusForbidden, "FORBIDDEN")
			return
		}
		next.ServeHTTP(w, r)
	})
}
