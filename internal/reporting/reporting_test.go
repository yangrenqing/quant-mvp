package reporting

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSummaryCardsUsesJSONReports(t *testing.T) {
	t.Setenv("REPORTING_TODAY", "2026-03-20")

	root := t.TempDir()
	reportsDir := filepath.Join(root, "reports")
	historyRoot := filepath.Join(root, "history")
	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(historyRoot, "2026-03-19", "a_share_scan"), 0o755); err != nil {
		t.Fatal(err)
	}

	writeJSONTestFile(t, filepath.Join(reportsDir, "portfolio_backtest.json"), map[string]any{
		"TotalReturn":     0.10,
		"ExcessReturn":    -0.02,
		"MaxDrawdown":     0.01,
		"CurrentHoldings": []map[string]any{{"Symbol": "600001", "Name": "Alpha", "Shares": 100}},
		"ExposureLevel":   0,
		"RegimeLabel":     "risk_off",
	})
	writeJSONTestFile(t, filepath.Join(reportsDir, "diagnostics.json"), map[string]any{
		"ProviderFailures": map[string]int{},
	})
	writeJSONTestFile(t, filepath.Join(reportsDir, "a_share_scan.json"), map[string]any{
		"watch":   []map[string]any{},
		"observe": []map[string]any{{"Symbol": "600001"}, {"Symbol": "600002"}},
		"avoid":   []map[string]any{},
	})
	writeJSONTestFile(t, filepath.Join(reportsDir, "a_share_focus.json"), []map[string]any{
		{"Symbol": "600001", "Name": "Alpha", "Score": 0.88, "ClosePrice": 10.5, "MarketDate": "2026-03-20", "Plan": "watch closely"},
	})
	writeJSONTestFile(t, filepath.Join(historyRoot, "2026-03-19", "a_share_scan", "a_share_focus.json"), []map[string]any{
		{"Symbol": "600003"},
	})
	writeJSONTestFile(t, filepath.Join(reportsDir, "health_monitor.json"), map[string]any{
		"status": "warning",
		"latest_live": map[string]any{
			"equity": 12345.67,
		},
	})
	writeJSONTestFile(t, filepath.Join(reportsDir, "factor_research.json"), map[string]any{
		"row_count": 10,
		"top_correlations": []map[string]any{
			{"feature": "f1", "correlation": 0.12, "quintile_spread": 0.03, "sample_count": 10},
		},
	})
	writeJSONTestFile(t, filepath.Join(reportsDir, "research_summary.json"), map[string]any{
		"summary": "research ok",
		"verdict": "keep going",
	})
	writeJSONTestFile(t, filepath.Join(reportsDir, "model_comparison.json"), map[string]any{
		"regression": map[string]any{
			"metrics": map[string]any{"directional_accuracy": 0.8},
		},
		"classifier": map[string]any{
			"rolling_directional_accuracy": 0.6,
		},
		"verdict": "regression wins",
	})
	writeJSONTestFile(t, filepath.Join(reportsDir, "strategy_quality.json"), map[string]any{
		"portfolio": map[string]any{
			"total_return":  0.1,
			"excess_return": -0.02,
		},
		"verdict": "stable",
	})
	writeJSONTestFile(t, filepath.Join(reportsDir, "runtime_report.json"), map[string]any{
		"started_at":    "2026-03-19T00:00:00+08:00",
		"runtime":       "1d 2h",
		"last_24h_runs": 9,
	})
	writeJSONTestFile(t, filepath.Join(reportsDir, "evolution_report.json"), map[string]any{
		"run_count":                 30,
		"regression_promotions":     1,
		"classifier_promotions":     2,
		"active_shadow_equity_diff": 1.5,
		"active_latest": map[string]any{
			"strategy_version": "active_v1",
			"market_date":      "2026-03-20",
			"equity":           100,
		},
		"shadow_latest": map[string]any{
			"strategy_version": "shadow_v1",
			"market_date":      "2026-03-20",
			"equity":           101.5,
		},
	})
	writeJSONTestFile(t, filepath.Join(reportsDir, "evolution_report_overnight.json"), map[string]any{
		"run_count":             4,
		"regression_promotions": 0,
		"classifier_promotions": 1,
	})
	writeJSONTestFile(t, filepath.Join(reportsDir, "strategy_lifecycle.json"), map[string]any{
		"rows": []map[string]any{
			{"version": "active_v1", "status": "active"},
		},
		"events": []map[string]any{
			{"event_type": "promotion", "from_version": "old_v1", "to_version": "active_v1"},
		},
	})

	cards := BuildSummaryCards(Inputs{
		ReportsDir:  reportsDir,
		HistoryRoot: historyRoot,
	})

	if !strings.Contains(cards.TodayConclusion, "Top focus: 600001 Alpha") {
		t.Fatalf("TodayConclusion = %q", cards.TodayConclusion)
	}
	if !strings.Contains(cards.Changes, "新增: 600001") || !strings.Contains(cards.Changes, "移出: 600003") {
		t.Fatalf("Changes = %q", cards.Changes)
	}
	if cards.StrongWeak != "最强候选: 600001 | 最弱候选: 600002" {
		t.Fatalf("StrongWeak = %q", cards.StrongWeak)
	}
	if !strings.Contains(cards.StrategyEvolution, "active=active_v1 equity=100.00") {
		t.Fatalf("StrategyEvolution = %q", cards.StrategyEvolution)
	}
	if !strings.Contains(cards.ModelComparison, "cls rolling directional accuracy: 60.00%") {
		t.Fatalf("ModelComparison = %q", cards.ModelComparison)
	}
	if !strings.Contains(cards.FactorResearch, "Rows: 10") {
		t.Fatalf("FactorResearch = %q", cards.FactorResearch)
	}
}

func TestBuildHistoryCompareReportSortsEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run_index.jsonl")
	lines := []string{
		`{"run_type":"daily","generated_at":"2026-03-19T10:00:00+08:00","summary":{"x":1}}`,
		`{"run_type":"dashboard","generated_at":"2026-03-20T10:00:00+08:00","summary":{"x":2}}`,
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	payload, text, _ := BuildHistoryCompareReport(path)
	if !strings.Contains(text, "2026-03-20T10:00:00+08:00 | dashboard") {
		t.Fatalf("text = %q", text)
	}

	entries, ok := payload["entries"].([]map[string]any)
	if !ok || len(entries) != 2 {
		t.Fatalf("payload entries = %#v", payload["entries"])
	}
}

func writeJSONTestFile(t *testing.T, path string, payload any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}
