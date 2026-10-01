#!/usr/bin/env bash
# Contract/state oracles and official Gremlins campaign; Docker integration is separate.
set -euo pipefail
cd "$(dirname "$0")/.."
./scripts/test-acceptance.sh
./scripts/test-unit-coverage.sh
bash scripts/test-mutations.sh
go test ./internal/domain/money -run '^$' -fuzz '^FuzzMoneyArithmeticMatchesBigInteger$' -fuzztime=30s -parallel=2
