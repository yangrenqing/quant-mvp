GO ?= /usr/local/go/bin/go
PYTHON ?= python3
FROM ?= 2025-01-01
TO ?= $(shell date +%F)
TOP ?= 10

.PHONY: help scan portfolio dataset model daily

help:
	@echo "make scan       # run A-share scan"
	@echo "make portfolio  # run portfolio backtest"
	@echo "make dataset    # export training dataset"
	@echo "make model      # run model pipeline"
	@echo "make daily      # run the full daily workflow"

scan:
	PATH=/usr/local/go/bin:$$PATH $(GO) run ./cmd/scheduler --scan-a-share --top $(TOP)

portfolio:
	PATH=/usr/local/go/bin:$$PATH $(GO) run ./cmd/scheduler --portfolio-backtest --from $(FROM) --to $(TO) --cash 100000 --fee-bps 10 --slippage-bps 5 --top 3

dataset:
	PATH=/usr/local/go/bin:$$PATH $(GO) run ./cmd/scheduler --export-dataset --from $(FROM) --to $(TO)

model:
	$(PYTHON) scripts/model_pipeline.py --from $(FROM) --to $(TO) --label label_10d

daily:
	bash scripts/daily_run.sh
