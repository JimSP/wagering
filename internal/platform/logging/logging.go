package logging

import (
	"log/slog"
	"os"

	"github.com/alexandre/wagering/internal/infra/config"
)

func New(cfg config.Config) *slog.Logger {
	var lvl slog.Level
	_ = lvl.UnmarshalText([]byte(cfg.LogLevel))
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
	slog.SetDefault(logger)
	return logger
}
