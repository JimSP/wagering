package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func qualityFixture(t *testing.T) (scriptFixture, string) {
	t.Helper()
	f := newScriptFixture(t)
	data, err := os.ReadFile("quality-tools.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(f.root, "scripts/quality-tools.sh"), data, 0o700); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(f.root, "tools/quality")
	if err = os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	lock := "tool v1.2.3 example.test/tool example.test/tool go\n"
	if err = os.WriteFile(filepath.Join(dir, "tools.lock"), []byte(lock), 0o600); err != nil {
		t.Fatal(err)
	}
	// No terminal or host package changes in these tests.
	runtime := filepath.Join(f.root, "scripts/runtime.sh")
	b, err := os.ReadFile(runtime)
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, []byte("\ndependency_confirm() { echo refused >&2; return 1; }\n")...)
	if err = os.WriteFile(runtime, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return f, fmt.Sprintf("%x", sha256.Sum256([]byte(lock)))
}

func TestQualityGateScansPublishableFilesAndPropagatesFindings(t *testing.T) {
	for _, scannerExit := range []int{0, 23} {
		t.Run(fmt.Sprint(scannerExit), func(t *testing.T) {
			f := newScriptFixture(t)
			data, err := os.ReadFile("check.sh")
			if err != nil {
				t.Fatal(err)
			}
			files := map[string][]byte{
				"scripts/check.sh":         data,
				"scripts/quality-tools.sh": []byte("quality_binary() { command -v \"$1\"; }\n"),
				"publishable file.txt":     []byte("scan-this-file"),
				".env":                     []byte("local-only"),
			}
			for path, content := range files {
				if err := os.WriteFile(filepath.Join(f.root, path), content, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			f.writeTool(t, "git", `if [[ "$1" == --version ]]; then echo 'git version 2.49.0'; exit; fi
[[ "$1" == ls-files ]] || exit 98
printf 'publishable file.txt\0deleted.txt\0'`)
			f.writeTool(t, "golangci-lint", "exit 0")
			f.writeTool(t, "go-arch-lint", "exit 0")
			f.writeTool(t, "gitleaks", fmt.Sprintf(`[[ "$1" == dir ]] || exit 97
[[ "$(cat "$2/publishable file.txt")" == scan-this-file ]] || exit 96
[[ ! -e "$2/.env" && ! -e "$2/deleted.txt" ]] || exit 95
echo scanned > "$SCRIPT_TRACE/scanner"
exit %d`, scannerExit))
			out, code := f.run(t, "check.sh", "fast")
			if f.log(t, "scanner") != "scanned\n" {
				logs, _ := filepath.Glob(filepath.Join(f.root, ".local/quality-results/run.*/secrets.log"))
				for _, path := range logs {
					content, _ := os.ReadFile(path)
					t.Log(string(content))
				}
				t.Fatalf("scanner did not receive publication snapshot: %s", out)
			}
			if (code == 0) != (scannerExit == 0) {
				t.Fatalf("scanner exit %d, gate exit %d: %s", scannerExit, code, out)
			}
		})
	}
}

func TestQualityInstallRequiresConsentAndMatchingLock(t *testing.T) {
	for _, args := range [][]string{{"--install"}, {"--install", "--approve-lock", "wrong"}, {"--check"}} {
		f, _ := qualityFixture(t)
		out, code := f.run(t, "quality-tools.sh", args...)
		if code == 0 {
			t.Fatal(out)
		}
		if strings.Contains(f.log(t, "go"), "install") {
			t.Fatal("installed without authorization")
		}
	}
}

func TestQualityInstallIsPrivateVerifiedAndReusable(t *testing.T) {
	f, hash := qualityFixture(t)
	f.writeTool(t, "tool", "echo global-tool-must-not-run; exit 99")
	f.writeTool(t, "go", `if [[ "$1" == version && "${2:-}" == -m ]]; then printf '\tmod\texample.test/tool\tv1.2.3\n'; exit; fi
if [[ "$1" == version ]]; then echo 'go version go1.27.1 darwin/arm64'; exit; fi
printf '%s\n' "$*" >> "$SCRIPT_TRACE/go"
[[ "$1" == install && "$2" == example.test/tool@v1.2.3 ]] || exit 97
printf '#!/bin/sh\nexit 0\n' > "$GOBIN/tool"
chmod 700 "$GOBIN/tool"`)
	out, code := f.run(t, "quality-tools.sh", "--install", "--approve-lock", hash)
	if code != 0 {
		t.Fatal(out)
	}
	if !strings.Contains(out, filepath.Join(f.root, ".local/quality/tool/v1.2.3")) {
		t.Fatal(out)
	}
	out, code = f.run(t, "quality-tools.sh", "--install")
	if code != 0 {
		t.Fatal(out)
	}
	if strings.Count(f.log(t, "go"), "install") != 1 {
		t.Fatal("reinstalled valid tool")
	}
	files, err := filepath.Glob(filepath.Join(f.root, ".local/quality/tool/v1.2.3/*/tool"))
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	if err = os.WriteFile(files[0], []byte("corrupted"), 0o700); err != nil {
		t.Fatal(err)
	}
	out, code = f.run(t, "quality-tools.sh", "--check")
	if code == 0 {
		t.Fatal("accepted changed binary", out)
	}
}

func TestQualityInstallFailureDoesNotReportSuccess(t *testing.T) {
	f, hash := qualityFixture(t)
	f.writeTool(t, "go", `if [[ "$1" == version ]]; then echo 'go version go1.27.1 darwin/arm64'; exit; fi
exit 17`)
	out, code := f.run(t, "quality-tools.sh", "--install", "--approve-lock", hash)
	if code == 0 {
		t.Fatal(out)
	}
	files, err := filepath.Glob(filepath.Join(f.root, ".local/quality/tool/*/*/tool"))
	if err != nil || len(files) != 0 {
		t.Fatal("partial installation", files, err)
	}
}
