# quant-mvp
Minimal Go-based semi-quant trading system.

Current MVP behavior:
- loads config from `configs/config.yaml`
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
- enriches A-share scan and focus reports with the latest batch backtest snapshot when available
- writes portfolio backtest reports to `reports/portfolio_backtest.txt` and `reports/portfolio_backtest.html`
- exports model-ready factor datasets with `--export-dataset --from 2025-01-01 --to 2026-03-18`
- includes a zero-dependency Python training script at `scripts/train_model.py`
- stores signal records, execution records, and current position state in SQLite
- runs one execution on startup
- waits for the next configured cron time and runs again

Model workflow:
```bash
cd /Users/yangrenqing/Downloads/quant-mvp
go run ./cmd/scheduler --export-dataset --from 2025-01-01 --to 2026-03-18
python3 scripts/train_model.py --dataset reports/training_dataset.csv --label label_10d
```

Training outputs:
- `reports/linear_model.json`
- `reports/model_train.txt`
- `reports/model_predictions.csv`
