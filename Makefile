GO ?= /usr/local/go/bin/go
GOCACHE ?= $(CURDIR)/.cache/go-build
PYTHON ?= python3
PYTHONPYCACHEPREFIX ?= $(CURDIR)/.cache/python
RUNTIME_CONFIG_SNAPSHOT ?= $(CURDIR)/reports/runtime_config.json
FROM ?= 2025-01-01
TO ?= $(shell date +%F)
TOP ?= 10

export GOCACHE PYTHONPYCACHEPREFIX

.PHONY: help scan portfolio dataset model validate-config export-runtime-config show-check-paths quick-check daily verify

help:
	@echo "make scan       # run A-share scan"
	@echo "make portfolio  # run portfolio backtest"
	@echo "make dataset    # export training dataset"
	@echo "make model      # run model pipeline"
	@echo "make validate-config # only validate layered runtime config"
	@echo "make export-runtime-config # write reports/runtime_config.json and exit"
	@echo "make show-check-paths # print cache/snapshot paths for local check troubleshooting"
	@echo "make quick-check # fast local checks: shell syntax, py_compile, and validate-config"
	@echo "make daily      # run the full daily workflow"
	@echo "make verify     # broader local preflight: Go tests plus make quick-check"
	@echo "Go-based make targets use GOCACHE=$(GOCACHE) unless GOCACHE is overridden"

scan:
	PATH=/usr/local/go/bin:$$PATH $(GO) run ./cmd/scheduler --scan-a-share --top $(TOP)

portfolio:
	PATH=/usr/local/go/bin:$$PATH $(GO) run ./cmd/scheduler --portfolio-backtest --from $(FROM) --to $(TO) --cash 100000 --fee-bps 10 --slippage-bps 5 --top 3

dataset:
	PATH=/usr/local/go/bin:$$PATH $(GO) run ./cmd/scheduler --export-dataset --from $(FROM) --to $(TO)

model:
	$(PYTHON) scripts/model_pipeline.py --from $(FROM) --to $(TO) --label label_10d

validate-config:
	@echo "==> validate-config (GOCACHE=$(GOCACHE))"
	@PATH=/usr/local/go/bin:$$PATH $(GO) run ./cmd/scheduler --validate-config >/dev/null
	@echo "config validation: ok"

export-runtime-config:
	@echo "==> export-runtime-config (GOCACHE=$(GOCACHE))"
	@PATH=/usr/local/go/bin:$$PATH $(GO) run ./cmd/scheduler --export-runtime-config >/dev/null
	@echo "runtime config snapshot: $(RUNTIME_CONFIG_SNAPSHOT)"

show-check-paths:
	@echo "go build cache: $(GOCACHE)"
	@echo "python bytecode cache: $(PYTHONPYCACHEPREFIX)"
	@echo "runtime config snapshot: $(RUNTIME_CONFIG_SNAPSHOT)"

quick-check:
	@echo "==> quick-check: shell syntax"
	@bash -n scripts/daily_run.sh
	@bash -n scripts/weekly_run.sh
	@bash -n scripts/intraday_run.sh
	@bash -n scripts/research_run.sh
	@echo "==> quick-check: Python bytecode"
	@$(PYTHON) -m py_compile scripts/*.py
	@echo "==> quick-check: layered runtime config"
	@$(MAKE) --no-print-directory validate-config

daily:
	bash scripts/daily_run.sh

verify:
	@echo "==> verify: Go tests"
	@PATH=/usr/local/go/bin:$$PATH $(GO) test ./...
	@echo "==> verify: fast local checks"
	@$(MAKE) --no-print-directory quick-check
