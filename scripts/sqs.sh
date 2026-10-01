#!/usr/bin/env bash
# Run the broker CLI inside MiniStack with a provisioned least-privilege profile.
# Credentials stay inside the volume; arguments and stdin pass through unchanged.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/runtime.sh"
cd "$wagering_root"
if [[ "${1:-}" == --help || $# == 0 ]]; then
  echo 'Usage: bash scripts/sqs.sh ingress|auditor|denied SQS_COMMAND [arguments]'
  echo 'Queue URLs inside this wrapper use http://localhost:4566/000000000000/QUEUE.'
  echo 'Use --message-body file:///dev/stdin to supply an envelope without shell quoting.'
  exit 0
fi
profile="$1"
shift
case "$profile" in ingress|auditor|denied) ;; *) echo 'Profile must be ingress, auditor or denied.' >&2; exit 2 ;; esac
[[ $# -gt 0 ]] || { echo 'Missing SQS command.' >&2; exit 2; }
exec docker compose exec -T localstack env -u AWS_ACCESS_KEY_ID -u AWS_SECRET_ACCESS_KEY -u AWS_SESSION_TOKEN \
  AWS_SHARED_CREDENTIALS_FILE=/credentials/config AWS_PROFILE="$profile" AWS_DEFAULT_REGION=us-east-1 AWS_PAGER= \
  aws --endpoint-url http://localhost:4566 --region us-east-1 --profile "$profile" sqs "$@"
