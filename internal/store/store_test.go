package store

import (
	"path/filepath"
	"testing"
)

func TestEnsureExecAndQueryString(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "quant.db")
	if err := EnsureSQLiteDB(dbPath); err != nil {
		t.Fatalf("EnsureSQLiteDB() error = %v", err)
	}
	if err := Exec(dbPath, "INSERT INTO execution_records (strategy_name, status, note, executed_at) VALUES ('demo', 'ok', 'note', '2026-03-20T10:00:00Z')"); err != nil {
		t.Fatalf("Exec() error = %v", err)
	}

	got, err := QueryString(dbPath, "SELECT strategy_name, status FROM execution_records ORDER BY id DESC LIMIT 1")
	if err != nil {
		t.Fatalf("QueryString() error = %v", err)
	}
	if got != "demo|ok" {
		t.Fatalf("QueryString() = %q", got)
	}
}

func TestExecTx(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "quant.db")
	if err := EnsureSQLiteDB(dbPath); err != nil {
		t.Fatalf("EnsureSQLiteDB() error = %v", err)
	}
	if err := ExecTx(
		dbPath,
		"INSERT INTO execution_records (strategy_name, status, note, executed_at) VALUES ('tx-demo', 'ok', 'note', '2026-03-20T10:00:00Z');",
		"INSERT INTO signal_records (strategy_name, symbol, signal, reason, short_ma, long_ma, open_price, high_price, low_price, close_price, volume, position_size, decided_at) VALUES ('tx-demo', 'IBM', 'BUY', 'reason', 1, 2, 3, 4, 5, 6, 7, 8, '2026-03-20T10:00:00Z');",
	); err != nil {
		t.Fatalf("ExecTx() error = %v", err)
	}

	got, err := QueryString(dbPath, "SELECT COUNT(*) FROM execution_records;")
	if err != nil {
		t.Fatalf("QueryString() error = %v", err)
	}
	if got != "1" {
		t.Fatalf("execution_records count = %q", got)
	}

	got, err = QueryString(dbPath, "SELECT COUNT(*) FROM signal_records;")
	if err != nil {
		t.Fatalf("QueryString() error = %v", err)
	}
	if got != "1" {
		t.Fatalf("signal_records count = %q", got)
	}
}
