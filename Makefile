GO ?= /usr/local/go/bin/go
GOCACHE ?= $(CURDIR)/.cache/go-build
PYTHON ?= python3
PYTHONPYCACHEPREFIX ?= $(CURDIR)/.cache/python
SCHEDULER_CMD ?= ./cmd/scheduler
RUNTIME_CONFIG_SNAPSHOT ?= $(CURDIR)/reports/runtime_config.json
LAYERED_CONFIG_LOAD_ORDER ?= configs/config.yaml configs/data.yaml configs/portfolio.yaml configs/model.yaml configs/market.yaml configs/report.yaml
LAYERED_CONFIG_FINAL_OVERRIDE ?= configs/local.yaml
LAYERED_CONFIG_PRESENT ?= $(filter $(wildcard $(LAYERED_CONFIG_LOAD_ORDER)),$(LAYERED_CONFIG_LOAD_ORDER))
LAYERED_CONFIG_ABSENT ?= $(filter-out $(LAYERED_CONFIG_PRESENT),$(LAYERED_CONFIG_LOAD_ORDER))
LAYERED_CONFIG_INPUTS ?= $(LAYERED_CONFIG_LOAD_ORDER) $(LAYERED_CONFIG_FINAL_OVERRIDE)
QUICK_CHECK_SHELL_SCRIPTS ?= scripts/daily_run.sh scripts/weekly_run.sh scripts/intraday_run.sh scripts/research_run.sh
QUICK_CHECK_STEPS ?= shell syntax, py_compile, and validate-config
VERIFY_STEPS ?= Go tests plus make quick-check
VALIDATE_CONFIG_FOLLOW_UP ?= console output from $(SCHEDULER_CMD) --validate-config
EXPORT_RUNTIME_CONFIG_FOLLOW_UP ?= $(RUNTIME_CONFIG_SNAPSHOT)
QUICK_CHECK_FOLLOW_UP ?= console output from $(QUICK_CHECK_STEPS)
VERIFY_FOLLOW_UP ?= console output from $(VERIFY_STEPS)
SCAN_REPORT_TEXT ?= $(CURDIR)/reports/a_share_scan.txt
SCAN_REPORT_HTML ?= $(CURDIR)/reports/a_share_scan.html
SCAN_REPORT_JSON ?= $(CURDIR)/reports/a_share_scan.json
SCAN_FOCUS_SHORTLIST_TEXT ?= $(CURDIR)/reports/a_share_focus.txt
SCAN_FOCUS_SHORTLIST_HTML ?= $(CURDIR)/reports/a_share_focus.html
SCAN_FOCUS_SHORTLIST_JSON ?= $(CURDIR)/reports/a_share_focus.json
SCAN_RECOMMENDED_OUTPUT_PATH ?= $(SCAN_REPORT_HTML)
SCAN_PRIMARY_OUTPUT_PATHS ?= $(SCAN_REPORT_TEXT)
SCAN_COMPANION_OUTPUT_PATHS ?= $(SCAN_REPORT_HTML) $(SCAN_REPORT_JSON)
SCAN_OUTPUT_PATHS ?= $(SCAN_PRIMARY_OUTPUT_PATHS) $(SCAN_COMPANION_OUTPUT_PATHS)
SCAN_FOCUS_SHORTLIST_OUTPUT_PATHS ?= $(SCAN_FOCUS_SHORTLIST_TEXT) $(SCAN_FOCUS_SHORTLIST_HTML) $(SCAN_FOCUS_SHORTLIST_JSON)
PORTFOLIO_REPORT_TEXT ?= $(CURDIR)/reports/portfolio_backtest.txt
PORTFOLIO_REPORT_HTML ?= $(CURDIR)/reports/portfolio_backtest.html
PORTFOLIO_REPORT_JSON ?= $(CURDIR)/reports/portfolio_backtest.json
PORTFOLIO_REPORT_CSV ?= $(CURDIR)/reports/portfolio_backtest.csv
PORTFOLIO_RECOMMENDED_OUTPUT_PATH ?= $(PORTFOLIO_REPORT_HTML)
PORTFOLIO_PRIMARY_OUTPUT_PATHS ?= $(PORTFOLIO_REPORT_TEXT)
PORTFOLIO_COMPANION_OUTPUT_PATHS ?= $(PORTFOLIO_REPORT_HTML) $(PORTFOLIO_REPORT_JSON) $(PORTFOLIO_REPORT_CSV)
PORTFOLIO_OUTPUT_PATHS ?= $(PORTFOLIO_PRIMARY_OUTPUT_PATHS) $(PORTFOLIO_COMPANION_OUTPUT_PATHS)
DATASET_EXPORT_CSV ?= $(CURDIR)/reports/training_dataset.csv
DATASET_EXPORT_TEXT ?= $(CURDIR)/reports/training_dataset.txt
DATASET_EXPORT_JSON ?= $(CURDIR)/reports/training_dataset.json
DATASET_RECOMMENDED_OUTPUT_PATH ?= $(DATASET_EXPORT_CSV)
DATASET_PRIMARY_OUTPUT_PATHS ?= $(DATASET_EXPORT_TEXT)
DATASET_COMPANION_OUTPUT_PATHS ?= $(DATASET_EXPORT_CSV) $(DATASET_EXPORT_JSON)
DATASET_OUTPUT_PATHS ?= $(DATASET_PRIMARY_OUTPUT_PATHS) $(DATASET_COMPANION_OUTPUT_PATHS)
MODEL_PIPELINE_REPORT ?= $(CURDIR)/reports/model_pipeline_latest.txt
MODEL_PREDICTIONS ?= $(CURDIR)/reports/model_predictions.csv
MODEL_REGRESSION_JSON ?= $(CURDIR)/reports/linear_model.json
MODEL_CLASSIFIER_JSON ?= $(CURDIR)/reports/benchmark_classifier.json
MODEL_VERSIONS_DIR ?= $(CURDIR)/reports/model_versions
MODEL_RECOMMENDED_OUTPUT_PATH ?= $(MODEL_PIPELINE_REPORT)
MODEL_PRIMARY_OUTPUT_PATHS ?= $(MODEL_PIPELINE_REPORT)
MODEL_COMPANION_OUTPUT_PATHS ?= $(MODEL_PREDICTIONS) $(MODEL_REGRESSION_JSON) $(MODEL_CLASSIFIER_JSON)
MODEL_OUTPUT_PATHS ?= $(MODEL_PRIMARY_OUTPUT_PATHS) $(MODEL_COMPANION_OUTPUT_PATHS) $(MODEL_VERSIONS_DIR)
MODEL_LATEST_OUTPUT_PATHS ?= $(MODEL_PRIMARY_OUTPUT_PATHS) $(MODEL_COMPANION_OUTPUT_PATHS)
MODEL_HISTORY_OUTPUT_PATHS ?= $(MODEL_VERSIONS_DIR)
FROM ?= 2025-01-01
TO ?= $(shell date +%F)
TOP ?= 10

export GOCACHE PYTHONPYCACHEPREFIX

.PHONY: help scan portfolio dataset model show-output-paths validate-config export-runtime-config show-check-paths quick-check daily verify

help:
	@echo "make scan       # run A-share scan"
	@echo "make portfolio  # run portfolio backtest"
	@echo "make dataset    # export training dataset"
	@echo "make model      # run model pipeline"
	@echo "make show-output-paths # print expected generated artifacts for scan (main report plus focus-only shortlist), portfolio (text plus HTML/JSON/CSV companions), dataset, and model, plus one open-this-first artifact per workflow, and whether each exists on disk"
	@echo "make validate-config # only validate layered runtime config"
	@echo "make export-runtime-config # write $(RUNTIME_CONFIG_SNAPSHOT) and exit"
	@echo "make show-check-paths # print caches, config inputs, checked scripts, export output, and follow-up artifact/output for check targets"
	@echo "make quick-check # fast local checks: $(QUICK_CHECK_STEPS)"
	@echo "make daily      # run the full daily workflow"
	@echo "make verify     # broader local preflight: $(VERIFY_STEPS)"
	@echo "Go-based make targets use GOCACHE=$(GOCACHE) unless GOCACHE is overridden"

scan:
	PATH=/usr/local/go/bin:$$PATH $(GO) run $(SCHEDULER_CMD) --scan-a-share --top $(TOP)

portfolio:
	PATH=/usr/local/go/bin:$$PATH $(GO) run $(SCHEDULER_CMD) --portfolio-backtest --from $(FROM) --to $(TO) --cash 100000 --fee-bps 10 --slippage-bps 5 --top 3

dataset:
	PATH=/usr/local/go/bin:$$PATH $(GO) run $(SCHEDULER_CMD) --export-dataset --from $(FROM) --to $(TO)

model:
	$(PYTHON) scripts/model_pipeline.py --from $(FROM) --to $(TO) --label label_10d

show-output-paths:
	@print_paths() { \
		label="$$1"; shift; \
		echo "$$label"; \
		for path in "$$@"; do \
			if [ -e "$$path" ]; then status="present"; else status="missing"; fi; \
			printf '  [%s] %s\n' "$$status" "$$path"; \
		done; \
	}; \
	print_paths "scan open this first artifact:" $(SCAN_RECOMMENDED_OUTPUT_PATH); \
	print_paths "scan main generated artifacts:" $(SCAN_OUTPUT_PATHS); \
	print_paths "scan focus-only shortlist artifacts:" $(SCAN_FOCUS_SHORTLIST_OUTPUT_PATHS); \
	print_paths "portfolio open this first artifact:" $(PORTFOLIO_RECOMMENDED_OUTPUT_PATH); \
	print_paths "portfolio all generated artifacts:" $(PORTFOLIO_OUTPUT_PATHS); \
	print_paths "dataset open this first artifact:" $(DATASET_RECOMMENDED_OUTPUT_PATH); \
	print_paths "dataset all generated artifacts:" $(DATASET_OUTPUT_PATHS); \
	print_paths "model open this first artifact:" $(MODEL_RECOMMENDED_OUTPUT_PATH); \
	print_paths "model all generated current/latest artifacts:" $(MODEL_LATEST_OUTPUT_PATHS); \
	print_paths "model generated history directory:" $(MODEL_HISTORY_OUTPUT_PATHS)

validate-config:
	@echo "==> validate-config (GOCACHE=$(GOCACHE))"
	@PATH=/usr/local/go/bin:$$PATH $(GO) run $(SCHEDULER_CMD) --validate-config >/dev/null
	@echo "config validation: ok"

export-runtime-config:
	@echo "==> export-runtime-config (GOCACHE=$(GOCACHE))"
	@PATH=/usr/local/go/bin:$$PATH $(GO) run $(SCHEDULER_CMD) --export-runtime-config >/dev/null
	@echo "runtime config snapshot: $(RUNTIME_CONFIG_SNAPSHOT)"

show-check-paths:
	@echo "go build cache: $(GOCACHE)"
	@echo "python bytecode cache: $(PYTHONPYCACHEPREFIX)"
	@echo "layered config inputs: $(LAYERED_CONFIG_INPUTS)"
	@echo "layered config load order: $(LAYERED_CONFIG_LOAD_ORDER)"
	@echo "layered config files present on disk: $(if $(strip $(LAYERED_CONFIG_PRESENT)),$(LAYERED_CONFIG_PRESENT),<none>)"
	@echo "layered config files absent on disk: $(if $(strip $(LAYERED_CONFIG_ABSENT)),$(LAYERED_CONFIG_ABSENT),<none>)"
	@echo "layered config optional final override: $(if $(wildcard $(LAYERED_CONFIG_FINAL_OVERRIDE)),$(LAYERED_CONFIG_FINAL_OVERRIDE),<not present>)"
	@echo "quick-check shell scripts: $(QUICK_CHECK_SHELL_SCRIPTS)"
	@echo "export-runtime-config output: $(RUNTIME_CONFIG_SNAPSHOT)"
	@echo "validate-config follow-up output: $(VALIDATE_CONFIG_FOLLOW_UP)"
	@echo "export-runtime-config follow-up artifact: $(EXPORT_RUNTIME_CONFIG_FOLLOW_UP)"
	@echo "quick-check follow-up output: $(QUICK_CHECK_FOLLOW_UP)"
	@echo "verify follow-up output: $(VERIFY_FOLLOW_UP)"

quick-check:
	@echo "==> quick-check: shell syntax"
	@for script in $(QUICK_CHECK_SHELL_SCRIPTS); do \
		bash -n "$$script"; \
	done
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
