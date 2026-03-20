# quant-mvp
Minimal Go-based semi-quant trading system.

Documentation:
- `docs/product_design.md`
- `docs/development_guide.md`
- `docs/user_guide.md`
- `docs/research_implementation_plan.md`
- `docs/research_platform_plan.md`
- `docs/auto_evolution_design.md`

Current behavior:
- loads layered config from:
  - `configs/config.yaml`
  - `configs/data.yaml`
  - `configs/portfolio.yaml`
  - `configs/model.yaml`
  - `configs/report.yaml`
- auto-selects the market data provider by symbol
- supports US equities through Alpha Vantage
- supports A-share symbols such as `000001`, `001696`, `600519`, `688041`, and `430047` through Tushare Pro with BaoStock fallback
- falls back to local CSV data at `data/market_data.csv` when remote fetching is unavailable
- writes execution records to SQLite at `data/quant.db`
- reads OHLCV market data from `data/market_data.csv`
- calculates a short/long moving-average crossover signal
- applies max-position, stop-loss, and repeat-signal filtering
- labels each run as `live` or `test` mode based on the active data source
- prints a readable next-day trading plan after every run
- writes `reports/latest_plan.txt` and `reports/latest_plan.html` after every run
- can scan the full A-share universe with `--scan-a-share --top 10`
- writes A-share scan reports grouped into `建议关注 / 观望 / 回避` at `reports/a_share_scan.txt` and `reports/a_share_scan.html`
- applies extra scan filters for ST names, low liquidity, trend strength, and stop-line breaks
- writes a focus-only shortlist to `reports/a_share_focus.txt` and `reports/a_share_focus.html`
- uses Tushare Pro for both the A-share universe list and daily bars whenever available
- reads the A-share scan universe from `data/a_share_universe.csv`, so you can control which symbols are scanned
- supports single-symbol backtests with `--backtest --symbol 001696 --from 2025-01-01 --to 2026-03-18 --cash 100000`
- writes backtest reports to `reports/backtest_latest.txt` and `reports/backtest_latest.html`
- supports batch backtests over `data/a_share_universe.csv` with `--backtest-scan --from 2025-01-01 --to 2026-03-18 --top 10`
- supports portfolio backtests with `--portfolio-backtest --from 2025-01-01 --to 2026-03-18 --top 5`
- backtests support `--fee-bps` and `--slippage-bps`
- backtests now include annualized return, buy-and-hold benchmark return, benchmark drawdown, and excess return
- writes a structured batch backtest snapshot to `reports/backtest_scan.csv`
- writes JSON exports for plan / scan / focus / backtest / portfolio / grid search / dashboard
- enriches A-share scan and focus reports with the latest batch backtest snapshot when available
- writes portfolio backtest reports to `reports/portfolio_backtest.txt` and `reports/portfolio_backtest.html`
- exports model-ready factor datasets with `--export-dataset --from 2025-01-01 --to 2026-03-18`
- includes a zero-dependency Python training script at `scripts/train_model.py`
- archives report snapshots under `reports/history/YYYY-MM-DD/<run-type>/`
- appends unified run metadata to `reports/run_index.jsonl`
- appends parameter experiments to `reports/experiments.jsonl` and `reports/experiments.csv`
- writes diagnostics to `reports/diagnostics.txt` and `reports/diagnostics.json`
- writes history comparison and market overview pages:
  - `reports/history_compare.html`
  - `reports/market_overview.html`
- persists run / experiment / dashboard / simulated account history into SQLite
- auto-cleans old history/model artifacts based on `report.cleanup_keep_days`
- stores signal records, execution records, and current position state in SQLite
- runs one execution on startup
- waits for the next configured cron time and runs again

Daily workflow:
```bash
cd /Users/yangrenqing/Downloads/quant-mvp
bash scripts/daily_run.sh
```

Useful flags:
```bash
bash scripts/daily_run.sh --from 2025-01-01 --to 2026-03-19
bash scripts/daily_run.sh --skip-model
bash scripts/daily_run.sh --skip-health
bash scripts/daily_run.sh --skip-factor
bash scripts/daily_run.sh --scan-only
```

Make workflow:
```bash
cd /Users/yangrenqing/Downloads/quant-mvp
make help
make scan
make portfolio
make dataset
make model
make show-output-paths
make validate-config
make export-runtime-config
make show-check-paths
make quick-check
make verify
make daily
```

`make help` now keeps the `make show-output-paths` summary intentionally short; run `make show-output-paths` itself for the detailed operator guidance.

`make show-output-paths` prints all expected current generated report/artifact paths after `make scan`, `make portfolio`, `make dataset`, or `make model`, and it now starts the overview section with one best high-level HTML page to open first for quick human scanning, `reports/dashboard.html`, paired with its matching first machine-readable JSON entry point, `reports/dashboard.json`, for automation or downstream tooling. Across the broad overview set, `reports/dashboard.html` plus `reports/market_overview.html` are framed as the quick operational monitoring pages, while `reports/history_compare.html` plus `reports/research_summary.html` are framed as the deeper narrative/review pages; the helper keeps those overview blocks and workflow pairing lines on shorter grouped labels, and the explanatory note directly above them carries the shared HTML-vs-JSON and same-order pairing guidance. For the explicit broad-overview start-here intents, the best starting page is paired with its closest structured counterpart: during the trading day, start with `reports/dashboard.html` plus `reports/dashboard.json`; for market context, start with `reports/market_overview.html` plus `reports/market_overview.json`; after the close, start with `reports/history_compare.html` plus `reports/history_compare.json`; for research wrap-up, start with `reports/research_summary.html` plus `reports/research_summary.json`. Use that overview-first pair and the full broad overview set when you want cross-workflow status/context; use a workflow-specific `open this first` path when you are actively monitoring one workflow or checking it immediately after that workflow completes, whether that means the latest output under `reports/` or the timestamped archive trail for one run. Before the per-workflow details, the workflow-specific area now includes one compact grouped latest-output view that separates quick operational monitoring (`reports/a_share_scan.html` and `reports/portfolio_backtest.html`) from deeper review (`reports/training_dataset.csv` and `reports/model_pipeline_latest.txt`). Within each workflow-specific section, keep the single `open this first` path as the first place to check immediately after completion, but read the latest scan/portfolio paths as quick operational monitoring entry points and the latest dataset/model paths as deeper review entry points; the next line under each latest path and each archived first-open path shows the closest machine-readable companion (for example scan HTML → scan JSON, portfolio HTML → portfolio JSON, dataset CSV → dataset JSON, and model text summary → predictions CSV); use the `summary views` plus `structured data/model files` lists when you want deeper follow-up or automation inputs. It keeps that overview-first pair alongside the full set of broad overview entry points split into quick operational monitoring (`reports/dashboard.html` and `reports/market_overview.html`) versus deeper narrative/review (`reports/history_compare.html` and `reports/research_summary.html`) and the matching machine-readable JSON companions for automation/downstream tooling (`reports/dashboard.json`, `reports/market_overview.json`, `reports/history_compare.json`, and `reports/research_summary.json`), with the HTML and JSON broad overview lists kept in the same order for quick visual pairing so the first HTML line matches the first JSON line and the second HTML line matches the second JSON line, plus the most relevant per-run history/archive location to inspect next, with each archived first-open path now also paired with its closest machine-readable companion, marking each current path, archive location, or overview artifact as currently `present` or `missing` on disk. It groups each workflow into compact operator buckets: summary views versus structured data/model files, while still highlighting one separate `open this first` default path per workflow so operators can distinguish the best first inspection target from the rest of the output set: `reports/a_share_scan.html` for `make scan` and `reports/portfolio_backtest.html` for `make portfolio` as quick operational monitoring latest paths, plus `reports/training_dataset.csv` for `make dataset` and `reports/model_pipeline_latest.txt` for `make model` as deeper review latest paths. The current/latest `open this first` path and the history/archive `open this first` file are chosen independently, so a format mismatch is intentional when it appears: the latest view under `reports/` may default to HTML, while the archive entry for that workflow may default to HTML or CSV/text depending on which artifact is most useful to inspect first for that archived run. For `make scan`, the scan section includes both the main scan report set (`reports/a_share_scan.txt`, `reports/a_share_scan.html`, `reports/a_share_scan.json`), the focus-only shortlist outputs produced alongside it (`reports/a_share_focus.txt`, `reports/a_share_focus.html`, `reports/a_share_focus.json`), the timestamped archive pattern `reports/history/YYYY-MM-DD/a_share_scan/`, and the recommended first file inside that archive pattern: `reports/history/YYYY-MM-DD/a_share_scan/a_share_scan.html`, plus its closest machine-readable companion `reports/history/YYYY-MM-DD/a_share_scan/a_share_scan.json`. For `make portfolio`, the portfolio section labels the text/HTML outputs as summary views, the JSON/CSV companions as structured data/model files, the timestamped archive pattern `reports/history/YYYY-MM-DD/portfolio_backtest/`, and the recommended first archive file `reports/history/YYYY-MM-DD/portfolio_backtest/portfolio_backtest.html`, plus its closest machine-readable companion `reports/history/YYYY-MM-DD/portfolio_backtest/portfolio_backtest.json`. For `make dataset`, it labels the text summary as a summary view, the CSV/JSON exports as structured data/model files, the timestamped archive pattern `reports/history/YYYY-MM-DD/training_dataset/`, and the recommended first archive file `reports/history/YYYY-MM-DD/training_dataset/training_dataset.csv`, plus its closest machine-readable companion `reports/history/YYYY-MM-DD/training_dataset/training_dataset.json`. For `make model`, the current/latest section under `reports/` labels the human-readable summaries (`reports/model_pipeline_latest.txt`, `reports/model_train.txt`, and `reports/benchmark_classifier.txt`) as summary views and the workflow's generated CSV/JSON/JSONL outputs (`reports/model_predictions.csv`, `reports/benchmark_classifier_predictions.csv`, `reports/linear_model.json`, `reports/benchmark_classifier.json`, and `reports/model_registry.jsonl`) as structured data/model files, while still keeping the workflow's per-run history explicit under `reports/model_versions/`. Use it right after one of those workflow targets finishes when you want the single best overview HTML page to open first for quick human scanning with its matching machine-readable JSON entry point for automation/downstream tooling, the broader overview snapshot split into quick operational monitoring versus deeper narrative/review pages, the quick-scan HTML list plus matching same-order JSON list for automation/downstream tooling, the grouped workflow-specific latest-output view before the per-workflow details, the workflow-specific latest-output entry point for quick operational monitoring in scan/portfolio or deeper review in dataset/model plus the first file to check immediately after completion, or the corresponding timestamped archive trail for a single run including the best first file inside that archive and its closest machine-readable companion.

`make validate-config` runs only layered runtime config validation.

`make export-runtime-config` writes the resolved runtime config snapshot to `reports/runtime_config.json` and exits. `make show-check-paths` prints the same output path used by that export, along with the layered config inputs, the layered config load order, which layered config files from that order are currently present versus absent on disk, and the optional final override file (`configs/local.yaml` when present).

`make show-check-paths` prints the resolved Go cache path, Python bytecode cache path, the layered config inputs and load order used to build runtime config, separate lists showing which files from that load order are currently present versus absent on disk, the optional final override file applied last when `configs/local.yaml` exists, the shell scripts syntax-checked by `make quick-check`, the runtime config snapshot path refreshed by `make export-runtime-config`, and the most useful follow-up artifact/output to inspect after `make validate-config`, `make export-runtime-config`, `make quick-check`, and `make verify`. Use it when troubleshooting these check-oriented targets, especially when you need to distinguish configured load order from files currently found and quickly see where to look next after a check runs.

`make quick-check` is the fast local preflight: it runs shell syntax checks for the scripts listed by `make show-check-paths`, Python bytecode compilation, and then `make validate-config`. `make show-check-paths` also points you at the follow-up console output to inspect after `make quick-check`.

`make verify` is the broader local preflight: it runs Go tests and then `make quick-check`. `make show-check-paths` also points you at the follow-up console output to inspect after `make verify`.

Go-based `make` targets default `GOCACHE` to the repo-local `.cache/go-build`, and Python bytecode checks default `PYTHONPYCACHEPREFIX` to the repo-local `.cache/python`. Override either variable explicitly if you want a different cache path.

Model workflow:
```bash
cd /Users/yangrenqing/Downloads/quant-mvp
go run ./cmd/scheduler --export-dataset --from 2025-01-01 --to 2026-03-18
python3 scripts/train_model.py --dataset reports/training_dataset.csv --label label_10d
python3 scripts/train_classifier.py --dataset reports/training_dataset.csv --label beat_benchmark_10d
python3 scripts/model_pipeline.py --from 2025-01-01 --to 2026-03-18 --label label_10d
```

Key outputs:
- `reports/linear_model.json`
- `reports/benchmark_classifier.json`
- `reports/model_train.txt`
- `reports/model_predictions.csv`
- `reports/model_pipeline_latest.txt`
- `reports/model_registry.jsonl`
- `reports/model_versions/<timestamp>/`
- `reports/dashboard.html`
- `reports/health_monitor.html`
- `reports/factor_research.html`
- `reports/factor_diagnostics.html`
- `reports/model_comparison.html`
- `reports/strategy_quality.html`
- `reports/research_summary.html`
- `reports/history_compare.html`
- `reports/market_overview.html`
- `reports/history/YYYY-MM-DD/...`
- `reports/run_index.jsonl`
- `reports/experiments.jsonl`

Intraday paper service:
```bash
cd /Users/yangrenqing/Downloads/quant-mvp
bash scripts/intraday_run.sh
```

Health and factor research:
```bash
cd /Users/yangrenqing/Downloads/quant-mvp
python3 scripts/health_monitor.py
python3 scripts/factor_research.py --dataset reports/training_dataset.csv --label label_10d
python3 scripts/factor_diagnostics.py --dataset reports/training_dataset.csv
python3 scripts/model_comparison.py
python3 scripts/strategy_quality.py
python3 scripts/research_summary.py
```

Research workspace:
```bash
cd /Users/yangrenqing/Downloads/quant-mvp
bash scripts/research_run.sh
```

Background automation:
```bash
cd /Users/yangrenqing/Downloads/quant-mvp
bash scripts/install_launchd.sh
launchctl list | rg quant-mvp
```
