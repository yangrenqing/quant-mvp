package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestBuildTrialModes(t *testing.T) {
	if got := buildTrialMode("exp 01", "exp007"); got != "trial:exp_01:exp007" {
		t.Fatalf("buildTrialMode = %q", got)
	}
	if got := buildTrialShadowMode("candidate_auto_v1", "night batch", "exp012"); got != "shadow:candidate_auto_v1:trial:night_batch:exp012" {
		t.Fatalf("buildTrialShadowMode = %q", got)
	}
}

func TestSanitizeReportToken(t *testing.T) {
	if got := sanitizeReportToken("  alpha/beta:2026-03-22  "); got != "alpha_beta_2026-03-22" {
		t.Fatalf("sanitizeReportToken = %q", got)
	}
	if got := sanitizeReportToken("   "); got != "latest" {
		t.Fatalf("sanitizeReportToken empty = %q", got)
	}
}

func TestSummarizePaperTrialGroups(t *testing.T) {
	accounts := []paperTrialAccountSummary{
		{Group: "live", Mode: "trial:a:exp001", Equity: 101000, Return: 0.01},
		{Group: "live", Mode: "trial:a:exp002", Equity: 99000, Return: -0.01},
		{Group: "shadow", Mode: "shadow:v1:trial:a:exp001", Equity: 103000, Return: 0.03},
	}

	groups, avgEquity, avgReturn, bestMode, bestEquity, worstMode, worstEquity := summarizePaperTrialGroups(accounts)
	if len(groups) != 2 {
		t.Fatalf("group count = %d", len(groups))
	}
	if avgEquity != 101000 {
		t.Fatalf("avgEquity = %.2f", avgEquity)
	}
	if avgReturn < 0.0099 || avgReturn > 0.0101 {
		t.Fatalf("avgReturn = %.6f", avgReturn)
	}
	if bestMode != "shadow:v1:trial:a:exp001" || bestEquity != 103000 {
		t.Fatalf("best = %s %.2f", bestMode, bestEquity)
	}
	if worstMode != "trial:a:exp002" || worstEquity != 99000 {
		t.Fatalf("worst = %s %.2f", worstMode, worstEquity)
	}
}

func TestGeneratePaperExperimentSpecs(t *testing.T) {
	base := config{
		Strategy: strategyConfig{ShortWindow: 5, LongWindow: 20},
		Portfolio: portfolioConfig{
			QualityWeight:          1.1,
			RiskWeight:             0.8,
			HeatPenaltyWeight:      1.1,
			TrendStrategyWeight:    1.0,
			BreakoutStrategyWeight: 0.9,
			PullbackStrategyWeight: 0.8,
			ReversalWeight:         0.7,
		},
	}

	specs := generatePaperExperimentSpecs(base, 7, 3, 10, 5)
	if len(specs) != 7 {
		t.Fatalf("len(specs) = %d", len(specs))
	}
	if specs[0].ID != "exp001" || specs[6].ID != "exp007" {
		t.Fatalf("unexpected ids: first=%s last=%s", specs[0].ID, specs[6].ID)
	}
	if specs[0].TopN <= 0 || specs[0].Strategy.LongWindow <= specs[0].Strategy.ShortWindow {
		t.Fatalf("invalid first spec: %+v", specs[0])
	}
	if specs[0].ParameterSummary == "" {
		t.Fatal("expected parameter summary")
	}
	if specs[0].Style != "trend_follow" || specs[1].Style != "balanced" || specs[2].Style != "quality_pullback" {
		t.Fatalf("unexpected style rotation: %q %q %q", specs[0].Style, specs[1].Style, specs[2].Style)
	}
	if !strings.Contains(specs[2].ParameterSummary, "style=quality_pullback") {
		t.Fatalf("missing style in summary: %q", specs[2].ParameterSummary)
	}
	if specs[0].Portfolio.PullbackEnabled {
		t.Fatalf("trend_follow spec should disable pullback: %+v", specs[0].Portfolio)
	}
	if !specs[2].Portfolio.PullbackEnabled || specs[2].Portfolio.BreakoutEnabled {
		t.Fatalf("quality_pullback spec flags invalid: %+v", specs[2].Portfolio)
	}
}

func TestBuildPaperTrialWinnerArtifactPrefersLive(t *testing.T) {
	batch := paperTrialBatchResult{
		ReportTag:     "demo",
		TrialPrefix:   "demo",
		GeneratedAt:   "2026-03-22T16:00:00+08:00",
		Market:        "a_share",
		ActiveVersion: "active_v1",
		Accounts: []paperTrialAccountSummary{
			{Group: "shadow", Mode: "shadow:trial:demo:exp001", ExperimentID: "exp001", Style: "balanced", Strategy: "shadow_v1", Equity: 103000, Return: 0.03, TopN: 2, ShortWindow: 4, LongWindow: 9, FeeBps: 10, SlippageBps: 5, ParameterSummary: "shadow"},
			{Group: "live", Mode: "trial:demo:exp002", ExperimentID: "exp002", Style: "quality_pullback", Strategy: "active_v1", Equity: 101000, Return: 0.01, TopN: 3, ShortWindow: 5, LongWindow: 12, FeeBps: 12, SlippageBps: 6, ParameterSummary: "live"},
		},
		variantConfigs: map[string]config{
			"trial:demo:exp002": {Strategy: strategyConfig{ShortWindow: 5, LongWindow: 12}},
		},
	}

	winner, ok := buildPaperTrialWinnerArtifact(batch)
	if !ok {
		t.Fatal("expected winner artifact")
	}
	if winner.SourceGroup != "live" {
		t.Fatalf("winner.SourceGroup = %q", winner.SourceGroup)
	}
	if winner.ExperimentID != "exp002" {
		t.Fatalf("winner.ExperimentID = %q", winner.ExperimentID)
	}
	if winner.Style != "quality_pullback" {
		t.Fatalf("winner.Style = %q", winner.Style)
	}
	if winner.CandidateVersion != "candidate_trial_demo_exp002" {
		t.Fatalf("winner.CandidateVersion = %q", winner.CandidateVersion)
	}
}

func TestCandidateOverlayScoresRewardsLowerVolatility(t *testing.T) {
	steadyBars := buildTestBars(
		[]float64{10.0, 10.2, 10.3, 10.4, 10.55, 10.7, 10.8, 10.95, 11.05, 11.15, 11.25, 11.35},
		[]float64{1_600_000, 1_620_000, 1_590_000, 1_610_000, 1_640_000, 1_650_000, 1_630_000, 1_660_000, 1_670_000, 1_680_000, 1_690_000, 1_700_000},
	)
	volatileBars := buildTestBars(
		[]float64{10.0, 10.7, 10.1, 10.95, 10.2, 11.1, 10.35, 11.35, 10.5, 11.55, 10.7, 11.9},
		[]float64{1_500_000, 1_800_000, 1_450_000, 1_900_000, 1_420_000, 2_050_000, 1_380_000, 2_200_000, 1_340_000, 2_350_000, 1_300_000, 2_500_000},
	)

	steadyTrend, steadyLiquidity, _, _, steadyPersistence, steadyBreakout, steadyVolumeTrend, steadyRiskPenalty, _ := scoreCandidate(steadyBars, 5, 10)
	volatileTrend, volatileLiquidity, _, _, volatilePersistence, volatileBreakout, volatileVolumeTrend, volatileRiskPenalty, _ := scoreCandidate(volatileBars, 5, 10)

	steadyShortMA, steadyLongMA, steadyAvgVolume := testMovingStats(steadyBars, 5, 10)
	volatileShortMA, volatileLongMA, volatileAvgVolume := testMovingStats(volatileBars, 5, 10)

	_, steadyLowVol, steadyCrowding, _, steadyRisk, steadyHeat, _ := candidateOverlayScores(
		steadyBars,
		steadyShortMA,
		steadyLongMA,
		steadyAvgVolume,
		trailingReturn(testCloses(steadyBars), 5),
		trailingReturn(testCloses(steadyBars), 10),
		steadyTrend,
		steadyLiquidity,
		steadyPersistence,
		steadyBreakout,
		steadyVolumeTrend,
		steadyRiskPenalty,
	)
	_, volatileLowVol, volatileCrowding, _, volatileRisk, volatileHeat, _ := candidateOverlayScores(
		volatileBars,
		volatileShortMA,
		volatileLongMA,
		volatileAvgVolume,
		trailingReturn(testCloses(volatileBars), 5),
		trailingReturn(testCloses(volatileBars), 10),
		volatileTrend,
		volatileLiquidity,
		volatilePersistence,
		volatileBreakout,
		volatileVolumeTrend,
		volatileRiskPenalty,
	)

	if steadyLowVol <= volatileLowVol {
		t.Fatalf("expected steady series to score higher on low-vol: steady=%.4f volatile=%.4f", steadyLowVol, volatileLowVol)
	}
	if steadyCrowding >= volatileCrowding {
		t.Fatalf("expected volatile series to score higher on crowding: steady=%.4f volatile=%.4f", steadyCrowding, volatileCrowding)
	}
	if steadyRisk <= volatileRisk {
		t.Fatalf("expected steady series to score higher on risk-adjusted profile: steady=%.4f volatile=%.4f", steadyRisk, volatileRisk)
	}
	if steadyHeat >= volatileHeat {
		t.Fatalf("expected volatile series to carry more heat penalty: steady=%.4f volatile=%.4f", steadyHeat, volatileHeat)
	}
}

func TestPortfolioSelectionScorePrefersBalancedCandidateInRiskOff(t *testing.T) {
	portfolio := portfolioConfig{
		QualityWeight:          1.10,
		RiskWeight:             0.80,
		ReversalWeight:         0.60,
		HeatPenaltyWeight:      0.80,
		MomentumWeight:         0.60,
		PersistenceWeight:      0.60,
		BacktestExcessWeight:   0.35,
		BacktestReturnWeight:   0.20,
		BacktestDrawdownWeight: 0.25,
		WatchPenalty:           0.02,
	}

	balanced := scanCandidate{
		Score:               0.18,
		QualityScore:        0.16,
		RiskScore:           0.12,
		ReversalScore:       0.04,
		ValueScore:          0.13,
		LowVolScore:         0.12,
		CrowdingScore:       0.02,
		FundamentalScore:    0.14,
		ValuationScore:      0.10,
		EventScore:          0.03,
		LiquidityScore:      0.04,
		HeatPenalty:         0.01,
		MomentumScore:       0.03,
		PersistenceScore:    0.05,
		BreakoutScore:       0.02,
		VolumeTrendScore:    0.01,
		RotationScore:       0.02,
		StrategyAlignment:   0.03,
		ModelScore:          0.04,
		BenchmarkModelScore: 0.56,
	}
	chased := scanCandidate{
		Score:               0.22,
		QualityScore:        0.08,
		RiskScore:           0.03,
		ReversalScore:       0.01,
		ValueScore:          -0.02,
		LowVolScore:         -0.03,
		CrowdingScore:       0.15,
		FundamentalScore:    0.01,
		ValuationScore:      -0.04,
		EventScore:          0.01,
		LiquidityScore:      0.03,
		HeatPenalty:         0.12,
		MomentumScore:       0.11,
		PersistenceScore:    0.02,
		BreakoutScore:       0.08,
		VolumeTrendScore:    0.05,
		RotationScore:       0.07,
		StrategyAlignment:   0.04,
		ModelScore:          0.05,
		BenchmarkModelScore: 0.52,
	}

	balancedScore := portfolioSelectionScore(balanced, portfolio, "risk_off")
	chasedScore := portfolioSelectionScore(chased, portfolio, "risk_off")
	if balancedScore <= chasedScore {
		t.Fatalf("expected balanced candidate to win in risk_off: balanced=%.4f chased=%.4f", balancedScore, chasedScore)
	}
}

func TestQualityPullbackOpportunityScoreRewardsSupportedSelloff(t *testing.T) {
	pullbackBars := buildTestBars(
		[]float64{10.0, 10.2, 10.4, 10.6, 10.9, 11.1, 11.4, 11.7, 11.9, 11.8, 11.5, 11.1},
		[]float64{1_200_000, 1_240_000, 1_260_000, 1_280_000, 1_320_000, 1_360_000, 1_420_000, 1_480_000, 1_500_000, 1_460_000, 1_520_000, 1_580_000},
	)
	extendedBars := buildTestBars(
		[]float64{10.0, 10.2, 10.4, 10.6, 10.9, 11.2, 11.5, 11.9, 12.3, 12.8, 13.2, 13.6},
		[]float64{1_200_000, 1_260_000, 1_280_000, 1_300_000, 1_340_000, 1_420_000, 1_520_000, 1_660_000, 1_800_000, 1_960_000, 2_100_000, 2_260_000},
	)

	pullbackShortMA, pullbackLongMA, _ := testMovingStats(pullbackBars, 5, 10)
	extendedShortMA, extendedLongMA, _ := testMovingStats(extendedBars, 5, 10)

	pullbackTrend, _, pullbackStructure, _, pullbackPersistence, pullbackBreakout, pullbackVolumeTrend, pullbackRiskPenalty, _ := scoreCandidate(pullbackBars, 5, 10)
	extendedTrend, _, extendedStructure, _, extendedPersistence, extendedBreakout, extendedVolumeTrend, extendedRiskPenalty, _ := scoreCandidate(extendedBars, 5, 10)

	pullbackValue, pullbackLowVol, pullbackCrowding, pullbackQuality, pullbackRisk, pullbackHeat, _ := candidateOverlayScores(
		pullbackBars,
		pullbackShortMA,
		pullbackLongMA,
		average([]float64{1_200_000, 1_240_000, 1_260_000, 1_280_000, 1_320_000, 1_360_000, 1_420_000, 1_480_000, 1_500_000, 1_460_000, 1_520_000, 1_580_000}[2:]),
		trailingReturn(testCloses(pullbackBars), 5),
		trailingReturn(testCloses(pullbackBars), 10),
		pullbackTrend,
		0.03,
		pullbackPersistence,
		pullbackBreakout,
		pullbackVolumeTrend,
		pullbackRiskPenalty,
	)
	extendedValue, extendedLowVol, extendedCrowding, extendedQuality, extendedRisk, extendedHeat, _ := candidateOverlayScores(
		extendedBars,
		extendedShortMA,
		extendedLongMA,
		average([]float64{1_200_000, 1_260_000, 1_280_000, 1_300_000, 1_340_000, 1_420_000, 1_520_000, 1_660_000, 1_800_000, 1_960_000, 2_100_000, 2_260_000}[2:]),
		trailingReturn(testCloses(extendedBars), 5),
		trailingReturn(testCloses(extendedBars), 10),
		extendedTrend,
		0.03,
		extendedPersistence,
		extendedBreakout,
		extendedVolumeTrend,
		extendedRiskPenalty,
	)

	pullbackScore := qualityPullbackOpportunityScore(pullbackBars, pullbackShortMA, pullbackLongMA, pullbackTrend, pullbackStructure, pullbackQuality, pullbackRisk, pullbackValue, pullbackLowVol, pullbackCrowding, pullbackHeat, 0.06, 0.03, 0.58)
	extendedScore := qualityPullbackOpportunityScore(extendedBars, extendedShortMA, extendedLongMA, extendedTrend, extendedStructure, extendedQuality, extendedRisk, extendedValue, extendedLowVol, extendedCrowding, extendedHeat, 0.06, 0.03, 0.58)
	if pullbackScore <= extendedScore {
		t.Fatalf("expected supported selloff to score higher on quality pullback: pullback=%.4f extended=%.4f", pullbackScore, extendedScore)
	}
}

func TestClassifyCandidatePromotesQualityPullbackSetup(t *testing.T) {
	bucket, setupTag, reason, trigger, triggerPrice, avoidTags := classifyCandidate(
		"测试股份",
		10.15,
		10.60,
		10.00,
		15_000_000,
		"BUY",
		0.08,
		0.14,
		0.02,
		0.04,
		"quality pullback setup is forming near long-term support",
		"Hold above 10.60 with follow-through volume",
	)

	if bucket != "建议关注" {
		t.Fatalf("bucket = %q", bucket)
	}
	if setupTag != "quality_pullback" {
		t.Fatalf("setupTag = %q", setupTag)
	}
	if triggerPrice != 10.00 {
		t.Fatalf("triggerPrice = %.2f", triggerPrice)
	}
	if len(avoidTags) != 0 {
		t.Fatalf("avoidTags = %v", avoidTags)
	}
	if reason == "" || trigger == "" {
		t.Fatalf("expected reason/trigger, got reason=%q trigger=%q", reason, trigger)
	}
}

func buildTestBars(closes []float64, volumes []float64) []marketBar {
	bars := make([]marketBar, 0, len(closes))
	for i := range closes {
		volume := 1_000_000.0
		if i < len(volumes) {
			volume = volumes[i]
		}
		closePrice := closes[i]
		bars = append(bars, marketBar{
			Date:   fmt.Sprintf("2026-01-%02d", i+1),
			Open:   closePrice * 0.99,
			High:   closePrice * 1.01,
			Low:    closePrice * 0.98,
			Close:  closePrice,
			Volume: volume,
		})
	}
	return bars
}

func testCloses(bars []marketBar) []float64 {
	closes := make([]float64, 0, len(bars))
	for _, bar := range bars {
		closes = append(closes, bar.Close)
	}
	return closes
}

func testMovingStats(bars []marketBar, shortWindow int, longWindow int) (float64, float64, float64) {
	closes := testCloses(bars)
	volumes := make([]float64, 0, len(bars))
	for _, bar := range bars {
		volumes = append(volumes, bar.Volume)
	}
	return average(closes[len(closes)-shortWindow:]), average(closes[len(closes)-longWindow:]), average(volumes[len(volumes)-longWindow:])
}
