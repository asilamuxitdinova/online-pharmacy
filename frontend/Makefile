# Dorixona frontend. `make help` lists the targets.

SHELL := /bin/bash
NPM   ?= npm

# Where the pharmacy backend listens. The dev server proxies /api, /oauth,
# /docs and /openapi.yaml to it, so the browser only sees one origin.
PHARMACY_API ?= http://127.0.0.1:8080

.DEFAULT_GOAL := help

.PHONY: help
help: ## List targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

node_modules: package.json package-lock.json
	$(NPM) ci
	@touch node_modules

.PHONY: install
install: node_modules ## Install dependencies (npm ci)

.PHONY: dev
dev: node_modules ## Dev server on :5173, proxying to PHARMACY_API
	PHARMACY_API=$(PHARMACY_API) $(NPM) run dev

.PHONY: typecheck
typecheck: node_modules ## TypeScript check, no output
	$(NPM) run typecheck

.PHONY: build
build: node_modules ## Typecheck and build to dist/
	$(NPM) run build

.PHONY: preview
preview: build ## Serve the production build locally
	PHARMACY_API=$(PHARMACY_API) $(NPM) run preview

.PHONY: check
check: typecheck build ## Everything CI should run

.PHONY: clean
clean: ## Remove build output
	rm -rf dist node_modules/.tmp

.PHONY: distclean
distclean: clean ## Also remove node_modules
	rm -rf node_modules
