package store

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func EnsureSQLiteDB(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return Exec(path, schemaSQL)
}

func Exec(path string, query string, args ...any) error {
	cmd := exec.Command("sqlite3", "-bail", path, query)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return wrapError(err, output)
	}
	return nil
}

func QueryString(path string, query string, args ...any) (string, error) {
	cmd := exec.Command("sqlite3", "-bail", "-noheader", path, query)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", wrapError(err, output)
	}
	return strings.TrimRight(string(output), "\n"), nil
}

func ExecTx(path string, statements ...string) error {
	filtered := make([]string, 0, len(statements))
	for _, statement := range statements {
		if strings.TrimSpace(statement) != "" {
			filtered = append(filtered, statement)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	script := "BEGIN IMMEDIATE;\n" + strings.Join(filtered, "\n") + "\nCOMMIT;"
	return Exec(path, script)
}

func wrapError(err error, output []byte) error {
	message := strings.TrimSpace(string(output))
	if message == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, message)
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS execution_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    strategy_name TEXT NOT NULL,
    status TEXT NOT NULL,
    note TEXT NOT NULL,
    executed_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS signal_records (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    strategy_name TEXT NOT NULL,
    symbol TEXT NOT NULL,
    signal TEXT NOT NULL,
    reason TEXT NOT NULL,
    short_ma REAL NOT NULL,
    long_ma REAL NOT NULL,
    open_price REAL NOT NULL,
    high_price REAL NOT NULL,
    low_price REAL NOT NULL,
    close_price REAL NOT NULL,
    volume REAL NOT NULL,
    position_size INTEGER NOT NULL,
    decided_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS position_state (
    symbol TEXT PRIMARY KEY,
    side TEXT NOT NULL,
    quantity INTEGER NOT NULL,
    entry_price REAL NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS run_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_type TEXT NOT NULL,
    git_commit TEXT NOT NULL,
    generated_at TEXT NOT NULL,
    history_dir TEXT NOT NULL,
    summary_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS experiment_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    experiment_type TEXT NOT NULL,
    git_commit TEXT NOT NULL,
    recorded_at TEXT NOT NULL,
    config_json TEXT NOT NULL,
    metrics_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS dashboard_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    generated_at TEXT NOT NULL,
    summary_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS simulated_account_ledger (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    snapshot_date TEXT NOT NULL,
    market TEXT NOT NULL,
    regime TEXT NOT NULL,
    exposure REAL NOT NULL,
    equity REAL NOT NULL,
    cash REAL NOT NULL,
    holdings_json TEXT NOT NULL,
    note TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS paper_accounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    market TEXT NOT NULL,
    mode TEXT NOT NULL,
    status TEXT NOT NULL,
    active_strategy TEXT NOT NULL,
    cash REAL NOT NULL,
    equity REAL NOT NULL,
    last_market_date TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    note TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS paper_positions (
    account_id INTEGER NOT NULL,
    symbol TEXT NOT NULL,
    name TEXT NOT NULL,
    shares INTEGER NOT NULL,
    entry_price REAL NOT NULL,
    entry_date TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY(account_id, symbol)
);

CREATE TABLE IF NOT EXISTS paper_orders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    symbol TEXT NOT NULL,
    name TEXT NOT NULL,
    side TEXT NOT NULL,
    order_type TEXT NOT NULL,
    quantity INTEGER NOT NULL,
    order_price REAL NOT NULL,
    status TEXT NOT NULL,
    placed_at TEXT NOT NULL,
    note TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS paper_fills (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id INTEGER,
    account_id INTEGER NOT NULL,
    symbol TEXT NOT NULL,
    name TEXT NOT NULL,
    side TEXT NOT NULL,
    quantity INTEGER NOT NULL,
    fill_price REAL NOT NULL,
    fee REAL NOT NULL,
    filled_at TEXT NOT NULL,
    note TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS paper_equity_curve (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    snapshot_time TEXT NOT NULL,
    market_date TEXT NOT NULL,
    market TEXT NOT NULL,
    equity REAL NOT NULL,
    cash REAL NOT NULL,
    holdings_json TEXT NOT NULL,
    note TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS strategy_registry (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    market TEXT NOT NULL,
    version_name TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL,
    parent_version TEXT NOT NULL,
    git_commit TEXT NOT NULL,
    config_json TEXT NOT NULL,
    model_path TEXT NOT NULL,
    created_at TEXT NOT NULL,
    activated_at TEXT NOT NULL,
    archived_at TEXT NOT NULL,
    notes TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS strategy_promotions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type TEXT NOT NULL,
    market TEXT NOT NULL,
    from_version TEXT NOT NULL,
    to_version TEXT NOT NULL,
    trigger_reason TEXT NOT NULL,
    metrics_json TEXT NOT NULL,
    recorded_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS paper_daily_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL,
    strategy_version TEXT NOT NULL,
    mode TEXT NOT NULL,
    market TEXT NOT NULL,
    market_date TEXT NOT NULL,
    equity REAL NOT NULL,
    cash REAL NOT NULL,
    holding_count INTEGER NOT NULL,
    order_count INTEGER NOT NULL,
    fill_count INTEGER NOT NULL,
    session TEXT NOT NULL,
    recorded_at TEXT NOT NULL,
    note TEXT NOT NULL
);`
