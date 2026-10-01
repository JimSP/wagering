package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/alexandre/wagering/internal/app/apperr"
	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/domain/wager"
	"github.com/alexandre/wagering/internal/infra/auth"
)

type handlers struct{ d Deps }

type corrKey struct{}

func correlationID(ctx context.Context) string { s, _ := ctx.Value(corrKey{}).(string); return s }

// withCorrelation propagates/creates X-Correlation-Id (logs, events and responses share it).
func withCorrelation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Correlation-Id")
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Correlation-Id", id)
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, corrKey{}, id)))
	})
}

func decode(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	var body json.RawMessage
	if err := dec.Decode(&body); err != nil {
		return apperr.Invalid("malformed body: %v", err)
	}
	// All request contracts require an object. encoding/json otherwise accepts
	// null into a struct and leaves its fields empty, confusing syntax with auth.
	if len(body) == 0 || body[0] != '{' {
		return apperr.Invalid("body must be a JSON object")
	}
	if dec.Decode(new(any)) != io.EOF {
		return apperr.Invalid("trailing JSON")
	}
	fields := json.NewDecoder(bytes.NewReader(body))
	fields.DisallowUnknownFields()
	fields.UseNumber()
	if err := fields.Decode(v); err != nil {
		return apperr.Invalid("malformed body: %v", err)
	}
	return nil
}

func (h handlers) openWallet(w http.ResponseWriter, r *http.Request) {
	var req OpenWalletRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, err)
		return
	}
	m, err := req.InitialBalance.parse()
	if err != nil {
		writeError(w, err)
		return
	}
	v, err := h.d.OpenWallet.Execute(r.Context(), usecase.OpenWalletInput{PlayerID: req.PlayerID, InitialBalance: m})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, WalletResponse{v.ID, v.PlayerID, toMoneyDTO(v.Balance), v.Version})
}

func (h handlers) getWallet(w http.ResponseWriter, r *http.Request) {
	v, err := h.d.GetWallet.Execute(r.Context(), r.PathValue("walletId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, WalletResponse{v.ID, v.PlayerID, toMoneyDTO(v.Balance), v.Version})
}

func (h handlers) listLedger(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 200 {
			writeError(w, apperr.Invalid("limit must be between 1 and 200"))
			return
		}
		limit = n
	}
	page, err := h.d.ListLedger.Execute(r.Context(), r.PathValue("walletId"), r.URL.Query().Get("cursor"), limit)
	if err != nil {
		writeError(w, err)
		return
	}
	out := LedgerResponse{Entries: make([]LedgerEntryDTO, 0, len(page.Entries)), NextCursor: page.NextCursor}
	for _, e := range page.Entries {
		out.Entries = append(out.Entries, LedgerEntryDTO{
			e.WalletID(), e.ID(), e.TransactionID(), string(e.Direction()),
			toMoneyDTO(e.Amount()), toMoneyDTO(e.BalanceBefore()), toMoneyDTO(e.BalanceAfter()), e.CreatedAt().UTC(),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h handlers) reconcile(w http.ResponseWriter, r *http.Request) {
	res, err := h.d.Reconcile.Execute(r.Context(), r.PathValue("walletId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ReconciliationResponse{
		res.WalletID, toMoneyDTO(res.Stored), toMoneyDTO(res.Calculated),
		toMoneyDTO(res.Difference), res.Consistent, res.CheckedEntries,
	})
}

func (h handlers) submit(w http.ResponseWriter, r *http.Request) {
	key := r.Header.Get("Idempotency-Key")
	if key == "" {
		writeError(w, apperr.Invalid("Idempotency-Key header is required"))
		return
	}
	var req SubmitTransactionRequest
	if err := decode(w, r, &req); err != nil {
		writeError(w, err)
		return
	}
	p, _ := auth.PrincipalFrom(r.Context())
	res, err := h.d.Submit.Execute(r.Context(), usecase.SubmitInput{
		Source: usecase.SourceHTTP, AuthorizedProviderID: p.ProviderID, IdempotencyKey: key,
		CorrelationID: correlationID(r.Context()),
		BetID:         req.BetID,
		ProviderID:    req.ProviderID, ExternalTransactionID: req.ExternalTransactionID, PlayerID: req.PlayerID,
		WalletID: req.WalletID, RoundID: req.RoundID, GameID: req.GameID, Kind: req.Kind,
		ReferenceExternalTransactionID: req.ReferenceExternalTransactionID,
		Amount:                         req.Money.Amount, Currency: req.Money.Currency,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	body := SubmitTransactionResponse{
		res.TransactionID, string(res.Status), toMoneyDTOPtr(res.Balance),
		string(res.FailureCode), res.IdempotentReplay,
	}
	switch res.Status {
	case wager.StatusProcessed:
		writeJSON(w, http.StatusOK, body)
	case wager.StatusRejected, wager.StatusFailed:
		writeJSON(w, http.StatusUnprocessableEntity, body) // definitive business outcome
	default: // PENDING / PENDING_REFERENCE / PENDING_ROLLBACK
		w.Header().Set("Location", "/wagering/transactions/"+res.TransactionID)
		writeJSON(w, http.StatusAccepted, body)
	}
}

func scopeOf(r *http.Request) usecase.Scope {
	p, _ := auth.PrincipalFrom(r.Context())
	return usecase.Scope{ProviderID: p.ProviderID, Internal: p.Internal}
}

func toTxResponse(v usecase.TransactionView) TransactionResponse {
	return TransactionResponse{
		v.ID, v.ProviderID, v.ExternalTransactionID, v.WalletID, string(v.Kind), string(v.Status),
		toMoneyDTO(v.Amount), toMoneyDTOPtr(v.Balance), string(v.FailureCode), v.NextAttemptAt,
	}
}

func (h handlers) getTransaction(w http.ResponseWriter, r *http.Request) {
	v, err := h.d.GetTx.ByID(r.Context(), r.PathValue("transactionId"), scopeOf(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTxResponse(v))
}

func (h handlers) getProviderTransaction(w http.ResponseWriter, r *http.Request) {
	v, err := h.d.GetTx.ByExternalID(r.Context(), r.PathValue("providerId"), r.PathValue("externalTransactionId"), scopeOf(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTxResponse(v))
}
