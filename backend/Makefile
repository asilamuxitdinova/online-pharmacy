# Test Pharmacy backend. `make help` lists the targets.

BINARY    := frege-pharmacy
PKG       := github.com/asilamuxitdinova/online-pharmacy/backend
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || date -u +%Y%m%d%H%M%S)
LDFLAGS   := -s -w -X main.version=$(VERSION)
GO        ?= go
SHELL     := /bin/bash

# Local development
ADDR      ?= 127.0.0.1:8080
DATA_DIR  ?= ./data
DEV_ENV   := ADDR=$(ADDR) DATABASE_PATH=$(DATA_DIR)/pharmacy.db COOKIE_SECURE=false

# Installation (deployment). Override PREFIX or the dirs as needed.
PREFIX     ?= /usr/local
BIN_DIR    ?= $(PREFIX)/bin
UNIT_DIR   ?= /etc/systemd/system
CONFIG_DIR ?= /etc/frege-pharmacy
SERVICE    := frege-pharmacy

# Frege checkout used by frege-check.
FREGE_DIR ?= $(HOME)/Documents/frege-backend

.DEFAULT_GOAL := help

.PHONY: help
help: ## List targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## ---- build and run

.PHONY: build
build: ## Build bin/frege-pharmacy
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags '$(LDFLAGS)' -o bin/$(BINARY) ./cmd/pharmacy

.PHONY: run
run: ## Run locally on $(ADDR) with a database in ./data
	$(DEV_ENV) $(GO) run ./cmd/pharmacy serve

.PHONY: dev
dev: ## Run locally with 60s access tokens and the fixed one-time code 123456
	$(DEV_ENV) ACCESS_TOKEN_TTL=60s FIXED_OTP_CODE=123456 JOB_DURATION=10s $(GO) run ./cmd/pharmacy serve

.PHONY: reset
reset: ## Wipe the local database and seed it again
	$(DEV_ENV) $(GO) run ./cmd/pharmacy reset

.PHONY: seed
seed: ## Seed the local database if it is empty
	$(DEV_ENV) $(GO) run ./cmd/pharmacy seed

## ---- quality

.PHONY: test
test: ## Run all tests
	$(GO) test -count=1 ./...

.PHONY: test-race
test-race: ## Run all tests with the race detector
	$(GO) test -race -count=1 ./...

.PHONY: cover
cover: ## Test with coverage and print the total
	$(GO) test -count=1 -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -1

.PHONY: spec-check
spec-check: ## Check the OpenAPI spec matches the router and the guide's rules
	$(GO) test -count=1 -run 'TestSpecMatchesRoutes|TestDocs' ./internal/api/

.PHONY: frege-check
frege-check: ## Load the spec with Frege's own parser (needs FREGE_DIR; writes nothing there)
	@test -d $(FREGE_DIR)/internal/mcp || { echo "FREGE_DIR=$(FREGE_DIR) is not a Frege checkout"; exit 1; }
	@tmp=$$(mktemp -d); \
	sed 's|https://pharmacy.example.test|https://pharmacy.localhost|g' openapi/openapi.yaml > $$tmp/spec.yaml; \
	cp tools/fregecheck/check_test.go.in $$tmp/check_test.go; \
	printf '{"Replace":{"%s":"%s"}}' "$(FREGE_DIR)/internal/mcp/zz_pharmacy_check_test.go" "$$tmp/check_test.go" > $$tmp/overlay.json; \
	cd $(FREGE_DIR) && PHARMACY_SPEC=$$tmp/spec.yaml $(GO) test -overlay $$tmp/overlay.json -run TestPharmacySpecLoadsInFrege -count=1 -v ./internal/mcp/ | grep -v '^=== RUN'; \
	status=$${PIPESTATUS[0]}; rm -rf $$tmp; exit $$status

.PHONY: vet
vet: ## go vet
	$(GO) vet ./...

.PHONY: fmt
fmt: ## Format the code
	gofmt -w .

.PHONY: lint
lint: vet ## vet plus a gofmt check
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "not gofmt-ed:"; echo "$$out"; exit 1; fi

.PHONY: check
check: lint test ## Everything CI should run

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

.PHONY: clean
clean: ## Remove build output and the local database
	rm -rf bin coverage.out $(DATA_DIR)

## ---- deployment (systemd); run with sudo

.PHONY: install
install: build ## Install the binary, units and config, and create the service user
	id -u $(SERVICE) >/dev/null 2>&1 || useradd --system --home-dir /var/lib/$(SERVICE) --shell /usr/sbin/nologin $(SERVICE)
	install -D -m 0755 bin/$(BINARY) $(DESTDIR)$(BIN_DIR)/$(BINARY)
	install -D -m 0644 deploy/systemd/$(SERVICE).service $(DESTDIR)$(UNIT_DIR)/$(SERVICE).service
	install -D -m 0644 deploy/systemd/$(SERVICE)-reset.service $(DESTDIR)$(UNIT_DIR)/$(SERVICE)-reset.service
	install -D -m 0644 deploy/systemd/$(SERVICE)-reset.timer $(DESTDIR)$(UNIT_DIR)/$(SERVICE)-reset.timer
	install -d -m 0750 -o root -g $(SERVICE) $(DESTDIR)$(CONFIG_DIR)
	test -f $(DESTDIR)$(CONFIG_DIR)/pharmacy.env || install -m 0640 -o root -g $(SERVICE) deploy/pharmacy.env.example $(DESTDIR)$(CONFIG_DIR)/pharmacy.env
	systemctl daemon-reload
	@echo "Edit $(CONFIG_DIR)/pharmacy.env, then: sudo make enable"

.PHONY: enable
enable: ## Start the service and the nightly reset timer, now and on boot
	systemctl enable --now $(SERVICE).service $(SERVICE)-reset.timer

.PHONY: restart
restart: ## Restart the service (after install or a config change)
	systemctl restart $(SERVICE).service

.PHONY: status
status: ## Service and timer status
	systemctl --no-pager status $(SERVICE).service $(SERVICE)-reset.timer

.PHONY: logs
logs: ## Follow the service logs
	journalctl -u $(SERVICE).service -f -o cat

.PHONY: uninstall
uninstall: ## Stop and remove the service (keeps config and data)
	-systemctl disable --now $(SERVICE).service $(SERVICE)-reset.timer
	rm -f $(DESTDIR)$(BIN_DIR)/$(BINARY) $(DESTDIR)$(UNIT_DIR)/$(SERVICE).service \
		$(DESTDIR)$(UNIT_DIR)/$(SERVICE)-reset.service $(DESTDIR)$(UNIT_DIR)/$(SERVICE)-reset.timer
	systemctl daemon-reload
