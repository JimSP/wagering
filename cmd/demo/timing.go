package main

import (
	"fmt"
	"os"
	"time"
)

// DESAFIO.md does not prescribe a refund deadline or a stake-sized WIN cap.
// Wait for the configured local window to isolate those restrictions from the
// separate early-WIN check. Never edit database timestamps to make a case pass.
func (e *evaluator) timedContracts() {
	var refundWallet, winWallet wallet
	var refundBet, winBet object
	var deadline time.Time
	e.run("timed-contracts-setup", func() {
		window, err := time.ParseDuration(env("BET_WINDOW", "5m"))
		must(err == nil && window > 0, "invalid BET_WINDOW")
		budget, err := time.ParseDuration(env("DEMO_MAX_WINDOW_WAIT", "6m"))
		must(err == nil && budget > 0 && window+time.Second <= budget, "BET_WINDOW exceeds DEMO_MAX_WINDOW_WAIT; timed contracts not verified")
		refundWallet = e.open("100.00")
		winWallet = e.open("100.00")
		refundBet = e.operation(refundWallet, "BET", "25.00", newID(), "")
		winBet = e.operation(winWallet, "BET", "25.00", newID(), "")
		expect(e.submit(refundBet), 200, "PROCESSED", "75.00")
		expect(e.submit(winBet), 200, "PROCESSED", "75.00")
		deadline = time.Now().Add(window + time.Second)
	})
	if deadline.IsZero() {
		e.results = append(e.results, result{Name: "REFUND-after-window-and-WIN-over-stake", Status: "BLOCKED", Detail: "timed setup failed"})
		return
	}
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		fmt.Printf("Waiting for persisted betting window: %s remaining\n", remaining.Round(time.Second))
		time.Sleep(min(remaining, 30*time.Second))
	}
	// Tokens may expire during the wait. Fetch new ones without printing them.
	e.run("timed-contracts-reauthenticate", func() {
		e.internal = e.token(env("TEST_INTERNAL_CLIENT_ID", "internal-service"), os.Getenv("TEST_INTERNAL_CLIENT_SECRET"))
		e.provider = e.token(e.providerID, os.Getenv("TEST_PROVIDER_A_CLIENT_SECRET"))
	})
	e.run("REFUND-after-window", func() {
		r := e.submit(e.operation(refundWallet, "REFUND", "25.00", str(refundBet, "roundId"), str(refundBet, "externalTransactionId")))
		expect(r, 422, "REJECTED", "")
		must(str(r.Body, "failureCode") == "BET_CLOSED", "expected documented refund deadline")
		e.reconcile(refundWallet, "75.00", 2)
	})
	e.run("WIN-over-stake-after-window", func() {
		r := e.submit(e.operation(winWallet, "WIN", "50.00", str(winBet, "roundId"), str(winBet, "externalTransactionId")))
		expect(r, 422, "REJECTED", "")
		must(str(r.Body, "failureCode") == "INSUFFICIENT_FUNDS", "expected commitment funding limit")
		e.reconcile(winWallet, "75.00", 2)
	})
	e.run("WIN-and-ROLLBACK-after-window", func() {
		win := e.operation(winWallet, "WIN", "10.00", str(winBet, "roundId"), str(winBet, "externalTransactionId"))
		expect(e.submit(win), 200, "PROCESSED", "85.00")
		expect(e.submit(e.operation(winWallet, "ROLLBACK", "10.00", str(winBet, "roundId"), str(win, "externalTransactionId"))), 200, "PROCESSED", "75.00")
		e.reconcile(winWallet, "75.00", 4)
	})
	e.run("WIN-implicit-reference-after-window", func() {
		expect(e.submit(e.operation(refundWallet, "WIN", "10.00", str(refundBet, "roundId"), "")), 200, "PROCESSED", "85.00")
		e.reconcile(refundWallet, "85.00", 3)
	})
}
