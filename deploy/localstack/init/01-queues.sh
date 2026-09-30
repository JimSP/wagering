#!/bin/bash
set -euo pipefail
rm -f /credentials/ready
export AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_DEFAULT_REGION=us-east-1
awslocal sqs create-queue --queue-name wager-transactions-dlq.fifo --attributes FifoQueue=true,ContentBasedDeduplication=false,MessageRetentionPeriod=1209600
DLQ_ARN=arn:aws:sqs:us-east-1:000000000000:wager-transactions-dlq.fifo
awslocal sqs create-queue --queue-name wager-transactions.fifo --attributes "{\"FifoQueue\":\"true\",\"ContentBasedDeduplication\":\"false\",\"VisibilityTimeout\":\"60\",\"RedrivePolicy\":\"{\\\"deadLetterTargetArn\\\":\\\"$DLQ_ARN\\\",\\\"maxReceiveCount\\\":\\\"5\\\"}\"}"
awslocal sqs create-queue --queue-name wager-settlements-dlq.fifo --attributes FifoQueue=true,ContentBasedDeduplication=false,MessageRetentionPeriod=1209600
SETTLEMENT_DLQ_ARN=arn:aws:sqs:us-east-1:000000000000:wager-settlements-dlq.fifo
awslocal sqs create-queue --queue-name wager-settlements.fifo --attributes "{\"FifoQueue\":\"true\",\"ContentBasedDeduplication\":\"false\",\"VisibilityTimeout\":\"60\",\"RedrivePolicy\":\"{\\\"deadLetterTargetArn\\\":\\\"$SETTLEMENT_DLQ_ARN\\\",\\\"maxReceiveCount\\\":\\\"5\\\"}\"}"
awslocal sqs create-queue --queue-name wager-events.fifo --attributes FifoQueue=true,ContentBasedDeduplication=false
mkdir -p /credentials
if [ ! -s /credentials/config ] || ! awslocal iam get-user --user-name worker >/dev/null 2>&1; then
: > /credentials/config
for principal in worker ingress auditor denied; do
 awslocal iam create-user --user-name "$principal" >/dev/null 2>&1 || awslocal iam get-user --user-name "$principal" >/dev/null
 key=$(awslocal iam create-access-key --user-name "$principal" --query 'AccessKey.[AccessKeyId,SecretAccessKey]' --output text)
 read -r access_key secret_key <<KEY
$key
KEY
 [ -n "$access_key" ] && [ -n "$secret_key" ] || { echo 'Invalid access key response' >&2; exit 1; }
 printf '[%s]\naws_access_key_id = %s\naws_secret_access_key = %s\n\n' "$principal" "$access_key" "$secret_key" >> /credentials/config
done
fi
awslocal iam put-user-policy --user-name worker --policy-name worker --policy-document '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["sqs:ReceiveMessage","sqs:DeleteMessage","sqs:ChangeMessageVisibility","sqs:GetQueueAttributes"],"Resource":"arn:aws:sqs:us-east-1:000000000000:wager-transactions.fifo"},{"Effect":"Allow","Action":["sqs:SendMessage","sqs:GetQueueAttributes"],"Resource":"arn:aws:sqs:us-east-1:000000000000:wager-events.fifo"},{"Effect":"Allow","Action":"sqs:GetQueueAttributes","Resource":"arn:aws:sqs:us-east-1:000000000000:wager-transactions-dlq.fifo"}]}'
awslocal iam put-user-policy --user-name worker --policy-name settlements --policy-document '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["sqs:SendMessage","sqs:ReceiveMessage","sqs:DeleteMessage","sqs:ChangeMessageVisibility","sqs:GetQueueAttributes"],"Resource":"arn:aws:sqs:us-east-1:000000000000:wager-settlements.fifo"}]}'
awslocal iam put-user-policy --user-name ingress --policy-name ingress --policy-document '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"sqs:SendMessage","Resource":"arn:aws:sqs:us-east-1:000000000000:wager-transactions.fifo"}]}'
awslocal iam put-user-policy --user-name auditor --policy-name auditor --policy-document '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["sqs:ReceiveMessage","sqs:DeleteMessage","sqs:GetQueueAttributes"],"Resource":["arn:aws:sqs:us-east-1:000000000000:wager-events.fifo","arn:aws:sqs:us-east-1:000000000000:wager-transactions-dlq.fifo"]}]}'
# A dedicated denied identity is used by the negative authorization integration test.
awslocal iam put-user-policy --user-name denied --policy-name denied --policy-document '{"Version":"2012-10-17","Statement":[{"Effect":"Deny","Action":"sqs:*","Resource":"*"}]}'
chmod 644 /credentials/config
touch /credentials/ready
