# quant-mvp
Minimal Go-based semi-quant trading system.

Current MVP behavior:
- loads config from `configs/config.yaml`
- fetches daily prices from Alpha Vantage when `ALPHAVANTAGE_API_KEY` is set
- falls back to local CSV data at `data/market_data.csv` when remote fetching is unavailable
- writes execution records to SQLite at `data/quant.db`
- reads OHLCV market data from `data/market_data.csv`
- calculates a short/long moving-average crossover signal
- applies max-position, stop-loss, and repeat-signal filtering
- stores signal records, execution records, and current position state in SQLite
- runs one execution on startup
- waits for the next configured cron time and runs again
