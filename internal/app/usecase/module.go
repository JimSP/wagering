package usecase

import (
	"go.uber.org/fx"

	"github.com/alexandre/wagering/internal/app/port"
)

var Module = fx.Module("usecase",
	fx.Provide(
		NewOpenWallet, NewGetWallet, NewListLedger, NewReconcileWallet,
		newConfiguredSubmitTransaction, NewGetTransaction, NewConsumeSettlementMessage,
		NewProcessPendingReferences, NewPublishOutbox,
		fx.Annotate(NewConsumeWagerMessage, fx.As(new(port.MessageHandler))),
	),
)
