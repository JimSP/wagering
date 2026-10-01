package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/platform/sys"
)

type settlementHTTPUOW struct {
	port.UnitOfWork
	calls int
}

func (u *settlementHTTPUOW) Do(context.Context, func(context.Context, port.Tx) error) error {
	u.calls++
	return nil
}

func TestSettlementHTTPRejectsMalformedMoneyBeforePersistence(t *testing.T) {
	for _, body := range []string{
		`{"resultId":"r","allocations":[{"fromExternalTransactionId":"a","toExternalTransactionId":"b","money":{"amount":"bad","currency":"BRL"}}]}`,
		`{"resultId":"r","returns":[{"externalTransactionId":"a","money":{"amount":"1.00","currency":"BAD"}}]}`,
	} {
		t.Run(body, func(t *testing.T) {
			u := &settlementHTTPUOW{}
			h := handlers{Deps{Submit: usecase.NewSubmitTransaction(u, sys.Clock{}, sys.UUIDv7{}, nil)}}
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			r.SetPathValue("betId", "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a7")
			w := httptest.NewRecorder()
			h.confirmResult(w, r)
			if w.Code != 400 || u.calls != 0 {
				t.Fatalf("status=%d database calls=%d", w.Code, u.calls)
			}
		})
	}
}

func TestSettlementHTTPDecodingAndReverseBody(t *testing.T) {
	for _, name := range []string{"create", "confirm", "reverse"} {
		t.Run(name, func(t *testing.T) {
			h := handlers{}
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"unknown":1}`))
			w := httptest.NewRecorder()
			switch name {
			case "create":
				h.createBet(w, r)
			case "confirm":
				h.confirmResult(w, r)
			case "reverse":
				h.reverseSettlement(w, r)
			}
			if w.Code != 400 {
				t.Fatal(w.Code)
			}
		})
	}
	for _, body := range []string{"", "{}"} {
		t.Run("reverse-valid-"+body, func(t *testing.T) {
			u := &settlementHTTPUOW{}
			h := handlers{Deps{Submit: usecase.NewSubmitTransaction(u, sys.Clock{}, sys.UUIDv7{}, nil)}}
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			r.SetPathValue("settlementId", "0192f28f-5dc0-7d58-bdb2-814ad6a0f4a7")
			w := httptest.NewRecorder()
			h.reverseSettlement(w, r)
			if w.Code != 200 || u.calls != 1 {
				t.Fatalf("status=%d calls=%d", w.Code, u.calls)
			}
		})
	}
}

func TestSettlementHTTPGetRejectsInvalidID(t *testing.T) {
	h := handlers{Deps{Submit: usecase.NewSubmitTransaction(nil, sys.Clock{}, sys.UUIDv7{}, nil)}}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.SetPathValue("settlementId", "invalid")
	w := httptest.NewRecorder()
	h.getSettlement(w, r)
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}
