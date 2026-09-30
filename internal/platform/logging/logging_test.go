package logging

import (
	"context"
	"log/slog"
	"testing"

	"github.com/alexandre/wagering/internal/infra/config"
)

func TestConfiguredLevelAndDefaultLogger(t *testing.T) {
	old := slog.Default()
	defer slog.SetDefault(old)
	for _, c := range []struct {
		level       string
		debug, info bool
	}{{"debug", true, true}, {"info", false, true}, {"error", false, false}} {
		log := New(config.Config{LogLevel: c.level})
		if slog.Default() != log || log.Enabled(context.Background(), slog.LevelDebug) != c.debug || log.Enabled(context.Background(), slog.LevelInfo) != c.info || !log.Enabled(context.Background(), slog.LevelError) {
			t.Fatal(c)
		}
	}
}
