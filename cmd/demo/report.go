package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func dataOrNull(b []byte) []byte {
	if len(b) == 0 {
		return []byte("null")
	}
	return b
}
func (e *evaluator) record(v object) {
	e.mu.Lock()
	defer e.mu.Unlock()
	v["scenario"] = e.current
	v["at"] = time.Now().UTC()
	v["sequence"] = len(e.exchanges) + 1
	e.exchanges = append(e.exchanges, v)
}

// Comparison notes describe the difference openly; PASS refers only to the
// stated project expectation, never to acceptance by the evaluator.
func scenarioNotes(name string) [2]string {
	switch name {
	case "WIN-after-BET-10.00", "WIN-after-BET-50.00":
		return [2]string{"WIN antes do fechamento: 422 REJECTED/BET_NOT_CLOSED; saldo 75.00.", "§7 descreve crédito positivo; não estabelece espera pela janela. Regra adicional do projeto."}
	case "WIN-optional-reference":
		return [2]string{"Sem BET candidata: 422 REJECTED/REFERENCE_NOT_FOUND; saldo 100.00.", "§7 torna a referência opcional. O projeto exige vínculo interno mesmo sem o campo externo."}
	case "REFUND-after-window":
		return [2]string{"Após BET_WINDOW: 422 REJECTED/BET_CLOSED; saldo 75.00.", "§7 não estabelece prazo para REFUND. O projeto limita a devolução à janela aberta."}
	case "WIN-over-stake-after-window":
		return [2]string{"WIN 50.00 com compromisso de 25.00: 422 REJECTED/INSUFFICIENT_FUNDS; saldo 75.00.", "§7 não limita WIN ao aporte. O projeto exige financiamento pelo compromisso."}
	case "ROLLBACK-recovers-open-BET":
		return [2]string{"Compensa outra BET aberta e reverte REFUND: 200 PROCESSED; saldo 0.00 e seis lançamentos.", "§7 exige rejeição da reversão sem saldo disponível. O projeto tenta recuperar recursos de outras BETs."}
	case "WIN-and-ROLLBACK-after-window":
		return [2]string{"Após a janela, WIN 10.00 leva saldo a 85.00; ROLLBACK leva a 75.00; quatro lançamentos.", "Demonstra crédito e inversão integral, incluindo a pré-condição de prazo adicional do projeto."}
	case "WIN-implicit-reference-after-window":
		return [2]string{"WIN sem referência explícita resolve BET elegível; saldo 85.00.", "Demonstra o campo opcional do §7 e a seleção interna de referência do projeto."}
	default:
		return [2]string{"Resultados definidos nas asserções deste cenário; entradas e respostas registradas em exchanges.", "Comparar o cenário e suas observações com o script independente do avaliador; não é certificação integral."}
	}
}

func (e *evaluator) writeComparison() error {
	var out strings.Builder
	out.WriteString("# Demonstração da implementação\n\nRoteiro do projeto para comparação com os scripts independentes do avaliador. PASS significa que o observado corresponde à expectativa documentada do projeto; não significa conformidade integral com DESAFIO.md. FAIL/BLOCKED indica demonstração incompleta ou comportamento inesperado.\n\nEntradas HTTP/SQS, respostas HTTP e eventos desta execução estão em `demonstration.json`, agrupados por cenário, sem tokens ou secrets.\n\n| Cenário | Execução | Esperado pelo projeto | Comparação com o desafio | Observação |\n| --- | --- | --- | --- | --- |\n")
	clean := func(s string) string { return strings.NewReplacer("|", "\\|", "\n", " ", "\r", " ").Replace(s) }
	for _, r := range e.results {
		fmt.Fprintf(&out, "| %s | %s | %s | %s | %s |\n", clean(r.Name), r.Status, clean(r.ProjectExpectation), clean(r.ChallengeComparison), clean(r.Detail))
	}
	path := env("DEMO_COMPARISON", filepath.Join(filepath.Dir(env("DEMO_REPORT", ".local/demonstration.json")), "comparison.md"))
	return os.WriteFile(path, []byte(out.String()), 0600)
}
