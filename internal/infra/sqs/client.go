package sqs

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/alexandre/wagering/internal/infra/config"
)

func NewClient(cfg config.Config) (*awssqs.Client, error) {
	ac, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		return nil, err
	}
	return awssqs.NewFromConfig(ac, func(o *awssqs.Options) {
		if cfg.AWSEndpointURL != "" { // LocalStack
			o.BaseEndpoint = aws.String(cfg.AWSEndpointURL)
		}
	}), nil
}

type Readiness struct {
	client   *awssqs.Client
	queueURL string
}

func NewReadiness(c *awssqs.Client, cfg config.Config) *Readiness {
	return &Readiness{c, cfg.WagerQueueURL}
}
func (*Readiness) Name() string { return "sqs" }
func (r *Readiness) Check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err := r.client.GetQueueAttributes(ctx, &awssqs.GetQueueAttributesInput{QueueUrl: &r.queueURL})
	return err
}
