GO ?= /usr/local/go/bin/go
GOCACHE ?= $(CURDIR)/.cache/go-build
PYTHON ?= python3
PYTHONPYCACHEPREFIX ?= $(CURDIR)/.cache/python
SCHEDULER_CMD ?= ./cmd/scheduler
RUNTIME_CONFIG_SNAPSHOT ?= $(CURDIR)/reports/runtime_config.json
REPORTS_DIR ?= $(CURDIR)/reports
REPORT_HISTORY_DIR ?= $(REPORTS_DIR)/history
REPORT_DASHBOARD_OVERVIEW_PATH ?= $(REPORTS_DIR)/dashboard.html
REPORT_DASHBOARD_OVERVIEW_JSON ?= $(REPORTS_DIR)/dashboard.json
REPORT_MARKET_OVERVIEW_PATH ?= $(REPORTS_DIR)/market_overview.html
REPORT_MARKET_OVERVIEW_JSON ?= $(REPORTS_DIR)/market_overview.json
REPORT_HISTORY_OVERVIEW_PATH ?= $(REPORTS_DIR)/history_compare.html
REPORT_HISTORY_OVERVIEW_JSON ?= $(REPORTS_DIR)/history_compare.json
REPORT_RESEARCH_SUMMARY_PATH ?= $(REPORTS_DIR)/research_summary.html
REPORT_RESEARCH_SUMMARY_JSON ?= $(REPORTS_DIR)/research_summary.json
REPORT_OVERVIEW_RECOMMENDED_OUTPUT_PATH ?= $(REPORT_DASHBOARD_OVERVIEW_PATH)
REPORT_OVERVIEW_RECOMMENDED_MACHINE_OUTPUT_PATH ?= $(REPORT_DASHBOARD_OVERVIEW_JSON)
REPORT_OVERVIEW_LIVE_STATUS_PATH ?= $(REPORT_DASHBOARD_OVERVIEW_PATH)
REPORT_OVERVIEW_LIVE_STATUS_MACHINE_PATH ?= $(REPORT_DASHBOARD_OVERVIEW_JSON)
REPORT_OVERVIEW_RETROSPECTIVE_PATH ?= $(REPORT_HISTORY_OVERVIEW_PATH)
REPORT_OVERVIEW_RETROSPECTIVE_MACHINE_PATH ?= $(REPORT_HISTORY_OVERVIEW_JSON)
REPORT_OVERVIEW_MONITORING_PATHS ?= $(REPORT_DASHBOARD_OVERVIEW_PATH) $(REPORT_MARKET_OVERVIEW_PATH)
REPORT_OVERVIEW_MONITORING_MACHINE_PATHS ?= $(REPORT_DASHBOARD_OVERVIEW_JSON) $(REPORT_MARKET_OVERVIEW_JSON)
REPORT_OVERVIEW_REVIEW_PATHS ?= $(REPORT_HISTORY_OVERVIEW_PATH) $(REPORT_RESEARCH_SUMMARY_PATH)
REPORT_OVERVIEW_REVIEW_MACHINE_PATHS ?= $(REPORT_HISTORY_OVERVIEW_JSON) $(REPORT_RESEARCH_SUMMARY_JSON)
REPORT_OVERVIEW_ENTRY_PATHS ?= $(REPORT_DASHBOARD_OVERVIEW_PATH) $(REPORT_MARKET_OVERVIEW_PATH) $(REPORT_HISTORY_OVERVIEW_PATH) $(REPORT_RESEARCH_SUMMARY_PATH)
REPORT_OVERVIEW_MACHINE_PATHS ?= $(REPORT_DASHBOARD_OVERVIEW_JSON) $(REPORT_MARKET_OVERVIEW_JSON) $(REPORT_HISTORY_OVERVIEW_JSON) $(REPORT_RESEARCH_SUMMARY_JSON)
REPORT_HISTORY_DATE_PATTERN ?= $(REPORT_HISTORY_DIR)/YYYY-MM-DD
ARCHIVE_ENTRY_FORMAT_NOTE ?= scan/portfolio archive entry files default to HTML for quick visual review; dataset archive entry files default to CSV because the export is data-first.
OVERVIEW_GROUP_PAIRING_NOTE ?= HTML = quick scan; JSON = automation; order matches line-for-line.
WORKFLOW_GROUP_PAIRING_NOTE ?= order = open this first → machine → summaries → structured → archive → archive start → archive machine.
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
SCAN_RECOMMENDED_MACHINE_OUTPUT_PATH ?= $(SCAN_REPORT_JSON)
SCAN_PRIMARY_OUTPUT_PATHS ?= $(SCAN_REPORT_TEXT)
SCAN_COMPANION_OUTPUT_PATHS ?= $(SCAN_REPORT_HTML) $(SCAN_REPORT_JSON)
SCAN_OUTPUT_PATHS ?= $(SCAN_PRIMARY_OUTPUT_PATHS) $(SCAN_COMPANION_OUTPUT_PATHS)
SCAN_HUMAN_OUTPUT_PATHS ?= $(SCAN_REPORT_TEXT) $(SCAN_REPORT_HTML)
SCAN_MACHINE_OUTPUT_PATHS ?= $(SCAN_REPORT_JSON)
SCAN_FOCUS_SHORTLIST_OUTPUT_PATHS ?= $(SCAN_FOCUS_SHORTLIST_TEXT) $(SCAN_FOCUS_SHORTLIST_HTML) $(SCAN_FOCUS_SHORTLIST_JSON)
SCAN_FOCUS_HUMAN_OUTPUT_PATHS ?= $(SCAN_FOCUS_SHORTLIST_TEXT) $(SCAN_FOCUS_SHORTLIST_HTML)
SCAN_FOCUS_MACHINE_OUTPUT_PATHS ?= $(SCAN_FOCUS_SHORTLIST_JSON)
SCAN_HISTORY_OUTPUT_PATHS ?= $(REPORT_HISTORY_DATE_PATTERN)/a_share_scan
SCAN_HISTORY_RECOMMENDED_ENTRY_FILE ?= a_share_scan.html
SCAN_HISTORY_RECOMMENDED_OUTPUT_PATH ?= $(SCAN_HISTORY_OUTPUT_PATHS)/$(SCAN_HISTORY_RECOMMENDED_ENTRY_FILE)
SCAN_HISTORY_RECOMMENDED_MACHINE_OUTPUT_PATH ?= $(SCAN_HISTORY_OUTPUT_PATHS)/a_share_scan.json
PORTFOLIO_REPORT_TEXT ?= $(CURDIR)/reports/portfolio_backtest.txt
PORTFOLIO_REPORT_HTML ?= $(CURDIR)/reports/portfolio_backtest.html
PORTFOLIO_REPORT_JSON ?= $(CURDIR)/reports/portfolio_backtest.json
PORTFOLIO_REPORT_CSV ?= $(CURDIR)/reports/portfolio_backtest.csv
PORTFOLIO_RECOMMENDED_OUTPUT_PATH ?= $(PORTFOLIO_REPORT_HTML)
PORTFOLIO_RECOMMENDED_MACHINE_OUTPUT_PATH ?= $(PORTFOLIO_REPORT_JSON)
PORTFOLIO_PRIMARY_OUTPUT_PATHS ?= $(PORTFOLIO_REPORT_TEXT)
PORTFOLIO_COMPANION_OUTPUT_PATHS ?= $(PORTFOLIO_REPORT_HTML) $(PORTFOLIO_REPORT_JSON) $(PORTFOLIO_REPORT_CSV)
PORTFOLIO_OUTPUT_PATHS ?= $(PORTFOLIO_PRIMARY_OUTPUT_PATHS) $(PORTFOLIO_COMPANION_OUTPUT_PATHS)
PORTFOLIO_HUMAN_OUTPUT_PATHS ?= $(PORTFOLIO_REPORT_TEXT) $(PORTFOLIO_REPORT_HTML)
PORTFOLIO_MACHINE_OUTPUT_PATHS ?= $(PORTFOLIO_REPORT_JSON) $(PORTFOLIO_REPORT_CSV)
PORTFOLIO_HISTORY_OUTPUT_PATHS ?= $(REPORT_HISTORY_DATE_PATTERN)/portfolio_backtest
PORTFOLIO_HISTORY_RECOMMENDED_ENTRY_FILE ?= portfolio_backtest.html
PORTFOLIO_HISTORY_RECOMMENDED_OUTPUT_PATH ?= $(PORTFOLIO_HISTORY_OUTPUT_PATHS)/$(PORTFOLIO_HISTORY_RECOMMENDED_ENTRY_FILE)
PORTFOLIO_HISTORY_RECOMMENDED_MACHINE_OUTPUT_PATH ?= $(PORTFOLIO_HISTORY_OUTPUT_PATHS)/portfolio_backtest.json
DATASET_EXPORT_CSV ?= $(CURDIR)/reports/training_dataset.csv
DATASET_EXPORT_TEXT ?= $(CURDIR)/reports/training_dataset.txt
DATASET_EXPORT_JSON ?= $(CURDIR)/reports/training_dataset.json
DATASET_RECOMMENDED_OUTPUT_PATH ?= $(DATASET_EXPORT_CSV)
DATASET_RECOMMENDED_MACHINE_OUTPUT_PATH ?= $(DATASET_EXPORT_JSON)
DATASET_PRIMARY_OUTPUT_PATHS ?= $(DATASET_EXPORT_TEXT)
DATASET_COMPANION_OUTPUT_PATHS ?= $(DATASET_EXPORT_CSV) $(DATASET_EXPORT_JSON)
DATASET_OUTPUT_PATHS ?= $(DATASET_PRIMARY_OUTPUT_PATHS) $(DATASET_COMPANION_OUTPUT_PATHS)
DATASET_HUMAN_OUTPUT_PATHS ?= $(DATASET_EXPORT_TEXT)
DATASET_MACHINE_OUTPUT_PATHS ?= $(DATASET_EXPORT_CSV) $(DATASET_EXPORT_JSON)
DATASET_HISTORY_OUTPUT_PATHS ?= $(REPORT_HISTORY_DATE_PATTERN)/training_dataset
DATASET_HISTORY_RECOMMENDED_ENTRY_FILE ?= training_dataset.csv
DATASET_HISTORY_RECOMMENDED_OUTPUT_PATH ?= $(DATASET_HISTORY_OUTPUT_PATHS)/$(DATASET_HISTORY_RECOMMENDED_ENTRY_FILE)
DATASET_HISTORY_RECOMMENDED_MACHINE_OUTPUT_PATH ?= $(DATASET_HISTORY_OUTPUT_PATHS)/training_dataset.json
WORKFLOW_ARCHIVE_MONITORING_PATHS ?= $(SCAN_HISTORY_RECOMMENDED_OUTPUT_PATH) $(PORTFOLIO_HISTORY_RECOMMENDED_OUTPUT_PATH)
WORKFLOW_ARCHIVE_MONITORING_MACHINE_PATHS ?= $(SCAN_HISTORY_RECOMMENDED_MACHINE_OUTPUT_PATH) $(PORTFOLIO_HISTORY_RECOMMENDED_MACHINE_OUTPUT_PATH)
WORKFLOW_ARCHIVE_REVIEW_PATHS ?= $(DATASET_HISTORY_RECOMMENDED_OUTPUT_PATH) $(MODEL_HISTORY_OUTPUT_PATHS)
WORKFLOW_ARCHIVE_REVIEW_MACHINE_PATHS ?= $(DATASET_HISTORY_RECOMMENDED_MACHINE_OUTPUT_PATH)
WORKFLOW_LATEST_MONITORING_MACHINE_PATHS ?= $(SCAN_RECOMMENDED_MACHINE_OUTPUT_PATH) $(PORTFOLIO_RECOMMENDED_MACHINE_OUTPUT_PATH)
WORKFLOW_LATEST_REVIEW_MACHINE_PATHS ?= $(DATASET_RECOMMENDED_MACHINE_OUTPUT_PATH) $(MODEL_RECOMMENDED_MACHINE_OUTPUT_PATH)
MODEL_PIPELINE_REPORT ?= $(CURDIR)/reports/model_pipeline_latest.txt
MODEL_TRAIN_REPORT ?= $(CURDIR)/reports/model_train.txt
MODEL_CLASSIFIER_REPORT ?= $(CURDIR)/reports/benchmark_classifier.txt
MODEL_PREDICTIONS ?= $(CURDIR)/reports/model_predictions.csv
MODEL_CLASSIFIER_PREDICTIONS ?= $(CURDIR)/reports/benchmark_classifier_predictions.csv
MODEL_REGRESSION_JSON ?= $(CURDIR)/reports/linear_model.json
MODEL_CLASSIFIER_JSON ?= $(CURDIR)/reports/benchmark_classifier.json
MODEL_REGISTRY_LOG ?= $(CURDIR)/reports/model_registry.jsonl
MODEL_VERSIONS_DIR ?= $(REPORTS_DIR)/model_versions
MODEL_RECOMMENDED_OUTPUT_PATH ?= $(MODEL_PIPELINE_REPORT)
MODEL_RECOMMENDED_MACHINE_OUTPUT_PATH ?= $(MODEL_PREDICTIONS)
MODEL_PRIMARY_OUTPUT_PATHS ?= $(MODEL_PIPELINE_REPORT)
MODEL_COMPANION_OUTPUT_PATHS ?= $(MODEL_PREDICTIONS) $(MODEL_CLASSIFIER_PREDICTIONS) $(MODEL_REGRESSION_JSON) $(MODEL_CLASSIFIER_JSON) $(MODEL_REGISTRY_LOG)
MODEL_ADDITIONAL_SUMMARY_OUTPUT_PATHS ?= $(MODEL_TRAIN_REPORT) $(MODEL_CLASSIFIER_REPORT)
MODEL_OUTPUT_PATHS ?= $(MODEL_PRIMARY_OUTPUT_PATHS) $(MODEL_ADDITIONAL_SUMMARY_OUTPUT_PATHS) $(MODEL_COMPANION_OUTPUT_PATHS) $(MODEL_VERSIONS_DIR)
MODEL_LATEST_OUTPUT_PATHS ?= $(MODEL_PRIMARY_OUTPUT_PATHS) $(MODEL_ADDITIONAL_SUMMARY_OUTPUT_PATHS) $(MODEL_COMPANION_OUTPUT_PATHS)
MODEL_HUMAN_OUTPUT_PATHS ?= $(MODEL_PRIMARY_OUTPUT_PATHS) $(MODEL_ADDITIONAL_SUMMARY_OUTPUT_PATHS)
MODEL_MACHINE_OUTPUT_PATHS ?= $(MODEL_PREDICTIONS) $(MODEL_CLASSIFIER_PREDICTIONS) $(MODEL_REGRESSION_JSON) $(MODEL_CLASSIFIER_JSON) $(MODEL_REGISTRY_LOG)
MODEL_HISTORY_OUTPUT_PATHS ?= $(MODEL_VERSIONS_DIR)
WORKFLOW_LATEST_MONITORING_OUTPUT_PATHS ?= $(SCAN_RECOMMENDED_OUTPUT_PATH) $(PORTFOLIO_RECOMMENDED_OUTPUT_PATH)
WORKFLOW_LATEST_REVIEW_OUTPUT_PATHS ?= $(DATASET_RECOMMENDED_OUTPUT_PATH) $(MODEL_RECOMMENDED_OUTPUT_PATH)
WORKFLOW_SUMMARY_MONITORING_PATHS ?= $(SCAN_HUMAN_OUTPUT_PATHS) $(PORTFOLIO_HUMAN_OUTPUT_PATHS)
WORKFLOW_SUMMARY_REVIEW_PATHS ?= $(DATASET_HUMAN_OUTPUT_PATHS) $(MODEL_HUMAN_OUTPUT_PATHS)
WORKFLOW_STRUCTURED_MONITORING_PATHS ?= $(SCAN_MACHINE_OUTPUT_PATHS) $(PORTFOLIO_MACHINE_OUTPUT_PATHS)
WORKFLOW_STRUCTURED_REVIEW_PATHS ?= $(DATASET_MACHINE_OUTPUT_PATHS) $(MODEL_MACHINE_OUTPUT_PATHS)
FROM ?= 2025-01-01
TO ?= $(shell date +%F)
TOP ?= 10

export GOCACHE PYTHONPYCACHEPREFIX

.PHONY: help scan portfolio dataset model show-start-here show-output-paths validate-config export-runtime-config show-check-paths quick-check daily verify

help:
	@echo "make scan       # run A-share scan"
	@echo "make portfolio  # run portfolio backtest"
	@echo "make dataset    # export training dataset"
	@echo "make model      # run model pipeline"
	@echo "make show-start-here # quick entry paths + grouped machine companions"
	@echo "make show-output-paths # full path map with grouped and archive details"
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

show-start-here:
	@print_paths() { \
		label="$$1"; shift; \
		echo "$$label"; \
		for path in "$$@"; do \
			if printf '%s\n' "$$path" | grep -q '/YYYY-MM-DD/'; then \
				history_root="$${path%%/YYYY-MM-DD/*}"; \
				history_suffix="$${path#*/YYYY-MM-DD/}"; \
				if [ -d "$$history_root" ] && find "$$history_root" -path "*/$$history_suffix" 2>/dev/null | grep -q .; then status="present"; else status="missing"; fi; \
			elif [ -e "$$path" ]; then status="present"; else status="missing"; fi; \
			printf '  [%s] %s\n' "$$status" "$$path"; \
		done; \
	}; \
	echo "note: lightweight companion to show-output-paths; use it when you only need the main start-here paths and grouped machine companions."; \
	print_paths "start: primary:" $(REPORT_OVERVIEW_RECOMMENDED_OUTPUT_PATH) $(REPORT_OVERVIEW_RECOMMENDED_MACHINE_OUTPUT_PATH); \
	print_paths "start: trading day:" $(REPORT_OVERVIEW_LIVE_STATUS_PATH) $(REPORT_OVERVIEW_LIVE_STATUS_MACHINE_PATH); \
	print_paths "start: market context:" $(REPORT_MARKET_OVERVIEW_PATH) $(REPORT_MARKET_OVERVIEW_JSON); \
	print_paths "start: after close:" $(REPORT_OVERVIEW_RETROSPECTIVE_PATH) $(REPORT_OVERVIEW_RETROSPECTIVE_MACHINE_PATH); \
	print_paths "start: research wrap-up:" $(REPORT_RESEARCH_SUMMARY_PATH) $(REPORT_RESEARCH_SUMMARY_JSON); \
	print_paths "latest: monitoring:" $(WORKFLOW_LATEST_MONITORING_OUTPUT_PATHS); \
	print_paths "machine: monitoring:" $(WORKFLOW_LATEST_MONITORING_MACHINE_PATHS); \
	print_paths "latest: review:" $(WORKFLOW_LATEST_REVIEW_OUTPUT_PATHS); \
	print_paths "machine: review:" $(WORKFLOW_LATEST_REVIEW_MACHINE_PATHS); \
	print_paths "scan open this first:" $(SCAN_RECOMMENDED_OUTPUT_PATH); \
	print_paths "portfolio open this first:" $(PORTFOLIO_RECOMMENDED_OUTPUT_PATH); \
	print_paths "dataset open this first:" $(DATASET_RECOMMENDED_OUTPUT_PATH); \
	print_paths "model open this first:" $(MODEL_RECOMMENDED_OUTPUT_PATH)

show-output-paths:
	@print_paths() { \
		label="$$1"; shift; \
		echo "$$label"; \
		for path in "$$@"; do \
			if printf '%s\n' "$$path" | grep -q '/YYYY-MM-DD/'; then \
				history_root="$${path%%/YYYY-MM-DD/*}"; \
				history_suffix="$${path#*/YYYY-MM-DD/}"; \
				if [ -d "$$history_root" ] && find "$$history_root" -path "*/$$history_suffix" 2>/dev/null | grep -q .; then status="present"; else status="missing"; fi; \
			elif [ -e "$$path" ]; then status="present"; else status="missing"; fi; \
			printf '  [%s] %s\n' "$$status" "$$path"; \
		done; \
	}; \
	echo "note: overview starts with the primary and intent-specific HTML+JSON pairs, then grouped broad-overview blocks. Shared pairing semantics: $(OVERVIEW_GROUP_PAIRING_NOTE)"; \
	echo "note: workflow blocks split quick monitoring (scan/portfolio) from deeper review (dataset/model); $(WORKFLOW_GROUP_PAIRING_NOTE) Here, 'machine' means the closest machine-readable companion. Model exception: latest human-readable entry is text, closest machine-readable companion is predictions CSV, not JSON."; \
	print_paths "start: primary:" $(REPORT_OVERVIEW_RECOMMENDED_OUTPUT_PATH) $(REPORT_OVERVIEW_RECOMMENDED_MACHINE_OUTPUT_PATH); \
	print_paths "start: trading day:" $(REPORT_OVERVIEW_LIVE_STATUS_PATH) $(REPORT_OVERVIEW_LIVE_STATUS_MACHINE_PATH); \
	print_paths "start: market context:" $(REPORT_MARKET_OVERVIEW_PATH) $(REPORT_MARKET_OVERVIEW_JSON); \
	print_paths "start: after close:" $(REPORT_OVERVIEW_RETROSPECTIVE_PATH) $(REPORT_OVERVIEW_RETROSPECTIVE_MACHINE_PATH); \
	print_paths "start: research wrap-up:" $(REPORT_RESEARCH_SUMMARY_PATH) $(REPORT_RESEARCH_SUMMARY_JSON); \
	print_paths "overview pages: monitoring:" $(REPORT_OVERVIEW_MONITORING_PATHS); \
	print_paths "overview pages: review:" $(REPORT_OVERVIEW_REVIEW_PATHS); \
	print_paths "overview JSON: monitoring:" $(REPORT_OVERVIEW_MONITORING_MACHINE_PATHS); \
	print_paths "overview JSON: review:" $(REPORT_OVERVIEW_REVIEW_MACHINE_PATHS); \
	print_paths "latest: monitoring:" $(WORKFLOW_LATEST_MONITORING_OUTPUT_PATHS); \
	print_paths "machine: monitoring:" $(WORKFLOW_LATEST_MONITORING_MACHINE_PATHS); \
	print_paths "latest: review:" $(WORKFLOW_LATEST_REVIEW_OUTPUT_PATHS); \
	print_paths "machine: review:" $(WORKFLOW_LATEST_REVIEW_MACHINE_PATHS); \
	print_paths "summaries: monitoring:" $(WORKFLOW_SUMMARY_MONITORING_PATHS); \
	print_paths "summaries: review:" $(WORKFLOW_SUMMARY_REVIEW_PATHS); \
	print_paths "structured: monitoring:" $(WORKFLOW_STRUCTURED_MONITORING_PATHS); \
	print_paths "structured: review:" $(WORKFLOW_STRUCTURED_REVIEW_PATHS); \
	print_paths "archive: monitoring:" $(WORKFLOW_ARCHIVE_MONITORING_PATHS); \
	print_paths "archive machine: monitoring:" $(WORKFLOW_ARCHIVE_MONITORING_MACHINE_PATHS); \
	print_paths "archive: review:" $(WORKFLOW_ARCHIVE_REVIEW_PATHS); \
	print_paths "archive machine: review:" $(WORKFLOW_ARCHIVE_REVIEW_MACHINE_PATHS); \
	print_paths "scan open this first:" $(SCAN_RECOMMENDED_OUTPUT_PATH); \
	print_paths "scan machine:" $(SCAN_RECOMMENDED_MACHINE_OUTPUT_PATH); \
	print_paths "scan summaries:" $(SCAN_HUMAN_OUTPUT_PATHS); \
	print_paths "scan structured:" $(SCAN_MACHINE_OUTPUT_PATHS); \
	print_paths "scan focus summaries:" $(SCAN_FOCUS_HUMAN_OUTPUT_PATHS); \
	print_paths "scan focus structured:" $(SCAN_FOCUS_MACHINE_OUTPUT_PATHS); \
	print_paths "scan archive:" $(SCAN_HISTORY_OUTPUT_PATHS); \
	print_paths "scan archive start:" $(SCAN_HISTORY_RECOMMENDED_OUTPUT_PATH); \
	print_paths "scan archive machine:" $(SCAN_HISTORY_RECOMMENDED_MACHINE_OUTPUT_PATH); \
	print_paths "portfolio open this first:" $(PORTFOLIO_RECOMMENDED_OUTPUT_PATH); \
	print_paths "portfolio machine:" $(PORTFOLIO_RECOMMENDED_MACHINE_OUTPUT_PATH); \
	print_paths "portfolio summaries:" $(PORTFOLIO_HUMAN_OUTPUT_PATHS); \
	print_paths "portfolio structured:" $(PORTFOLIO_MACHINE_OUTPUT_PATHS); \
	print_paths "portfolio archive:" $(PORTFOLIO_HISTORY_OUTPUT_PATHS); \
	print_paths "portfolio archive start:" $(PORTFOLIO_HISTORY_RECOMMENDED_OUTPUT_PATH); \
	print_paths "portfolio archive machine:" $(PORTFOLIO_HISTORY_RECOMMENDED_MACHINE_OUTPUT_PATH); \
	print_paths "dataset open this first:" $(DATASET_RECOMMENDED_OUTPUT_PATH); \
	print_paths "dataset machine:" $(DATASET_RECOMMENDED_MACHINE_OUTPUT_PATH); \
	print_paths "dataset summaries:" $(DATASET_HUMAN_OUTPUT_PATHS); \
	print_paths "dataset structured:" $(DATASET_MACHINE_OUTPUT_PATHS); \
	print_paths "dataset archive:" $(DATASET_HISTORY_OUTPUT_PATHS); \
	print_paths "dataset archive start:" $(DATASET_HISTORY_RECOMMENDED_OUTPUT_PATH); \
	print_paths "dataset archive machine:" $(DATASET_HISTORY_RECOMMENDED_MACHINE_OUTPUT_PATH); \
	print_paths "model open this first:" $(MODEL_RECOMMENDED_OUTPUT_PATH); \
	print_paths "model machine:" $(MODEL_RECOMMENDED_MACHINE_OUTPUT_PATH); \
	print_paths "model summaries:" $(MODEL_HUMAN_OUTPUT_PATHS); \
	print_paths "model structured:" $(MODEL_MACHINE_OUTPUT_PATHS); \
	print_paths "model archive:" $(MODEL_HISTORY_OUTPUT_PATHS)

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
