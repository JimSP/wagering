package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type mutation struct {
	Name    string `json:"name"`
	File    string `json:"file"`
	Old     string `json:"old"`
	New     string `json:"new"`
	Meaning string `json:"meaning"`
}

var mutations = []mutation{
	{"orphan_ledger_reference", "internal/app/usecase/process.go", `wager.NewLedgerEntry(u.ids.NewID(), t.WalletID(), t.ID(), decision.GuaranteeDirection`, `wager.NewLedgerEntry(u.ids.NewID(), t.WalletID(), "10000000"+t.ID()[8:], decision.GuaranteeDirection`, "Link ledger to a nonexistent transaction"},
	{"opening_event_wrong_version", "internal/app/usecase/wallet.go", `helper.balanceEvent(ctx, tx, t, entry, 1)`, `helper.balanceEvent(ctx, tx, t, entry, 2)`, "Publish walletVersion 2 for opening whose wallet version is 1"},
	{"replace_received_sqs_key", "internal/app/usecase/transaction.go", `IdempotencyKey: m.Data.IdempotencyKey,`, `IdempotencyKey: "key:" + m.Data.ExternalTransactionID,`, "Replace received SQS key"},
	{"omit_reference_expired_event", "internal/app/usecase/process.go", "s := t.Snapshot()\n\te, err := event.NewRejected", "if code == wager.FailReferenceNotFound { return nil }\n\ts := t.Snapshot()\n\te, err := event.NewRejected", "Omit rejection event"},
	{"http_null_accepted", "internal/transport/httpapi/handlers.go", `if len(body) == 0 || body[0] != '{' {`, `if false {`, "Accept non-object body"},
	{"http_extra_fields_accepted", "internal/transport/httpapi/handlers.go", `fields.DisallowUnknownFields()`, `// unknown fields accepted`, "Accept extra fields"},
	{"http_processed_wrong_status", "internal/transport/httpapi/handlers.go", `writeJSON(w, http.StatusOK, body)`, `writeJSON(w, http.StatusAccepted, body)`, "Return 202 for completed operation"},
	{"http_missing_balance_omission", "internal/transport/httpapi/dto.go", "`json:\"balance,omitempty\"`", "`json:\"balance\"`", "Emit null balance"},
	{"http_transient_secret_leak", "internal/transport/httpapi/errors.go", `"temporary failure, retry with the same Idempotency-Key"`, `err.Error()`, "Expose internal errors"},
	{"oidc_audience_not_checked", "internal/infra/auth/verifier.go", `jwt.WithAudience(v.audience), `, "", "Accept wrong audience"},
	{"control_debit_becomes_credit", "internal/domain/wager/accounting.go", `if _, _, err = debit.Debit(s.Amount, now);`, `if _, _, err = debit.Credit(s.Amount, now);`, "Debit becomes credit"},
}

type experiment struct {
	Exit           int      `json:"exit_code"`
	Failed         []string `json:"failed_tests"`
	Passed         []string `json:"passed_tests"`
	Classification string   `json:"classification"`
}

func copySources(source, target string) error {
	return filepath.WalkDir(source, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(source, path)
		if e != nil {
			return e
		}
		if d.IsDir() {
			if rel != "." && (strings.HasPrefix(d.Name(), ".") || d.Name() == "dist" || d.Name() == "graphify-out" || d.Name() == "docs") {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(target, rel), 0o755)
		}
		if strings.HasPrefix(d.Name(), ".") || d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		return os.WriteFile(filepath.Join(target, rel), b, info.Mode().Perm())
	})
}

func targetedMutations(source, goBin, out string) error {
	source, e := filepath.Abs(source)
	if e != nil {
		return e
	}
	out, e = filepath.Abs(out)
	if e != nil {
		return e
	}
	goBin, e = exec.LookPath(goBin)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(out, 0o755); e != nil {
		return e
	}
	args := []string{"test", "-count=1", "-json", "./internal/app/usecase", "./internal/transport/httpapi", "./internal/infra/auth", "./internal/domain/..."}
	run := func(dir, name string) (experiment, error) {
		r := experiment{Failed: []string{}, Passed: []string{}}
		stdout, e := os.Create(filepath.Join(out, name+".jsonl"))
		if e != nil {
			return r, e
		}
		defer func() { _ = stdout.Close() }()
		stderr, e := os.Create(filepath.Join(out, name+".stderr"))
		if e != nil {
			return r, e
		}
		defer func() { _ = stderr.Close() }()
		ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, goBin, args...)
		cmd.Dir = dir
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		err := cmd.Run()
		if cmd.ProcessState != nil {
			r.Exit = cmd.ProcessState.ExitCode()
		} else {
			r.Exit = -1
		}
		if ctx.Err() != nil {
			return r, ctx.Err()
		}
		if err != nil && cmd.ProcessState == nil {
			return r, err
		}
		if e = stdout.Close(); e != nil {
			return r, e
		}
		if e = stderr.Close(); e != nil {
			return r, e
		}
		observed, e := events(filepath.Join(out, name+".jsonl"))
		if e != nil {
			return r, e
		}
		for k, v := range observed.Tests {
			if strings.Contains(k.Test, "/") {
				continue
			}
			if v == "fail" {
				r.Failed = append(r.Failed, k.Test)
			}
			if v == "pass" {
				r.Passed = append(r.Passed, k.Test)
			}
		}
		r.Classification = "ERROR"
		if r.Exit == 0 {
			r.Classification = "SURVIVED"
		} else if len(r.Failed) > 0 {
			r.Classification = "DETECTED"
		}
		return r, nil
	}
	baseline, e := run(source, "baseline")
	if e != nil {
		return e
	}
	results := map[string]any{"command": append([]string{goBin}, args...), "source": source, "baseline": baseline, "mutations": []any{}}
	save := func() error { return writeJSON(filepath.Join(out, "results.json"), results) }
	if e = save(); e != nil {
		return e
	}
	if baseline.Exit != 0 {
		return fmt.Errorf("baseline not green; experiment stopped")
	}
	passed := true
	for _, m := range mutations {
		e = func() error {
			temp, e := os.MkdirTemp("", "wager-test-audit-")
			if e != nil {
				return e
			}
			defer func() { _ = os.RemoveAll(temp) }()
			target := filepath.Join(temp, "source")
			if e = copySources(source, target); e != nil {
				return e
			}
			path := filepath.Join(target, m.File)
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			count := strings.Count(string(b), m.Old)
			if count != 1 && (m.Name != "http_missing_balance_omission" || count != 2) {
				return fmt.Errorf("mutation target not unique: %s (%d matches)", m.Name, count)
			}
			if e = os.WriteFile(path, []byte(strings.Replace(string(b), m.Old, m.New, 1)), 0o644); e != nil {
				return e
			}
			r, e := run(target, m.Name)
			if e != nil {
				return e
			}
			results["mutations"] = append(results["mutations"].([]any), struct {
				mutation
				experiment
			}{m, r})
			if r.Classification != "DETECTED" {
				passed = false
			}
			fmt.Println(m.Name, r.Classification)
			return save()
		}()
		if e != nil {
			return e
		}
	}
	if !passed {
		return fmt.Errorf("mutation gate failed: surviving mutant or invalid experiment")
	}
	return nil
}
