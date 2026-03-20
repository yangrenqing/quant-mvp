package config

import (
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

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
