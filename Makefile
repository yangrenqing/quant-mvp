GO ?= /usr/local/go/bin/go
PYTHON ?= python3
FROM ?= 2025-01-01
TO ?= $(shell date +%F)
TOP ?= 10

.PHONY: help scan portfolio dataset model validate-config daily verify

help:
	@echo "make scan       # run A-share scan"
	@echo "make portfolio  # run portfolio backtest"
	@echo "make dataset    # export training dataset"
	@echo "make model      # run model pipeline"
	@echo "make validate-config # validate layered runtime config"
	@echo "make daily      # run the full daily workflow"
	@echo "make verify     # run Go tests, script smoke checks, and config validation"

scan:
	PATH=/usr/local/go/bin:$$PATH $(GO) run ./cmd/scheduler --scan-a-share --top $(TOP)

portfolio:
	PATH=/usr/local/go/bin:$$PATH $(GO) run ./cmd/scheduler --portfolio-backtest --from $(FROM) --to $(TO) --cash 100000 --fee-bps 10 --slippage-bps 5 --top 3

dataset:
	PATH=/usr/local/go/bin:$$PATH $(GO) run ./cmd/scheduler --export-dataset --from $(FROM) --to $(TO)

model:
	$(PYTHON) scripts/model_pipeline.py --from $(FROM) --to $(TO) --label label_10d

validate-config:
	PATH=/usr/local/go/bin:$$PATH $(GO) run ./cmd/scheduler --validate-config >/dev/null

daily:
	bash scripts/daily_run.sh

verify:
	PATH=/usr/local/go/bin:$$PATH $(GO) test ./...
	bash -n scripts/daily_run.sh
	bash -n scripts/weekly_run.sh
	bash -n scripts/intraday_run.sh
	bash -n scripts/research_run.sh
	$(PYTHON) -m py_compile scripts/*.py
	$(MAKE) validate-config
