package main

import (
	"fmt"
	"testing"
)

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
