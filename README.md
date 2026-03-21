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
make show-start-here
make show-output-paths
make validate-config
make export-runtime-config
make show-check-paths
make quick-check
make verify
make daily
```

`make show-start-here` is the lightweight companion to `make show-output-paths` when you only need the main entry paths.

`make show-output-paths` prints the current report and artifact paths after `make scan`, `make portfolio`, `make dataset`, or `make model`. It starts with the main HTML/JSON pair: `reports/dashboard.html` for quick human scanning and `reports/dashboard.json` for the matching machine-readable view.

Broad overview: quick monitoring (`reports/dashboard.html`, `reports/market_overview.html`) and deeper review (`reports/history_compare.html`, `reports/research_summary.html`).

Overview key:
- `start: ...` → first stop for the given intent
- `overview pages` → quick human scan
- `overview JSON` → automation view in the same order

Start:
- trading day → `reports/dashboard.html` + `reports/dashboard.json`
- market context → `reports/market_overview.html` + `reports/market_overview.json`
- after the close → `reports/history_compare.html` + `reports/history_compare.json`
- research wrap-up → `reports/research_summary.html` + `reports/research_summary.json`

Workflow groups: `latest`, `machine`, `summaries`, `structured`, and `archive`, split into quick monitoring vs deeper review.

Order: `open this first`, `machine`, `summaries`, `structured`, `archive`, `archive start here`, and `archive machine` when present. Here, `machine` means the closest machine-readable companion.

Workflow key:
- `open this first` → live monitoring or immediate post-run check
- `machine` → closest machine-readable companion
- `summaries` / `structured` → deeper review or automation
- archive → prior-run inspection

Latest vs archive:
- latest usually favors HTML
- archive may use HTML, CSV, or text by workflow
- scan, portfolio, and dataset archive start-here paths are paired with the closest machine-readable companion
- model uses a text summary for the latest human-readable entry, pairs it with `reports/model_predictions.csv` instead of JSON, and keeps history under `reports/model_versions/`

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
