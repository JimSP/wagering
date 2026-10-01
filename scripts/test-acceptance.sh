#!/usr/bin/env bash
set -uo pipefail
cd "$(dirname "$0")/.."
GO_BIN="${GO_BIN:-go}"
command -v "$GO_BIN" >/dev/null || { echo 'Go compiler not found' >&2; exit 127; }
"$GO_BIN" run ./cmd/reports acceptance --check-only || exit 1
mkdir -p docs/acceptance/evidence
status=0
"$GO_BIN" test -count=1 -json ./... > docs/acceptance/evidence/test.jsonl 2> docs/acceptance/evidence/test-stderr.txt
normal=$?
if (( normal != 0 )); then status=1; fi
"$GO_BIN" test -count=1 -race -json ./... > docs/acceptance/evidence/race.jsonl 2> docs/acceptance/evidence/race-stderr.txt
race=$?
if (( race != 0 )); then status=1; fi
"$GO_BIN" vet ./... > docs/acceptance/evidence/vet.txt 2>&1
vet=$?
if (( vet != 0 )); then status=1; fi
"$GO_BIN" run ./cmd/reports command-status "$normal" "$race" "$vet" || exit 1
"$GO_BIN" run ./cmd/reports acceptance || exit 1
printf 'normal=%s race=%s vet=%s\n' "$normal" "$race" "$vet"
exit "$status"
