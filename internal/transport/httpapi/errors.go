package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/domain/settlement"
	"github.com/alexandre/wagering/internal/domain/wager"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError maps application errors to distinguishable HTTP contracts:
//
//	400 INVALID_INPUT · 403 FORBIDDEN · 404 NOT_FOUND · 409 IDEMPOTENCY_CONFLICT/WALLET_EXISTS
//	503 UNAVAILABLE (+Retry-After) · 500 INTERNAL
//
// (422 REJECTED and 202 PENDING are produced by the submit handler from the persisted result.)
func writeError(w http.ResponseWriter, err error) {
	code, domainFailure := wager.CodeOf(err)
	switch {
	case errors.Is(err, settlement.ErrReversalState):
		writeJSON(w, http.StatusConflict, ErrorResponse{"SETTLEMENT_NOT_PROCESSED", err.Error()})
	case domainFailure:
		writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{string(code), err.Error()})
	case errors.Is(err, settlement.ErrDistribution):
		writeJSON(w, http.StatusUnprocessableEntity, ErrorResponse{"INVALID_DISTRIBUTION", err.Error()})
	case errors.Is(err, apperr.ErrInvalidInput):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{"INVALID_INPUT", err.Error()})
	case errors.Is(err, apperr.ErrForbidden):
		writeJSON(w, http.StatusForbidden, ErrorResponse{"FORBIDDEN", ""})
	case errors.Is(err, apperr.ErrNotFound), errors.Is(err, apperr.ErrWalletNotFound):
		writeJSON(w, http.StatusNotFound, ErrorResponse{"NOT_FOUND", ""})
	case errors.Is(err, apperr.ErrIdempotencyConflict):
		writeJSON(w, http.StatusConflict, ErrorResponse{"IDEMPOTENCY_CONFLICT", err.Error()})
	case errors.Is(err, apperr.ErrWalletExists):
		writeJSON(w, http.StatusConflict, ErrorResponse{"WALLET_EXISTS", err.Error()})
	case apperr.IsTransient(err):
		w.Header().Set("Retry-After", "1")
		writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{"UNAVAILABLE", "temporary failure, retry with the same Idempotency-Key"})
	default:
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{"INTERNAL", ""})
	}
}
