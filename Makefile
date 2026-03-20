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
REPORT_OVERVIEW_RETROSPECTIVE_PATH ?= $(REPORT_HISTORY_OVERVIEW_PATH)
REPORT_OVERVIEW_MONITORING_PATHS ?= $(REPORT_DASHBOARD_OVERVIEW_PATH) $(REPORT_MARKET_OVERVIEW_PATH)
REPORT_OVERVIEW_MONITORING_MACHINE_PATHS ?= $(REPORT_DASHBOARD_OVERVIEW_JSON) $(REPORT_MARKET_OVERVIEW_JSON)
REPORT_OVERVIEW_REVIEW_PATHS ?= $(REPORT_HISTORY_OVERVIEW_PATH) $(REPORT_RESEARCH_SUMMARY_PATH)
REPORT_OVERVIEW_REVIEW_MACHINE_PATHS ?= $(REPORT_HISTORY_OVERVIEW_JSON) $(REPORT_RESEARCH_SUMMARY_JSON)
REPORT_OVERVIEW_ENTRY_PATHS ?= $(REPORT_DASHBOARD_OVERVIEW_PATH) $(REPORT_MARKET_OVERVIEW_PATH) $(REPORT_HISTORY_OVERVIEW_PATH) $(REPORT_RESEARCH_SUMMARY_PATH)
REPORT_OVERVIEW_MACHINE_PATHS ?= $(REPORT_DASHBOARD_OVERVIEW_JSON) $(REPORT_MARKET_OVERVIEW_JSON) $(REPORT_HISTORY_OVERVIEW_JSON) $(REPORT_RESEARCH_SUMMARY_JSON)
REPORT_HISTORY_DATE_PATTERN ?= $(REPORT_HISTORY_DIR)/YYYY-MM-DD
ARCHIVE_ENTRY_FORMAT_NOTE ?= scan/portfolio archive entry files default to HTML for quick visual review; dataset archive entry files default to CSV because the export is data-first.
OVERVIEW_ENTRY_USE_NOTE ?= within the broad overview set, use $(REPORT_DASHBOARD_OVERVIEW_PATH) and $(REPORT_MARKET_OVERVIEW_PATH) for quick operational monitoring, and use $(REPORT_HISTORY_OVERVIEW_PATH) and $(REPORT_RESEARCH_SUMMARY_PATH) for deeper narrative/review; the broad overview HTML lists are for quick human scanning, the matching JSON lists are for automation or downstream tooling, and each JSON list stays in the same order as its HTML list so operators can pair them visually line for line (first HTML with first JSON, second HTML with second JSON); during the trading day, start with $(REPORT_OVERVIEW_LIVE_STATUS_PATH); after the close, start with $(REPORT_OVERVIEW_RETROSPECTIVE_PATH); use a workflow-specific open-this-first path when you are actively monitoring one workflow or checking it immediately after that workflow completes.
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
SCAN_HUMAN_OUTPUT_PATHS ?= $(SCAN_REPORT_TEXT) $(SCAN_REPORT_HTML)
SCAN_MACHINE_OUTPUT_PATHS ?= $(SCAN_REPORT_JSON)
SCAN_FOCUS_SHORTLIST_OUTPUT_PATHS ?= $(SCAN_FOCUS_SHORTLIST_TEXT) $(SCAN_FOCUS_SHORTLIST_HTML) $(SCAN_FOCUS_SHORTLIST_JSON)
SCAN_FOCUS_HUMAN_OUTPUT_PATHS ?= $(SCAN_FOCUS_SHORTLIST_TEXT) $(SCAN_FOCUS_SHORTLIST_HTML)
SCAN_FOCUS_MACHINE_OUTPUT_PATHS ?= $(SCAN_FOCUS_SHORTLIST_JSON)
SCAN_HISTORY_OUTPUT_PATHS ?= $(REPORT_HISTORY_DATE_PATTERN)/a_share_scan
SCAN_HISTORY_RECOMMENDED_ENTRY_FILE ?= a_share_scan.html
SCAN_HISTORY_RECOMMENDED_OUTPUT_PATH ?= $(SCAN_HISTORY_OUTPUT_PATHS)/$(SCAN_HISTORY_RECOMMENDED_ENTRY_FILE)
PORTFOLIO_REPORT_TEXT ?= $(CURDIR)/reports/portfolio_backtest.txt
PORTFOLIO_REPORT_HTML ?= $(CURDIR)/reports/portfolio_backtest.html
PORTFOLIO_REPORT_JSON ?= $(CURDIR)/reports/portfolio_backtest.json
PORTFOLIO_REPORT_CSV ?= $(CURDIR)/reports/portfolio_backtest.csv
PORTFOLIO_RECOMMENDED_OUTPUT_PATH ?= $(PORTFOLIO_REPORT_HTML)
PORTFOLIO_PRIMARY_OUTPUT_PATHS ?= $(PORTFOLIO_REPORT_TEXT)
PORTFOLIO_COMPANION_OUTPUT_PATHS ?= $(PORTFOLIO_REPORT_HTML) $(PORTFOLIO_REPORT_JSON) $(PORTFOLIO_REPORT_CSV)
PORTFOLIO_OUTPUT_PATHS ?= $(PORTFOLIO_PRIMARY_OUTPUT_PATHS) $(PORTFOLIO_COMPANION_OUTPUT_PATHS)
PORTFOLIO_HUMAN_OUTPUT_PATHS ?= $(PORTFOLIO_REPORT_TEXT) $(PORTFOLIO_REPORT_HTML)
PORTFOLIO_MACHINE_OUTPUT_PATHS ?= $(PORTFOLIO_REPORT_JSON) $(PORTFOLIO_REPORT_CSV)
PORTFOLIO_HISTORY_OUTPUT_PATHS ?= $(REPORT_HISTORY_DATE_PATTERN)/portfolio_backtest
PORTFOLIO_HISTORY_RECOMMENDED_ENTRY_FILE ?= portfolio_backtest.html
PORTFOLIO_HISTORY_RECOMMENDED_OUTPUT_PATH ?= $(PORTFOLIO_HISTORY_OUTPUT_PATHS)/$(PORTFOLIO_HISTORY_RECOMMENDED_ENTRY_FILE)
DATASET_EXPORT_CSV ?= $(CURDIR)/reports/training_dataset.csv
DATASET_EXPORT_TEXT ?= $(CURDIR)/reports/training_dataset.txt
DATASET_EXPORT_JSON ?= $(CURDIR)/reports/training_dataset.json
DATASET_RECOMMENDED_OUTPUT_PATH ?= $(DATASET_EXPORT_CSV)
DATASET_PRIMARY_OUTPUT_PATHS ?= $(DATASET_EXPORT_TEXT)
DATASET_COMPANION_OUTPUT_PATHS ?= $(DATASET_EXPORT_CSV) $(DATASET_EXPORT_JSON)
DATASET_OUTPUT_PATHS ?= $(DATASET_PRIMARY_OUTPUT_PATHS) $(DATASET_COMPANION_OUTPUT_PATHS)
DATASET_HUMAN_OUTPUT_PATHS ?= $(DATASET_EXPORT_TEXT)
DATASET_MACHINE_OUTPUT_PATHS ?= $(DATASET_EXPORT_CSV) $(DATASET_EXPORT_JSON)
DATASET_HISTORY_OUTPUT_PATHS ?= $(REPORT_HISTORY_DATE_PATTERN)/training_dataset
DATASET_HISTORY_RECOMMENDED_ENTRY_FILE ?= training_dataset.csv
DATASET_HISTORY_RECOMMENDED_OUTPUT_PATH ?= $(DATASET_HISTORY_OUTPUT_PATHS)/$(DATASET_HISTORY_RECOMMENDED_ENTRY_FILE)
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
MODEL_PRIMARY_OUTPUT_PATHS ?= $(MODEL_PIPELINE_REPORT)
MODEL_COMPANION_OUTPUT_PATHS ?= $(MODEL_PREDICTIONS) $(MODEL_CLASSIFIER_PREDICTIONS) $(MODEL_REGRESSION_JSON) $(MODEL_CLASSIFIER_JSON) $(MODEL_REGISTRY_LOG)
MODEL_ADDITIONAL_SUMMARY_OUTPUT_PATHS ?= $(MODEL_TRAIN_REPORT) $(MODEL_CLASSIFIER_REPORT)
MODEL_OUTPUT_PATHS ?= $(MODEL_PRIMARY_OUTPUT_PATHS) $(MODEL_ADDITIONAL_SUMMARY_OUTPUT_PATHS) $(MODEL_COMPANION_OUTPUT_PATHS) $(MODEL_VERSIONS_DIR)
MODEL_LATEST_OUTPUT_PATHS ?= $(MODEL_PRIMARY_OUTPUT_PATHS) $(MODEL_ADDITIONAL_SUMMARY_OUTPUT_PATHS) $(MODEL_COMPANION_OUTPUT_PATHS)
MODEL_HUMAN_OUTPUT_PATHS ?= $(MODEL_PRIMARY_OUTPUT_PATHS) $(MODEL_ADDITIONAL_SUMMARY_OUTPUT_PATHS)
MODEL_MACHINE_OUTPUT_PATHS ?= $(MODEL_PREDICTIONS) $(MODEL_CLASSIFIER_PREDICTIONS) $(MODEL_REGRESSION_JSON) $(MODEL_CLASSIFIER_JSON) $(MODEL_REGISTRY_LOG)
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
	@echo "make show-output-paths # print $(REPORT_OVERVIEW_RECOMMENDED_OUTPUT_PATH) as the single best high-level overview HTML page to open first for quick human scanning with matching machine-readable $(REPORT_OVERVIEW_RECOMMENDED_MACHINE_OUTPUT_PATH) for automation/downstream tooling, make the best overview starting point explicit for during the trading day vs after the close, separate the broad overview set into quick operational monitoring pages vs deeper narrative/review pages, keep the broad overview HTML quick-scan lists and matching JSON automation lists in the same order for easy visual pairing, and keep the full overview set plus per-run history/archive inspection locations and workflow-specific latest outputs for scan, portfolio, dataset, and model, where scan/portfolio open-this-first latest paths are framed for quick operational monitoring and the immediate post-run check while dataset/model open-this-first latest paths are framed for deeper review and the immediate post-run check, whether each exists on disk, and when latest vs archive recommendations intentionally differ by format"
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
			if printf '%s\n' "$$path" | grep -q '/YYYY-MM-DD/'; then \
				history_root="$${path%%/YYYY-MM-DD/*}"; \
				history_suffix="$${path#*/YYYY-MM-DD/}"; \
				if [ -d "$$history_root" ] && find "$$history_root" -path "*/$$history_suffix" 2>/dev/null | grep -q .; then status="present"; else status="missing"; fi; \
			elif [ -e "$$path" ]; then status="present"; else status="missing"; fi; \
			printf '  [%s] %s\n' "$$status" "$$path"; \
		done; \
	}; \
	echo "note: show-output-paths starts with the single best high-level overview page and its matching machine-readable JSON companion, then calls out the best broad overview starting page for during the trading day vs after the close before listing the broader overview set for cross-workflow status/context; $(OVERVIEW_ENTRY_USE_NOTE)"; \
	echo "note: current/latest and history/archive 'open this first' recommendations are chosen independently, so their formats may intentionally differ by workflow (latest often HTML; archive entry may be HTML or CSV/text)"; \
	echo "note: in each workflow-specific section below, keep the single 'open this first' path as the first place to check immediately after that workflow completes; for scan and portfolio, that latest path is framed for quick operational monitoring, while dataset and model latest paths are framed for deeper review; use the summary views and structured data/model files lists for deeper follow-up or automation inputs, and use the history/archive paths when you need an archived run."; \
	print_paths "high-level overview open this first HTML page for quick human scanning + matching JSON for automation/downstream tooling:" $(REPORT_OVERVIEW_RECOMMENDED_OUTPUT_PATH) $(REPORT_OVERVIEW_RECOMMENDED_MACHINE_OUTPUT_PATH); \
	print_paths "broad overview open this first during the trading day (live status):" $(REPORT_OVERVIEW_LIVE_STATUS_PATH); \
	print_paths "broad overview open this first after the close (retrospective analysis):" $(REPORT_OVERVIEW_RETROSPECTIVE_PATH); \
	print_paths "broad overview HTML entry points for quick operational monitoring and quick human scanning (dashboard, market overview; pair line 1 with line 1 in the matching JSON list):" $(REPORT_OVERVIEW_MONITORING_PATHS); \
	print_paths "broad overview HTML entry points for deeper narrative/review and quick human scanning (history compare, research summary; pair line 1 with line 1 in the matching JSON list):" $(REPORT_OVERVIEW_REVIEW_PATHS); \
	print_paths "matching broad overview JSON companions for quick operational monitoring automation/downstream tooling, in the same order as the HTML list above for line-for-line visual pairing:" $(REPORT_OVERVIEW_MONITORING_MACHINE_PATHS); \
	print_paths "matching broad overview JSON companions for deeper narrative/review automation/downstream tooling, in the same order as the HTML list above for line-for-line visual pairing:" $(REPORT_OVERVIEW_REVIEW_MACHINE_PATHS); \
	print_paths "scan open this first path for quick operational monitoring and the immediate post-run check:" $(SCAN_RECOMMENDED_OUTPUT_PATH); \
	print_paths "scan summary views:" $(SCAN_HUMAN_OUTPUT_PATHS); \
	print_paths "scan structured data/model files:" $(SCAN_MACHINE_OUTPUT_PATHS); \
	print_paths "scan focus-only shortlist summary views:" $(SCAN_FOCUS_HUMAN_OUTPUT_PATHS); \
	print_paths "scan focus-only shortlist structured data/model files:" $(SCAN_FOCUS_MACHINE_OUTPUT_PATHS); \
	print_paths "scan timestamped history/archive pattern:" $(SCAN_HISTORY_OUTPUT_PATHS); \
	print_paths "scan history/archive open this first file:" $(SCAN_HISTORY_RECOMMENDED_OUTPUT_PATH); \
	print_paths "portfolio open this first path for quick operational monitoring and the immediate post-run check:" $(PORTFOLIO_RECOMMENDED_OUTPUT_PATH); \
	print_paths "portfolio summary views:" $(PORTFOLIO_HUMAN_OUTPUT_PATHS); \
	print_paths "portfolio structured data/model files:" $(PORTFOLIO_MACHINE_OUTPUT_PATHS); \
	print_paths "portfolio timestamped history/archive pattern:" $(PORTFOLIO_HISTORY_OUTPUT_PATHS); \
	print_paths "portfolio history/archive open this first file:" $(PORTFOLIO_HISTORY_RECOMMENDED_OUTPUT_PATH); \
	print_paths "dataset open this first path for deeper review and the immediate post-run check:" $(DATASET_RECOMMENDED_OUTPUT_PATH); \
	print_paths "dataset summary views:" $(DATASET_HUMAN_OUTPUT_PATHS); \
	print_paths "dataset structured data/model files:" $(DATASET_MACHINE_OUTPUT_PATHS); \
	print_paths "dataset timestamped history/archive pattern:" $(DATASET_HISTORY_OUTPUT_PATHS); \
	print_paths "dataset history/archive open this first file:" $(DATASET_HISTORY_RECOMMENDED_OUTPUT_PATH); \
	print_paths "model open this first path for deeper review and the immediate post-run check:" $(MODEL_RECOMMENDED_OUTPUT_PATH); \
	print_paths "model current/latest summary views:" $(MODEL_HUMAN_OUTPUT_PATHS); \
	print_paths "model current/latest structured data/model files:" $(MODEL_MACHINE_OUTPUT_PATHS); \
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
