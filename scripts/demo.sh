#!/usr/bin/env bash
# Authenticated local example: opening, BET, replay, ledger and reconciliation.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/runtime.sh"
cd "$wagering_root"
if [[ "${1:-}" == --help || "${1:-}" == -h ]]; then
  echo 'Usage: scripts/demo.sh (creates a local wallet with 100.00 BRL and a BET of 20.00 BRL)'; exit 0
fi
[[ $# == 0 ]] || { echo 'Unexpected arguments.' >&2; exit 2; }
for tool in curl jq uuidgen; do require_command "$tool"; done
[[ -f .env ]] || { echo 'Run scripts/setup.sh first.' >&2; exit 1; }
source .env
get_token() {
  curl --fail --silent --show-error --connect-timeout 5 --max-time 30 \
    http://localhost:8081/realms/wagering/protocol/openid-connect/token \
    --data-urlencode grant_type=client_credentials \
    --data-urlencode "client_id=$1" --data-urlencode "client_secret=$2" | jq -er '.access_token'
}
internal_token=$(get_token "$TEST_INTERNAL_CLIENT_ID" "$TEST_INTERNAL_CLIENT_SECRET")
provider_token=$(get_token "$TEST_PROVIDER_A_CLIENT_ID" "$TEST_PROVIDER_A_CLIENT_SECRET")
player_id=$(uuidgen | tr '[:upper:]' '[:lower:]')
external_id=$(uuidgen | tr '[:upper:]' '[:lower:]')
request() {
  local token="$1" route="$2"
  shift 2
  curl --fail --silent --show-error --connect-timeout 5 --max-time 40 \
    -H "Authorization: Bearer $token" "http://localhost:8080$route" "$@"
}
wallet=$(request "$internal_token" /wallets -H 'Content-Type: application/json' \
  --data "{\"playerId\":\"$player_id\",\"initialBalance\":{\"amount\":\"100.00\",\"currency\":\"BRL\"}}")
wallet_id=$(jq -er '.id' <<< "$wallet")
bet_body=$(jq -nc --arg provider "$TEST_PROVIDER_A_CLIENT_ID" --arg external "$external_id" \
  --arg player "$player_id" --arg wallet "$wallet_id" \
  '{providerId:$provider,externalTransactionId:$external,playerId:$player,walletId:$wallet,roundId:$external,gameId:"script-demo",kind:"BET",money:{amount:"20.00",currency:"BRL"}}')
submit() {
  request "$provider_token" /wagering/transactions -H 'Content-Type: application/json' \
    -H "Idempotency-Key: $TEST_PROVIDER_A_CLIENT_ID:$external_id" --data "$bet_body"
}
bet=$(submit)
jq -e '.status == "PROCESSED" and .balance.amount == "80.00" and .idempotentReplay == false' <<< "$bet" >/dev/null
replay=$(submit)
jq -e '.status == "PROCESSED" and .balance.amount == "80.00" and .idempotentReplay == true' <<< "$replay" >/dev/null
request "$internal_token" "/wallets/$wallet_id/ledger?limit=50" | jq .
reconciliation=$(request "$internal_token" "/wallets/$wallet_id/reconciliation" -X POST)
jq -e '.consistent == true and .difference.amount == "0.00"' <<< "$reconciliation" >/dev/null
jq . <<< "$reconciliation"
printf 'Example passed. Wallet: %s; available balance: 80.00 BRL; replay did not debit again.\n' "$wallet_id"
