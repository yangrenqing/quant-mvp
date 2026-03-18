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
- stores signal records, execution records, and current position state in SQLite
- runs one execution on startup
- waits for the next configured cron time and runs again
