package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMergesAndAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), "app:\n  name: quant-mvp-test\nschedule:\n  daily_run: \"30 15 * * 1-5\"\n")
	writeFile(t, filepath.Join(dir, "data.yaml"), "db:\n  path: data/custom.db\nstrategy:\n  symbol: 600519\n  short_window: 5\n  long_window: 9\n")
	writeFile(t, filepath.Join(dir, "model.yaml"), "model:\n  shadow_version: candidate_x\nhealth:\n  max_run_age_hours: 12\n")

	cfg, err := Load(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.AppName != "quant-mvp-test" {
		t.Fatalf("AppName = %q", cfg.AppName)
	}
	if cfg.DB.Path != "data/custom.db" {
		t.Fatalf("DB.Path = %q", cfg.DB.Path)
	}
	if cfg.Strategy.Symbol != "600519" || cfg.Strategy.ShortWindow != 5 || cfg.Strategy.LongWindow != 9 {
		t.Fatalf("unexpected strategy config: %+v", cfg.Strategy)
	}
	if cfg.Model.ShadowVersion != "candidate_x" {
		t.Fatalf("ShadowVersion = %q", cfg.Model.ShadowVersion)
	}
	if cfg.Health.MaxRunAgeHours != 12 {
		t.Fatalf("MaxRunAgeHours = %v", cfg.Health.MaxRunAgeHours)
	}
	if cfg.Model.DefaultLabel != "label_10d" {
		t.Fatalf("DefaultLabel = %q", cfg.Model.DefaultLabel)
	}
}

func TestLoadRejectsInvalidWindows(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), "schedule:\n  daily_run: \"30 15 * * 1-5\"\nstrategy:\n  short_window: 9\n  long_window: 9\n")

	if _, err := Load(filepath.Join(dir, "config.yaml")); err == nil {
		t.Fatal("expected invalid short/long window error")
	}
}

func TestLoadDefaultsPortfolioMaxCashShareWhenOmitted(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), "schedule:\n  daily_run: \"30 15 * * 1-5\"\nportfolio:\n  rebalance_interval_days: 5\n")

	cfg, err := Load(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Portfolio.MaxCashShare != 0.20 {
		t.Fatalf("MaxCashShare = %v, want 0.20", cfg.Portfolio.MaxCashShare)
	}
}

func TestLoadLocalOverridesEarlierLayers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), "schedule:\n  daily_run: \"30 15 * * 1-5\"\nstrategy:\n  short_window: 3\n  long_window: 5\n")
	writeFile(t, filepath.Join(dir, "data.yaml"), "db:\n  path: data/from-data.db\n")
	writeFile(t, filepath.Join(dir, "portfolio.yaml"), "portfolio:\n  max_cash_share: 0.35\n")
	writeFile(t, filepath.Join(dir, "report.yaml"), "report:\n  history_root: reports/from-report\n")
	writeFile(t, filepath.Join(dir, "local.yaml"), "db:\n  path: data/from-local.db\nportfolio:\n  max_cash_share: 0.10\nreport:\n  history_root: reports/from-local\n")

	cfg, err := Load(filepath.Join(dir, "config.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DB.Path != "data/from-local.db" {
		t.Fatalf("DB.Path = %q, want local override", cfg.DB.Path)
	}
	if cfg.Portfolio.MaxCashShare != 0.10 {
		t.Fatalf("MaxCashShare = %v, want local override", cfg.Portfolio.MaxCashShare)
	}
	if cfg.Report.HistoryRoot != "reports/from-local" {
		t.Fatalf("HistoryRoot = %q, want local override", cfg.Report.HistoryRoot)
	}
}

func TestWriteRuntimeSnapshotCreatesParentsAndFormatsJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "runtime", "snapshot.json")
	cfg := Config{
		AppName: "quant-mvp-test",
		App: AppConfig{
			Name: "quant-mvp-test",
		},
		DB: DBConfig{
			Path: "data/custom.db",
		},
	}

	if err := WriteRuntimeSnapshot(path, cfg); err != nil {
		t.Fatalf("WriteRuntimeSnapshot() error = %v", err)
	}

	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("parent directory not created: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}

	want, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}
	want = append(want, '\n')

	if string(got) != string(want) {
		t.Fatalf("snapshot contents = %q, want %q", got, want)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
