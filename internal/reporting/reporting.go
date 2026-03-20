package reporting

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type QueryFunc func(sql string) (string, error)

type Inputs struct {
	ReportsDir   string
	ReportRoot   string
	HistoryRoot  string
	RunIndexPath string
}

type SummaryCards struct {
	TodayConclusion    string
	RiskAlerts         string
	Changes            string
	StrongWeak         string
	CurrentHoldings    string
	StrategyEvolution  string
	LifecycleSummary   string
	ResearchSummary    string
	ModelComparison    string
	StrategyQuality    string
	RuntimeSummary     string
	EvolutionSummary   string
	OvernightEvolution string
	SystemHealth       string
	FactorResearch     string
}

type DashboardSection struct {
	Title   string
	Content string
	Stamp   string
}

type historyEntry struct {
	RunType string         `json:"run_type"`
	When    string         `json:"generated_at"`
	Summary map[string]any `json:"summary"`
}

type lifecycleRow struct {
	Version       string `json:"version"`
	Status        string `json:"status"`
	ParentVersion string `json:"parent_version"`
	ActivatedAt   string `json:"activated_at"`
	ArchivedAt    string `json:"archived_at"`
}

type lifecycleEvent struct {
	EventType string `json:"event_type"`
	From      string `json:"from_version"`
	To        string `json:"to_version"`
	Reason    string `json:"reason"`
	Recorded  string `json:"recorded_at"`
}

type scanCandidate struct {
	Symbol     string  `json:"Symbol"`
	Name       string  `json:"Name"`
	Score      float64 `json:"Score"`
	ClosePrice float64 `json:"ClosePrice"`
	MarketDate string  `json:"MarketDate"`
	Plan       string  `json:"Plan"`
}

type scanReport struct {
	Watch   []scanCandidate `json:"watch"`
	Observe []scanCandidate `json:"observe"`
	Avoid   []scanCandidate `json:"avoid"`
}

type portfolioHolding struct {
	Symbol string  `json:"Symbol"`
	Name   string  `json:"Name"`
	Shares int     `json:"Shares"`
	Entry  float64 `json:"Entry"`
}

type portfolioBacktestReport struct {
	TotalReturn     float64            `json:"TotalReturn"`
	Annualized      float64            `json:"AnnualizedReturn"`
	BenchmarkReturn float64            `json:"BenchmarkReturn"`
	ExcessReturn    float64            `json:"ExcessReturn"`
	MaxDrawdown     float64            `json:"MaxDrawdown"`
	LatestSelection []scanCandidate    `json:"LatestSelection"`
	CurrentHoldings []portfolioHolding `json:"CurrentHoldings"`
	ExposureLevel   float64            `json:"ExposureLevel"`
	RegimeLabel     string             `json:"RegimeLabel"`
}

type diagnosticsReport struct {
	ProviderFailures map[string]int `json:"ProviderFailures"`
}

type healthMonitorReport struct {
	Status    string `json:"status"`
	LatestRun struct {
		RunType string `json:"run_type"`
		When    string `json:"generated_at"`
	} `json:"latest_run"`
	LatestLive struct {
		Equity float64 `json:"equity"`
	} `json:"latest_live"`
	LatestShadow struct {
		Equity float64 `json:"equity"`
	} `json:"latest_shadow"`
	ProviderFailureTotal int      `json:"provider_failure_total"`
	Warnings             []string `json:"warnings"`
	Alerts               []string `json:"alerts"`
}

type factorMetric struct {
	Feature        string  `json:"feature"`
	Correlation    float64 `json:"correlation"`
	QuintileSpread float64 `json:"quintile_spread"`
	SampleCount    int     `json:"sample_count"`
}

type factorResearchReport struct {
	RowCount        int            `json:"row_count"`
	FeatureCount    int            `json:"feature_count"`
	TopCorrelations []factorMetric `json:"top_correlations"`
}

type researchSummaryReport struct {
	Summary string `json:"summary"`
	Verdict string `json:"verdict"`
}

type modelSide struct {
	Metrics struct {
		DirectionalAccuracy float64 `json:"directional_accuracy"`
	} `json:"metrics"`
	RollingDirectionalAccuracy float64 `json:"rolling_directional_accuracy"`
}

type modelComparisonReport struct {
	Regression modelSide `json:"regression"`
	Classifier modelSide `json:"classifier"`
	Verdict    string    `json:"verdict"`
}

type strategyQualityReport struct {
	Portfolio struct {
		TotalReturn  float64 `json:"total_return"`
		ExcessReturn float64 `json:"excess_return"`
		MaxDrawdown  float64 `json:"max_drawdown"`
		Regime       string  `json:"regime"`
	} `json:"portfolio"`
	Verdict string `json:"verdict"`
}

type runtimeReport struct {
	StartedAt   string `json:"started_at"`
	Runtime     string `json:"runtime"`
	Last24HRuns int    `json:"last_24h_runs"`
}

type evolutionLatest struct {
	StrategyVersion string  `json:"strategy_version"`
	MarketDate      string  `json:"market_date"`
	Equity          float64 `json:"equity"`
}

type evolutionReport struct {
	RunCount               int              `json:"run_count"`
	RegressionPromotions   int              `json:"regression_promotions"`
	ClassifierPromotions   int              `json:"classifier_promotions"`
	ActiveShadowEquityDiff float64          `json:"active_shadow_equity_diff"`
	ActiveLatest           *evolutionLatest `json:"active_latest"`
	ShadowLatest           *evolutionLatest `json:"shadow_latest"`
}

type latestPlanReport struct {
	Mode       string `json:"mode"`
	MarketDate string `json:"market_date"`
	Signal     string `json:"signal"`
	Reason     string `json:"reason"`
	PlanDate   string `json:"plan_date"`
	Plan       string `json:"plan"`
}

type lifecycleReport struct {
	Rows   []lifecycleRow   `json:"rows"`
	Events []lifecycleEvent `json:"events"`
}

func BuildSummaryCards(inputs Inputs) SummaryCards {
	portfolio := portfolioBacktestReport{}
	loadJSONFile(filepath.Join(inputs.ReportsDir, "portfolio_backtest.json"), &portfolio)

	diagnostics := diagnosticsReport{}
	loadJSONFile(filepath.Join(inputs.ReportsDir, "diagnostics.json"), &diagnostics)

	scan := scanReport{}
	loadJSONFile(filepath.Join(inputs.ReportsDir, "a_share_scan.json"), &scan)

	focus := []scanCandidate{}
	loadJSONFile(filepath.Join(inputs.ReportsDir, "a_share_focus.json"), &focus)

	health := healthMonitorReport{}
	loadJSONFile(filepath.Join(inputs.ReportsDir, "health_monitor.json"), &health)

	factors := factorResearchReport{}
	loadJSONFile(filepath.Join(inputs.ReportsDir, "factor_research.json"), &factors)

	research := researchSummaryReport{}
	loadJSONFile(filepath.Join(inputs.ReportsDir, "research_summary.json"), &research)

	models := modelComparisonReport{}
	loadJSONFile(filepath.Join(inputs.ReportsDir, "model_comparison.json"), &models)

	quality := strategyQualityReport{}
	loadJSONFile(filepath.Join(inputs.ReportsDir, "strategy_quality.json"), &quality)

	runtime := runtimeReport{}
	loadJSONFile(filepath.Join(inputs.ReportsDir, "runtime_report.json"), &runtime)

	evolution := evolutionReport{}
	loadJSONFile(filepath.Join(inputs.ReportsDir, "evolution_report.json"), &evolution)

	overnight := evolutionReport{}
	loadJSONFile(filepath.Join(inputs.ReportsDir, "evolution_report_overnight.json"), &overnight)

	lifecycle := lifecycleReport{}
	loadJSONFile(filepath.Join(inputs.ReportsDir, "strategy_lifecycle.json"), &lifecycle)

	return SummaryCards{
		TodayConclusion:    buildTodayConclusionCard(focus, portfolio),
		RiskAlerts:         buildRiskAlertCard(portfolio, diagnostics),
		Changes:            buildChangeCard(inputs.HistoryRoot, focus),
		StrongWeak:         buildStrengthCard(scan),
		CurrentHoldings:    buildHoldingCard(portfolio),
		StrategyEvolution:  buildStrategyEvolutionCard(evolution),
		LifecycleSummary:   buildLifecycleSummaryCard(lifecycle),
		ResearchSummary:    buildResearchSummaryCard(research),
		ModelComparison:    buildModelComparisonCard(models),
		StrategyQuality:    buildStrategyQualitySummaryCard(quality),
		RuntimeSummary:     buildRuntimeSummaryCard(runtime),
		EvolutionSummary:   buildEvolutionSummaryCard(evolution),
		OvernightEvolution: buildEvolutionSummaryCard(overnight),
		SystemHealth:       buildHealthSummaryCard(health),
		FactorResearch:     buildFactorSummaryCard(factors),
	}
}

func BuildHistoryCompareReport(runIndexPath string) (map[string]any, string, string) {
	lines := make([]string, 0)
	rows := make([]string, 0)
	payload := []map[string]any{}
	content, err := os.ReadFile(runIndexPath)
	if err != nil {
		text := "History Compare\n\nNo run history yet.\n"
		htmlContent := `<!doctype html><html><body><pre>No run history yet.</pre></body></html>`
		return map[string]any{"entries": payload}, text, htmlContent
	}

	entries := make([]historyEntry, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry historyEntry
		if json.Unmarshal([]byte(line), &entry) == nil {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].When > entries[j].When })

	limit := minInt(12, len(entries))
	lines = append(lines, "History Compare", "")
	for i := 0; i < limit; i++ {
		entry := entries[i]
		lines = append(lines, fmt.Sprintf("%s | %s | %v", entry.When, entry.RunType, entry.Summary))
		rows = append(rows, fmt.Sprintf("<tr><td>%s</td><td>%s</td><td><pre>%s</pre></td></tr>",
			html.EscapeString(entry.When),
			html.EscapeString(entry.RunType),
			html.EscapeString(fmt.Sprintf("%v", entry.Summary)),
		))
		payload = append(payload, map[string]any{
			"generated_at": entry.When,
			"run_type":     entry.RunType,
			"summary":      entry.Summary,
		})
	}
	text := strings.Join(lines, "\n") + "\n"
	htmlContent := fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><title>History Compare</title><style>body{font-family:Georgia,serif;background:#f4efe6;color:#1f1b16}.wrap{max-width:1100px;margin:36px auto;padding:0 20px}.card{background:#fffaf3;border:1px solid #d9cfbf;border-radius:18px;padding:24px}table{width:100%%;border-collapse:collapse}th,td{text-align:left;padding:10px;border-bottom:1px solid #e7dece;vertical-align:top}pre{margin:0;white-space:pre-wrap}</style></head><body><div class="wrap"><div class="card"><h1>History Compare</h1><table><thead><tr><th>When</th><th>Run Type</th><th>Summary</th></tr></thead><tbody>%s</tbody></table></div></div></body></html>`, strings.Join(rows, ""))
	return map[string]any{"entries": payload}, text, htmlContent
}

func BuildMarketOverviewReport(reportsDir string) (map[string]any, string, string) {
	plan := latestPlanReport{}
	loadJSONFile(filepath.Join(reportsDir, "latest_plan.json"), &plan)

	focus := []scanCandidate{}
	loadJSONFile(filepath.Join(reportsDir, "a_share_focus.json"), &focus)

	planText := renderPlanText(plan)
	focusText := renderFocusText(focus)
	payload := map[string]any{
		"latest_plan":        plan,
		"a_share_focus":      focus,
		"latest_plan_text":   planText,
		"a_share_focus_text": focusText,
	}
	text := "Market Overview\n\nUS / Single-symbol Plan\n" + planText + "\n\nA-Share Focus\n" + focusText + "\n"
	htmlContent := fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><title>Market Overview</title><style>body{font-family:Georgia,serif;background:#f4efe6;color:#1f1b16}.wrap{max-width:1100px;margin:36px auto;padding:0 20px}.grid{display:grid;grid-template-columns:1fr 1fr;gap:18px}.card{background:#fffaf3;border:1px solid #d9cfbf;border-radius:18px;padding:24px}pre{white-space:pre-wrap}</style></head><body><div class="wrap"><h1>Market Overview</h1><div class="grid"><div class="card"><h2>US / Single-symbol</h2><pre>%s</pre></div><div class="card"><h2>A-Share Focus</h2><pre>%s</pre></div></div></div></body></html>`, html.EscapeString(planText), html.EscapeString(focusText))
	return payload, text, htmlContent
}

func BuildStrategyLifecycleReport(dbPath string, query QueryFunc) (map[string]any, string, string) {
	if strings.TrimSpace(dbPath) == "" || query == nil {
		payload := map[string]any{"rows": []lifecycleRow{}, "events": []lifecycleEvent{}}
		return payload, "Strategy Lifecycle\n\nDatabase disabled.\n", "<html><body><pre>Database disabled.</pre></body></html>"
	}

	registryOutput, _ := query("SELECT version_name, status, parent_version, activated_at, archived_at FROM strategy_registry ORDER BY id DESC LIMIT 20;")
	eventOutput, _ := query("SELECT event_type, from_version, to_version, trigger_reason, recorded_at FROM strategy_promotions ORDER BY id DESC LIMIT 20;")

	rows := make([]lifecycleRow, 0)
	if strings.TrimSpace(registryOutput) != "" {
		for _, line := range strings.Split(strings.TrimSpace(registryOutput), "\n") {
			parts := strings.Split(line, "|")
			if len(parts) != 5 {
				continue
			}
			rows = append(rows, lifecycleRow{
				Version:       parts[0],
				Status:        parts[1],
				ParentVersion: parts[2],
				ActivatedAt:   parts[3],
				ArchivedAt:    parts[4],
			})
		}
	}

	events := make([]lifecycleEvent, 0)
	if strings.TrimSpace(eventOutput) != "" {
		for _, line := range strings.Split(strings.TrimSpace(eventOutput), "\n") {
			parts := strings.Split(line, "|")
			if len(parts) != 5 {
				continue
			}
			events = append(events, lifecycleEvent{
				EventType: parts[0],
				From:      parts[1],
				To:        parts[2],
				Reason:    parts[3],
				Recorded:  parts[4],
			})
		}
	}

	var textBuilder strings.Builder
	textBuilder.WriteString("Strategy Lifecycle\n\nRegistry\n")
	if len(rows) == 0 {
		textBuilder.WriteString("No strategy versions recorded.\n")
	} else {
		for i, row := range rows {
			fmt.Fprintf(&textBuilder, "%d. %s status=%s parent=%s activated=%s archived=%s\n", i+1, row.Version, row.Status, row.ParentVersion, row.ActivatedAt, row.ArchivedAt)
		}
	}
	textBuilder.WriteString("\nEvents\n")
	if len(events) == 0 {
		textBuilder.WriteString("No promotion events recorded.\n")
	} else {
		for i, event := range events {
			fmt.Fprintf(&textBuilder, "%d. %s %s -> %s at %s reason=%s\n", i+1, event.EventType, event.From, event.To, event.Recorded, event.Reason)
		}
	}

	var registryRows strings.Builder
	for _, row := range rows {
		fmt.Fprintf(&registryRows, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			html.EscapeString(row.Version),
			html.EscapeString(row.Status),
			html.EscapeString(row.ParentVersion),
			html.EscapeString(row.ActivatedAt),
			html.EscapeString(row.ArchivedAt),
		)
	}

	var eventRows strings.Builder
	for _, event := range events {
		fmt.Fprintf(&eventRows, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			html.EscapeString(event.EventType),
			html.EscapeString(event.From),
			html.EscapeString(event.To),
			html.EscapeString(event.Recorded),
			html.EscapeString(event.Reason),
		)
	}

	htmlContent := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Strategy Lifecycle</title>
  <style>
    body { margin: 0; font-family: Georgia, "Times New Roman", serif; background: #f4efe6; color: #1f1b16; }
    .wrap { max-width: 1200px; margin: 36px auto; padding: 0 20px; }
    .card { background: #fffaf3; border: 1px solid #d9cfbf; border-radius: 18px; padding: 24px; box-shadow: 0 18px 40px rgba(70, 50, 20, 0.08); }
    table { width: 100%%; border-collapse: collapse; font-size: 15px; margin-bottom: 24px; }
    th, td { text-align: left; padding: 10px 8px; border-bottom: 1px solid #e7dece; vertical-align: top; }
    th { font-size: 12px; text-transform: uppercase; color: #6d6559; }
  </style>
</head>
<body><div class="wrap"><div class="card"><h1>Strategy Lifecycle</h1><h2>Registry</h2><table><thead><tr><th>Version</th><th>Status</th><th>Parent</th><th>Activated</th><th>Archived</th></tr></thead><tbody>%s</tbody></table><h2>Events</h2><table><thead><tr><th>Event</th><th>From</th><th>To</th><th>Recorded</th><th>Reason</th></tr></thead><tbody>%s</tbody></table></div></div></body></html>`, registryRows.String(), eventRows.String())

	payload := map[string]any{"rows": rows, "events": events}
	return payload, textBuilder.String(), htmlContent
}

func buildTodayConclusionCard(focus []scanCandidate, portfolio portfolioBacktestReport) string {
	parts := make([]string, 0, 2)
	if len(focus) > 0 {
		top := focus[0]
		parts = append(parts, fmt.Sprintf("Top focus: %s %s score=%.4f", top.Symbol, top.Name, top.Score))
	}
	if portfolio.RegimeLabel != "" {
		parts = append(parts, fmt.Sprintf("Regime: %s", portfolio.RegimeLabel))
	}
	if len(parts) == 0 {
		return "今天还没有完整日报，先运行 scan 和 portfolio backtest。"
	}
	return strings.Join(parts, " | ")
}

func buildRiskAlertCard(portfolio portfolioBacktestReport, diagnostics diagnosticsReport) string {
	alerts := make([]string, 0, 4)
	if portfolio.RegimeLabel != "" {
		alerts = append(alerts, "Regime: "+portfolio.RegimeLabel)
	}
	if portfolio.ExcessReturn != 0 {
		alerts = append(alerts, fmt.Sprintf("Excess return: %.2f%%", portfolio.ExcessReturn*100))
	}
	if portfolio.MaxDrawdown != 0 {
		alerts = append(alerts, fmt.Sprintf("Max drawdown: %.2f%%", portfolio.MaxDrawdown*100))
	}
	if len(diagnostics.ProviderFailures) == 0 {
		alerts = append(alerts, "Diagnostics: Provider failures: none")
	} else {
		alerts = append(alerts, fmt.Sprintf("Diagnostics: Provider failures: %d", len(diagnostics.ProviderFailures)))
	}
	return strings.Join(alerts, " | ")
}

func buildChangeCard(historyRoot string, current []scanCandidate) string {
	previousPath := latestHistoricalFileBeforeToday(historyRoot, "a_share_scan", "a_share_focus.json")
	if previousPath == "" {
		return "还没有昨日快照，今天起会自动归档。"
	}

	previous := []scanCandidate{}
	if !loadJSONFile(previousPath, &previous) {
		return "昨日快照读取失败。"
	}

	currentSymbols := extractSymbols(current)
	previousSymbols := extractSymbols(previous)
	added := diffStrings(currentSymbols, previousSymbols)
	removed := diffStrings(previousSymbols, currentSymbols)
	if len(added) == 0 && len(removed) == 0 {
		return "关注名单与昨日相同。"
	}

	parts := make([]string, 0, 2)
	if len(added) > 0 {
		parts = append(parts, "新增: "+strings.Join(added, ", "))
	}
	if len(removed) > 0 {
		parts = append(parts, "移出: "+strings.Join(removed, ", "))
	}
	return strings.Join(parts, " | ")
}

func buildStrengthCard(scan scanReport) string {
	symbols := extractSymbols(append(append([]scanCandidate{}, scan.Watch...), append(scan.Observe, scan.Avoid...)...))
	if len(symbols) == 0 {
		return "还没有扫描结果。"
	}
	return fmt.Sprintf("最强候选: %s | 最弱候选: %s", symbols[0], symbols[len(symbols)-1])
}

func buildHoldingCard(portfolio portfolioBacktestReport) string {
	if len(portfolio.CurrentHoldings) == 0 {
		return "None"
	}
	parts := make([]string, 0, len(portfolio.CurrentHoldings))
	for _, holding := range portfolio.CurrentHoldings {
		parts = append(parts, fmt.Sprintf("%s %s x%d", holding.Symbol, holding.Name, holding.Shares))
	}
	return strings.Join(parts, "; ")
}

func buildStrategyEvolutionCard(report evolutionReport) string {
	if report.ActiveLatest == nil && report.ShadowLatest == nil {
		return "还没有 active / shadow 对比数据。"
	}
	if report.ActiveLatest != nil && report.ShadowLatest == nil {
		return fmt.Sprintf("当前 active=%s equity=%.2f，尚无 shadow 账户。", report.ActiveLatest.StrategyVersion, report.ActiveLatest.Equity)
	}
	if report.ActiveLatest == nil && report.ShadowLatest != nil {
		return fmt.Sprintf("当前 shadow=%s equity=%.2f，尚无 active 对比。", report.ShadowLatest.StrategyVersion, report.ShadowLatest.Equity)
	}
	return fmt.Sprintf("active=%s equity=%.2f | shadow=%s equity=%.2f | diff=%.2f | market_date=%s",
		report.ActiveLatest.StrategyVersion,
		report.ActiveLatest.Equity,
		report.ShadowLatest.StrategyVersion,
		report.ShadowLatest.Equity,
		report.ActiveShadowEquityDiff,
		report.ActiveLatest.MarketDate,
	)
}

func buildLifecycleSummaryCard(report lifecycleReport) string {
	activeVersion := "none"
	for _, row := range report.Rows {
		if row.Status == "active" {
			activeVersion = row.Version
			break
		}
	}
	lastEvent := "no events"
	if len(report.Events) > 0 {
		lastEvent = fmt.Sprintf("%s %s -> %s", report.Events[0].EventType, report.Events[0].From, report.Events[0].To)
	}
	if len(report.Rows) == 0 && len(report.Events) == 0 {
		return "生命周期报告尚未生成。"
	}
	return fmt.Sprintf("active=%s | versions=%d | last_event=%s", activeVersion, len(report.Rows), lastEvent)
}

func buildHealthSummaryCard(report healthMonitorReport) string {
	if report.Status == "" {
		return "系统健康报告尚未生成。"
	}
	parts := []string{"Status: " + report.Status}
	if report.LatestLive.Equity != 0 {
		parts = append(parts, fmt.Sprintf("Active equity: %.2f", report.LatestLive.Equity))
	} else if report.LatestShadow.Equity != 0 {
		parts = append(parts, fmt.Sprintf("Shadow equity: %.2f", report.LatestShadow.Equity))
	}
	return strings.Join(parts, " | ")
}

func buildFactorSummaryCard(report factorResearchReport) string {
	if report.RowCount == 0 {
		return "因子研究报告尚未生成。"
	}
	parts := []string{fmt.Sprintf("Rows: %d", report.RowCount)}
	if len(report.TopCorrelations) > 0 {
		item := report.TopCorrelations[0]
		parts = append(parts, fmt.Sprintf("- %s: corr=%.4f spread=%.4f samples=%d", item.Feature, item.Correlation, item.QuintileSpread, item.SampleCount))
	}
	return strings.Join(parts, " | ")
}

func buildResearchSummaryCard(report researchSummaryReport) string {
	if report.Summary == "" {
		return "研究摘要尚未生成。"
	}
	if report.Verdict == "" {
		return report.Summary
	}
	return report.Summary + " | Verdict: " + report.Verdict
}

func buildModelComparisonCard(report modelComparisonReport) string {
	parts := make([]string, 0, 3)
	if report.Regression.Metrics.DirectionalAccuracy > 0 {
		parts = append(parts, fmt.Sprintf("reg test directional accuracy: %.2f%%", report.Regression.Metrics.DirectionalAccuracy*100))
	}
	if report.Classifier.RollingDirectionalAccuracy > 0 {
		parts = append(parts, fmt.Sprintf("cls rolling directional accuracy: %.2f%%", report.Classifier.RollingDirectionalAccuracy*100))
	}
	if report.Verdict != "" {
		parts = append(parts, "Verdict: "+report.Verdict)
	}
	if len(parts) == 0 {
		return "模型对比报告尚未生成。"
	}
	return strings.Join(parts, " | ")
}

func buildStrategyQualitySummaryCard(report strategyQualityReport) string {
	parts := make([]string, 0, 3)
	if report.Portfolio.TotalReturn != 0 {
		parts = append(parts, fmt.Sprintf("- total return: %.2f%%", report.Portfolio.TotalReturn*100))
	}
	if report.Portfolio.ExcessReturn != 0 {
		parts = append(parts, fmt.Sprintf("- excess return: %.2f%%", report.Portfolio.ExcessReturn*100))
	}
	if report.Verdict != "" {
		parts = append(parts, "Verdict: "+report.Verdict)
	}
	if len(parts) == 0 {
		return "策略质量报告尚未生成。"
	}
	return strings.Join(parts, " | ")
}

func buildRuntimeSummaryCard(report runtimeReport) string {
	parts := make([]string, 0, 3)
	if report.StartedAt != "" {
		parts = append(parts, "Started at: "+report.StartedAt)
	}
	if report.Runtime != "" {
		parts = append(parts, "Runtime: "+report.Runtime)
	}
	if report.Last24HRuns > 0 {
		parts = append(parts, fmt.Sprintf("Last 24h runs: %d", report.Last24HRuns))
	}
	if len(parts) == 0 {
		return "运行时长报告尚未生成。"
	}
	return strings.Join(parts, " | ")
}

func buildEvolutionSummaryCard(report evolutionReport) string {
	if report.RunCount == 0 {
		return "演化报告尚未生成。"
	}
	return fmt.Sprintf("Run count: %d | Regression promotions: %d | Classifier promotions: %d", report.RunCount, report.RegressionPromotions, report.ClassifierPromotions)
}

func renderPlanText(plan latestPlanReport) string {
	if plan.Signal == "" && plan.Plan == "" {
		return "Not generated yet."
	}
	return fmt.Sprintf("Mode: %s\nMarket date: %s\nSignal: %s\nReason: %s\nPlan for %s: %s",
		emptyFallback(plan.Mode, "live"),
		plan.MarketDate,
		plan.Signal,
		plan.Reason,
		plan.PlanDate,
		plan.Plan,
	)
}

func renderFocusText(candidates []scanCandidate) string {
	if len(candidates) == 0 {
		return "A-Share Focus List\n\n今日无建议关注标的。"
	}
	var builder strings.Builder
	builder.WriteString("A-Share Focus List\n\n")
	for i, candidate := range candidates {
		fmt.Fprintf(&builder, "%d. %s %s\n", i+1, candidate.Symbol, candidate.Name)
		fmt.Fprintf(&builder, "   Market date: %s\n", candidate.MarketDate)
		fmt.Fprintf(&builder, "   Score: %.4f\n", candidate.Score)
		fmt.Fprintf(&builder, "   Close: %.2f\n", candidate.ClosePrice)
		if strings.TrimSpace(candidate.Plan) != "" {
			fmt.Fprintf(&builder, "   Plan: %s\n", candidate.Plan)
		}
	}
	return strings.TrimSpace(builder.String())
}

func loadJSONFile(path string, dest any) bool {
	content, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	if err := json.Unmarshal(content, dest); err != nil {
		return false
	}
	return true
}

func latestHistoricalFileBeforeToday(historyRoot string, runType string, fileName string) string {
	entries, err := os.ReadDir(historyRoot)
	if err != nil {
		return ""
	}
	today := todayDirName()
	dates := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() < today {
			dates = append(dates, entry.Name())
		}
	}
	sort.Strings(dates)
	for i := len(dates) - 1; i >= 0; i-- {
		path := filepath.Join(historyRoot, dates[i], runType, fileName)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func extractSymbols(candidates []scanCandidate) []string {
	symbols := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Symbol) != "" {
			symbols = append(symbols, candidate.Symbol)
		}
	}
	return symbols
}

func diffStrings(left []string, right []string) []string {
	rightSet := map[string]struct{}{}
	for _, item := range right {
		rightSet[item] = struct{}{}
	}
	diff := make([]string, 0)
	for _, item := range left {
		if _, ok := rightSet[item]; !ok {
			diff = append(diff, item)
		}
	}
	return diff
}

func emptyFallback(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func todayDirName() string {
	if override := strings.TrimSpace(os.Getenv("REPORTING_TODAY")); override != "" {
		return override
	}
	return time.Now().Format("2006-01-02")
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
