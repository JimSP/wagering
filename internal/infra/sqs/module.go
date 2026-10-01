package sqs

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/alexandre/wagering/internal/platform/metrics"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"go.uber.org/fx"

	"github.com/alexandre/wagering/internal/app/port"
	"github.com/alexandre/wagering/internal/app/usecase"
	"github.com/alexandre/wagering/internal/infra/config"
	"github.com/alexandre/wagering/internal/platform/worker"
)

var Module = fx.Module("sqs",
	fx.Provide(
		NewClient,
		fx.Annotate(NewPublisher, fx.As(new(port.EventPublisher))),
		fx.Annotate(NewReadiness, fx.As(new(port.ReadinessChecker)), fx.ResultTags(`group:"readiness"`)),
	),
	fx.Invoke(register),
)

func register(lc fx.Lifecycle, group *worker.Group, cfg config.Config, cl *awssqs.Client, h port.MessageHandler, log *slog.Logger, m port.Metrics, recorder *metrics.Recorder, settlements *usecase.ConsumeSettlementMessage) {
	lc.Append(fx.Hook{OnStart: func(ctx context.Context) error {
		for _, url := range []string{cfg.WagerQueueURL, cfg.EventsQueueURL, cfg.SettlementQueueURL} {
			if url == "" {
				continue
			}
			if _, err := cl.GetQueueAttributes(ctx, &awssqs.GetQueueAttributesInput{QueueUrl: &url}); err != nil {
				return err
			}
		}
		return nil
	}})
	if !cfg.HasRole("sqs-consumer") {
		return
	}
	if cfg.WagerDLQURL != "" {
		worker.Register(group, "dlq-monitor", log, worker.Every(5*time.Second, func(ctx context.Context) error {
			return updateDLQDepth(ctx, cl, cfg.WagerDLQURL, recorder)
		}, func(err error) { log.Warn("DLQ metrics unavailable", "err", err) }))
	}
	consumer := configuredConsumer(cl, cfg, h, log, m)
	worker.Register(group, "sqs-consumer", log, consumer.Run)
	if cfg.SettlementQueueURL != "" {
		privateCfg := cfg
		privateCfg.WagerQueueURL = cfg.SettlementQueueURL
		privateConsumer := configuredConsumer(cl, privateCfg, settlements, log, m)
		worker.Register(group, "settlement-consumer", log, privateConsumer.Run)
	}
}

func updateDLQDepth(ctx context.Context, cl *awssqs.Client, queueURL string, recorder *metrics.Recorder) error {
	resp, err := cl.GetQueueAttributes(ctx, &awssqs.GetQueueAttributesInput{QueueUrl: &queueURL, AttributeNames: []types.QueueAttributeName{types.QueueAttributeNameApproximateNumberOfMessages, types.QueueAttributeNameApproximateNumberOfMessagesNotVisible}})
	if err != nil {
		return err
	}
	visible, err := strconv.ParseInt(resp.Attributes["ApproximateNumberOfMessages"], 10, 64)
	if err != nil {
		return err
	}
	inflight, err := strconv.ParseInt(resp.Attributes["ApproximateNumberOfMessagesNotVisible"], 10, 64)
	if err != nil {
		return err
	}
	recorder.DLQDepth(float64(visible), float64(inflight))
	return nil
}

func configuredConsumer(cl *awssqs.Client, cfg config.Config, h port.MessageHandler, log *slog.Logger, m port.Metrics) *Consumer {
	consumer := NewConsumer(cl, cfg.WagerQueueURL, h, log, m)
	consumer.waitSeconds = int32(min(20, max(0, int(cfg.ShutdownTimeout/time.Second)-3)))
	consumer.workTimeout = min(10*time.Second, cfg.ShutdownTimeout-5*time.Second)
	return consumer
}
