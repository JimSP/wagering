#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/runtime.sh"
cd "$wagering_root"
test_kind="${1:-help}"
if (($#)); then shift; fi
case "$test_kind" in
  help|--help|-h)
    echo 'Usage: scripts/test.sh TYPE [arguments]'
    echo 'Types: unit race vet coverage postgres integration schema concurrency recovery mutations fuzz acceptance all'
    echo 'all: unit, race, vet, schema and integration (excludes long mutation/fuzz campaigns).'
    exit 0 ;;
  unit|race|vet|coverage|postgres|integration|schema|concurrency|recovery|mutations|fuzz|acceptance|all) ;;
  *) echo "Unknown test type: $test_kind. Use --help." >&2; exit 2 ;;
esac
case "$test_kind" in
  coverage|schema|acceptance)
    [[ $# == 0 ]] || { echo "$test_kind does not accept extra arguments." >&2; exit 2; } ;;
esac
require_command go
case "$test_kind" in
  race|coverage|postgres|integration|schema|concurrency|recovery|mutations|acceptance|all)
    [[ "$(go env CGO_ENABLED)" == 1 ]] || { echo 'Race tests require CGO_ENABLED=1 and a C compiler.' >&2; exit 1; }
    require_command "$(go env CC)" ;;
esac
case "$test_kind" in
  postgres|integration|schema|concurrency|recovery|all) require_docker; prepare_test_images ;;
esac
case "$test_kind" in
  unit) go test "$@" ./... ;;
  race) go test -race "$@" ./... ;;
  vet) go vet "$@" ./... ;;
  coverage) bash scripts/test-unit-coverage.sh "$@" ;;
  postgres) bash scripts/test-postgres-isolated.sh "$@" ;;
  integration) bash scripts/test-postgres-isolated.sh "$@"; bash scripts/test-integration.sh "$@" ;;
  schema) bash scripts/test-schema.sh "$@" ;;
  concurrency) bash scripts/test-integration.sh -run '^TestThreeProcessesConcurrentMoney$' "$@" ;;
  recovery) bash scripts/test-integration.sh -run '^(TestHTTPAndSQSReplayAndCrash|TestOutboxRecoveryTwoPublishers|TestPendingReferenceRestart|TestDependencyOutagesRecoverDurableWork|TestSIGTERMDrainsInFlightSQS)$' "$@" ;;
  mutations)
    if [[ -z "${GREMLINS_BIN:-}" ]]; then
      source scripts/quality-tools.sh
      export GREMLINS_BIN
      GREMLINS_BIN=$(quality_binary gremlins)
      [[ -x "$GREMLINS_BIN" ]] || { echo "Prepare quality tools: bash bootstrap-go-stack.sh" >&2; exit 1; }
    fi
    bash scripts/test-mutations.sh "$@" ;;
  fuzz) go test ./internal/domain/money -run '^$' -fuzz '^FuzzMoneyArithmeticMatchesBigInteger$' -fuzztime=30s -parallel=2 "$@" ;;
  acceptance) bash scripts/test-acceptance.sh "$@" ;;
  all)
    go test "$@" ./...
    go test -race "$@" ./...
    go vet ./...
    bash scripts/test-schema.sh
    bash scripts/test-postgres-isolated.sh "$@"
    bash scripts/test-integration.sh "$@" ;;
esac
