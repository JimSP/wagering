#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
coverage_dir="${COVERAGE_DIR:-.local/coverage}"
mkdir -p "$coverage_dir"
packages='./internal/domain/...,./internal/app/usecase,./internal/infra/auth'
go test -count=1 -race -tags faults -covermode=atomic -coverpkg="$packages" \
  -coverprofile="$coverage_dir/unit.out" ./... 2>&1 | tee "$coverage_dir/test.log"
coverage_status=0
go run ./cmd/reports coverage "$coverage_dir/unit.out" || coverage_status=$?
go tool cover -func="$coverage_dir/unit.out" > "$coverage_dir/functions.txt"
go tool cover -html="$coverage_dir/unit.out" -o "$coverage_dir/unit.html"
exit "$coverage_status"
