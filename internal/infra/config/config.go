package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	Roles              []string
	HTTPAddr           string
	ShutdownTimeout    time.Duration
	BetWindow          time.Duration
	LogLevel           string
	DatabaseURL        string
	AWSRegion          string
	AWSEndpointURL     string
	WagerQueueURL      string
	WagerDLQURL        string
	EventsQueueURL     string
	SettlementQueueURL string
	OIDCIssuer         string
	OIDCJWKSURL        string
	OIDCAudience       string
}

func (c Config) HasRole(r string) bool {
	for _, x := range c.Roles {
		if x == r {
			return true
		}
	}
	return false
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Load reads and validates configuration from the environment.
func Load() (Config, error) {
	d, err := time.ParseDuration(env("SHUTDOWN_TIMEOUT", "25s"))
	if err != nil {
		return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT: %w", err)
	}
	window, err := time.ParseDuration(env("BET_WINDOW", "5m"))
	if err != nil || window < time.Second || window%time.Second != 0 {
		return Config{}, errors.New("BET_WINDOW must be a positive whole-second duration (for example 30s or 5m)")
	}
	c := Config{
		BetWindow:          window,
		Roles:              strings.Split(env("ROLES", "api"), ","),
		HTTPAddr:           env("HTTP_ADDR", ":8080"),
		ShutdownTimeout:    d,
		LogLevel:           env("LOG_LEVEL", "info"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		AWSRegion:          env("AWS_REGION", "us-east-1"),
		AWSEndpointURL:     os.Getenv("AWS_ENDPOINT_URL"),
		WagerQueueURL:      os.Getenv("WAGER_QUEUE_URL"),
		WagerDLQURL:        os.Getenv("WAGER_DLQ_URL"),
		EventsQueueURL:     os.Getenv("EVENTS_QUEUE_URL"),
		SettlementQueueURL: os.Getenv("SETTLEMENT_QUEUE_URL"),
		OIDCIssuer:         os.Getenv("OIDC_ISSUER"),
		OIDCJWKSURL:        os.Getenv("OIDC_JWKS_URL"),
		OIDCAudience:       os.Getenv("OIDC_AUDIENCE"),
	}
	var errs []error
	if d < 8*time.Second || d > 25*time.Second {
		errs = append(errs, errors.New("SHUTDOWN_TIMEOUT must be in [8s,25s]"))
	}
	for _, r := range c.Roles {
		switch r {
		case "api", "sqs-consumer", "outbox-publisher", "reference-worker":
		default:
			errs = append(errs, fmt.Errorf("unknown role %s", r))
		}
	}
	for k, v := range map[string]string{
		"OIDC_AUDIENCE": c.OIDCAudience, "DATABASE_URL": c.DatabaseURL, "OIDC_ISSUER": c.OIDCIssuer, "OIDC_JWKS_URL": c.OIDCJWKSURL,
		"WAGER_QUEUE_URL": c.WagerQueueURL, "EVENTS_QUEUE_URL": c.EventsQueueURL, "SETTLEMENT_QUEUE_URL": c.SettlementQueueURL,
	} {
		if v == "" {
			errs = append(errs, fmt.Errorf("missing required env %s", k))
		}
	}
	return c, errors.Join(errs...)
}
