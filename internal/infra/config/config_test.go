package config

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func validEnvironment(t *testing.T) {
	t.Helper()
	for k, v := range map[string]string{
		"OTEL_SDK_DISABLED": "", "OTEL_SERVICE_NAME": "", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "", "OTEL_TRACES_SAMPLER_ARG": "", "METRICS_ADDR": "",
		"BET_WINDOW": "", "SHUTDOWN_TIMEOUT": "", "ROLES": "", "HTTP_ADDR": "", "LOG_LEVEL": "", "AWS_REGION": "", "AWS_ENDPOINT_URL": "", "WAGER_DLQ_URL": "",
		"DATABASE_URL": "postgres://test@localhost/test", "OIDC_ISSUER": "https://issuer.test", "OIDC_JWKS_URL": "https://issuer.test/keys", "OIDC_AUDIENCE": "wagering",
		"WAGER_QUEUE_URL": "https://queue.test/wager", "EVENTS_QUEUE_URL": "https://queue.test/events", "SETTLEMENT_QUEUE_URL": "https://queue.test/settlements",
	} {
		t.Setenv(k, v)
	}
}

func TestConfigurationDefaultsAndExplicitValues(t *testing.T) {
	validEnvironment(t)
	want := Config{OTelDisabled: true, OTelServiceName: "wagering", OTelTracesEndpoint: "http://localhost:4318/v1/traces", OTelSampleRatio: 1, BetWindow: 5 * time.Minute, Roles: []string{"api"}, HTTPAddr: ":8080", ShutdownTimeout: 25 * time.Second, LogLevel: "info", DatabaseURL: "postgres://test@localhost/test", AWSRegion: "us-east-1", WagerQueueURL: "https://queue.test/wager", EventsQueueURL: "https://queue.test/events", SettlementQueueURL: "https://queue.test/settlements", OIDCIssuer: "https://issuer.test", OIDCJWKSURL: "https://issuer.test/keys", OIDCAudience: "wagering"}
	got, err := Load()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%+v want=%+v err=%v", got, want, err)
	}
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("AWS_REGION", "sa-east-1")
	t.Setenv("SHUTDOWN_TIMEOUT", "8s")
	t.Setenv("ROLES", "api,sqs-consumer,outbox-publisher,reference-worker")
	t.Setenv("AWS_ENDPOINT_URL", "http://localhost:4566")
	t.Setenv("WAGER_DLQ_URL", "https://queue.test/dead")
	want.HTTPAddr = ":9090"
	want.LogLevel = "debug"
	want.AWSRegion = "sa-east-1"
	want.ShutdownTimeout = 8 * time.Second
	want.Roles = []string{"api", "sqs-consumer", "outbox-publisher", "reference-worker"}
	want.AWSEndpointURL = "http://localhost:4566"
	want.WagerDLQURL = "https://queue.test/dead"
	got, err = Load()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%+v want=%+v err=%v", got, want, err)
	}
	for _, role := range want.Roles {
		if !got.HasRole(role) {
			t.Fatal("missing role", role)
		}
	}
	if got.HasRole("unknown") {
		t.Fatal("invented role")
	}
}

func TestConfigurationRejectsEachMissingRequiredVariable(t *testing.T) {
	for _, key := range []string{"DATABASE_URL", "OIDC_ISSUER", "OIDC_JWKS_URL", "OIDC_AUDIENCE", "WAGER_QUEUE_URL", "EVENTS_QUEUE_URL", "SETTLEMENT_QUEUE_URL"} {
		t.Run(key, func(t *testing.T) {
			validEnvironment(t)
			t.Setenv(key, "")
			_, err := Load()
			if err == nil || err.Error() != "missing required env "+key {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestShutdownBudgetBoundsAndInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{{"7.999999999s", false}, {"8s", true}, {"8.000000001s", true}, {"24.999999999s", true}, {"25s", true}, {"25.000000001s", false}, {"-1s", false}, {"invalid", false}} {
		t.Run(tc.value, func(t *testing.T) {
			validEnvironment(t)
			t.Setenv("SHUTDOWN_TIMEOUT", tc.value)
			got, err := Load()
			if tc.valid {
				want, e := time.ParseDuration(tc.value)
				if e != nil || err != nil || got.ShutdownTimeout != want {
					t.Fatalf("budget=%v err=%v", got.ShutdownTimeout, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "SHUTDOWN_TIMEOUT") {
				t.Fatalf("invalid duration accepted: %+v %v", got, err)
			}
		})
	}
	t.Run("unknown role", func(t *testing.T) {
		validEnvironment(t)
		t.Setenv("ROLES", "api,unknown")
		_, err := Load()
		if err == nil || err.Error() != "unknown role unknown" {
			t.Fatal(err)
		}
	})
}

func TestBetWindowConfiguration(t *testing.T) {
	for _, value := range []string{"1s", "30s", "5m", "2h", "0s", "-1s", "500ms", "1.5s", "invalid"} {
		t.Run(value, func(t *testing.T) {
			validEnvironment(t)
			t.Setenv("BET_WINDOW", value)
			got, err := Load()
			want, parseErr := time.ParseDuration(value)
			valid := parseErr == nil && want >= time.Second && want%time.Second == 0
			if valid && (err != nil || got.BetWindow != want) {
				t.Fatalf("config=%+v error=%v", got, err)
			}
			if !valid && (err == nil || !strings.Contains(err.Error(), "BET_WINDOW")) {
				t.Fatalf("accepted invalid window: %v", err)
			}
		})
	}
}
