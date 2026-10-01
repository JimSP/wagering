.PHONY: up down infra migrate-up migrate-down test race vet fmt integration

up:            ; bash scripts/up.sh
down:          ; bash scripts/down.sh
infra:         ; docker compose up -d postgres localstack keycloak
migrate-up:    ; go run ./cmd/schema up
migrate-down:  ; go run ./cmd/schema down $(STEPS)
test:          ; bash scripts/test.sh unit
race:          ; bash scripts/test.sh race
vet:           ; bash scripts/test.sh vet
fmt:           ; gofmt -l -w .
integration:   ; bash scripts/test.sh integration

.PHONY: schema-check schema-new schema-seal schema-status schema-dump schema-test
schema-check: ; go run ./cmd/schema check
schema-new: ; go run ./cmd/schema new $(NAME)
schema-seal: ; go run ./cmd/schema seal
schema-status: ; go run ./cmd/schema status
schema-dump: ; go run ./cmd/schema dump
schema-test: ; bash scripts/test-schema.sh

.PHONY: schema-snapshot
schema-snapshot: ; SCHEMA_SNAPSHOT=docs/database/schema.sql bash scripts/test-schema.sh

.PHONY: setup
setup: ; bash scripts/setup.sh
