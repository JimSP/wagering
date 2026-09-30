package httpapi

import (
	"net/http"

	"github.com/alexandre/wagering/internal/domain/settlement"
)

func (h handlers) createBet(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID         string `json:"id"`
		ProviderID string `json:"providerId"`
		RoundID    string `json:"roundId"`
		GameID     string `json:"gameId"`
		Currency   string `json:"currency"`
	}
	if err := decode(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	err := h.d.Submit.Settlements().Create(r.Context(), settlement.Bet{ID: in.ID, ProviderID: in.ProviderID, RoundID: in.RoundID, GameID: in.GameID, Currency: in.Currency})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"betId": in.ID})
}

func (h handlers) confirmResult(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ResultID    string `json:"resultId"`
		Allocations []struct {
			From  string   `json:"fromExternalTransactionId"`
			To    string   `json:"toExternalTransactionId"`
			Money MoneyDTO `json:"money"`
		} `json:"allocations"`
		Returns []struct {
			ExternalID string   `json:"externalTransactionId"`
			Money      MoneyDTO `json:"money"`
		} `json:"returns"`
	}
	if err := decode(w, r, &in); err != nil {
		writeError(w, err)
		return
	}
	d := settlement.Distribution{ResultID: in.ResultID}
	for _, a := range in.Allocations {
		m, e := a.Money.parse()
		if e != nil {
			writeError(w, e)
			return
		}
		d.Allocations = append(d.Allocations, settlement.Transfer{From: a.From, To: a.To, Money: m})
	}
	for _, a := range in.Returns {
		m, e := a.Money.parse()
		if e != nil {
			writeError(w, e)
			return
		}
		d.Returns = append(d.Returns, settlement.Return{ExternalID: a.ExternalID, Money: m})
	}
	out, err := h.d.Submit.Settlements().Confirm(r.Context(), r.PathValue("betId"), d)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, out)
}

func (h handlers) getSettlement(w http.ResponseWriter, r *http.Request) {
	out, err := h.d.Submit.Settlements().Get(r.Context(), r.PathValue("settlementId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h handlers) reverseSettlement(w http.ResponseWriter, r *http.Request) {
	var in struct{}
	if r.ContentLength != 0 {
		if err := decode(w, r, &in); err != nil {
			writeError(w, err)
			return
		}
	}
	out, err := h.d.Submit.Settlements().Reverse(r.Context(), r.PathValue("settlementId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
