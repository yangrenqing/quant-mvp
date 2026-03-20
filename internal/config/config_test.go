package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

	_, err := Load(filepath.Join(dir, "config.yaml"))
	if err == nil {
		t.Fatal("expected invalid short/long window error")
	}
	if !strings.Contains(err.Error(), "strategy.short_window must be smaller than strategy.long_window") {
		t.Fatalf("Load() error = %v, want explicit window validation message", err)
	}
}

func TestLoadRejectsMalformedConfigLine(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), "schedule\n  daily_run: \"30 15 * * 1-5\"\n")

	_, err := Load(filepath.Join(dir, "config.yaml"))
	if err == nil {
		t.Fatal("expected invalid config line error")
	}
	if !strings.Contains(err.Error(), "invalid config line") {
		t.Fatalf("Load() error = %v, want invalid config line", err)
	}
}

func TestLoadRejectsInvalidTypedValueWithFieldName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), "schedule:\n  daily_run: \"30 15 * * 1-5\"\nstrategy:\n  short_window: nope\n")

	_, err := Load(filepath.Join(dir, "config.yaml"))
	if err == nil {
		t.Fatal("expected invalid typed value error")
	}
	if !strings.Contains(err.Error(), "strategy.short_window") {
		t.Fatalf("Load() error = %v, want concrete field name", err)
	}
}

func TestLoadRejectsLayeredConfigMissingRequiredDailyRun(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), "app:\n  name: quant-mvp-test\nstrategy:\n  short_window: 3\n  long_window: 5\n")
	writeFile(t, filepath.Join(dir, "data.yaml"), "db:\n  path: data/custom.db\n")
	writeFile(t, filepath.Join(dir, "report.yaml"), "report:\n  history_root: reports/from-report\n")

	_, err := Load(filepath.Join(dir, "config.yaml"))
	if err == nil {
		t.Fatal("expected missing required schedule.daily_run error")
	}
	if !strings.Contains(err.Error(), "schedule.daily_run") {
		t.Fatalf("Load() error = %v, want missing required field name", err)
	}
}

func TestLoadRejectsUnknownSection(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), "schedule:\n  daily_run: \"30 15 * * 1-5\"\nunknown_section:\n  enabled: true\n")

	_, err := Load(filepath.Join(dir, "config.yaml"))
	if err == nil {
		t.Fatal("expected unknown section error")
	}
	if !strings.Contains(err.Error(), `unknown config section "unknown_section"`) {
		t.Fatalf("Load() error = %v, want unknown section name", err)
	}
}

func TestLoadRejectsUnknownKeyWithinKnownSection(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), "schedule:\n  daily_run: \"30 15 * * 1-5\"\n  unexpected_key: true\n")

	_, err := Load(filepath.Join(dir, "config.yaml"))
	if err == nil {
		t.Fatal("expected unknown key error")
	}
	if !strings.Contains(err.Error(), `unknown config key "unexpected_key" in section "schedule"`) {
		t.Fatalf("Load() error = %v, want unknown key and section name", err)
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

func TestLoadLocalOverridesEarlierLayersAndPreservesMergedFields(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "config.yaml"), "schedule:\n  daily_run: \"30 15 * * 1-5\"\nstrategy:\n  short_window: 3\n  long_window: 5\n")
	writeFile(t, filepath.Join(dir, "data.yaml"), "db:\n  path: data/from-data.db\nstrategy:\n  symbol: 600519\n")
	writeFile(t, filepath.Join(dir, "portfolio.yaml"), "portfolio:\n  rebalance_interval_days: 7\n  max_cash_share: 0.35\n")
	writeFile(t, filepath.Join(dir, "report.yaml"), "report:\n  history_root: reports/from-report\n  cleanup_keep_days: 21\n")
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
	if cfg.Portfolio.RebalanceIntervalDays != 7 {
		t.Fatalf("RebalanceIntervalDays = %d, want preserved layered value", cfg.Portfolio.RebalanceIntervalDays)
	}
	if cfg.Report.HistoryRoot != "reports/from-local" {
		t.Fatalf("HistoryRoot = %q, want local override", cfg.Report.HistoryRoot)
	}
	if cfg.Report.CleanupKeepDays != 21 {
		t.Fatalf("CleanupKeepDays = %d, want preserved layered value", cfg.Report.CleanupKeepDays)
	}
	if cfg.Strategy.Symbol != "600519" {
		t.Fatalf("Symbol = %q, want preserved layered value", cfg.Strategy.Symbol)
	}
	if cfg.Strategy.ShortWindow != 3 || cfg.Strategy.LongWindow != 5 {
		t.Fatalf("unexpected strategy windows: %+v", cfg.Strategy)
	}
}

func TestWriteRuntimeSnapshotCreatesParentsAndWritesNewlineTerminatedPrettyJSON(t *testing.T) {
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
	if len(got) == 0 || got[len(got)-1] != '\n' {
		t.Fatalf("snapshot missing trailing newline: %q", got)
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
