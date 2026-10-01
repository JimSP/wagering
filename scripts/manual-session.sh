#!/usr/bin/env bash
# Source from Bash at the repository root. No requests run when sourced.
manual_id() { uuidgen | tr A-Z a-z; }
manual_init() {
  RUN_ID="$(manual_id)"
  RUN_DIR="$PWD/.local/manual/$RUN_ID"
  mkdir -p "$RUN_DIR"
  printf 'Evidências: %s\n' "$RUN_DIR"
  manual_login
}
manual_token() {
  curl --fail --silent --show-error http://localhost:8081/realms/wagering/protocol/openid-connect/token \
    -d grant_type=client_credentials -d "client_id=$1" -d "client_secret=$2" |
    jq -er '.access_token'
}
manual_login() {
  set -a
  source .env || return
  set +a
  INTERNAL_TOKEN="$(manual_token internal-service "${TEST_INTERNAL_CLIENT_SECRET:?Run go run ./cmd/init-env first}")" || return
  PROVIDER_TOKEN="$(manual_token provider-a "${TEST_PROVIDER_A_CLIENT_SECRET:?Run go run ./cmd/init-env first}")" || return
  OTHER_TOKEN="$(manual_token provider-b "${TEST_PROVIDER_B_CLIENT_SECRET:?Run go run ./cmd/init-env first}")" || return
}
# name identity method path [request-file] [idempotency-key]
manual_http() {
  local name="$1" identity="$2" method="$3" route="$4" body="${5:-}" key="${6:-}" token
  local args=()
  case "$identity" in
    internal) token="$INTERNAL_TOKEN";;
    a) token="$PROVIDER_TOKEN";;
    b) token="$OTHER_TOKEN";;
    none) token='';;
    *) printf 'Identidade inválida\n' >&2; return 2;;
  esac
  [[ -z "$token" ]] || args+=(-H "Authorization: Bearer $token")
  [[ -z "$body" ]] || args+=(-H 'Content-Type: application/json' --data-binary "@$body")
  [[ -z "$key" ]] || args+=(-H "Idempotency-Key: $key")
  curl --silent --show-error --connect-timeout 5 --max-time 40 -X "$method" \
    -H "X-Correlation-Id: manual-$RUN_ID-$name" "${args[@]}" \
    -D "$RUN_DIR/$name.headers" -o "$RUN_DIR/$name.response.json" \
    -w '%{http_code}\n' "http://localhost:8080$route" > "$RUN_DIR/$name.status" || return
  printf '\n%s: HTTP ' "$name"; cat "$RUN_DIR/$name.status"
  if [[ -s "$RUN_DIR/$name.response.json" ]]; then
    jq . "$RUN_DIR/$name.response.json" 2>/dev/null || cat "$RUN_DIR/$name.response.json"
  fi
}
# name initial-balance; updates PLAYER_ID and WALLET_ID
manual_wallet() {
  local name="$1" amount="$2"
  PLAYER_ID="$(manual_id)"
  jq -n --arg player "$PLAYER_ID" --arg amount "$amount" '{playerId:$player,initialBalance:{amount:$amount,currency:"BRL"}}' > "$RUN_DIR/$name.request.json"
  manual_http "$name" internal POST /wallets "$RUN_DIR/$name.request.json" || return
  WALLET_ID="$(jq -er '.id' "$RUN_DIR/$name.response.json")" || return
  printf 'PLAYER_ID=%s\nWALLET_ID=%s\n' "$PLAYER_ID" "$WALLET_ID" > "$RUN_DIR/$name.ids"
}
# name kind amount [reference-external-id] [currency]; writes request, does not send
manual_payload() {
  jq -n --arg external "$RUN_ID-$1" --arg player "$PLAYER_ID" --arg wallet "$WALLET_ID" --arg round "$RUN_ID" --arg kind "$2" --arg amount "$3" --arg reference "${4:-}" --arg currency "${5:-BRL}" '{providerId:"provider-a",externalTransactionId:$external,playerId:$player,walletId:$wallet,roundId:$round,gameId:"manual",kind:$kind,money:{amount:$amount,currency:$currency}} + (if $reference == "" then {} else {referenceExternalTransactionId:$reference} end)' > "$RUN_DIR/$1.request.json"
}
manual_send() {
  manual_http "$1" a POST /wagering/transactions "$RUN_DIR/$1.request.json" "$RUN_ID-$1"
}
manual_check() {
  manual_http "$1-wallet" internal GET "/wallets/$WALLET_ID"
  manual_http "$1-ledger" internal GET "/wallets/$WALLET_ID/ledger?limit=200"
  manual_http "$1-reconciliation" internal POST "/wallets/$WALLET_ID/reconciliation"
}
