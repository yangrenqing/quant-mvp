package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const configPath = "configs/config.yaml"
const reportsDir = "reports"
const aShareUniversePath = "data/a_share_universe.csv"
const cacheDir = "data/cache"
const aShareBenchmarkSymbol = "000300.SH"

var cachedLinearModel *linearModel
var linearModelLoaded bool

type marketKind string

const (
	marketKindAShare marketKind = "a_share"
	marketKindUS     marketKind = "us"
)

type config struct {
	AppName   string
	DB        dbConfig
	Schedule  scheduleConfig
	Strategy  strategyConfig
	Risk      riskConfig
	Portfolio portfolioConfig
	Regime    regimeConfig
}

type dbConfig struct {
	Path string
}

type scheduleConfig struct {
	DailyRun string
	CacheTTL string
}

type strategyConfig struct {
	Name        string
	Symbol      string
	DataPath    string
	DataSource  string
	APIKeyEnv   string
	ShortWindow int
	LongWindow  int
}

type riskConfig struct {
	MaxPosition      int
	StopLossPct      float64
	SkipRepeatSignal bool
}

type portfolioConfig struct {
	RebalanceIntervalDays  int
	WeightDriftThreshold   float64
	MinHoldings            int
	MaxPositionWeight      float64
	MaxCashShare           float64
	MaxVolatility          float64
	MinAverageTurnover     float64
	OverheatThreshold      float64
	MaxHoldingDrawdown     float64
	MinTrendGap            float64
	StopCooldownDays       int
	ExitCooldownDays       int
	TrendBreakCooldownDays int
	MomentumWeight         float64
	PersistenceWeight      float64
	BacktestExcessWeight   float64
	BacktestReturnWeight   float64
	BacktestDrawdownWeight float64
	WatchPenalty           float64
	TrendStrategyEnabled   bool
	BreakoutEnabled        bool
	PullbackEnabled        bool
	TrendStrategyWeight    float64
	BreakoutStrategyWeight float64
	PullbackStrategyWeight float64
	MinPrice               float64
	MinBacktestExcess      float64
	MaxBacktestDrawdown    float64
	LimitMoveThreshold     float64
	QualityWeight          float64
	RiskWeight             float64
	HeatPenaltyWeight      float64
	ReversalWeight         float64
}

type regimeConfig struct {
	CautiousExposure float64
	RiskOffExposure  float64
	RiskOffDrawdown  float64
	CautiousDrawdown float64
	BreadthRiskOff   float64
	BreadthCautious  float64
}

type marketBar struct {
	Date   string
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

type strategySignal struct {
	Symbol       string
	Action       string
	Mode         string
	Plan         string
	MarketDate   string
	ShortMA      float64
	LongMA       float64
	ClosePrice   float64
	OpenPrice    float64
	HighPrice    float64
	LowPrice     float64
	Volume       float64
	Reason       string
	DataSource   string
	PositionSize int
}

type positionState struct {
	Side       string
	Quantity   int
	EntryPrice float64
}

type scanCandidate struct {
	Symbol             string
	Name               string
	Industry           string
	Action             string
	Bucket             string
	Score              float64
	QualityScore       float64
	RiskScore          float64
	HeatPenalty        float64
	ReversalScore      float64
	TrendScore         float64
	LiquidityScore     float64
	StructureScore     float64
	MomentumScore      float64
	PersistenceScore   float64
	BreakoutScore      float64
	VolumeTrendScore   float64
	ShortReturnScore   float64
	MediumReturnScore  float64
	IndustryStrength   float64
	RotationScore      float64
	StrategyAlignment  float64
	StrategyVotes      string
	ModelScore         float64
	RiskPenalty        float64
	AvgVolume          float64
	Trigger            string
	TriggerPrice       float64
	AvoidTags          []string
	ShortMA            float64
	LongMA             float64
	ClosePrice         float64
	MarketDate         string
	Reason             string
	Plan               string
	HasBacktest        bool
	BacktestMode       string
	BacktestFrom       string
	BacktestTo         string
	BacktestReturn     float64
	BacktestAnnualized float64
	BacktestBenchmark  float64
	BacktestExcess     float64
	BacktestDrawdown   float64
	BacktestWinRate    float64
	BacktestTrades     int
	InPortfolio        bool
}

type backtestTrade struct {
	Date   string
	Action string
	Price  float64
	Shares int
	Fee    float64
	Cash   float64
	Equity float64
	Reason string
}

type backtestResult struct {
	Symbol            string
	Name              string
	FromDate          string
	ToDate            string
	InitialCash       float64
	FinalEquity       float64
	TotalReturn       float64
	MaxDrawdown       float64
	TradeCount        int
	WinRate           float64
	Mode              string
	FeeBps            float64
	SlippageBps       float64
	TotalFees         float64
	AnnualizedReturn  float64
	BenchmarkReturn   float64
	BenchmarkEquity   float64
	BenchmarkDrawdown float64
	ExcessReturn      float64
	TradingDays       int
	Trades            []backtestTrade
	EquityCurve       []backtestTrade
}

type portfolioHolding struct {
	Symbol string
	Name   string
	Shares int
	Entry  float64
}

type portfolioSnapshot struct {
	Date     string
	Equity   float64
	Cash     float64
	Holdings []portfolioHolding
}

type portfolioBacktestResult struct {
	FromDate         string
	ToDate           string
	InitialCash      float64
	FinalEquity      float64
	TotalReturn      float64
	AnnualizedReturn float64
	BenchmarkReturn  float64
	ExcessReturn     float64
	MaxDrawdown      float64
	Mode             string
	FeeBps           float64
	SlippageBps      float64
	RebalanceCount   int
	TradingDays      int
	Positions        int
	Snapshots        []portfolioSnapshot
	BenchmarkCurve   []backtestTrade
	LatestSelection  []scanCandidate
	CurrentHoldings  []portfolioHolding
	ExposureLevel    float64
	RegimeLabel      string
}

type gridSearchResult struct {
	ShortWindow      int
	LongWindow       int
	FinalEquity      float64
	TotalReturn      float64
	AnnualizedReturn float64
	BenchmarkReturn  float64
	ExcessReturn     float64
	MaxDrawdown      float64
	Rebalances       int
}

type datasetRow struct {
	Symbol            string
	Name              string
	Industry          string
	Date              string
	Close             float64
	Volume            float64
	ShortMA           float64
	LongMA            float64
	Score             float64
	QualityScore      float64
	RiskScore         float64
	HeatPenalty       float64
	ReversalScore     float64
	TrendScore        float64
	LiquidityScore    float64
	StructureScore    float64
	MomentumScore     float64
	PersistenceScore  float64
	BreakoutScore     float64
	VolumeTrendScore  float64
	ShortReturnScore  float64
	MediumReturnScore float64
	RotationScore     float64
	StrategyAlignment float64
	Breadth           float64
	RegimeExposure    float64
	Label5D           float64
	Label10D          float64
	Label20D          float64
}

type linearModelFeature struct {
	Feature string  `json:"feature"`
	Weight  float64 `json:"weight"`
	Mean    float64 `json:"mean"`
	Std     float64 `json:"std"`
}

type linearModel struct {
	Label          string               `json:"label"`
	Bias           float64              `json:"bias"`
	Features       []linearModelFeature `json:"features"`
	RollingMetrics []map[string]any     `json:"rolling_metrics"`
}

type marketSeries struct {
	meta aShareSymbol
	bars []marketBar
}

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	symbolOverride := flag.String("symbol", "", "Override the configured symbol for this run")
	once := flag.Bool("once", false, "Run the strategy once and exit")
	scanAShare := flag.Bool("scan-a-share", false, "Scan the full A-share universe and generate a ranked report")
	topN := flag.Int("top", 10, "Number of candidates to keep in the A-share scan report")
	backtest := flag.Bool("backtest", false, "Run a single-symbol backtest")
	backtestScan := flag.Bool("backtest-scan", false, "Run a backtest across the local A-share universe")
	portfolioBacktest := flag.Bool("portfolio-backtest", false, "Run a portfolio backtest across the local A-share universe")
	gridSearch := flag.Bool("grid-search", false, "Run a portfolio parameter grid search across short/long windows")
	exportDataset := flag.Bool("export-dataset", false, "Export a training dataset with factor features and forward-return labels")
	fromDate := flag.String("from", "", "Backtest start date in YYYY-MM-DD")
	toDate := flag.String("to", "", "Backtest end date in YYYY-MM-DD")
	initialCash := flag.Float64("cash", 100000, "Backtest initial cash")
	feeBps := flag.Float64("fee-bps", 10, "Backtest transaction fee in basis points")
	slippageBps := flag.Float64("slippage-bps", 5, "Backtest slippage in basis points")
	shortMin := flag.Int("short-min", 3, "Grid search minimum short window")
	shortMax := flag.Int("short-max", 8, "Grid search maximum short window")
	longMin := flag.Int("long-min", 8, "Grid search minimum long window")
	longMax := flag.Int("long-max", 20, "Grid search maximum long window")
	flag.Parse()

	cfg, err := loadConfig(configPath)
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}
	if strings.TrimSpace(*symbolOverride) != "" {
		cfg.Strategy.Symbol = strings.TrimSpace(*symbolOverride)
	}

	if err := ensureSQLiteDB(cfg.DB.Path); err != nil {
		logger.Fatalf("ensure sqlite db: %v", err)
	}
	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		logger.Fatalf("ensure reports dir: %v", err)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		logger.Fatalf("ensure cache dir: %v", err)
	}
	if *backtest {
		if strings.TrimSpace(cfg.Strategy.Symbol) == "" {
			logger.Fatalf("backtest requires a symbol")
		}
		if *fromDate == "" || *toDate == "" {
			logger.Fatalf("backtest requires --from and --to")
		}
		result, err := runBacktest(cfg.Strategy, cfg.Risk, *fromDate, *toDate, *initialCash, *feeBps, *slippageBps)
		if err != nil {
			logger.Fatalf("backtest failed: %v", err)
		}
		if err := writeBacktestReports(result); err != nil {
			logger.Fatalf("write backtest reports: %v", err)
		}
		printBacktestSummary(result)
		return
	}
	if *backtestScan {
		if *fromDate == "" || *toDate == "" {
			logger.Fatalf("backtest scan requires --from and --to")
		}
		results, err := runBatchBacktest(cfg.Strategy, cfg.Risk, *fromDate, *toDate, *initialCash, *feeBps, *slippageBps, *topN)
		if err != nil {
			logger.Fatalf("backtest scan failed: %v", err)
		}
		if err := writeBatchBacktestReports(results, *fromDate, *toDate, *initialCash, *feeBps, *slippageBps); err != nil {
			logger.Fatalf("write batch backtest reports: %v", err)
		}
		fmt.Printf("Batch backtest complete. %d results written to %s and %s\n\n", len(results), filepath.Join(reportsDir, "backtest_scan.txt"), filepath.Join(reportsDir, "backtest_scan.html"))
		return
	}
	if *portfolioBacktest {
		if *fromDate == "" || *toDate == "" {
			logger.Fatalf("portfolio backtest requires --from and --to")
		}
		result, err := runPortfolioBacktest(cfg.Strategy, cfg.Risk, cfg.Portfolio, cfg.Regime, *fromDate, *toDate, *initialCash, *feeBps, *slippageBps, *topN)
		if err != nil {
			logger.Fatalf("portfolio backtest failed: %v", err)
		}
		if err := writePortfolioBacktestReports(result); err != nil {
			logger.Fatalf("write portfolio backtest reports: %v", err)
		}
		printPortfolioBacktestSummary(result)
		return
	}
	if *gridSearch {
		if *fromDate == "" || *toDate == "" {
			logger.Fatalf("grid search requires --from and --to")
		}
		results, err := runPortfolioGridSearch(cfg.Strategy, cfg.Risk, cfg.Portfolio, cfg.Regime, *fromDate, *toDate, *initialCash, *feeBps, *slippageBps, *topN, *shortMin, *shortMax, *longMin, *longMax)
		if err != nil {
			logger.Fatalf("grid search failed: %v", err)
		}
		if err := writeGridSearchReports(results, *fromDate, *toDate); err != nil {
			logger.Fatalf("write grid search reports: %v", err)
		}
		printGridSearchSummary(results, *fromDate, *toDate)
		return
	}
	if *exportDataset {
		if *fromDate == "" || *toDate == "" {
			logger.Fatalf("dataset export requires --from and --to")
		}
		rows, err := exportTrainingDataset(cfg.Strategy, cfg.Portfolio, cfg.Regime, *fromDate, *toDate)
		if err != nil {
			logger.Fatalf("dataset export failed: %v", err)
		}
		if err := writeDatasetReports(rows, *fromDate, *toDate); err != nil {
			logger.Fatalf("write dataset reports: %v", err)
		}
		fmt.Printf("Dataset export complete. %d rows written to %s and %s\n\n", len(rows), filepath.Join(reportsDir, "training_dataset.csv"), filepath.Join(reportsDir, "training_dataset.txt"))
		return
	}
	if *scanAShare {
		if err := runAShareScan(cfg.Strategy, cfg.Portfolio, *topN); err != nil {
			logger.Fatalf("a-share scan failed: %v", err)
		}
		return
	}

	logger.Printf("scheduler started for %s", cfg.AppName)
	if err := runStrategy(cfg.DB.Path, cfg.Strategy, cfg.Risk); err != nil {
		logger.Printf("initial run failed: %v", err)
	} else {
		logger.Printf("initial run finished")
	}

	if *once {
		return
	}

	for {
		nextRun, err := nextRunTime(cfg.Schedule.DailyRun, time.Now())
		if err != nil {
			logger.Fatalf("parse schedule: %v", err)
		}

		wait := time.Until(nextRun)
		logger.Printf("next run at %s", nextRun.Format(time.RFC3339))
		time.Sleep(wait)

		if err := runStrategy(cfg.DB.Path, cfg.Strategy, cfg.Risk); err != nil {
			logger.Printf("scheduled run failed: %v", err)
		} else {
			logger.Printf("scheduled run finished")
		}
	}
}

func runAShareScan(strategy strategyConfig, portfolio portfolioConfig, topN int) error {
	if topN <= 0 {
		topN = 10
	}

	symbols, err := loadAShareUniverse()
	if err != nil {
		return err
	}
	backtestSnapshot, _ := loadBacktestSnapshot(filepath.Join(reportsDir, "backtest_scan.csv"))
	portfolioSnapshot, _ := loadPortfolioHoldingsSnapshot(filepath.Join(reportsDir, "portfolio_backtest.csv"))

	candidates := make([]scanCandidate, 0, len(symbols))
	for _, symbol := range symbols {
		bars, dataSource, sourceErr, err := loadSymbolBars(symbol.Symbol, "auto", "", "ALPHAVANTAGE_API_KEY", false)
		if err != nil || len(bars) < strategy.LongWindow {
			continue
		}

		candidate, err := rankCandidate(symbol.Symbol, symbol.Name, symbol.Industry, bars, dataSource, sourceErr, strategy, portfolio)
		if err != nil {
			continue
		}
		if metrics, ok := backtestSnapshot[candidate.Symbol]; ok {
			applyBacktestMetrics(&candidate, metrics)
		}
		if portfolioSnapshot[candidate.Symbol] {
			candidate.InPortfolio = true
		}
		candidates = append(candidates, candidate)
	}

	if len(candidates) == 0 {
		return errors.New("no A-share candidates were generated")
	}

	sort.Slice(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if bucketPriority(left.Bucket) != bucketPriority(right.Bucket) {
			return bucketPriority(left.Bucket) < bucketPriority(right.Bucket)
		}
		return left.Score > right.Score
	})

	if topN > len(candidates) {
		topN = len(candidates)
	}
	selected := candidates[:topN]

	if err := writeAShareScanReports(selected); err != nil {
		return err
	}

	fmt.Printf("A-share scan complete. Top %d candidates written to %s and %s\n", topN, filepath.Join(reportsDir, "a_share_scan.txt"), filepath.Join(reportsDir, "a_share_scan.html"))
	for i, candidate := range selected {
		fmt.Printf("%d. [%s] %s %s %s score=%.4f close=%.2f\n", i+1, candidate.Bucket, candidate.Symbol, candidate.Name, candidate.Action, candidate.Score, candidate.ClosePrice)
	}
	fmt.Println()

	return nil
}

func runBacktest(strategy strategyConfig, risk riskConfig, fromDate string, toDate string, initialCash float64, feeBps float64, slippageBps float64) (backtestResult, error) {
	bars, dataSource, _, err := loadBars(strategy)
	if err != nil {
		return backtestResult{}, err
	}
	return simulateBacktest(strategy.Symbol, "", bars, modeFromDataSource(dataSource), strategy.ShortWindow, strategy.LongWindow, risk, fromDate, toDate, initialCash, feeBps, slippageBps)
}

func runBatchBacktest(strategy strategyConfig, risk riskConfig, fromDate string, toDate string, initialCash float64, feeBps float64, slippageBps float64, topN int) ([]backtestResult, error) {
	symbols, err := loadAShareUniverse()
	if err != nil {
		return nil, err
	}

	results := make([]backtestResult, 0, len(symbols))
	for _, symbol := range symbols {
		bars, dataSource, _, err := loadSymbolBars(symbol.Symbol, "auto", "", "ALPHAVANTAGE_API_KEY", false)
		mode := modeFromDataSource(dataSource)
		if err != nil {
			continue
		}

		result, err := simulateBacktest(symbol.Symbol, symbol.Name, bars, mode, strategy.ShortWindow, strategy.LongWindow, risk, fromDate, toDate, initialCash, feeBps, slippageBps)
		if err != nil {
			continue
		}
		results = append(results, result)
	}

	if len(results) == 0 {
		return nil, errors.New("no backtest results were generated")
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].TotalReturn != results[j].TotalReturn {
			return results[i].TotalReturn > results[j].TotalReturn
		}
		return results[i].MaxDrawdown < results[j].MaxDrawdown
	})

	if topN > 0 && topN < len(results) {
		results = results[:topN]
	}
	return results, nil
}

func runPortfolioBacktest(strategy strategyConfig, risk riskConfig, portfolio portfolioConfig, regime regimeConfig, fromDate string, toDate string, initialCash float64, feeBps float64, slippageBps float64, topN int) (portfolioBacktestResult, error) {
	symbols, err := loadAShareUniverse()
	if err != nil {
		return portfolioBacktestResult{}, err
	}
	if topN <= 0 {
		topN = 5
	}

	series := make([]marketSeries, 0, len(symbols))
	mode := "live"
	for _, symbol := range symbols {
		bars, dataSource, _, err := loadSymbolBars(symbol.Symbol, "auto", "", "ALPHAVANTAGE_API_KEY", false)
		if err != nil {
			continue
		}
		if modeFromDataSource(dataSource) == "test" {
			mode = "test"
		}
		series = append(series, marketSeries{meta: symbol, bars: bars})
	}
	if len(series) == 0 {
		return portfolioBacktestResult{}, errors.New("no market data available for portfolio backtest")
	}
	backtestSnapshot, _ := loadBacktestSnapshot(filepath.Join(reportsDir, "backtest_scan.csv"))
	benchmarkBars, benchmarkErr := loadAShareBenchmarkBars()
	if benchmarkErr != nil || len(benchmarkBars) < strategy.LongWindow {
		benchmarkBars = nil
	}

	dates := tradingDatesInRange(series[0].bars, fromDate, toDate)
	if len(dates) == 0 {
		return portfolioBacktestResult{}, errors.New("no trading dates available in requested range")
	}

	barBySymbolDate := make(map[string]map[string]marketBar, len(series))
	for _, item := range series {
		dateMap := make(map[string]marketBar, len(item.bars))
		for _, bar := range item.bars {
			dateMap[bar.Date] = bar
		}
		barBySymbolDate[item.meta.Symbol] = dateMap
	}

	feeRate := feeBps / 10000
	slippageRate := slippageBps / 10000
	cash := initialCash
	holdings := map[string]int{}
	entryPrices := map[string]float64{}
	holdingPeaks := map[string]float64{}
	cooldownUntil := map[string]string{}
	snapshots := make([]portfolioSnapshot, 0, len(dates))
	peakEquity := initialCash
	maxDrawdown := 0.0
	rebalanceCount := 0
	latestSelection := make([]scanCandidate, 0)
	lastRegimeLabel := "neutral"
	lastExposureLevel := 1.0

	for dayIdx, date := range dates {
		candidates := make([]scanCandidate, 0, len(series))
		for _, item := range series {
			history := barsUpToDate(item.bars, date)
			if len(history) < strategy.LongWindow {
				continue
			}
			candidate, err := rankCandidate(item.meta.Symbol, item.meta.Name, item.meta.Industry, history, "baostock", "", strategy, portfolio)
			if err != nil {
				continue
			}
			if metrics, ok := backtestSnapshot[candidate.Symbol]; ok {
				applyBacktestMetrics(&candidate, metrics)
			}
			if cooldownUntil[item.meta.Symbol] != "" && date <= cooldownUntil[item.meta.Symbol] {
				continue
			}
			if candidate.Score > 0 && candidate.Bucket != "回避" && passPortfolioCandidateFilters(history, candidate, portfolio.MinAverageTurnover, portfolio.MaxVolatility, portfolio.OverheatThreshold, portfolio.MinTrendGap, portfolio.MinPrice, portfolio.MinBacktestExcess, portfolio.MaxBacktestDrawdown) {
				candidates = append(candidates, candidate)
			}
		}
		applyRotationOverlay(candidates)

		relativeStrengthFloor := candidateMedianScore(candidates)
		breadth := marketBreadth(candidates)
		regimeLabel, targetExposure := benchmarkMarketRegime(barsUpToDate(benchmarkBars, date), breadth, strategy.LongWindow, regime)
		lastRegimeLabel = regimeLabel
		lastExposureLevel = targetExposure
		if targetExposure > 0 {
			filtered := make([]scanCandidate, 0, len(candidates))
			for _, candidate := range candidates {
				if candidate.Score >= relativeStrengthFloor {
					filtered = append(filtered, candidate)
				}
			}
			candidates = filtered
		} else {
			candidates = nil
		}

		candidates = selectPortfolioCandidates(candidates, topN, portfolio.MinHoldings, portfolio)
		if len(candidates) > 0 {
			latestSelection = append([]scanCandidate(nil), candidates...)
		}

		targetSet := make(map[string]scanCandidate, len(candidates))
		for _, candidate := range candidates {
			targetSet[candidate.Symbol] = candidate
		}

		dayEquity := cash
		for symbol, shares := range holdings {
			if shares <= 0 {
				continue
			}
			bar, ok := barBySymbolDate[symbol][date]
			if !ok {
				continue
			}
			dayEquity += float64(shares) * bar.Close
		}

		shouldRebalance := dayIdx == 0 || dayIdx%portfolio.RebalanceIntervalDays == 0

		// Always remove names that dropped out of the target set.
		for symbol, shares := range holdings {
			if shares <= 0 {
				continue
			}
			bar, ok := barBySymbolDate[symbol][date]
			if !ok {
				continue
			}
			if bar.Close > holdingPeaks[symbol] {
				holdingPeaks[symbol] = bar.Close
			}
			if entryPrice := entryPrices[symbol]; entryPrice > 0 && bar.Close <= entryPrice*(1-risk.StopLossPct) {
				if isSellRestricted(bar, portfolio.LimitMoveThreshold) {
					continue
				}
				execPrice := bar.Close * (1 - slippageRate)
				fee := float64(shares) * execPrice * feeRate
				cash += float64(shares)*execPrice - fee
				holdings[symbol] = 0
				delete(entryPrices, symbol)
				delete(holdingPeaks, symbol)
				cooldownUntil[symbol] = portfolioCooldownDate(date, portfolio.StopCooldownDays)
				rebalanceCount++
				continue
			}
			if peak := holdingPeaks[symbol]; peak > 0 && bar.Close <= peak*(1-portfolio.MaxHoldingDrawdown) {
				if isSellRestricted(bar, portfolio.LimitMoveThreshold) {
					continue
				}
				execPrice := bar.Close * (1 - slippageRate)
				fee := float64(shares) * execPrice * feeRate
				cash += float64(shares)*execPrice - fee
				holdings[symbol] = 0
				delete(entryPrices, symbol)
				delete(holdingPeaks, symbol)
				cooldownUntil[symbol] = portfolioCooldownDate(date, portfolio.StopCooldownDays)
				rebalanceCount++
				continue
			}
			if candidate, keep := targetSet[symbol]; keep && candidate.Action == "SELL" {
				if isSellRestricted(bar, portfolio.LimitMoveThreshold) {
					continue
				}
				execPrice := bar.Close * (1 - slippageRate)
				fee := float64(shares) * execPrice * feeRate
				cash += float64(shares)*execPrice - fee
				holdings[symbol] = 0
				delete(entryPrices, symbol)
				delete(holdingPeaks, symbol)
				cooldownUntil[symbol] = portfolioCooldownDate(date, portfolio.TrendBreakCooldownDays)
				rebalanceCount++
				continue
			}
			if _, keep := targetSet[symbol]; keep {
				continue
			}
			if isSellRestricted(bar, portfolio.LimitMoveThreshold) {
				continue
			}
			execPrice := bar.Close * (1 - slippageRate)
			fee := float64(shares) * execPrice * feeRate
			cash += float64(shares)*execPrice - fee
			holdings[symbol] = 0
			delete(entryPrices, symbol)
			delete(holdingPeaks, symbol)
			cooldownUntil[symbol] = portfolioCooldownDate(date, portfolio.ExitCooldownDays)
			rebalanceCount++
		}

		if len(candidates) > 0 && shouldRebalance {
			targetValue := cash
			for symbol, shares := range holdings {
				if shares <= 0 {
					continue
				}
				if bar, ok := barBySymbolDate[symbol][date]; ok {
					targetValue += float64(shares) * bar.Close
				}
			}
			targetSlots := len(candidates)
			if targetSlots < portfolio.MinHoldings {
				targetSlots = portfolio.MinHoldings
			}
			effectiveCashShare := portfolio.MaxCashShare + (1 - targetExposure)
			if effectiveCashShare > 0.85 {
				effectiveCashShare = 0.85
			}
			deployableCapital := targetValue * (1 - effectiveCashShare)
			slotValue := deployableCapital / float64(targetSlots)
			maxSlotValue := targetValue * portfolio.MaxPositionWeight
			if slotValue > maxSlotValue {
				slotValue = maxSlotValue
			}

			for _, candidate := range candidates {
				bar, ok := barBySymbolDate[candidate.Symbol][date]
				if !ok {
					continue
				}
				currentShares := holdings[candidate.Symbol]
				currentValue := float64(currentShares) * bar.Close
				targetShares := int(slotValue / (bar.Close * (1 + feeRate + slippageRate)))
				if targetShares < 0 {
					targetShares = 0
				}
				targetValueForName := float64(targetShares) * bar.Close
				drift := 1.0
				if targetValueForName > 0 {
					drift = math.Abs(currentValue-targetValueForName) / targetValueForName
				}
				if currentShares > 0 && drift < portfolio.WeightDriftThreshold {
					continue
				}
				diff := targetShares - currentShares
				if diff == 0 {
					continue
				}

				if diff < 0 {
					sellShares := -diff
					if isSellRestricted(bar, portfolio.LimitMoveThreshold) {
						continue
					}
					execPrice := bar.Close * (1 - slippageRate)
					fee := float64(sellShares) * execPrice * feeRate
					cash += float64(sellShares)*execPrice - fee
					holdings[candidate.Symbol] = currentShares - sellShares
					if holdings[candidate.Symbol] <= 0 {
						delete(entryPrices, candidate.Symbol)
						delete(holdingPeaks, candidate.Symbol)
					}
					rebalanceCount++
					continue
				}

				execPrice := bar.Close * (1 + slippageRate)
				if isBuyRestricted(bar, portfolio.LimitMoveThreshold) {
					continue
				}
				cost := float64(diff) * execPrice
				fee := cost * feeRate
				if cost+fee > cash {
					maxAffordable := int(cash / (execPrice * (1 + feeRate)))
					diff = maxAffordable
					if diff <= 0 {
						continue
					}
					cost = float64(diff) * execPrice
					fee = cost * feeRate
				}
				cash -= cost + fee
				holdings[candidate.Symbol] = currentShares + diff
				if currentShares == 0 {
					entryPrices[candidate.Symbol] = execPrice
					holdingPeaks[candidate.Symbol] = bar.Close
				}
				rebalanceCount++
			}
		}

		holdingsList := make([]portfolioHolding, 0)
		equity := cash
		for _, item := range series {
			shares := holdings[item.meta.Symbol]
			if shares <= 0 {
				continue
			}
			bar, ok := barBySymbolDate[item.meta.Symbol][date]
			if !ok {
				continue
			}
			equity += float64(shares) * bar.Close
			holdingsList = append(holdingsList, portfolioHolding{
				Symbol: item.meta.Symbol,
				Name:   item.meta.Name,
				Shares: shares,
				Entry:  entryPrices[item.meta.Symbol],
			})
		}
		sort.Slice(holdingsList, func(i, j int) bool { return holdingsList[i].Symbol < holdingsList[j].Symbol })
		snapshots = append(snapshots, portfolioSnapshot{
			Date:     date,
			Equity:   equity,
			Cash:     cash,
			Holdings: holdingsList,
		})

		if equity > peakEquity {
			peakEquity = equity
		}
		if peakEquity > 0 {
			drawdown := (peakEquity - equity) / peakEquity
			if drawdown > maxDrawdown {
				maxDrawdown = drawdown
			}
		}
	}

	if len(snapshots) == 0 {
		return portfolioBacktestResult{}, errors.New("portfolio backtest produced no snapshots")
	}

	finalEquity := snapshots[len(snapshots)-1].Equity
	currentHoldings := snapshots[len(snapshots)-1].Holdings
	benchmarkReturn := 0.0
	benchmarkCurve := make([]backtestTrade, 0)
	if len(benchmarkBars) > 0 {
		benchmarkCurve = buildBenchmarkCurve(benchmarkBars, fromDate, toDate, initialCash, feeRate, slippageRate)
	}
	if len(benchmarkCurve) == 0 {
		benchmarkCurve = buildUniverseBenchmarkCurve(series, dates, initialCash)
	}
	if len(benchmarkCurve) > 0 {
		benchmarkReturn = (benchmarkCurve[len(benchmarkCurve)-1].Equity - initialCash) / initialCash
	}

	return portfolioBacktestResult{
		FromDate:         fromDate,
		ToDate:           toDate,
		InitialCash:      initialCash,
		FinalEquity:      finalEquity,
		TotalReturn:      (finalEquity - initialCash) / initialCash,
		AnnualizedReturn: annualizeReturn(finalEquity/initialCash, len(snapshots)),
		BenchmarkReturn:  benchmarkReturn,
		ExcessReturn:     ((finalEquity - initialCash) / initialCash) - benchmarkReturn,
		MaxDrawdown:      maxDrawdown,
		Mode:             mode,
		FeeBps:           feeBps,
		SlippageBps:      slippageBps,
		RebalanceCount:   rebalanceCount,
		TradingDays:      len(snapshots),
		Positions:        topN,
		Snapshots:        snapshots,
		BenchmarkCurve:   benchmarkCurve,
		LatestSelection:  latestSelection,
		CurrentHoldings:  currentHoldings,
		ExposureLevel:    lastExposureLevel,
		RegimeLabel:      lastRegimeLabel,
	}, nil
}

func runPortfolioGridSearch(strategy strategyConfig, risk riskConfig, portfolio portfolioConfig, regime regimeConfig, fromDate string, toDate string, initialCash float64, feeBps float64, slippageBps float64, topN int, shortMin int, shortMax int, longMin int, longMax int) ([]gridSearchResult, error) {
	if shortMin <= 0 || longMin <= 0 {
		return nil, errors.New("grid search windows must be positive")
	}
	if shortMin > shortMax || longMin > longMax {
		return nil, errors.New("grid search min window cannot exceed max window")
	}

	results := make([]gridSearchResult, 0)
	for shortWindow := shortMin; shortWindow <= shortMax; shortWindow++ {
		for longWindow := longMin; longWindow <= longMax; longWindow++ {
			if shortWindow >= longWindow {
				continue
			}
			strategyVariant := strategy
			strategyVariant.ShortWindow = shortWindow
			strategyVariant.LongWindow = longWindow

			result, err := runPortfolioBacktest(strategyVariant, risk, portfolio, regime, fromDate, toDate, initialCash, feeBps, slippageBps, topN)
			if err != nil {
				continue
			}
			results = append(results, gridSearchResult{
				ShortWindow:      shortWindow,
				LongWindow:       longWindow,
				FinalEquity:      result.FinalEquity,
				TotalReturn:      result.TotalReturn,
				AnnualizedReturn: result.AnnualizedReturn,
				BenchmarkReturn:  result.BenchmarkReturn,
				ExcessReturn:     result.ExcessReturn,
				MaxDrawdown:      result.MaxDrawdown,
				Rebalances:       result.RebalanceCount,
			})
		}
	}
	if len(results) == 0 {
		return nil, errors.New("no valid grid search results were generated")
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].TotalReturn != results[j].TotalReturn {
			return results[i].TotalReturn > results[j].TotalReturn
		}
		if results[i].MaxDrawdown != results[j].MaxDrawdown {
			return results[i].MaxDrawdown < results[j].MaxDrawdown
		}
		return results[i].ExcessReturn > results[j].ExcessReturn
	})
	return results, nil
}

func exportTrainingDataset(strategy strategyConfig, portfolio portfolioConfig, regime regimeConfig, fromDate string, toDate string) ([]datasetRow, error) {
	symbols, err := loadAShareUniverse()
	if err != nil {
		return nil, err
	}

	series := make([]marketSeries, 0, len(symbols))
	for _, symbol := range symbols {
		bars, _, _, err := loadSymbolBars(symbol.Symbol, "auto", "", "ALPHAVANTAGE_API_KEY", false)
		if err != nil || len(bars) < max(strategy.LongWindow, 21) {
			continue
		}
		series = append(series, marketSeries{meta: symbol, bars: bars})
	}
	if len(series) == 0 {
		return nil, errors.New("no market data available for dataset export")
	}

	dates := tradingDatesInRange(series[0].bars, fromDate, toDate)
	if len(dates) == 0 {
		return nil, errors.New("no trading dates available in requested range")
	}

	benchmarkBars, _ := loadAShareBenchmarkBars()
	rows := make([]datasetRow, 0, len(series)*len(dates))
	for _, item := range series {
		for _, date := range dates {
			history := barsUpToDate(item.bars, date)
			if len(history) < strategy.LongWindow {
				continue
			}
			candidate, err := rankCandidate(item.meta.Symbol, item.meta.Name, item.meta.Industry, history, "baostock", "", strategy, portfolio)
			if err != nil {
				continue
			}
			breadth := 0.0
			exposure := 1.0
			if len(benchmarkBars) >= strategy.LongWindow {
				breadth = 0.5
				_, exposure = benchmarkMarketRegime(barsUpToDate(benchmarkBars, date), breadth, strategy.LongWindow, regime)
			}
			label5 := forwardReturn(item.bars, date, 5)
			label10 := forwardReturn(item.bars, date, 10)
			label20 := forwardReturn(item.bars, date, 20)
			if math.IsNaN(label5) || math.IsNaN(label10) || math.IsNaN(label20) {
				continue
			}
			rows = append(rows, datasetRow{
				Symbol:            candidate.Symbol,
				Name:              candidate.Name,
				Industry:          candidate.Industry,
				Date:              candidate.MarketDate,
				Close:             candidate.ClosePrice,
				Volume:            candidate.AvgVolume,
				ShortMA:           candidate.ShortMA,
				LongMA:            candidate.LongMA,
				Score:             candidate.Score,
				QualityScore:      candidate.QualityScore,
				RiskScore:         candidate.RiskScore,
				HeatPenalty:       candidate.HeatPenalty,
				ReversalScore:     candidate.ReversalScore,
				TrendScore:        candidate.TrendScore,
				LiquidityScore:    candidate.LiquidityScore,
				StructureScore:    candidate.StructureScore,
				MomentumScore:     candidate.MomentumScore,
				PersistenceScore:  candidate.PersistenceScore,
				BreakoutScore:     candidate.BreakoutScore,
				VolumeTrendScore:  candidate.VolumeTrendScore,
				ShortReturnScore:  candidate.ShortReturnScore,
				MediumReturnScore: candidate.MediumReturnScore,
				RotationScore:     candidate.RotationScore,
				StrategyAlignment: candidate.StrategyAlignment,
				Breadth:           breadth,
				RegimeExposure:    exposure,
				Label5D:           label5,
				Label10D:          label10,
				Label20D:          label20,
			})
		}
	}
	if len(rows) == 0 {
		return nil, errors.New("dataset export produced no rows")
	}
	return rows, nil
}

func forwardReturn(bars []marketBar, date string, horizon int) float64 {
	startIdx := -1
	for i, bar := range bars {
		if bar.Date == date {
			startIdx = i
			break
		}
	}
	if startIdx < 0 || startIdx+horizon >= len(bars) {
		return math.NaN()
	}
	start := bars[startIdx].Close
	end := bars[startIdx+horizon].Close
	if start <= 0 {
		return math.NaN()
	}
	return end/start - 1
}

func simulateBacktest(symbol string, name string, bars []marketBar, mode string, shortWindow int, longWindow int, risk riskConfig, fromDate string, toDate string, initialCash float64, feeBps float64, slippageBps float64) (backtestResult, error) {

	filtered := make([]marketBar, 0, len(bars))
	for _, bar := range bars {
		if bar.Date >= fromDate && bar.Date <= toDate {
			filtered = append(filtered, bar)
		}
	}
	if len(filtered) < longWindow {
		return backtestResult{}, fmt.Errorf("not enough bars in backtest window: need %d, got %d", longWindow, len(filtered))
	}

	cash := initialCash
	shares := 0
	entryPrice := 0.0
	trades := make([]backtestTrade, 0)
	equityCurve := make([]backtestTrade, 0, len(filtered))
	winningTrades := 0
	completedTrades := 0
	peakEquity := initialCash
	maxDrawdown := 0.0
	totalFees := 0.0
	feeRate := feeBps / 10000
	slippageRate := slippageBps / 10000

	closes := make([]float64, 0, len(filtered))
	for i, bar := range filtered {
		closes = append(closes, bar.Close)
		equity := cash + float64(shares)*bar.Close
		if equity > peakEquity {
			peakEquity = equity
		}
		drawdown := 0.0
		if peakEquity > 0 {
			drawdown = (peakEquity - equity) / peakEquity
		}
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}

		equityCurve = append(equityCurve, backtestTrade{
			Date:   bar.Date,
			Action: "MARK",
			Price:  bar.Close,
			Shares: shares,
			Cash:   cash,
			Equity: equity,
		})

		if i+1 < longWindow {
			continue
		}

		shortMA := average(closes[len(closes)-shortWindow:])
		longMA := average(closes[len(closes)-longWindow:])
		action := ""
		reason := ""

		if shares > 0 && bar.Close <= entryPrice*(1-risk.StopLossPct) {
			action = "SELL"
			reason = fmt.Sprintf("stop loss triggered at %.2f%%", risk.StopLossPct*100)
		} else if shares == 0 && shortMA > longMA {
			action = "BUY"
			reason = "short moving average crossed above long moving average"
		} else if shares > 0 && shortMA < longMA {
			action = "SELL"
			reason = "short moving average crossed below long moving average"
		}

		switch action {
		case "BUY":
			execPrice := bar.Close * (1 + slippageRate)
			buyShares := int(cash / (execPrice * (1 + feeRate)))
			if buyShares <= 0 {
				continue
			}
			fee := float64(buyShares) * execPrice * feeRate
			cash -= float64(buyShares)*execPrice + fee
			totalFees += fee
			shares = buyShares
			entryPrice = execPrice
			trades = append(trades, backtestTrade{
				Date:   bar.Date,
				Action: action,
				Price:  execPrice,
				Shares: buyShares,
				Fee:    fee,
				Cash:   cash,
				Equity: cash + float64(shares)*bar.Close,
				Reason: reason,
			})
		case "SELL":
			if shares <= 0 {
				continue
			}
			execPrice := bar.Close * (1 - slippageRate)
			fee := float64(shares) * execPrice * feeRate
			proceeds := float64(shares)*execPrice - fee
			cash += proceeds
			totalFees += fee
			pnl := (execPrice - entryPrice) * float64(shares)
			completedTrades++
			if pnl > 0 {
				winningTrades++
			}
			trades = append(trades, backtestTrade{
				Date:   bar.Date,
				Action: action,
				Price:  execPrice,
				Shares: shares,
				Fee:    fee,
				Cash:   cash,
				Equity: cash,
				Reason: reason,
			})
			shares = 0
			entryPrice = 0
		}
	}

	finalEquity := cash
	if len(filtered) > 0 && shares > 0 {
		finalEquity += float64(shares) * filtered[len(filtered)-1].Close
	}
	winRate := 0.0
	if completedTrades > 0 {
		winRate = float64(winningTrades) / float64(completedTrades)
	}
	benchmarkEquity, benchmarkReturn, benchmarkDrawdown := simulateBuyAndHoldBenchmark(filtered, initialCash, feeRate, slippageRate)
	annualizedReturn := annualizeReturn(finalEquity/initialCash, len(filtered))

	return backtestResult{
		Symbol:            symbol,
		Name:              name,
		FromDate:          fromDate,
		ToDate:            toDate,
		InitialCash:       initialCash,
		FinalEquity:       finalEquity,
		TotalReturn:       (finalEquity - initialCash) / initialCash,
		MaxDrawdown:       maxDrawdown,
		TradeCount:        len(trades),
		WinRate:           winRate,
		Mode:              mode,
		FeeBps:            feeBps,
		SlippageBps:       slippageBps,
		TotalFees:         totalFees,
		AnnualizedReturn:  annualizedReturn,
		BenchmarkReturn:   benchmarkReturn,
		BenchmarkEquity:   benchmarkEquity,
		BenchmarkDrawdown: benchmarkDrawdown,
		ExcessReturn:      ((finalEquity - initialCash) / initialCash) - benchmarkReturn,
		TradingDays:       len(filtered),
		Trades:            trades,
		EquityCurve:       equityCurve,
	}, nil
}

func simulateBuyAndHoldBenchmark(bars []marketBar, initialCash float64, feeRate float64, slippageRate float64) (float64, float64, float64) {
	if len(bars) == 0 {
		return initialCash, 0, 0
	}

	buyPrice := bars[0].Close * (1 + slippageRate)
	shares := int(initialCash / (buyPrice * (1 + feeRate)))
	if shares <= 0 {
		return initialCash, 0, 0
	}

	buyFee := float64(shares) * buyPrice * feeRate
	cash := initialCash - float64(shares)*buyPrice - buyFee
	peakEquity := initialCash
	maxDrawdown := 0.0

	for _, bar := range bars {
		equity := cash + float64(shares)*bar.Close
		if equity > peakEquity {
			peakEquity = equity
		}
		if peakEquity > 0 {
			drawdown := (peakEquity - equity) / peakEquity
			if drawdown > maxDrawdown {
				maxDrawdown = drawdown
			}
		}
	}

	sellPrice := bars[len(bars)-1].Close * (1 - slippageRate)
	sellFee := float64(shares) * sellPrice * feeRate
	finalEquity := cash + float64(shares)*sellPrice - sellFee
	return finalEquity, (finalEquity - initialCash) / initialCash, maxDrawdown
}

func annualizeReturn(growth float64, tradingDays int) float64 {
	if growth <= 0 || tradingDays <= 0 {
		return 0
	}
	return math.Pow(growth, 252.0/float64(tradingDays)) - 1
}

func tradingDatesInRange(bars []marketBar, fromDate string, toDate string) []string {
	dates := make([]string, 0, len(bars))
	for _, bar := range bars {
		if bar.Date >= fromDate && bar.Date <= toDate {
			dates = append(dates, bar.Date)
		}
	}
	return dates
}

func barsUpToDate(bars []marketBar, date string) []marketBar {
	idx := sort.Search(len(bars), func(i int) bool { return bars[i].Date > date })
	return bars[:idx]
}

func averageBenchmarkReturn(selection []scanCandidate, fromDate string, toDate string, series []marketSeries, initialCash float64, feeRate float64, slippageRate float64) float64 {
	if len(selection) == 0 {
		return 0
	}
	seriesMap := make(map[string][]marketBar, len(series))
	for _, item := range series {
		seriesMap[item.meta.Symbol] = item.bars
	}

	values := make([]float64, 0, len(selection))
	for _, candidate := range selection {
		bars := seriesMap[candidate.Symbol]
		filtered := make([]marketBar, 0, len(bars))
		for _, bar := range bars {
			if bar.Date >= fromDate && bar.Date <= toDate {
				filtered = append(filtered, bar)
			}
		}
		if len(filtered) == 0 {
			continue
		}
		_, ret, _ := simulateBuyAndHoldBenchmark(filtered, initialCash, feeRate, slippageRate)
		values = append(values, ret)
	}
	if len(values) == 0 {
		return 0
	}
	return average(values)
}

func buildBenchmarkCurve(bars []marketBar, fromDate string, toDate string, initialCash float64, feeRate float64, slippageRate float64) []backtestTrade {
	filtered := make([]marketBar, 0, len(bars))
	for _, bar := range bars {
		if bar.Date >= fromDate && bar.Date <= toDate {
			filtered = append(filtered, bar)
		}
	}
	if len(filtered) == 0 {
		return nil
	}

	buyPrice := filtered[0].Close * (1 + slippageRate)
	shares := int(initialCash / (buyPrice * (1 + feeRate)))
	if shares <= 0 {
		return nil
	}
	buyFee := float64(shares) * buyPrice * feeRate
	cash := initialCash - float64(shares)*buyPrice - buyFee

	curve := make([]backtestTrade, 0, len(filtered))
	for _, bar := range filtered {
		equity := cash + float64(shares)*bar.Close
		curve = append(curve, backtestTrade{
			Date:   bar.Date,
			Price:  bar.Close,
			Shares: shares,
			Cash:   cash,
			Equity: equity,
		})
	}
	return curve
}

func buildUniverseBenchmarkCurve(series []marketSeries, dates []string, initialCash float64) []backtestTrade {
	if len(series) == 0 || len(dates) == 0 {
		return nil
	}
	barBySymbolDate := make(map[string]map[string]marketBar, len(series))
	for _, item := range series {
		dateMap := make(map[string]marketBar, len(item.bars))
		for _, bar := range item.bars {
			dateMap[bar.Date] = bar
		}
		barBySymbolDate[item.meta.Symbol] = dateMap
	}

	equity := initialCash
	curve := make([]backtestTrade, 0, len(dates))
	for i, date := range dates {
		if i > 0 {
			totalReturn := 0.0
			count := 0
			for _, item := range series {
				prevBar, okPrev := barBySymbolDate[item.meta.Symbol][dates[i-1]]
				currBar, okCurr := barBySymbolDate[item.meta.Symbol][date]
				if !okPrev || !okCurr || prevBar.Close <= 0 {
					continue
				}
				totalReturn += currBar.Close/prevBar.Close - 1
				count++
			}
			if count > 0 {
				equity *= 1 + totalReturn/float64(count)
			}
		}
		curve = append(curve, backtestTrade{
			Date:   date,
			Equity: equity,
		})
	}
	return curve
}

func passPortfolioCandidateFilters(history []marketBar, candidate scanCandidate, minAverageTurnover float64, maxVolatility float64, overheatThreshold float64, minTrendGap float64, minPrice float64, minBacktestExcess float64, maxBacktestDrawdown float64) bool {
	if len(history) < 5 {
		return false
	}
	if candidate.ClosePrice < minPrice {
		return false
	}

	avgTurnover := 0.0
	returns := make([]float64, 0, len(history)-1)
	for i := 0; i < len(history); i++ {
		avgTurnover += history[i].Close * history[i].Volume
		if i == 0 || history[i-1].Close <= 0 {
			continue
		}
		returns = append(returns, history[i].Close/history[i-1].Close-1)
	}
	avgTurnover /= float64(len(history))
	if avgTurnover < minAverageTurnover {
		return false
	}

	volatility := standardDeviation(returns)
	if volatility > maxVolatility {
		return false
	}
	if candidate.StructureScore > overheatThreshold {
		return false
	}
	if candidate.TrendScore < minTrendGap {
		return false
	}
	if candidate.HasBacktest {
		if candidate.BacktestExcess < minBacktestExcess {
			return false
		}
		if candidate.BacktestDrawdown > maxBacktestDrawdown {
			return false
		}
	}
	return true
}

func portfolioCooldownDate(date string, days int) string {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return parsed.AddDate(0, 0, days).Format("2006-01-02")
}

func standardDeviation(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	mean := average(values)
	var sum float64
	for _, value := range values {
		diff := value - mean
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(values)))
}

func dailyMove(bar marketBar) float64 {
	if bar.Open <= 0 {
		return 0
	}
	return bar.Close/bar.Open - 1
}

func isBuyRestricted(bar marketBar, threshold float64) bool {
	return dailyMove(bar) >= threshold
}

func isSellRestricted(bar marketBar, threshold float64) bool {
	return dailyMove(bar) <= -threshold
}

func candidateMedianScore(candidates []scanCandidate) float64 {
	if len(candidates) == 0 {
		return 0
	}

	scores := make([]float64, 0, len(candidates))
	for _, candidate := range candidates {
		scores = append(scores, candidate.Score)
	}
	sort.Float64s(scores)
	return scores[len(scores)/2]
}

func marketBreadth(candidates []scanCandidate) float64 {
	if len(candidates) == 0 {
		return 0
	}
	positive := 0
	for _, candidate := range candidates {
		if candidate.Score > 0 && candidate.Action != "SELL" {
			positive++
		}
	}
	return float64(positive) / float64(len(candidates))
}

func benchmarkMarketRegime(history []marketBar, breadth float64, longWindow int, regime regimeConfig) (string, float64) {
	if len(history) < longWindow {
		return "cautious", regime.CautiousExposure
	}

	closes := make([]float64, 0, len(history))
	for _, bar := range history {
		closes = append(closes, bar.Close)
	}
	latest := history[len(history)-1]
	shortWindow := min(5, longWindow)
	shortMA := average(closes[len(closes)-shortWindow:])
	longMA := average(closes[len(closes)-longWindow:])
	peak := closes[0]
	for _, closePrice := range closes {
		if closePrice > peak {
			peak = closePrice
		}
	}
	drawdown := 0.0
	if peak > 0 {
		drawdown = (peak - latest.Close) / peak
	}

	if latest.Close < longMA || drawdown > regime.RiskOffDrawdown || breadth <= regime.BreadthRiskOff {
		return "risk_off", regime.RiskOffExposure
	}
	if latest.Close < shortMA || drawdown > regime.CautiousDrawdown || breadth <= regime.BreadthCautious {
		return "cautious", regime.CautiousExposure
	}
	return "risk_on", 1.0
}

func selectPortfolioCandidates(candidates []scanCandidate, topN int, minHoldings int, portfolio portfolioConfig) []scanCandidate {
	if len(candidates) == 0 {
		return candidates
	}

	sort.Slice(candidates, func(i, j int) bool {
		leftScore := portfolioSelectionScore(candidates[i], portfolio)
		rightScore := portfolioSelectionScore(candidates[j], portfolio)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		return candidates[i].Score > candidates[j].Score
	})

	limit := len(candidates)
	if topN > 0 && limit > topN*3 {
		limit = topN * 3
	}
	candidates = candidates[:limit]

	selected := make([]scanCandidate, 0, min(limit, max(minHoldings, topN)))
	industryCount := make(map[string]int)
	for _, candidate := range candidates {
		// Prefer names with either strong standalone signal quality or
		// acceptable historical validation.
		if candidate.Score < 0.05 && candidate.BacktestExcess < 0 && candidate.BacktestReturn < 0 {
			continue
		}
		if len(candidates) > minHoldings*2 && candidate.RotationScore < 0 {
			continue
		}
		industryKey := normalizedIndustry(candidate.Industry)
		if industryKey != "" && industryCount[industryKey] >= 1 {
			continue
		}
		if isSimilarToSelected(candidate, selected) {
			continue
		}
		selected = append(selected, candidate)
		if industryKey != "" {
			industryCount[industryKey]++
		}
		if topN > 0 && len(selected) >= topN {
			break
		}
	}

	if len(selected) >= minHoldings {
		return selected
	}

	for _, candidate := range candidates {
		if containsCandidate(selected, candidate.Symbol) {
			continue
		}
		selected = append(selected, candidate)
		if len(selected) >= minHoldings {
			return selected
		}
	}
	if len(selected) == 0 {
		if len(candidates) < minHoldings {
			return candidates
		}
		return candidates[:minHoldings]
	}
	return selected
}

func portfolioSelectionScore(candidate scanCandidate, portfolio portfolioConfig) float64 {
	score := candidate.Score
	score += candidate.QualityScore * portfolio.QualityWeight
	score += candidate.RiskScore * portfolio.RiskWeight
	score += candidate.ReversalScore * portfolio.ReversalWeight
	score -= candidate.HeatPenalty * portfolio.HeatPenaltyWeight
	if candidate.HasBacktest {
		score += candidate.BacktestExcess * portfolio.BacktestExcessWeight
		score += candidate.BacktestReturn * portfolio.BacktestReturnWeight
		score -= candidate.BacktestDrawdown * portfolio.BacktestDrawdownWeight
	}
	score += candidate.MomentumScore * portfolio.MomentumWeight
	score += candidate.PersistenceScore * portfolio.PersistenceWeight
	score += candidate.BreakoutScore * 0.80
	score += candidate.VolumeTrendScore * 0.70
	score += candidate.RotationScore * 1.20
	score += candidate.StrategyAlignment * 1.50
	score += candidate.ModelScore * 0.80
	if candidate.RiskPenalty > 0 {
		score -= candidate.RiskPenalty
	}
	if candidate.Bucket == "观望" {
		score -= portfolio.WatchPenalty
	}
	return score
}

func candidateOverlayScores(bars []marketBar, shortMA float64, longMA float64, avgVolume float64, shortReturn float64, mediumReturn float64, trendScore float64, liquidityScore float64, persistenceScore float64, breakoutScore float64, volumeTrendScore float64, riskPenalty float64) (float64, float64, float64, float64) {
	if len(bars) == 0 {
		return 0, 0, 0, 0
	}
	latest := bars[len(bars)-1]
	closes := make([]float64, 0, len(bars))
	for _, bar := range bars {
		closes = append(closes, bar.Close)
	}

	qualityScore := clampFloat(trendScore*1.60+persistenceScore*1.40+liquidityScore*8.0+volumeTrendScore*1.10, -0.20, 0.35)

	volatilityPenalty := 0.0
	lookback := min(10, len(closes)-1)
	if lookback > 0 {
		sumMoves := 0.0
		count := 0.0
		for i := len(closes) - lookback; i < len(closes); i++ {
			if i == 0 || closes[i-1] <= 0 {
				continue
			}
			sumMoves += math.Abs(closes[i]/closes[i-1] - 1)
			count++
		}
		if count > 0 {
			volatilityPenalty = clampFloat(sumMoves/count, 0, 0.08)
		}
	}

	riskScore := 0.0
	if longMA > 0 {
		distance := clampFloat(latest.Close/longMA-1, -0.08, 0.12)
		if distance > 0 {
			riskScore += distance * 0.60
		}
	}
	if avgVolume > 0 {
		riskScore += clampFloat(math.Log10(avgVolume+1)/10.0, 0, 0.08)
	}
	riskScore -= volatilityPenalty * 0.90
	riskScore -= riskPenalty * 1.50
	riskScore = clampFloat(riskScore, -0.15, 0.18)

	heatPenalty := 0.0
	if shortReturn > 0.05 {
		heatPenalty += (shortReturn - 0.05) * 1.80
	}
	if mediumReturn > 0.15 {
		heatPenalty += (mediumReturn - 0.15) * 1.20
	}
	heatPenalty += clampFloat(breakoutScore-0.01, 0, 0.05) * 3.0
	heatPenalty += clampFloat(volumeTrendScore-0.02, 0, 0.06) * 2.0
	heatPenalty = clampFloat(heatPenalty, 0, 0.25)

	reversalScore := 0.0
	threeDayReturn := trailingReturn(closes, min(3, len(closes)-1))
	if mediumReturn > 0 && latest.Close >= longMA && latest.Close <= shortMA*1.01 {
		reversalScore += clampFloat(-threeDayReturn, 0, 0.06) * 1.50
		reversalScore += clampFloat(shortMA/latest.Close-1, 0, 0.04) * 1.20
		reversalScore += clampFloat(0.08-heatPenalty, 0, 0.08) * 0.60
	}
	reversalScore = clampFloat(reversalScore, 0, 0.18)

	return qualityScore, riskScore, heatPenalty, reversalScore
}

func applyRotationOverlay(candidates []scanCandidate) {
	if len(candidates) == 0 {
		return
	}

	shortValues := make([]float64, 0, len(candidates))
	mediumValues := make([]float64, 0, len(candidates))
	for _, candidate := range candidates {
		shortValues = append(shortValues, candidate.ShortReturnScore)
		mediumValues = append(mediumValues, candidate.MediumReturnScore)
	}
	shortMedian := medianFloat(shortValues)
	mediumMedian := medianFloat(mediumValues)

	industrySums := make(map[string]float64)
	industryCounts := make(map[string]int)
	for _, candidate := range candidates {
		industryKey := normalizedIndustry(candidate.Industry)
		if industryKey == "" {
			continue
		}
		industrySums[industryKey] += candidate.MediumReturnScore
		industryCounts[industryKey]++
	}

	for i := range candidates {
		industryStrength := 0.0
		industryKey := normalizedIndustry(candidates[i].Industry)
		if industryKey != "" && industryCounts[industryKey] > 0 {
			industryStrength = industrySums[industryKey] / float64(industryCounts[industryKey])
		}
		candidates[i].IndustryStrength = industryStrength
		candidates[i].RotationScore =
			clampFloat(candidates[i].ShortReturnScore-shortMedian, -0.15, 0.15)*0.45 +
				clampFloat(candidates[i].MediumReturnScore-mediumMedian, -0.20, 0.20)*0.45 +
				clampFloat(industryStrength-mediumMedian, -0.10, 0.10)*0.30
	}
}

func trailingReturn(closes []float64, lookback int) float64 {
	if len(closes) < 2 {
		return 0
	}
	if lookback <= 0 {
		lookback = 1
	}
	if lookback >= len(closes) {
		lookback = len(closes) - 1
	}
	base := closes[len(closes)-1-lookback]
	latest := closes[len(closes)-1]
	if base <= 0 {
		return 0
	}
	return latest/base - 1
}

func medianFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func normalizedIndustry(industry string) string {
	normalized := strings.TrimSpace(industry)
	if normalized == "" || normalized == "-" {
		return ""
	}
	return normalized
}

func isSimilarToSelected(candidate scanCandidate, selected []scanCandidate) bool {
	for _, existing := range selected {
		if normalizedIndustry(candidate.Industry) != "" && normalizedIndustry(candidate.Industry) == normalizedIndustry(existing.Industry) {
			return true
		}
		if math.Abs(candidate.TrendScore-existing.TrendScore) < 0.01 &&
			math.Abs(candidate.MomentumScore-existing.MomentumScore) < 0.01 &&
			math.Abs(candidate.BreakoutScore-existing.BreakoutScore) < 0.01 &&
			math.Abs(candidate.VolumeTrendScore-existing.VolumeTrendScore) < 0.01 {
			return true
		}
	}
	return false
}

func containsCandidate(selected []scanCandidate, symbol string) bool {
	for _, candidate := range selected {
		if candidate.Symbol == symbol {
			return true
		}
	}
	return false
}

func printBacktestSummary(result backtestResult) {
	fmt.Printf(
		"Backtest %s %s -> %s\nMode: %s\nInitial cash: %.2f\nFinal equity: %.2f\nTotal return: %.2f%%\nAnnualized return: %.2f%%\nBenchmark return: %.2f%%\nExcess return: %.2f%%\nMax drawdown: %.2f%%\nBenchmark drawdown: %.2f%%\nTrades: %d\nWin rate: %.2f%%\n\n",
		result.Symbol,
		result.FromDate,
		result.ToDate,
		result.Mode,
		result.InitialCash,
		result.FinalEquity,
		result.TotalReturn*100,
		result.AnnualizedReturn*100,
		result.BenchmarkReturn*100,
		result.ExcessReturn*100,
		result.MaxDrawdown*100,
		result.BenchmarkDrawdown*100,
		result.TradeCount,
		result.WinRate*100,
	)
}

func writeBacktestReports(result backtestResult) error {
	textPath := filepath.Join(reportsDir, "backtest_latest.txt")
	htmlPath := filepath.Join(reportsDir, "backtest_latest.html")
	svg := buildEquityCurveSVG(result.EquityCurve)

	var tradeLines strings.Builder
	for _, trade := range result.Trades {
		fmt.Fprintf(&tradeLines, "%s %s price=%.2f shares=%d fee=%.2f cash=%.2f equity=%.2f reason=%s\n",
			trade.Date, trade.Action, trade.Price, trade.Shares, trade.Fee, trade.Cash, trade.Equity, trade.Reason)
	}

	textContent := fmt.Sprintf(
		"Backtest %s %s -> %s\nMode: %s\nInitial cash: %.2f\nFinal equity: %.2f\nTotal return: %.2f%%\nAnnualized return: %.2f%%\nBenchmark equity: %.2f\nBenchmark return: %.2f%%\nExcess return: %.2f%%\nMax drawdown: %.2f%%\nBenchmark drawdown: %.2f%%\nTrading days: %d\nTrades: %d\nWin rate: %.2f%%\nFee bps: %.2f\nSlippage bps: %.2f\nTotal fees: %.2f\n\nTrade Log\n%s",
		result.Symbol,
		result.FromDate,
		result.ToDate,
		result.Mode,
		result.InitialCash,
		result.FinalEquity,
		result.TotalReturn*100,
		result.AnnualizedReturn*100,
		result.BenchmarkEquity,
		result.BenchmarkReturn*100,
		result.ExcessReturn*100,
		result.MaxDrawdown*100,
		result.BenchmarkDrawdown*100,
		result.TradingDays,
		result.TradeCount,
		result.WinRate*100,
		result.FeeBps,
		result.SlippageBps,
		result.TotalFees,
		tradeLines.String(),
	)

	var rows strings.Builder
	for _, trade := range result.Trades {
		fmt.Fprintf(&rows, `<tr><td>%s</td><td>%s</td><td>%.2f</td><td>%d</td><td>%.2f</td><td>%.2f</td><td>%.2f</td><td>%s</td></tr>`,
			html.EscapeString(trade.Date),
			html.EscapeString(trade.Action),
			trade.Price,
			trade.Shares,
			trade.Fee,
			trade.Cash,
			trade.Equity,
			html.EscapeString(trade.Reason),
		)
	}
	if len(result.Trades) == 0 {
		rows.WriteString(`<tr><td colspan="8">No trades</td></tr>`)
	}

	htmlContent := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Backtest Report</title>
  <style>
    body { margin: 0; font-family: Georgia, "Times New Roman", serif; background: #f4efe6; color: #1f1b16; }
    .wrap { max-width: 1100px; margin: 36px auto; padding: 0 20px; }
    .card { background: #fffaf3; border: 1px solid #d9cfbf; border-radius: 18px; padding: 24px; box-shadow: 0 18px 40px rgba(70, 50, 20, 0.08); }
    h1 { margin: 0 0 16px; font-size: 36px; }
    .meta { line-height: 1.8; color: #6d6559; }
    table { width: 100%%; border-collapse: collapse; margin-top: 18px; }
    th, td { text-align: left; padding: 12px 10px; border-bottom: 1px solid #e7dece; vertical-align: top; }
    th { font-size: 12px; text-transform: uppercase; letter-spacing: 0.06em; color: #6d6559; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <h1>Backtest %s</h1>
      <div class="meta">
        <div>Range: %s to %s</div>
        <div>Mode: %s</div>
        <div>Initial cash: %.2f</div>
        <div>Final equity: %.2f</div>
        <div>Total return: %.2f%%</div>
        <div>Annualized return: %.2f%%</div>
        <div>Benchmark equity: %.2f</div>
        <div>Benchmark return: %.2f%%</div>
        <div>Excess return: %.2f%%</div>
        <div>Max drawdown: %.2f%%</div>
        <div>Benchmark drawdown: %.2f%%</div>
        <div>Trading days: %d</div>
        <div>Trades: %d</div>
        <div>Win rate: %.2f%%</div>
        <div>Fee bps: %.2f</div>
        <div>Slippage bps: %.2f</div>
        <div>Total fees: %.2f</div>
      </div>
      <div style="margin-top:18px">%s</div>
      <table>
        <thead>
          <tr><th>Date</th><th>Action</th><th>Price</th><th>Shares</th><th>Fee</th><th>Cash</th><th>Equity</th><th>Reason</th></tr>
        </thead>
        <tbody>%s</tbody>
      </table>
    </div>
  </div>
</body>
</html>`,
		html.EscapeString(result.Symbol),
		html.EscapeString(result.FromDate),
		html.EscapeString(result.ToDate),
		html.EscapeString(result.Mode),
		result.InitialCash,
		result.FinalEquity,
		result.TotalReturn*100,
		result.AnnualizedReturn*100,
		result.BenchmarkEquity,
		result.BenchmarkReturn*100,
		result.ExcessReturn*100,
		result.MaxDrawdown*100,
		result.BenchmarkDrawdown*100,
		result.TradingDays,
		result.TradeCount,
		result.WinRate*100,
		result.FeeBps,
		result.SlippageBps,
		result.TotalFees,
		svg,
		rows.String(),
	)

	if err := os.WriteFile(textPath, []byte(textContent), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0o644); err != nil {
		return err
	}
	return nil
}

func writeBatchBacktestReports(results []backtestResult, fromDate string, toDate string, initialCash float64, feeBps float64, slippageBps float64) error {
	textPath := filepath.Join(reportsDir, "backtest_scan.txt")
	htmlPath := filepath.Join(reportsDir, "backtest_scan.html")
	csvPath := filepath.Join(reportsDir, "backtest_scan.csv")

	var textBuilder strings.Builder
	fmt.Fprintf(&textBuilder, "Batch Backtest %s -> %s\nInitial cash: %.2f\nFee bps: %.2f\nSlippage bps: %.2f\n\n", fromDate, toDate, initialCash, feeBps, slippageBps)
	for i, result := range results {
		nameLine := result.Symbol
		if result.Name != "" {
			nameLine += " " + result.Name
		}
		fmt.Fprintf(&textBuilder, "%d. %s\n", i+1, nameLine)
		fmt.Fprintf(&textBuilder, "   Return: %.2f%%\n", result.TotalReturn*100)
		fmt.Fprintf(&textBuilder, "   Annualized: %.2f%%\n", result.AnnualizedReturn*100)
		fmt.Fprintf(&textBuilder, "   Benchmark: %.2f%%\n", result.BenchmarkReturn*100)
		fmt.Fprintf(&textBuilder, "   Excess: %.2f%%\n", result.ExcessReturn*100)
		fmt.Fprintf(&textBuilder, "   Max drawdown: %.2f%%\n", result.MaxDrawdown*100)
		fmt.Fprintf(&textBuilder, "   Trades: %d\n", result.TradeCount)
		fmt.Fprintf(&textBuilder, "   Win rate: %.2f%%\n\n", result.WinRate*100)
	}

	var rows strings.Builder
	for i, result := range results {
		name := result.Name
		if name == "" {
			name = "-"
		}
		fmt.Fprintf(&rows, `<tr><td>%d</td><td>%s</td><td>%s</td><td>%.2f%%</td><td>%.2f%%</td><td>%.2f%%</td><td>%.2f%%</td><td>%d</td><td>%.2f%%</td><td>%.2f</td></tr>`,
			i+1,
			html.EscapeString(result.Symbol),
			html.EscapeString(name),
			result.TotalReturn*100,
			result.AnnualizedReturn*100,
			result.BenchmarkReturn*100,
			result.ExcessReturn*100,
			result.MaxDrawdown*100,
			result.TradeCount,
			result.WinRate*100,
			result.FinalEquity,
		)
	}

	htmlContent := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Batch Backtest</title>
  <style>
    body { margin: 0; font-family: Georgia, "Times New Roman", serif; background: #f4efe6; color: #1f1b16; }
    .wrap { max-width: 1100px; margin: 36px auto; padding: 0 20px; }
    .card { background: #fffaf3; border: 1px solid #d9cfbf; border-radius: 18px; padding: 24px; box-shadow: 0 18px 40px rgba(70, 50, 20, 0.08); }
    h1 { margin: 0 0 16px; font-size: 36px; }
    .meta { line-height: 1.8; color: #6d6559; }
    table { width: 100%%; border-collapse: collapse; margin-top: 18px; }
    th, td { text-align: left; padding: 12px 10px; border-bottom: 1px solid #e7dece; vertical-align: top; }
    th { font-size: 12px; text-transform: uppercase; letter-spacing: 0.06em; color: #6d6559; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <h1>Batch Backtest</h1>
      <div class="meta">
        <div>Range: %s to %s</div>
        <div>Initial cash: %.2f</div>
        <div>Fee bps: %.2f</div>
        <div>Slippage bps: %.2f</div>
      </div>
      <table>
        <thead>
          <tr><th>#</th><th>Symbol</th><th>Name</th><th>Return</th><th>Annualized</th><th>Benchmark</th><th>Excess</th><th>Max DD</th><th>Trades</th><th>Win Rate</th><th>Final Equity</th></tr>
        </thead>
        <tbody>%s</tbody>
      </table>
    </div>
  </div>
</body>
</html>`, html.EscapeString(fromDate), html.EscapeString(toDate), initialCash, feeBps, slippageBps, rows.String())

	csvRows := [][]string{{
		"symbol", "name", "from_date", "to_date", "mode", "initial_cash", "final_equity", "total_return",
		"annualized_return", "benchmark_return", "benchmark_equity", "excess_return", "max_drawdown",
		"benchmark_drawdown", "trade_count", "win_rate", "fee_bps", "slippage_bps", "total_fees",
	}}
	for _, result := range results {
		csvRows = append(csvRows, []string{
			result.Symbol,
			result.Name,
			result.FromDate,
			result.ToDate,
			result.Mode,
			fmt.Sprintf("%.2f", result.InitialCash),
			fmt.Sprintf("%.2f", result.FinalEquity),
			fmt.Sprintf("%.8f", result.TotalReturn),
			fmt.Sprintf("%.8f", result.AnnualizedReturn),
			fmt.Sprintf("%.8f", result.BenchmarkReturn),
			fmt.Sprintf("%.2f", result.BenchmarkEquity),
			fmt.Sprintf("%.8f", result.ExcessReturn),
			fmt.Sprintf("%.8f", result.MaxDrawdown),
			fmt.Sprintf("%.8f", result.BenchmarkDrawdown),
			strconv.Itoa(result.TradeCount),
			fmt.Sprintf("%.8f", result.WinRate),
			fmt.Sprintf("%.2f", result.FeeBps),
			fmt.Sprintf("%.2f", result.SlippageBps),
			fmt.Sprintf("%.2f", result.TotalFees),
		})
	}

	if err := os.WriteFile(textPath, []byte(textBuilder.String()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0o644); err != nil {
		return err
	}
	if err := writeCSVFile(csvPath, csvRows); err != nil {
		return err
	}
	return nil
}

func printPortfolioBacktestSummary(result portfolioBacktestResult) {
	fmt.Printf(
		"Portfolio Backtest %s -> %s\nMode: %s\nPositions: %d\nRegime: %s\nTarget exposure: %.0f%%\nInitial cash: %.2f\nFinal equity: %.2f\nTotal return: %.2f%%\nAnnualized return: %.2f%%\nBenchmark return: %.2f%%\nExcess return: %.2f%%\nMax drawdown: %.2f%%\nRebalances: %d\nTrading days: %d\n\n",
		result.FromDate,
		result.ToDate,
		result.Mode,
		result.Positions,
		result.RegimeLabel,
		result.ExposureLevel*100,
		result.InitialCash,
		result.FinalEquity,
		result.TotalReturn*100,
		result.AnnualizedReturn*100,
		result.BenchmarkReturn*100,
		result.ExcessReturn*100,
		result.MaxDrawdown*100,
		result.RebalanceCount,
		result.TradingDays,
	)
}

func printGridSearchSummary(results []gridSearchResult, fromDate string, toDate string) {
	if len(results) == 0 {
		fmt.Printf("Grid Search %s -> %s\nNo valid parameter combinations.\n\n", fromDate, toDate)
		return
	}
	best := results[0]
	fmt.Printf(
		"Grid Search %s -> %s\nBest short/long: %d / %d\nFinal equity: %.2f\nTotal return: %.2f%%\nAnnualized return: %.2f%%\nBenchmark return: %.2f%%\nExcess return: %.2f%%\nMax drawdown: %.2f%%\nRebalances: %d\n\n",
		fromDate,
		toDate,
		best.ShortWindow,
		best.LongWindow,
		best.FinalEquity,
		best.TotalReturn*100,
		best.AnnualizedReturn*100,
		best.BenchmarkReturn*100,
		best.ExcessReturn*100,
		best.MaxDrawdown*100,
		best.Rebalances,
	)
}

func writePortfolioBacktestReports(result portfolioBacktestResult) error {
	textPath := filepath.Join(reportsDir, "portfolio_backtest.txt")
	htmlPath := filepath.Join(reportsDir, "portfolio_backtest.html")
	csvPath := filepath.Join(reportsDir, "portfolio_backtest.csv")

	var latest strings.Builder
	for i, candidate := range result.LatestSelection {
		fmt.Fprintf(&latest, "%d. %s %s score=%.4f close=%.2f\n", i+1, candidate.Symbol, candidate.Name, candidate.Score, candidate.ClosePrice)
	}
	if latest.Len() == 0 {
		latest.WriteString("No active selection\n")
	}

	var curve []backtestTrade
	for _, snapshot := range result.Snapshots {
		curve = append(curve, backtestTrade{Date: snapshot.Date, Equity: snapshot.Equity})
	}
	svg := buildEquityCurveSVG(curve)
	comparisonSVG := buildComparisonCurveSVG(result.Snapshots, result.BenchmarkCurve)

	lastHoldings := "None"
	if len(result.Snapshots) > 0 && len(result.Snapshots[len(result.Snapshots)-1].Holdings) > 0 {
		names := make([]string, 0, len(result.Snapshots[len(result.Snapshots)-1].Holdings))
		for _, holding := range result.Snapshots[len(result.Snapshots)-1].Holdings {
			names = append(names, fmt.Sprintf("%s %s x%d", holding.Symbol, holding.Name, holding.Shares))
		}
		lastHoldings = strings.Join(names, "; ")
	}

	textContent := fmt.Sprintf(
		"Portfolio Backtest %s -> %s\nMode: %s\nPositions: %d\nRegime: %s\nTarget exposure: %.0f%%\nInitial cash: %.2f\nFinal equity: %.2f\nTotal return: %.2f%%\nAnnualized return: %.2f%%\nBenchmark return: %.2f%%\nExcess return: %.2f%%\nMax drawdown: %.2f%%\nRebalances: %d\nTrading days: %d\nCurrent holdings: %s\n\nLatest selection\n%s",
		result.FromDate,
		result.ToDate,
		result.Mode,
		result.Positions,
		result.RegimeLabel,
		result.ExposureLevel*100,
		result.InitialCash,
		result.FinalEquity,
		result.TotalReturn*100,
		result.AnnualizedReturn*100,
		result.BenchmarkReturn*100,
		result.ExcessReturn*100,
		result.MaxDrawdown*100,
		result.RebalanceCount,
		result.TradingDays,
		lastHoldings,
		latest.String(),
	)

	var selectionRows strings.Builder
	if len(result.LatestSelection) == 0 {
		selectionRows.WriteString(`<tr><td colspan="5">No active selection</td></tr>`)
	} else {
		for i, candidate := range result.LatestSelection {
			fmt.Fprintf(&selectionRows, `<tr><td>%d</td><td>%s</td><td>%s</td><td>%.4f</td><td>%.2f</td></tr>`,
				i+1,
				html.EscapeString(candidate.Symbol),
				html.EscapeString(candidate.Name),
				candidate.Score,
				candidate.ClosePrice,
			)
		}
	}

	htmlContent := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Portfolio Backtest</title>
  <style>
    body { margin: 0; font-family: Georgia, "Times New Roman", serif; background: #f4efe6; color: #1f1b16; }
    .wrap { max-width: 1100px; margin: 36px auto; padding: 0 20px; }
    .card { background: #fffaf3; border: 1px solid #d9cfbf; border-radius: 18px; padding: 24px; box-shadow: 0 18px 40px rgba(70, 50, 20, 0.08); }
    h1 { margin: 0 0 16px; font-size: 36px; }
    .meta { line-height: 1.8; color: #6d6559; }
    table { width: 100%%; border-collapse: collapse; margin-top: 18px; }
    th, td { text-align: left; padding: 12px 10px; border-bottom: 1px solid #e7dece; vertical-align: top; }
    th { font-size: 12px; text-transform: uppercase; letter-spacing: 0.06em; color: #6d6559; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <h1>Portfolio Backtest</h1>
      <div class="meta">
        <div>Range: %s to %s</div>
        <div>Mode: %s</div>
        <div>Positions: %d</div>
        <div>Regime: %s</div>
        <div>Target exposure: %.0f%%</div>
        <div>Initial cash: %.2f</div>
        <div>Final equity: %.2f</div>
        <div>Total return: %.2f%%</div>
        <div>Annualized return: %.2f%%</div>
        <div>Benchmark return: %.2f%%</div>
        <div>Excess return: %.2f%%</div>
        <div>Max drawdown: %.2f%%</div>
        <div>Rebalances: %d</div>
        <div>Trading days: %d</div>
        <div>Current holdings: %s</div>
      </div>
      <div style="margin-top:18px">%s</div>
      <div style="margin-top:18px">%s</div>
      <table>
        <thead>
          <tr><th>#</th><th>Symbol</th><th>Name</th><th>Score</th><th>Close</th></tr>
        </thead>
        <tbody>%s</tbody>
      </table>
    </div>
  </div>
</body>
</html>`,
		html.EscapeString(result.FromDate),
		html.EscapeString(result.ToDate),
		html.EscapeString(result.Mode),
		result.Positions,
		html.EscapeString(result.RegimeLabel),
		result.ExposureLevel*100,
		result.InitialCash,
		result.FinalEquity,
		result.TotalReturn*100,
		result.AnnualizedReturn*100,
		result.BenchmarkReturn*100,
		result.ExcessReturn*100,
		result.MaxDrawdown*100,
		result.RebalanceCount,
		result.TradingDays,
		html.EscapeString(lastHoldings),
		svg,
		comparisonSVG,
		selectionRows.String(),
	)

	if err := os.WriteFile(textPath, []byte(textContent), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0o644); err != nil {
		return err
	}
	csvRows := [][]string{{"symbol", "name", "shares"}}
	for _, holding := range result.CurrentHoldings {
		csvRows = append(csvRows, []string{
			holding.Symbol,
			holding.Name,
			strconv.Itoa(holding.Shares),
		})
	}
	if err := writeCSVFile(csvPath, csvRows); err != nil {
		return err
	}
	return writeDashboardReports()
}

func writeGridSearchReports(results []gridSearchResult, fromDate string, toDate string) error {
	textPath := filepath.Join(reportsDir, "grid_search.txt")
	htmlPath := filepath.Join(reportsDir, "grid_search.html")
	csvPath := filepath.Join(reportsDir, "grid_search.csv")

	var textBuilder strings.Builder
	fmt.Fprintf(&textBuilder, "Portfolio Grid Search %s -> %s\n\n", fromDate, toDate)
	for i, result := range results {
		fmt.Fprintf(&textBuilder, "%d. short=%d long=%d return=%.2f%% annualized=%.2f%% benchmark=%.2f%% excess=%.2f%% max_dd=%.2f%% rebalances=%d final_equity=%.2f\n",
			i+1,
			result.ShortWindow,
			result.LongWindow,
			result.TotalReturn*100,
			result.AnnualizedReturn*100,
			result.BenchmarkReturn*100,
			result.ExcessReturn*100,
			result.MaxDrawdown*100,
			result.Rebalances,
			result.FinalEquity,
		)
	}

	var rows strings.Builder
	for i, result := range results {
		fmt.Fprintf(&rows, `<tr><td>%d</td><td>%d</td><td>%d</td><td>%.2f%%</td><td>%.2f%%</td><td>%.2f%%</td><td>%.2f%%</td><td>%.2f%%</td><td>%d</td><td>%.2f</td></tr>`,
			i+1,
			result.ShortWindow,
			result.LongWindow,
			result.TotalReturn*100,
			result.AnnualizedReturn*100,
			result.BenchmarkReturn*100,
			result.ExcessReturn*100,
			result.MaxDrawdown*100,
			result.Rebalances,
			result.FinalEquity,
		)
	}

	htmlContent := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Portfolio Grid Search</title>
  <style>
    body { margin: 0; font-family: Georgia, "Times New Roman", serif; background: #f3efe8; color: #1f1b16; }
    .wrap { max-width: 1100px; margin: 36px auto; padding: 0 20px; }
    .card { background: #fffaf3; border: 1px solid #d9cfbf; border-radius: 18px; padding: 24px; box-shadow: 0 18px 40px rgba(70, 50, 20, 0.08); }
    h1 { margin: 0 0 16px; font-size: 36px; }
    table { width: 100%%; border-collapse: collapse; font-size: 15px; }
    th, td { text-align: left; padding: 12px 10px; border-bottom: 1px solid #e7dece; vertical-align: top; }
    th { font-size: 12px; text-transform: uppercase; letter-spacing: 0.06em; color: #6d6559; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <h1>Portfolio Grid Search %s -> %s</h1>
      <table>
        <thead>
          <tr><th>#</th><th>Short</th><th>Long</th><th>Return</th><th>Annualized</th><th>Benchmark</th><th>Excess</th><th>Max DD</th><th>Rebalances</th><th>Final Equity</th></tr>
        </thead>
        <tbody>%s</tbody>
      </table>
    </div>
  </div>
</body>
</html>`, html.EscapeString(fromDate), html.EscapeString(toDate), rows.String())

	var csvBuilder strings.Builder
	csvBuilder.WriteString("rank,short_window,long_window,total_return,annualized_return,benchmark_return,excess_return,max_drawdown,rebalances,final_equity\n")
	for i, result := range results {
		fmt.Fprintf(&csvBuilder, "%d,%d,%d,%.6f,%.6f,%.6f,%.6f,%.6f,%d,%.2f\n",
			i+1,
			result.ShortWindow,
			result.LongWindow,
			result.TotalReturn,
			result.AnnualizedReturn,
			result.BenchmarkReturn,
			result.ExcessReturn,
			result.MaxDrawdown,
			result.Rebalances,
			result.FinalEquity,
		)
	}

	if err := os.WriteFile(textPath, []byte(textBuilder.String()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(csvPath, []byte(csvBuilder.String()), 0o644); err != nil {
		return err
	}
	return writeDashboardReports()
}

func writeDatasetReports(rows []datasetRow, fromDate string, toDate string) error {
	csvPath := filepath.Join(reportsDir, "training_dataset.csv")
	textPath := filepath.Join(reportsDir, "training_dataset.txt")

	var csvBuilder strings.Builder
	csvBuilder.WriteString("symbol,name,industry,date,close,avg_volume,short_ma,long_ma,score,quality_score,risk_score,heat_penalty,reversal_score,trend_score,liquidity_score,structure_score,momentum_score,persistence_score,breakout_score,volume_trend_score,short_return_score,medium_return_score,rotation_score,strategy_alignment,breadth,regime_exposure,label_5d,label_10d,label_20d\n")
	for _, row := range rows {
		fmt.Fprintf(&csvBuilder, "%s,%s,%s,%s,%.4f,%.0f,%.4f,%.4f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f\n",
			row.Symbol,
			sanitizeCSV(row.Name),
			sanitizeCSV(row.Industry),
			row.Date,
			row.Close,
			row.Volume,
			row.ShortMA,
			row.LongMA,
			row.Score,
			row.QualityScore,
			row.RiskScore,
			row.HeatPenalty,
			row.ReversalScore,
			row.TrendScore,
			row.LiquidityScore,
			row.StructureScore,
			row.MomentumScore,
			row.PersistenceScore,
			row.BreakoutScore,
			row.VolumeTrendScore,
			row.ShortReturnScore,
			row.MediumReturnScore,
			row.RotationScore,
			row.StrategyAlignment,
			row.Breadth,
			row.RegimeExposure,
			row.Label5D,
			row.Label10D,
			row.Label20D,
		)
	}

	var textBuilder strings.Builder
	fmt.Fprintf(&textBuilder, "Training Dataset %s -> %s\n\nRows: %d\nFeatures: 22\nLabels: label_5d, label_10d, label_20d\nCSV: %s\n\nSample Rows\n",
		fromDate,
		toDate,
		len(rows),
		csvPath,
	)
	sampleCount := min(5, len(rows))
	for i := 0; i < sampleCount; i++ {
		row := rows[i]
		fmt.Fprintf(&textBuilder, "%d. %s %s %s score=%.4f label5=%.2f%% label10=%.2f%% label20=%.2f%%\n",
			i+1,
			row.Date,
			row.Symbol,
			row.Name,
			row.Score,
			row.Label5D*100,
			row.Label10D*100,
			row.Label20D*100,
		)
	}

	if err := os.WriteFile(csvPath, []byte(csvBuilder.String()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(textPath, []byte(textBuilder.String()), 0o644); err != nil {
		return err
	}
	return writeDashboardReports()
}

func sanitizeCSV(value string) string {
	value = strings.ReplaceAll(value, ",", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}

func getLinearModel() *linearModel {
	if linearModelLoaded {
		return cachedLinearModel
	}
	linearModelLoaded = true

	path := filepath.Join(reportsDir, "linear_model.json")
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var model linearModel
	if err := json.Unmarshal(content, &model); err != nil {
		return nil
	}
	cachedLinearModel = &model
	return cachedLinearModel
}

func predictLinearModel(candidate scanCandidate) float64 {
	model := getLinearModel()
	if model == nil {
		return 0
	}

	featureValues := map[string]float64{
		"score":               candidate.Score,
		"quality_score":       candidate.QualityScore,
		"risk_score":          candidate.RiskScore,
		"heat_penalty":        candidate.HeatPenalty,
		"reversal_score":      candidate.ReversalScore,
		"trend_score":         candidate.TrendScore,
		"liquidity_score":     candidate.LiquidityScore,
		"structure_score":     candidate.StructureScore,
		"momentum_score":      candidate.MomentumScore,
		"persistence_score":   candidate.PersistenceScore,
		"breakout_score":      candidate.BreakoutScore,
		"volume_trend_score":  candidate.VolumeTrendScore,
		"short_return_score":  candidate.ShortReturnScore,
		"medium_return_score": candidate.MediumReturnScore,
		"rotation_score":      candidate.RotationScore,
		"strategy_alignment":  candidate.StrategyAlignment,
		"breadth":             0.5,
		"regime_exposure":     1.0,
	}

	score := model.Bias
	for _, feature := range model.Features {
		value := featureValues[feature.Feature]
		denom := feature.Std
		if denom == 0 {
			denom = 1
		}
		score += ((value - feature.Mean) / denom) * feature.Weight
	}
	return score
}

func buildEquityCurveSVG(curve []backtestTrade) string {
	if len(curve) == 0 {
		return ""
	}

	minEquity := curve[0].Equity
	maxEquity := curve[0].Equity
	for _, point := range curve {
		if point.Equity < minEquity {
			minEquity = point.Equity
		}
		if point.Equity > maxEquity {
			maxEquity = point.Equity
		}
	}
	if maxEquity == minEquity {
		maxEquity = minEquity + 1
	}

	width := 900.0
	height := 240.0
	points := make([]string, 0, len(curve))
	for i, point := range curve {
		x := (float64(i) / float64(max(1, len(curve)-1))) * width
		y := height - ((point.Equity-minEquity)/(maxEquity-minEquity))*height
		points = append(points, fmt.Sprintf("%.2f,%.2f", x, y))
	}

	return fmt.Sprintf(`<svg viewBox="0 0 %.0f %.0f" width="100%%" height="240" role="img" aria-label="Equity curve"><rect x="0" y="0" width="100%%" height="100%%" fill="#f7f1e7"/><polyline fill="none" stroke="#0f766e" stroke-width="3" points="%s"/></svg>`,
		width, height, strings.Join(points, " "))
}

func buildComparisonCurveSVG(portfolio []portfolioSnapshot, benchmark []backtestTrade) string {
	if len(portfolio) == 0 || len(benchmark) == 0 {
		return ""
	}

	benchmarkByDate := make(map[string]float64, len(benchmark))
	for _, point := range benchmark {
		benchmarkByDate[point.Date] = point.Equity
	}

	type comparePoint struct {
		date      string
		portfolio float64
		benchmark float64
	}
	points := make([]comparePoint, 0, len(portfolio))
	for _, snapshot := range portfolio {
		if benchmarkEquity, ok := benchmarkByDate[snapshot.Date]; ok {
			points = append(points, comparePoint{
				date:      snapshot.Date,
				portfolio: snapshot.Equity,
				benchmark: benchmarkEquity,
			})
		}
	}
	if len(points) == 0 {
		return ""
	}

	minValue := points[0].portfolio
	maxValue := points[0].portfolio
	for _, point := range points {
		if point.portfolio < minValue {
			minValue = point.portfolio
		}
		if point.benchmark < minValue {
			minValue = point.benchmark
		}
		if point.portfolio > maxValue {
			maxValue = point.portfolio
		}
		if point.benchmark > maxValue {
			maxValue = point.benchmark
		}
	}
	if maxValue == minValue {
		maxValue = minValue + 1
	}

	width := 900.0
	height := 260.0
	portfolioLine := make([]string, 0, len(points))
	benchmarkLine := make([]string, 0, len(points))
	for i, point := range points {
		x := (float64(i) / float64(max(1, len(points)-1))) * width
		portfolioY := height - ((point.portfolio-minValue)/(maxValue-minValue))*height
		benchmarkY := height - ((point.benchmark-minValue)/(maxValue-minValue))*height
		portfolioLine = append(portfolioLine, fmt.Sprintf("%.2f,%.2f", x, portfolioY))
		benchmarkLine = append(benchmarkLine, fmt.Sprintf("%.2f,%.2f", x, benchmarkY))
	}

	return fmt.Sprintf(`<svg viewBox="0 0 %.0f %.0f" width="100%%" height="260" role="img" aria-label="Portfolio versus benchmark curve"><rect x="0" y="0" width="100%%" height="100%%" fill="#f7f1e7"/><polyline fill="none" stroke="#0f766e" stroke-width="3" points="%s"/><polyline fill="none" stroke="#b45309" stroke-width="2.5" stroke-dasharray="8 6" points="%s"/><text x="18" y="24" fill="#0f766e" font-size="14">Portfolio</text><text x="110" y="24" fill="#b45309" font-size="14">Benchmark</text></svg>`,
		width, height, strings.Join(portfolioLine, " "), strings.Join(benchmarkLine, " "))
}

func writeCSVFile(path string, rows [][]string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.WriteAll(rows); err != nil {
		return err
	}
	writer.Flush()
	return writer.Error()
}

func writeDashboardReports() error {
	textPath := filepath.Join(reportsDir, "dashboard.txt")
	htmlPath := filepath.Join(reportsDir, "dashboard.html")

	sections := []struct {
		title string
		path  string
	}{
		{title: "Latest Plan", path: filepath.Join(reportsDir, "latest_plan.txt")},
		{title: "A-Share Focus", path: filepath.Join(reportsDir, "a_share_focus.txt")},
		{title: "A-Share Scan", path: filepath.Join(reportsDir, "a_share_scan.txt")},
		{title: "Portfolio Backtest", path: filepath.Join(reportsDir, "portfolio_backtest.txt")},
	}

	type dashboardSection struct {
		Title   string
		Content string
		Stamp   string
	}
	rendered := make([]dashboardSection, 0, len(sections))
	for _, section := range sections {
		content, stamp := readDashboardSection(section.path)
		rendered = append(rendered, dashboardSection{
			Title:   section.title,
			Content: content,
			Stamp:   stamp,
		})
	}

	var textBuilder strings.Builder
	textBuilder.WriteString("Quant MVP Dashboard\n\n")
	for _, section := range rendered {
		textBuilder.WriteString(section.Title + "\n")
		if section.Stamp != "" {
			textBuilder.WriteString("Updated: " + section.Stamp + "\n")
		}
		textBuilder.WriteString(section.Content + "\n\n")
	}

	var cards strings.Builder
	for _, section := range rendered {
		fmt.Fprintf(&cards, `<section class="card"><h2>%s</h2><div class="stamp">%s</div><pre>%s</pre></section>`,
			html.EscapeString(section.Title),
			html.EscapeString(section.Stamp),
			html.EscapeString(section.Content),
		)
	}

	htmlContent := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Quant MVP Dashboard</title>
  <style>
    :root { --bg: #efe8dc; --card: #fffaf3; --ink: #1f1b16; --muted: #746a5d; --border: #d8cebe; }
    * { box-sizing: border-box; }
    body { margin: 0; font-family: Georgia, "Times New Roman", serif; background: radial-gradient(circle at top, #f7f1e7, #e8dcc7 68%%); color: var(--ink); }
    .wrap { max-width: 1280px; margin: 36px auto; padding: 0 20px 40px; }
    h1 { margin: 0 0 18px; font-size: 42px; }
    .lead { margin: 0 0 24px; color: var(--muted); font-size: 17px; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(320px, 1fr)); gap: 18px; }
    .card { background: var(--card); border: 1px solid var(--border); border-radius: 18px; padding: 20px; box-shadow: 0 18px 40px rgba(70, 50, 20, 0.08); min-height: 280px; }
    h2 { margin: 0 0 8px; font-size: 24px; }
    .stamp { margin-bottom: 12px; color: var(--muted); font-size: 13px; text-transform: uppercase; letter-spacing: 0.06em; }
    pre { margin: 0; white-space: pre-wrap; word-break: break-word; font: 14px/1.55 "SFMono-Regular", Menlo, Consolas, monospace; }
  </style>
</head>
<body>
  <div class="wrap">
    <h1>Quant MVP Dashboard</h1>
    <p class="lead">Daily overview of the latest plan, focus list, market scan, and portfolio backtest.</p>
    <div class="grid">%s</div>
  </div>
</body>
</html>`, cards.String())

	if err := os.WriteFile(textPath, []byte(textBuilder.String()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0o644); err != nil {
		return err
	}
	return nil
}

func readDashboardSection(path string) (string, string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "Not generated yet.", ""
	}
	info, statErr := os.Stat(path)
	stamp := ""
	if statErr == nil {
		stamp = info.ModTime().Format("2006-01-02 15:04:05")
	}
	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		trimmed = "Not generated yet."
	}
	return trimmed, stamp
}

func loadBacktestSnapshot(path string) (map[string]backtestResult, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]backtestResult{}, nil
		}
		return nil, err
	}
	defer file.Close()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return map[string]backtestResult{}, nil
	}

	header := make(map[string]int)
	for i, col := range rows[0] {
		header[strings.TrimSpace(col)] = i
	}

	parse := func(row []string, name string) float64 {
		idx, ok := header[name]
		if !ok || idx >= len(row) {
			return 0
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(row[idx]), 64)
		if err != nil {
			return 0
		}
		return value
	}
	get := func(row []string, name string) string {
		idx, ok := header[name]
		if !ok || idx >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[idx])
	}

	results := make(map[string]backtestResult, len(rows)-1)
	for _, row := range rows[1:] {
		symbol := get(row, "symbol")
		if symbol == "" {
			continue
		}
		tradeCount, _ := strconv.Atoi(get(row, "trade_count"))
		results[symbol] = backtestResult{
			Symbol:           symbol,
			Name:             get(row, "name"),
			FromDate:         get(row, "from_date"),
			ToDate:           get(row, "to_date"),
			Mode:             get(row, "mode"),
			TotalReturn:      parse(row, "total_return"),
			AnnualizedReturn: parse(row, "annualized_return"),
			BenchmarkReturn:  parse(row, "benchmark_return"),
			ExcessReturn:     parse(row, "excess_return"),
			MaxDrawdown:      parse(row, "max_drawdown"),
			WinRate:          parse(row, "win_rate"),
			TradeCount:       tradeCount,
		}
	}
	return results, nil
}

func loadPortfolioHoldingsSnapshot(path string) (map[string]bool, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	defer file.Close()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return map[string]bool{}, nil
	}

	results := make(map[string]bool, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) == 0 {
			continue
		}
		symbol := strings.TrimSpace(row[0])
		if symbol == "" {
			continue
		}
		results[symbol] = true
	}
	return results, nil
}

func applyBacktestMetrics(candidate *scanCandidate, metrics backtestResult) {
	candidate.HasBacktest = true
	candidate.BacktestMode = metrics.Mode
	candidate.BacktestFrom = metrics.FromDate
	candidate.BacktestTo = metrics.ToDate
	candidate.BacktestReturn = metrics.TotalReturn
	candidate.BacktestAnnualized = metrics.AnnualizedReturn
	candidate.BacktestBenchmark = metrics.BenchmarkReturn
	candidate.BacktestExcess = metrics.ExcessReturn
	candidate.BacktestDrawdown = metrics.MaxDrawdown
	candidate.BacktestWinRate = metrics.WinRate
	candidate.BacktestTrades = metrics.TradeCount
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampFloat(value float64, lower float64, upper float64) float64 {
	if value < lower {
		return lower
	}
	if value > upper {
		return upper
	}
	return value
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func loadConfig(path string) (config, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return config{}, err
	}

	var cfg config
	var section string

	for lineNumber, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasSuffix(line, ":") {
			section = strings.TrimSuffix(line, ":")
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return config{}, fmt.Errorf("invalid config line %d", lineNumber+1)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

		switch section {
		case "app":
			if key == "name" {
				cfg.AppName = value
			}
		case "db":
			if key == "path" {
				cfg.DB.Path = value
			}
		case "schedule":
			if key == "daily_run" {
				cfg.Schedule.DailyRun = value
			}
			if key == "cache_ttl" {
				cfg.Schedule.CacheTTL = value
			}
		case "strategy":
			switch key {
			case "name":
				cfg.Strategy.Name = value
			case "symbol":
				cfg.Strategy.Symbol = value
			case "data_path":
				cfg.Strategy.DataPath = value
			case "data_source":
				cfg.Strategy.DataSource = value
			case "api_key_env":
				cfg.Strategy.APIKeyEnv = value
			case "short_window":
				window, convErr := strconv.Atoi(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid strategy.short_window: %w", convErr)
				}
				cfg.Strategy.ShortWindow = window
			case "long_window":
				window, convErr := strconv.Atoi(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid strategy.long_window: %w", convErr)
				}
				cfg.Strategy.LongWindow = window
			}
		case "risk":
			switch key {
			case "max_position":
				size, convErr := strconv.Atoi(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid risk.max_position: %w", convErr)
				}
				cfg.Risk.MaxPosition = size
			case "stop_loss_pct":
				pct, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid risk.stop_loss_pct: %w", convErr)
				}
				cfg.Risk.StopLossPct = pct
			case "skip_repeat_signal":
				flag, convErr := strconv.ParseBool(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid risk.skip_repeat_signal: %w", convErr)
				}
				cfg.Risk.SkipRepeatSignal = flag
			}
		case "portfolio":
			switch key {
			case "rebalance_interval_days":
				v, convErr := strconv.Atoi(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.rebalance_interval_days: %w", convErr)
				}
				cfg.Portfolio.RebalanceIntervalDays = v
			case "weight_drift_threshold":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.weight_drift_threshold: %w", convErr)
				}
				cfg.Portfolio.WeightDriftThreshold = v
			case "min_holdings":
				v, convErr := strconv.Atoi(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.min_holdings: %w", convErr)
				}
				cfg.Portfolio.MinHoldings = v
			case "max_position_weight":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.max_position_weight: %w", convErr)
				}
				cfg.Portfolio.MaxPositionWeight = v
			case "max_cash_share":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.max_cash_share: %w", convErr)
				}
				cfg.Portfolio.MaxCashShare = v
			case "max_volatility":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.max_volatility: %w", convErr)
				}
				cfg.Portfolio.MaxVolatility = v
			case "min_average_turnover":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.min_average_turnover: %w", convErr)
				}
				cfg.Portfolio.MinAverageTurnover = v
			case "overheat_threshold":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.overheat_threshold: %w", convErr)
				}
				cfg.Portfolio.OverheatThreshold = v
			case "max_holding_drawdown":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.max_holding_drawdown: %w", convErr)
				}
				cfg.Portfolio.MaxHoldingDrawdown = v
			case "min_trend_gap":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.min_trend_gap: %w", convErr)
				}
				cfg.Portfolio.MinTrendGap = v
			case "stop_cooldown_days":
				v, convErr := strconv.Atoi(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.stop_cooldown_days: %w", convErr)
				}
				cfg.Portfolio.StopCooldownDays = v
			case "exit_cooldown_days":
				v, convErr := strconv.Atoi(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.exit_cooldown_days: %w", convErr)
				}
				cfg.Portfolio.ExitCooldownDays = v
			case "trend_break_cooldown_days":
				v, convErr := strconv.Atoi(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.trend_break_cooldown_days: %w", convErr)
				}
				cfg.Portfolio.TrendBreakCooldownDays = v
			case "momentum_weight":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.momentum_weight: %w", convErr)
				}
				cfg.Portfolio.MomentumWeight = v
			case "persistence_weight":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.persistence_weight: %w", convErr)
				}
				cfg.Portfolio.PersistenceWeight = v
			case "backtest_excess_weight":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.backtest_excess_weight: %w", convErr)
				}
				cfg.Portfolio.BacktestExcessWeight = v
			case "backtest_return_weight":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.backtest_return_weight: %w", convErr)
				}
				cfg.Portfolio.BacktestReturnWeight = v
			case "backtest_drawdown_weight":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.backtest_drawdown_weight: %w", convErr)
				}
				cfg.Portfolio.BacktestDrawdownWeight = v
			case "watch_penalty":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.watch_penalty: %w", convErr)
				}
				cfg.Portfolio.WatchPenalty = v
			case "trend_strategy_enabled":
				v, convErr := strconv.ParseBool(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.trend_strategy_enabled: %w", convErr)
				}
				cfg.Portfolio.TrendStrategyEnabled = v
			case "breakout_enabled":
				v, convErr := strconv.ParseBool(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.breakout_enabled: %w", convErr)
				}
				cfg.Portfolio.BreakoutEnabled = v
			case "pullback_enabled":
				v, convErr := strconv.ParseBool(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.pullback_enabled: %w", convErr)
				}
				cfg.Portfolio.PullbackEnabled = v
			case "trend_strategy_weight":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.trend_strategy_weight: %w", convErr)
				}
				cfg.Portfolio.TrendStrategyWeight = v
			case "breakout_strategy_weight":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.breakout_strategy_weight: %w", convErr)
				}
				cfg.Portfolio.BreakoutStrategyWeight = v
			case "pullback_strategy_weight":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.pullback_strategy_weight: %w", convErr)
				}
				cfg.Portfolio.PullbackStrategyWeight = v
			case "min_price":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.min_price: %w", convErr)
				}
				cfg.Portfolio.MinPrice = v
			case "min_backtest_excess":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.min_backtest_excess: %w", convErr)
				}
				cfg.Portfolio.MinBacktestExcess = v
			case "max_backtest_drawdown":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.max_backtest_drawdown: %w", convErr)
				}
				cfg.Portfolio.MaxBacktestDrawdown = v
			case "limit_move_threshold":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.limit_move_threshold: %w", convErr)
				}
				cfg.Portfolio.LimitMoveThreshold = v
			case "quality_weight":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.quality_weight: %w", convErr)
				}
				cfg.Portfolio.QualityWeight = v
			case "risk_weight":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.risk_weight: %w", convErr)
				}
				cfg.Portfolio.RiskWeight = v
			case "heat_penalty_weight":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.heat_penalty_weight: %w", convErr)
				}
				cfg.Portfolio.HeatPenaltyWeight = v
			case "reversal_weight":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.reversal_weight: %w", convErr)
				}
				cfg.Portfolio.ReversalWeight = v
			}
		case "regime":
			switch key {
			case "cautious_exposure":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid regime.cautious_exposure: %w", convErr)
				}
				cfg.Regime.CautiousExposure = v
			case "risk_off_exposure":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid regime.risk_off_exposure: %w", convErr)
				}
				cfg.Regime.RiskOffExposure = v
			case "risk_off_drawdown":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid regime.risk_off_drawdown: %w", convErr)
				}
				cfg.Regime.RiskOffDrawdown = v
			case "cautious_drawdown":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid regime.cautious_drawdown: %w", convErr)
				}
				cfg.Regime.CautiousDrawdown = v
			case "breadth_risk_off":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid regime.breadth_risk_off: %w", convErr)
				}
				cfg.Regime.BreadthRiskOff = v
			case "breadth_cautious":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid regime.breadth_cautious: %w", convErr)
				}
				cfg.Regime.BreadthCautious = v
			}
		}
	}

	if cfg.AppName == "" {
		cfg.AppName = "quant-mvp"
	}
	if cfg.DB.Path == "" {
		cfg.DB.Path = "data/quant.db"
	}
	if cfg.Schedule.DailyRun == "" {
		return config{}, errors.New("schedule.daily_run is required")
	}
	if cfg.Schedule.CacheTTL == "" {
		cfg.Schedule.CacheTTL = "4h"
	}
	if cfg.Strategy.Name == "" {
		cfg.Strategy.Name = "sma-crossover"
	}
	if cfg.Strategy.Symbol == "" {
		cfg.Strategy.Symbol = "DEMO"
	}
	if cfg.Strategy.DataPath == "" {
		cfg.Strategy.DataPath = "data/market_data.csv"
	}
	if cfg.Strategy.DataSource == "" {
		cfg.Strategy.DataSource = "auto"
	}
	if cfg.Strategy.APIKeyEnv == "" {
		cfg.Strategy.APIKeyEnv = "ALPHAVANTAGE_API_KEY"
	}
	if cfg.Strategy.ShortWindow == 0 {
		cfg.Strategy.ShortWindow = 3
	}
	if cfg.Strategy.LongWindow == 0 {
		cfg.Strategy.LongWindow = 5
	}
	if cfg.Strategy.ShortWindow >= cfg.Strategy.LongWindow {
		return config{}, errors.New("strategy.short_window must be smaller than strategy.long_window")
	}
	if cfg.Risk.MaxPosition <= 0 {
		cfg.Risk.MaxPosition = 1
	}
	if cfg.Risk.StopLossPct <= 0 {
		cfg.Risk.StopLossPct = 0.03
	}
	if cfg.Portfolio.RebalanceIntervalDays <= 0 {
		cfg.Portfolio.RebalanceIntervalDays = 5
	}
	if cfg.Portfolio.WeightDriftThreshold <= 0 {
		cfg.Portfolio.WeightDriftThreshold = 0.20
	}
	if cfg.Portfolio.MinHoldings <= 0 {
		cfg.Portfolio.MinHoldings = 2
	}
	if cfg.Portfolio.MaxPositionWeight <= 0 {
		cfg.Portfolio.MaxPositionWeight = 0.45
	}
	if cfg.Portfolio.MaxCashShare < 0 {
		cfg.Portfolio.MaxCashShare = 0.20
	}
	if cfg.Portfolio.MaxVolatility <= 0 {
		cfg.Portfolio.MaxVolatility = 0.18
	}
	if cfg.Portfolio.MinAverageTurnover <= 0 {
		cfg.Portfolio.MinAverageTurnover = 30_000_000
	}
	if cfg.Portfolio.OverheatThreshold <= 0 {
		cfg.Portfolio.OverheatThreshold = 0.12
	}
	if cfg.Portfolio.MaxHoldingDrawdown <= 0 {
		cfg.Portfolio.MaxHoldingDrawdown = 0.15
	}
	if cfg.Portfolio.MinTrendGap <= 0 {
		cfg.Portfolio.MinTrendGap = 0.02
	}
	if cfg.Portfolio.StopCooldownDays <= 0 {
		cfg.Portfolio.StopCooldownDays = 5
	}
	if cfg.Portfolio.ExitCooldownDays <= 0 {
		cfg.Portfolio.ExitCooldownDays = 3
	}
	if cfg.Portfolio.TrendBreakCooldownDays <= 0 {
		cfg.Portfolio.TrendBreakCooldownDays = 4
	}
	if cfg.Portfolio.MomentumWeight == 0 {
		cfg.Portfolio.MomentumWeight = 0.60
	}
	if cfg.Portfolio.PersistenceWeight == 0 {
		cfg.Portfolio.PersistenceWeight = 0.60
	}
	if cfg.Portfolio.BacktestExcessWeight == 0 {
		cfg.Portfolio.BacktestExcessWeight = 0.35
	}
	if cfg.Portfolio.BacktestReturnWeight == 0 {
		cfg.Portfolio.BacktestReturnWeight = 0.15
	}
	if cfg.Portfolio.BacktestDrawdownWeight == 0 {
		cfg.Portfolio.BacktestDrawdownWeight = 0.20
	}
	if cfg.Portfolio.WatchPenalty == 0 {
		cfg.Portfolio.WatchPenalty = 0.02
	}
	if !cfg.Portfolio.TrendStrategyEnabled && !cfg.Portfolio.BreakoutEnabled && !cfg.Portfolio.PullbackEnabled {
		cfg.Portfolio.TrendStrategyEnabled = true
		cfg.Portfolio.BreakoutEnabled = true
		cfg.Portfolio.PullbackEnabled = true
	}
	if cfg.Portfolio.TrendStrategyWeight == 0 {
		cfg.Portfolio.TrendStrategyWeight = 1.0
	}
	if cfg.Portfolio.BreakoutStrategyWeight == 0 {
		cfg.Portfolio.BreakoutStrategyWeight = 1.0
	}
	if cfg.Portfolio.PullbackStrategyWeight == 0 {
		cfg.Portfolio.PullbackStrategyWeight = 1.0
	}
	if cfg.Portfolio.MinPrice <= 0 {
		cfg.Portfolio.MinPrice = 3.0
	}
	if cfg.Portfolio.MaxBacktestDrawdown <= 0 {
		cfg.Portfolio.MaxBacktestDrawdown = 0.25
	}
	if cfg.Portfolio.LimitMoveThreshold <= 0 {
		cfg.Portfolio.LimitMoveThreshold = 0.095
	}
	if cfg.Portfolio.QualityWeight == 0 {
		cfg.Portfolio.QualityWeight = 1.10
	}
	if cfg.Portfolio.RiskWeight == 0 {
		cfg.Portfolio.RiskWeight = 0.80
	}
	if cfg.Portfolio.HeatPenaltyWeight == 0 {
		cfg.Portfolio.HeatPenaltyWeight = 1.10
	}
	if cfg.Portfolio.ReversalWeight == 0 {
		cfg.Portfolio.ReversalWeight = 0.90
	}
	if cfg.Regime.CautiousExposure <= 0 {
		cfg.Regime.CautiousExposure = 0.45
	}
	if cfg.Regime.RiskOffExposure < 0 {
		cfg.Regime.RiskOffExposure = 0
	}
	if cfg.Regime.RiskOffDrawdown <= 0 {
		cfg.Regime.RiskOffDrawdown = 0.12
	}
	if cfg.Regime.CautiousDrawdown <= 0 {
		cfg.Regime.CautiousDrawdown = 0.06
	}
	if cfg.Regime.BreadthRiskOff <= 0 {
		cfg.Regime.BreadthRiskOff = 0.25
	}
	if cfg.Regime.BreadthCautious <= 0 {
		cfg.Regime.BreadthCautious = 0.45
	}

	return cfg, nil
}

func ensureSQLiteDB(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	if _, err := os.Stat(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return execSQLite(path, `
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
);`)
}

func runStrategy(path string, strategy strategyConfig, risk riskConfig) error {
	now := time.Now()
	signal, err := evaluateSignal(path, strategy, risk)
	if err != nil {
		note := fmt.Sprintf("strategy evaluation failed: %v", err)
		sql := fmt.Sprintf(
			"INSERT INTO execution_records (strategy_name, status, note, executed_at) VALUES (%s, %s, %s, %s);",
			quoteSQL(strategy.Name),
			quoteSQL("failed"),
			quoteSQL(note),
			quoteSQL(now.Format(time.RFC3339)),
		)
		if execErr := execSQLite(path, sql); execErr != nil {
			return fmt.Errorf("%v; additionally failed to persist error: %w", err, execErr)
		}
		return err
	}

	if signal.Action == "SKIP" {
		if err := writePlanReports(signal, now); err != nil {
			return err
		}
		printTradingPlan(signal, now)
		sql := fmt.Sprintf(
			"INSERT INTO execution_records (strategy_name, status, note, executed_at) VALUES (%s, %s, %s, %s);",
			quoteSQL(strategy.Name),
			quoteSQL("skipped"),
			quoteSQL(fmt.Sprintf("%s source=%s", signal.Reason, signal.DataSource)),
			quoteSQL(now.Format(time.RFC3339)),
		)
		return execSQLite(path, sql)
	}

	if err := writePlanReports(signal, now); err != nil {
		return err
	}
	printTradingPlan(signal, now)
	sql := buildPersistSQL(strategy.Name, signal, now)
	return execSQLite(path, sql)
}

func buildPersistSQL(strategyName string, signal strategySignal, now time.Time) string {
	note := fmt.Sprintf(
		"signal=%s mode=%s symbol=%s source=%s reason=%s short_ma=%.2f long_ma=%.2f close=%.2f position=%d plan=%s",
		signal.Action,
		signal.Mode,
		signal.Symbol,
		signal.DataSource,
		signal.Reason,
		signal.ShortMA,
		signal.LongMA,
		signal.ClosePrice,
		signal.PositionSize,
		signal.Plan,
	)

	positionSide := "FLAT"
	entryPrice := 0.0
	if signal.PositionSize > 0 {
		positionSide = "LONG"
		entryPrice = signal.ClosePrice
	}

	return fmt.Sprintf(`
INSERT INTO signal_records (
    strategy_name, symbol, signal, reason, short_ma, long_ma, open_price, high_price, low_price, close_price, volume, position_size, decided_at
) VALUES (
    %s, %s, %s, %s, %.6f, %.6f, %.6f, %.6f, %.6f, %.6f, %.6f, %d, %s
);
INSERT INTO execution_records (strategy_name, status, note, executed_at)
VALUES (%s, %s, %s, %s);
INSERT INTO position_state (symbol, side, quantity, entry_price, updated_at)
VALUES (%s, %s, %d, %.6f, %s)
ON CONFLICT(symbol) DO UPDATE SET
    side=excluded.side,
    quantity=excluded.quantity,
    entry_price=excluded.entry_price,
    updated_at=excluded.updated_at;`,
		quoteSQL(strategyName),
		quoteSQL(signal.Symbol),
		quoteSQL(signal.Action),
		quoteSQL(signal.Reason),
		signal.ShortMA,
		signal.LongMA,
		signal.OpenPrice,
		signal.HighPrice,
		signal.LowPrice,
		signal.ClosePrice,
		signal.Volume,
		signal.PositionSize,
		quoteSQL(now.Format(time.RFC3339)),
		quoteSQL(strategyName),
		quoteSQL("success"),
		quoteSQL(note),
		quoteSQL(now.Format(time.RFC3339)),
		quoteSQL(signal.Symbol),
		quoteSQL(positionSide),
		signal.PositionSize,
		entryPrice,
		quoteSQL(now.Format(time.RFC3339)),
	)
}

func evaluateSignal(dbPath string, strategy strategyConfig, risk riskConfig) (strategySignal, error) {
	bars, dataSource, sourceErr, err := loadBars(strategy)
	if err != nil {
		return strategySignal{}, err
	}
	if len(bars) < strategy.LongWindow {
		return strategySignal{}, fmt.Errorf("not enough price points: need %d, got %d", strategy.LongWindow, len(bars))
	}

	closes := make([]float64, 0, len(bars))
	for _, bar := range bars {
		closes = append(closes, bar.Close)
	}

	latest := bars[len(bars)-1]
	shortMA := average(closes[len(closes)-strategy.ShortWindow:])
	longMA := average(closes[len(closes)-strategy.LongWindow:])
	mode := modeFromDataSource(dataSource)

	state, err := loadPositionState(dbPath, strategy.Symbol)
	if err != nil {
		return strategySignal{}, err
	}
	lastSignal, err := loadLastSignal(dbPath, strategy.Symbol)
	if err != nil {
		return strategySignal{}, err
	}

	action := "HOLD"
	reason := "moving averages are neutral"
	positionSize := state.Quantity

	if state.Quantity > 0 && latest.Close <= state.EntryPrice*(1-risk.StopLossPct) {
		action = "SELL"
		reason = fmt.Sprintf("stop loss triggered at %.2f%%", risk.StopLossPct*100)
		positionSize = 0
	} else if shortMA > longMA && state.Quantity < risk.MaxPosition {
		action = "BUY"
		reason = "short moving average crossed above long moving average"
		positionSize = risk.MaxPosition
	} else if shortMA < longMA && state.Quantity > 0 {
		action = "SELL"
		reason = "short moving average crossed below long moving average"
		positionSize = 0
	}

	if risk.SkipRepeatSignal && action == lastSignal && (action == "BUY" || action == "SELL" || action == "HOLD") {
		return strategySignal{
			Symbol:       strategy.Symbol,
			Action:       "SKIP",
			Mode:         mode,
			Plan:         planForAction(action, state.Quantity > 0, mode),
			MarketDate:   latest.Date,
			ShortMA:      shortMA,
			LongMA:       longMA,
			ClosePrice:   latest.Close,
			OpenPrice:    latest.Open,
			HighPrice:    latest.High,
			LowPrice:     latest.Low,
			Volume:       latest.Volume,
			Reason:       fmt.Sprintf("repeat signal filtered: %s", action),
			DataSource:   withSourceSuffix(dataSource, sourceErr),
			PositionSize: state.Quantity,
		}, nil
	}

	reason = buildReasonWithSource(reason, withSourceSuffix(dataSource, sourceErr))

	return strategySignal{
		Symbol:       strategy.Symbol,
		Action:       action,
		Mode:         mode,
		Plan:         planForAction(action, state.Quantity > 0, mode),
		MarketDate:   latest.Date,
		ShortMA:      shortMA,
		LongMA:       longMA,
		ClosePrice:   latest.Close,
		OpenPrice:    latest.Open,
		HighPrice:    latest.High,
		LowPrice:     latest.Low,
		Volume:       latest.Volume,
		Reason:       reason,
		DataSource:   dataSource,
		PositionSize: positionSize,
	}, nil
}

func loadBars(strategy strategyConfig) ([]marketBar, string, string, error) {
	return loadSymbolBars(strategy.Symbol, strategy.DataSource, strategy.DataPath, strategy.APIKeyEnv, true)
}

func loadSymbolBars(symbol string, dataSource string, dataPath string, apiKeyEnv string, allowCSVFallback bool) ([]marketBar, string, string, error) {
	switch strings.ToLower(dataSource) {
	case "auto":
		if isAShareSymbol(symbol) {
			bars, source, sourceErr, err := loadAShareBars(symbol)
			if err == nil {
				return bars, source, sourceErr, nil
			}
			if allowCSVFallback && dataPath != "" {
				csvBars, csvErr := loadBarsFromCSV(dataPath)
				if csvErr == nil {
					return csvBars, "csv", err.Error(), nil
				}
				return nil, "", "", fmt.Errorf("a-share providers failed: %v; csv fallback failed: %w", err, csvErr)
			}
			return nil, "", "", err
		}

		bars, err := loadCachedProviderBars("alphavantage", symbol, func() ([]marketBar, error) {
			return loadBarsFromAlphaVantage(symbol, os.Getenv(apiKeyEnv))
		})
		if err == nil {
			return bars, "alphavantage", "", nil
		}
		if allowCSVFallback && dataPath != "" {
			csvBars, csvErr := loadBarsFromCSV(dataPath)
			if csvErr == nil {
				return csvBars, "csv", err.Error(), nil
			}
			return nil, "", "", fmt.Errorf("alphavantage failed: %v; csv fallback failed: %w", err, csvErr)
		}
		return nil, "", "", err
	case "tushare":
		bars, source, sourceErr, err := loadAShareBarsWithPrimary(symbol, "tushare")
		if err == nil {
			return bars, source, sourceErr, nil
		}
		if allowCSVFallback && dataPath != "" {
			csvBars, csvErr := loadBarsFromCSV(dataPath)
			if csvErr == nil {
				return csvBars, "csv", err.Error(), nil
			}
			return nil, "", "", fmt.Errorf("tushare failed: %v; csv fallback failed: %w", err, csvErr)
		}
		return nil, "", "", err
	case "baostock":
		bars, source, sourceErr, err := loadAShareBarsWithPrimary(symbol, "baostock")
		if err == nil {
			return bars, source, sourceErr, nil
		}
		if allowCSVFallback && dataPath != "" {
			csvBars, csvErr := loadBarsFromCSV(dataPath)
			if csvErr == nil {
				return csvBars, "csv", err.Error(), nil
			}
			return nil, "", "", fmt.Errorf("baostock failed: %v; csv fallback failed: %w", err, csvErr)
		}
		return nil, "", "", err
	case "alphavantage":
		bars, err := loadCachedProviderBars("alphavantage", symbol, func() ([]marketBar, error) {
			return loadBarsFromAlphaVantage(symbol, os.Getenv(apiKeyEnv))
		})
		if err == nil {
			return bars, "alphavantage", "", nil
		}
		if allowCSVFallback && dataPath != "" {
			csvBars, csvErr := loadBarsFromCSV(dataPath)
			if csvErr == nil {
				return csvBars, "csv", err.Error(), nil
			}
			return nil, "", "", fmt.Errorf("alphavantage failed: %v; csv fallback failed: %w", err, csvErr)
		}
		return nil, "", "", err
	case "csv":
		bars, err := loadBarsFromCSV(dataPath)
		return bars, "csv", "", err
	default:
		return nil, "", "", fmt.Errorf("unsupported data source %q", dataSource)
	}
}

func loadAShareBars(symbol string) ([]marketBar, string, string, error) {
	return loadAShareBarsWithPrimary(symbol, "baostock")
}

func loadAShareBarsWithPrimary(symbol string, primary string) ([]marketBar, string, string, error) {
	type providerSpec struct {
		name  string
		fetch func() ([]marketBar, error)
	}

	providers := []providerSpec{
		{
			name: "baostock",
			fetch: func() ([]marketBar, error) {
				return loadBarsFromBaoStock(symbol)
			},
		},
		{
			name: "tushare",
			fetch: func() ([]marketBar, error) {
				return loadBarsFromTushare(symbol, os.Getenv("TUSHARE_TOKEN"))
			},
		},
	}
	if primary == "tushare" {
		providers[0], providers[1] = providers[1], providers[0]
	}

	var errs []string
	for idx, provider := range providers {
		bars, err := loadCachedProviderBars(provider.name, symbol, provider.fetch)
		if err == nil {
			if idx == 0 {
				return bars, provider.name, "", nil
			}
			return bars, provider.name, strings.Join(errs, "; "), nil
		}
		errs = append(errs, err.Error())
	}

	return nil, "", "", errors.New(strings.Join(errs, "; "))
}

func loadCachedProviderBars(provider string, symbol string, fetch func() ([]marketBar, error)) ([]marketBar, error) {
	path := providerCachePath(provider, symbol)
	if bars, fresh, err := loadCachedBars(path, 4*time.Hour); err == nil && fresh {
		return bars, nil
	}

	bars, err := fetch()
	if err == nil {
		if writeErr := writeBarsCache(path, bars); writeErr != nil {
			return bars, nil
		}
		return bars, nil
	}

	if bars, _, cacheErr := loadCachedBars(path, 365*24*time.Hour); cacheErr == nil && len(bars) > 0 {
		return bars, nil
	}
	return nil, err
}

func providerCachePath(provider string, symbol string) string {
	safeSymbol := strings.NewReplacer(".", "_", "/", "_", "\\", "_", " ", "_").Replace(strings.ToLower(symbol))
	return filepath.Join(cacheDir, fmt.Sprintf("%s_%s.csv", provider, safeSymbol))
}

func loadCachedBars(path string, maxAge time.Duration) ([]marketBar, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, false, err
	}
	bars, err := loadBarsFromCSV(path)
	if err != nil {
		return nil, false, err
	}
	return bars, time.Since(info.ModTime()) <= maxAge, nil
}

func writeBarsCache(path string, bars []marketBar) error {
	rows := [][]string{{"timestamp", "open", "high", "low", "close", "volume"}}
	for _, bar := range bars {
		rows = append(rows, []string{
			bar.Date,
			fmt.Sprintf("%.6f", bar.Open),
			fmt.Sprintf("%.6f", bar.High),
			fmt.Sprintf("%.6f", bar.Low),
			fmt.Sprintf("%.6f", bar.Close),
			fmt.Sprintf("%.6f", bar.Volume),
		})
	}
	return writeCSVFile(path, rows)
}

func withSourceSuffix(dataSource string, sourceErr string) string {
	if sourceErr == "" {
		return fmt.Sprintf("source=%s", dataSource)
	}
	return fmt.Sprintf("source=%s fallback_reason=%s", dataSource, sourceErr)
}

func buildReasonWithSource(reason string, sourceDetail string) string {
	if sourceDetail == "" {
		return reason
	}
	return fmt.Sprintf("%s; %s", reason, sourceDetail)
}

func modeFromDataSource(dataSource string) string {
	if dataSource == "alphavantage" || dataSource == "eastmoney" || dataSource == "tushare" || dataSource == "baostock" {
		return "live"
	}
	return "test"
}

func planForAction(action string, hasPosition bool, mode string) string {
	prefix := "Tomorrow plan"
	if mode == "test" {
		prefix = "Test-mode tomorrow plan"
	}

	switch action {
	case "BUY":
		if hasPosition {
			return prefix + ": hold existing long, do not add beyond max position"
		}
		return prefix + ": open a starter long position and cap size at configured max position"
	case "SELL":
		if hasPosition {
			return prefix + ": reduce or close the long position, do not add new longs"
		}
		return prefix + ": stay flat and avoid opening a new long"
	case "HOLD":
		if hasPosition {
			return prefix + ": keep the current position and wait for the next close"
		}
		return prefix + ": stay flat and wait for confirmation"
	case "SKIP":
		if hasPosition {
			return prefix + ": keep the current position, repeated signal was filtered"
		}
		return prefix + ": no change, repeated signal was filtered"
	default:
		return prefix + ": no action"
	}
}

func printTradingPlan(signal strategySignal, now time.Time) {
	nextTradeDate := nextTradingDateFromSignal(signal, now).Format("2006-01-02")
	fmt.Printf(
		"Mode: %s\nMarket date: %s\nSignal: %s\nReason: %s\nPlan for %s: %s\n\n",
		signal.Mode,
		signal.MarketDate,
		signal.Action,
		signal.Reason,
		nextTradeDate,
		signal.Plan,
	)
}

func writePlanReports(signal strategySignal, now time.Time) error {
	reportDate := nextTradingDateFromSignal(signal, now).Format("2006-01-02")
	textPath := filepath.Join(reportsDir, "latest_plan.txt")
	htmlPath := filepath.Join(reportsDir, "latest_plan.html")

	textContent := fmt.Sprintf(
		"Mode: %s\nMarket date: %s\nSignal: %s\nReason: %s\nPlan for %s: %s\n",
		signal.Mode,
		signal.MarketDate,
		signal.Action,
		signal.Reason,
		reportDate,
		signal.Plan,
	)

	htmlContent := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Trading Plan</title>
  <style>
    :root {
      --bg: #f4efe6;
      --card: #fffaf2;
      --ink: #1c1a17;
      --muted: #6f6658;
      --accent: #0f766e;
      --warn: #b45309;
      --border: #d9cfbf;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: Georgia, "Times New Roman", serif;
      background: linear-gradient(135deg, #f4efe6, #e7dcc8);
      color: var(--ink);
    }
    .wrap {
      max-width: 760px;
      margin: 48px auto;
      padding: 0 20px;
    }
    .card {
      background: var(--card);
      border: 1px solid var(--border);
      border-radius: 20px;
      padding: 28px;
      box-shadow: 0 18px 50px rgba(60, 40, 10, 0.08);
    }
    .eyebrow {
      margin: 0 0 8px;
      color: var(--muted);
      text-transform: uppercase;
      letter-spacing: 0.08em;
      font-size: 12px;
    }
    h1 {
      margin: 0 0 8px;
      font-size: 40px;
      line-height: 1;
    }
    .signal {
      color: var(--accent);
      font-weight: 700;
    }
    .meta, .reason {
      color: var(--muted);
      font-size: 16px;
      line-height: 1.6;
    }
    .plan {
      margin-top: 20px;
      padding: 18px;
      border-radius: 16px;
      background: #f7f1e7;
      border-left: 6px solid var(--warn);
      font-size: 20px;
      line-height: 1.5;
    }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <p class="eyebrow">%s mode</p>
      <h1><span class="signal">%s</span></h1>
      <p class="meta">Market date: %s</p>
      <p class="meta">Plan for %s</p>
      <p class="reason">%s</p>
      <div class="plan">%s</div>
    </div>
  </div>
</body>
</html>
`,
		html.EscapeString(strings.ToUpper(signal.Mode)),
		html.EscapeString(signal.Action),
		html.EscapeString(signal.MarketDate),
		html.EscapeString(reportDate),
		html.EscapeString(signal.Reason),
		html.EscapeString(signal.Plan),
	)

	if err := os.WriteFile(textPath, []byte(textContent), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0o644); err != nil {
		return err
	}
	return writeDashboardReports()
}

func nextTradingDateFromSignal(signal strategySignal, now time.Time) time.Time {
	kind := marketKindUS
	if isAShareSymbol(signal.Symbol) {
		kind = marketKindAShare
	}
	base := now
	if parsed, err := time.ParseInLocation("2006-01-02", signal.MarketDate, now.Location()); err == nil {
		base = parsed
	}
	return nextTradingDay(base, kind)
}

func nextTradingDay(base time.Time, kind marketKind) time.Time {
	candidate := time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location()).AddDate(0, 0, 1)
	for {
		if isTradingDay(candidate, kind) {
			return candidate
		}
		candidate = candidate.AddDate(0, 0, 1)
	}
}

func isTradingDay(day time.Time, kind marketKind) bool {
	switch day.Weekday() {
	case time.Saturday, time.Sunday:
		return false
	}
	// Keep the calendar deliberately conservative for now: skip weekends
	// and a minimal fixed-date holiday set so reports do not point to obvious
	// non-trading days.
	switch kind {
	case marketKindAShare:
		if isFixedHoliday(day, [][2]int{{1, 1}, {5, 1}, {10, 1}, {10, 2}, {10, 3}}) {
			return false
		}
	case marketKindUS:
		if isFixedHoliday(day, [][2]int{{1, 1}, {7, 4}, {12, 25}}) {
			return false
		}
	}
	return true
}

func isFixedHoliday(day time.Time, monthDays [][2]int) bool {
	for _, item := range monthDays {
		if int(day.Month()) == item[0] && day.Day() == item[1] {
			return true
		}
	}
	return false
}

func loadBarsFromAlphaVantage(symbol string, apiKey string) ([]marketBar, error) {
	if apiKey == "" {
		return nil, errors.New("missing Alpha Vantage API key")
	}

	url := fmt.Sprintf(
		"https://www.alphavantage.co/query?function=TIME_SERIES_DAILY&symbol=%s&outputsize=compact&datatype=csv&apikey=%s",
		symbol,
		apiKey,
	)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("alphavantage returned status %s", resp.Status)
	}

	reader := csv.NewReader(resp.Body)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	return extractBars(rows)
}

func loadBarsFromTushare(symbol string, token string) ([]marketBar, error) {
	if token == "" {
		return nil, errors.New("missing TUSHARE_TOKEN")
	}

	tsCode, err := tushareTSCode(symbol)
	if err != nil {
		return nil, err
	}

	requestBody, err := json.Marshal(tushareRequest{
		APIName: "daily",
		Token:   token,
		Params: map[string]any{
			"ts_code":    tsCode,
			"start_date": "20250101",
			"end_date":   "20300101",
		},
		Fields: "ts_code,trade_date,open,high,low,close,vol",
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.tushare.pro", strings.NewReader(string(requestBody)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tushare returned status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response tushareResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if response.Code != 0 {
		return nil, fmt.Errorf("tushare error %d: %s", response.Code, response.Msg)
	}
	if response.Data == nil || len(response.Data.Items) == 0 {
		return nil, errors.New("tushare returned no daily data")
	}

	bars := make([]marketBar, 0, len(response.Data.Items))
	for i := len(response.Data.Items) - 1; i >= 0; i-- {
		item := response.Data.Items[i]
		if len(item) < 7 {
			return nil, errors.New("unexpected tushare item format")
		}
		tradeDate, _ := item[1].(string)
		openPrice, err := toFloat(item[2])
		if err != nil {
			return nil, err
		}
		highPrice, err := toFloat(item[3])
		if err != nil {
			return nil, err
		}
		lowPrice, err := toFloat(item[4])
		if err != nil {
			return nil, err
		}
		closePrice, err := toFloat(item[5])
		if err != nil {
			return nil, err
		}
		volume, err := toFloat(item[6])
		if err != nil {
			return nil, err
		}

		bars = append(bars, marketBar{
			Date:   formatTradeDate(tradeDate),
			Open:   openPrice,
			High:   highPrice,
			Low:    lowPrice,
			Close:  closePrice,
			Volume: volume * 100,
		})
	}

	return bars, nil
}

func loadBarsFromBaoStock(symbol string) ([]marketBar, error) {
	normalized, err := normalizeAShareSymbol(symbol)
	if err != nil {
		return nil, err
	}
	parts := strings.SplitN(normalized, ".", 2)
	baoCode := strings.ToLower(parts[1]) + "." + parts[0]
	return loadBarsFromBaoStockCode(baoCode)
}

func loadAShareBenchmarkBars() ([]marketBar, error) {
	return loadCachedProviderBars("baostock", aShareBenchmarkSymbol, func() ([]marketBar, error) {
		return loadBarsFromBaoStockCode("sh.000300")
	})
}

func loadBarsFromBaoStockCode(baoCode string) ([]marketBar, error) {
	script := fmt.Sprintf(`import baostock as bs
lg = bs.login()
if lg.error_code != '0':
    raise SystemExit(lg.error_msg)
rs = bs.query_history_k_data_plus("%s","date,open,high,low,close,volume", start_date='2025-01-01', end_date='2030-12-31', frequency='d', adjustflag='3')
rows = []
while (rs.error_code == '0') and rs.next():
    rows.append(rs.get_row_data())
bs.logout()
for row in rows:
    print(",".join(row))
`, baoCode)

	cmd := exec.Command("python3", "-c", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("baostock failed: %w: %s", err, strings.TrimSpace(string(output)))
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, errors.New("baostock returned no history")
	}

	bars := make([]marketBar, 0, len(lines))
	for _, line := range lines {
		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			continue
		}
		openPrice, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return nil, err
		}
		highPrice, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return nil, err
		}
		lowPrice, err := strconv.ParseFloat(parts[3], 64)
		if err != nil {
			return nil, err
		}
		closePrice, err := strconv.ParseFloat(parts[4], 64)
		if err != nil {
			return nil, err
		}
		volume, err := strconv.ParseFloat(parts[5], 64)
		if err != nil {
			return nil, err
		}

		bars = append(bars, marketBar{
			Date:   parts[0],
			Open:   openPrice,
			High:   highPrice,
			Low:    lowPrice,
			Close:  closePrice,
			Volume: volume,
		})
	}

	return bars, nil
}

func loadBarsFromEastmoney(symbol string) ([]marketBar, error) {
	normalized, err := normalizeAShareSymbol(symbol)
	if err != nil {
		return nil, err
	}
	secID, err := eastmoneySecID(normalized)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf(
		"https://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&ut=fa5fd1943c7b386f172d6893dbfba10b&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61&klt=101&fqt=1&end=20500101&lmt=120",
		secID,
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("eastmoney returned status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return extractEastmoneyBars(body)
}

func loadBarsFromCSV(path string) ([]marketBar, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	return extractBars(rows)
}

type eastmoneyKlineResponse struct {
	Data *struct {
		Klines []string `json:"klines"`
	} `json:"data"`
}

type tushareRequest struct {
	APIName string         `json:"api_name"`
	Token   string         `json:"token"`
	Params  map[string]any `json:"params"`
	Fields  string         `json:"fields"`
}

type tushareResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data *struct {
		Fields []string `json:"fields"`
		Items  [][]any  `json:"items"`
	} `json:"data"`
}

type tushareStockBasicResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data *struct {
		Fields []string `json:"fields"`
		Items  [][]any  `json:"items"`
	} `json:"data"`
}

type aShareSymbol struct {
	Symbol   string
	Name     string
	Industry string
}

func extractEastmoneyBars(body []byte) ([]marketBar, error) {
	var response eastmoneyKlineResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if response.Data == nil || len(response.Data.Klines) == 0 {
		return nil, errors.New("eastmoney returned no kline data")
	}

	bars := make([]marketBar, 0, len(response.Data.Klines))
	for index, line := range response.Data.Klines {
		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			return nil, fmt.Errorf("unexpected eastmoney kline format at row %d", index+1)
		}

		openPrice, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid eastmoney open price at row %d: %w", index+1, err)
		}
		closePrice, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid eastmoney close price at row %d: %w", index+1, err)
		}
		highPrice, err := strconv.ParseFloat(parts[3], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid eastmoney high price at row %d: %w", index+1, err)
		}
		lowPrice, err := strconv.ParseFloat(parts[4], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid eastmoney low price at row %d: %w", index+1, err)
		}
		volume, err := strconv.ParseFloat(parts[5], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid eastmoney volume at row %d: %w", index+1, err)
		}

		bars = append(bars, marketBar{
			Date:   parts[0],
			Open:   openPrice,
			High:   highPrice,
			Low:    lowPrice,
			Close:  closePrice,
			Volume: volume,
		})
	}

	return bars, nil
}

func loadAShareUniverse() ([]aShareSymbol, error) {
	file, err := os.Open(aShareUniversePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, errors.New("a-share universe csv must include header and at least one row")
	}

	symbols := make([]aShareSymbol, 0, len(rows)-1)
	for rowNumber, row := range rows[1:] {
		if len(row) < 1 {
			continue
		}
		symbol := strings.TrimSpace(row[0])
		if symbol == "" {
			continue
		}
		name := ""
		industry := ""
		if len(row) > 1 {
			name = strings.TrimSpace(row[1])
		}
		if len(row) > 2 {
			industry = strings.TrimSpace(row[2])
		}
		if !isAShareSymbol(symbol) {
			return nil, fmt.Errorf("invalid A-share symbol %q at row %d", symbol, rowNumber+2)
		}
		symbols = append(symbols, aShareSymbol{
			Symbol:   symbol,
			Name:     name,
			Industry: industry,
		})
	}
	return symbols, nil
}

func rankCandidate(symbol string, name string, industry string, bars []marketBar, dataSource string, sourceErr string, strategy strategyConfig, portfolio portfolioConfig) (scanCandidate, error) {
	closes := make([]float64, 0, len(bars))
	volumes := make([]float64, 0, len(bars))
	for _, bar := range bars {
		closes = append(closes, bar.Close)
		volumes = append(volumes, bar.Volume)
	}
	if len(closes) < strategy.LongWindow {
		return scanCandidate{}, errors.New("not enough bars")
	}

	latest := bars[len(bars)-1]
	shortMA := average(closes[len(closes)-strategy.ShortWindow:])
	longMA := average(closes[len(closes)-strategy.LongWindow:])
	avgVolume := average(volumes[len(volumes)-strategy.LongWindow:])
	trendScore, liquidityScore, structureScore, momentumScore, persistenceScore, breakoutScore, volumeTrendScore, riskPenalty, score := scoreCandidate(bars, strategy.ShortWindow, strategy.LongWindow)
	shortReturnScore := trailingReturn(closes, min(5, len(closes)-1))
	mediumReturnScore := trailingReturn(closes, min(20, len(closes)-1))
	qualityScore, riskScore, heatPenalty, reversalScore := candidateOverlayScores(bars, shortMA, longMA, avgVolume, shortReturnScore, mediumReturnScore, trendScore, liquidityScore, persistenceScore, breakoutScore, volumeTrendScore, riskPenalty)
	action, strategyAlignment, strategyVotes, reason, trigger := evaluateStrategyEnsemble(bars, shortMA, longMA, avgVolume, shortReturnScore, mediumReturnScore, dataSource, sourceErr, portfolio)
	score = score*0.35 +
		qualityScore*portfolio.QualityWeight +
		riskScore*portfolio.RiskWeight +
		reversalScore*portfolio.ReversalWeight -
		heatPenalty*portfolio.HeatPenaltyWeight +
		strategyAlignment*0.50

	bucket, reason, trigger, triggerPrice, avoidTags := classifyCandidate(name, latest.Close, shortMA, longMA, avgVolume, action, score, reason, trigger)
	candidate := scanCandidate{
		Symbol:            symbol,
		Name:              name,
		Industry:          industry,
		Action:            action,
		Bucket:            bucket,
		Score:             score,
		QualityScore:      qualityScore,
		RiskScore:         riskScore,
		HeatPenalty:       heatPenalty,
		ReversalScore:     reversalScore,
		TrendScore:        trendScore,
		LiquidityScore:    liquidityScore,
		StructureScore:    structureScore,
		MomentumScore:     momentumScore,
		PersistenceScore:  persistenceScore,
		BreakoutScore:     breakoutScore,
		VolumeTrendScore:  volumeTrendScore,
		ShortReturnScore:  shortReturnScore,
		MediumReturnScore: mediumReturnScore,
		StrategyAlignment: strategyAlignment,
		StrategyVotes:     strategyVotes,
		RiskPenalty:       riskPenalty,
		AvgVolume:         avgVolume,
		Trigger:           trigger,
		TriggerPrice:      triggerPrice,
		AvoidTags:         avoidTags,
		ShortMA:           shortMA,
		LongMA:            longMA,
		ClosePrice:        latest.Close,
		MarketDate:        latest.Date,
		Reason:            reason,
		Plan:              planForBucket(bucket, action, shortMA, longMA, avgVolume),
	}
	candidate.ModelScore = predictLinearModel(candidate)
	if candidate.ModelScore != 0 {
		candidate.Score = candidate.Score*0.70 + candidate.ModelScore*0.30
	}
	return candidate, nil
}

func scoreCandidate(bars []marketBar, shortWindow int, longWindow int) (float64, float64, float64, float64, float64, float64, float64, float64, float64) {
	if len(bars) < longWindow {
		return 0, 0, 0, 0, 0, 0, 0, 0, 0
	}
	closes := make([]float64, 0, len(bars))
	volumes := make([]float64, 0, len(bars))
	for _, bar := range bars {
		closes = append(closes, bar.Close)
		volumes = append(volumes, bar.Volume)
	}
	latest := bars[len(bars)-1]
	shortMA := average(closes[len(closes)-shortWindow:])
	longMA := average(closes[len(closes)-longWindow:])
	avgVolume := average(volumes[len(volumes)-longWindow:])

	trendScore := 0.0
	if longMA > 0 {
		trendScore = (shortMA - longMA) / longMA
	}

	liquidityScore := 0.0
	if avgVolume > 0 {
		liquidityScore = math.Min(math.Log10(avgVolume+1)/8.0, 0.03)
	}

	structureScore := 0.0
	if shortMA > 0 {
		structureScore = (latest.Close - shortMA) / shortMA
	}

	momentumScore := 0.0
	baseClose := bars[len(bars)-longWindow].Close
	if baseClose > 0 {
		momentumScore = clampFloat(latest.Close/baseClose-1, -0.12, 0.12) * 0.35
	}

	persistenceScore := 0.0
	if longWindow > 1 {
		windowBars := bars[len(bars)-longWindow:]
		positiveDays := 0
		aboveLongDays := 0
		for i, bar := range windowBars {
			if bar.Close >= longMA {
				aboveLongDays++
			}
			if i > 0 && bar.Close > windowBars[i-1].Close {
				positiveDays++
			}
		}
		upRatio := float64(positiveDays) / float64(len(windowBars)-1)
		aboveRatio := float64(aboveLongDays) / float64(len(windowBars))
		persistenceScore = ((upRatio+aboveRatio)/2.0 - 0.5) * 0.10
	}

	breakoutScore := 0.0
	highestClose := closes[len(closes)-longWindow]
	for _, closePrice := range closes[len(closes)-longWindow:] {
		if closePrice > highestClose {
			highestClose = closePrice
		}
	}
	if highestClose > 0 {
		breakoutGap := latest.Close/highestClose - 1
		if breakoutGap >= -0.01 {
			breakoutScore = clampFloat(breakoutGap+0.01, 0, 0.03)
		}
	}

	volumeTrendScore := 0.0
	recentVolumeWindow := min(3, len(volumes))
	recentAvgVolume := average(volumes[len(volumes)-recentVolumeWindow:])
	if avgVolume > 0 {
		volumeRatio := recentAvgVolume/avgVolume - 1
		volumeTrendScore = clampFloat(volumeRatio, -0.30, 0.50) * 0.08
	}

	riskPenalty := 0.0
	stopLine := longMA * 0.97
	if stopLine > 0 && latest.Close < stopLine {
		riskPenalty = (stopLine - latest.Close) / stopLine
	}

	total := trendScore + liquidityScore + structureScore + momentumScore + persistenceScore + breakoutScore + volumeTrendScore - riskPenalty
	return trendScore, liquidityScore, structureScore, momentumScore, persistenceScore, breakoutScore, volumeTrendScore, riskPenalty, total
}

func evaluateStrategyEnsemble(bars []marketBar, shortMA float64, longMA float64, avgVolume float64, shortReturn float64, mediumReturn float64, dataSource string, sourceErr string, portfolio portfolioConfig) (string, float64, string, string, string) {
	latest := bars[len(bars)-1]
	recentVolumeWindow := min(3, len(bars))
	recentVolumeSum := 0.0
	for _, bar := range bars[len(bars)-recentVolumeWindow:] {
		recentVolumeSum += bar.Volume
	}
	recentAvgVolume := recentVolumeSum / float64(recentVolumeWindow)

	buyVotes := 0
	sellVotes := 0
	votes := make([]string, 0, 3)

	if portfolio.TrendStrategyEnabled {
		if shortMA > longMA {
			buyVotes += int(portfolio.TrendStrategyWeight * 100)
			votes = append(votes, fmt.Sprintf("trend=BUY(%.2f)", portfolio.TrendStrategyWeight))
		} else if shortMA < longMA {
			sellVotes += int(portfolio.TrendStrategyWeight * 100)
			votes = append(votes, fmt.Sprintf("trend=SELL(%.2f)", portfolio.TrendStrategyWeight))
		} else {
			votes = append(votes, fmt.Sprintf("trend=HOLD(%.2f)", portfolio.TrendStrategyWeight))
		}
	} else {
		votes = append(votes, "trend=OFF")
	}

	lookback := min(20, len(bars)-1)
	if lookback > 0 {
		prevHigh := bars[len(bars)-1-lookback].Close
		prevLow := bars[len(bars)-1-lookback].Close
		for _, bar := range bars[len(bars)-1-lookback : len(bars)-1] {
			if bar.Close > prevHigh {
				prevHigh = bar.Close
			}
			if bar.Close < prevLow {
				prevLow = bar.Close
			}
		}
		if !portfolio.BreakoutEnabled {
			votes = append(votes, "breakout=OFF")
		} else if latest.Close >= prevHigh*0.995 && recentAvgVolume > avgVolume*1.05 {
			buyVotes += int(portfolio.BreakoutStrategyWeight * 100)
			votes = append(votes, fmt.Sprintf("breakout=BUY(%.2f)", portfolio.BreakoutStrategyWeight))
		} else if latest.Close <= prevLow*1.01 && recentAvgVolume > avgVolume*1.05 {
			sellVotes += int(portfolio.BreakoutStrategyWeight * 100)
			votes = append(votes, fmt.Sprintf("breakout=SELL(%.2f)", portfolio.BreakoutStrategyWeight))
		} else {
			votes = append(votes, fmt.Sprintf("breakout=HOLD(%.2f)", portfolio.BreakoutStrategyWeight))
		}
	}

	if !portfolio.PullbackEnabled {
		votes = append(votes, "pullback=OFF")
	} else if shortMA > longMA && mediumReturn > 0.03 && latest.Close >= shortMA*0.99 && latest.Close <= shortMA*1.05 {
		buyVotes += int(portfolio.PullbackStrategyWeight * 100)
		votes = append(votes, fmt.Sprintf("pullback=BUY(%.2f)", portfolio.PullbackStrategyWeight))
	} else if latest.Close < longMA*0.98 && shortReturn < -0.03 {
		sellVotes += int(portfolio.PullbackStrategyWeight * 100)
		votes = append(votes, fmt.Sprintf("pullback=SELL(%.2f)", portfolio.PullbackStrategyWeight))
	} else {
		votes = append(votes, fmt.Sprintf("pullback=HOLD(%.2f)", portfolio.PullbackStrategyWeight))
	}

	action := "HOLD"
	reason := buildReasonWithSource("multi-strategy votes are neutral", withSourceSuffix(dataSource, sourceErr))
	trigger := fmt.Sprintf("Watch %.2f for a confirmed multi-strategy breakout", shortMA)
	if buyVotes > sellVotes && buyVotes > 0 {
		action = "BUY"
		reason = buildReasonWithSource("multi-strategy alignment turned positive", withSourceSuffix(dataSource, sourceErr))
		trigger = fmt.Sprintf("Hold above %.2f with follow-through volume", shortMA)
	} else if sellVotes > buyVotes && sellVotes > 0 {
		action = "SELL"
		reason = buildReasonWithSource("multi-strategy alignment turned negative", withSourceSuffix(dataSource, sourceErr))
		trigger = fmt.Sprintf("Only reassess if price retakes %.2f", longMA)
	}

	alignment := float64(buyVotes-sellVotes) / 100.0 * 0.04
	return action, alignment, strings.Join(votes, " | "), reason, trigger
}

func bucketPriority(bucket string) int {
	switch bucket {
	case "建议关注":
		return 0
	case "观望":
		return 1
	case "回避":
		return 2
	default:
		return 3
	}
}

func classifyCandidate(name string, closePrice float64, shortMA float64, longMA float64, avgVolume float64, action string, score float64, reason string, trigger string) (string, string, string, float64, []string) {
	if isSTName(name) {
		return "回避", "ST or *ST stock excluded from attention list", "Only reassess after ST risk is removed", longMA, []string{"ST风险"}
	}
	if avgVolume < 1_000_000 {
		return "回避", fmt.Sprintf("average volume %.0f is too low for this scan", avgVolume), "Only reassess after liquidity improves", shortMA, []string{"低流动性"}
	}

	stopLine := longMA * 0.97
	if closePrice < stopLine {
		return "回避", fmt.Sprintf("price %.2f is below the stop line %.2f", closePrice, stopLine), fmt.Sprintf("Only reassess if price reclaims %.2f", longMA), longMA, []string{"跌破止损线"}
	}

	if action == "BUY" && score > 0.01 && closePrice > shortMA && shortMA > longMA {
		return "建议关注", reason, trigger, shortMA, nil
	}

	if action == "HOLD" || (action == "BUY" && score > 0) {
		return "观望", reason, fmt.Sprintf("Watch for a clean move above %.2f and volume above %.0f", shortMA, avgVolume), shortMA, nil
	}

	return "回避", reason, trigger, longMA, []string{"趋势偏弱"}
}

func isSTName(name string) bool {
	upper := strings.ToUpper(strings.TrimSpace(name))
	return strings.HasPrefix(upper, "ST") || strings.HasPrefix(upper, "*ST")
}

func planForBucket(bucket string, action string, shortMA float64, longMA float64, avgVolume float64) string {
	switch bucket {
	case "建议关注":
		return fmt.Sprintf("可纳入明日优先观察名单，若继续站稳 %.2f 上方并维持放量，可考虑分批跟踪", shortMA)
	case "观望":
		return fmt.Sprintf("暂不追入，等待价格有效站上 %.2f 且成交量高于 %.0f 后再确认", shortMA, avgVolume)
	case "回避":
		if action == "SELL" {
			return fmt.Sprintf("当前偏弱，优先回避，至少等重新站回 %.2f 上方再评估", longMA)
		}
		return "当前不纳入候选池，先回避，等待流动性或风险条件改善"
	default:
		return "无明确计划"
	}
}

func writeAShareScanReports(candidates []scanCandidate) error {
	textPath := filepath.Join(reportsDir, "a_share_scan.txt")
	htmlPath := filepath.Join(reportsDir, "a_share_scan.html")
	focusTextPath := filepath.Join(reportsDir, "a_share_focus.txt")
	focusHTMLPath := filepath.Join(reportsDir, "a_share_focus.html")
	watch := filterCandidatesByBucket(candidates, "建议关注")
	observe := filterCandidatesByBucket(candidates, "观望")
	avoid := filterCandidatesByBucket(candidates, "回避")

	var textBuilder strings.Builder
	textBuilder.WriteString("A-Share Scan Report\n\n")
	writeBucketText(&textBuilder, "建议关注", watch)
	writeBucketText(&textBuilder, "观望", observe)
	writeBucketText(&textBuilder, "回避", avoid)

	watchRows := buildBucketRows(watch)
	observeRows := buildBucketRows(observe)
	avoidRows := buildBucketRows(avoid)

	htmlContent := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>A-Share Scan</title>
  <style>
    body { margin: 0; font-family: Georgia, "Times New Roman", serif; background: #f3efe8; color: #1f1b16; }
    .wrap { max-width: 1100px; margin: 36px auto; padding: 0 20px; }
    .card { background: #fffaf3; border: 1px solid #d9cfbf; border-radius: 18px; padding: 24px; box-shadow: 0 18px 40px rgba(70, 50, 20, 0.08); }
    h1 { margin: 0 0 16px; font-size: 36px; }
    h2 { margin: 28px 0 12px; font-size: 24px; }
    table { width: 100%%; border-collapse: collapse; font-size: 15px; }
    th, td { text-align: left; padding: 12px 10px; border-bottom: 1px solid #e7dece; vertical-align: top; }
    th { font-size: 12px; text-transform: uppercase; letter-spacing: 0.06em; color: #6d6559; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <h1>A-Share Scan Report</h1>
      <h2>建议关注</h2>
      <table>
        <thead>
          <tr><th>#</th><th>Symbol</th><th>Name</th><th>Action</th><th>Score</th><th>Avg Volume</th><th>Short MA</th><th>Long MA</th><th>Close</th><th>Reason / Trigger</th><th>Backtest</th><th>Plan</th></tr>
        </thead>
        <tbody>%s</tbody>
      </table>
      <h2>观望</h2>
      <table>
        <thead>
          <tr><th>#</th><th>Symbol</th><th>Name</th><th>Action</th><th>Score</th><th>Avg Volume</th><th>Short MA</th><th>Long MA</th><th>Close</th><th>Reason / Trigger</th><th>Backtest</th><th>Plan</th></tr>
        </thead>
        <tbody>%s</tbody>
      </table>
      <h2>回避</h2>
      <table>
        <thead>
          <tr><th>#</th><th>Symbol</th><th>Name</th><th>Action</th><th>Score</th><th>Avg Volume</th><th>Short MA</th><th>Long MA</th><th>Close</th><th>Reason / Trigger</th><th>Backtest</th><th>Plan</th></tr>
        </thead>
        <tbody>%s</tbody>
      </table>
    </div>
  </div>
</body>
</html>`, watchRows, observeRows, avoidRows)

	if err := os.WriteFile(textPath, []byte(textBuilder.String()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0o644); err != nil {
		return err
	}
	focusText := buildFocusText(watch)
	focusHTML := buildFocusHTML(watch)
	if err := os.WriteFile(focusTextPath, []byte(focusText), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(focusHTMLPath, []byte(focusHTML), 0o644); err != nil {
		return err
	}
	return writeDashboardReports()
}

func buildFocusText(candidates []scanCandidate) string {
	var builder strings.Builder
	builder.WriteString("A-Share Focus List\n\n")
	if len(candidates) == 0 {
		builder.WriteString("今日无建议关注标的。\n")
		return builder.String()
	}

	for i, candidate := range candidates {
		fmt.Fprintf(&builder, "%d. %s %s\n", i+1, candidate.Symbol, candidate.Name)
		fmt.Fprintf(&builder, "   Market date: %s\n", candidate.MarketDate)
		fmt.Fprintf(&builder, "   Score: %.4f\n", candidate.Score)
		fmt.Fprintf(&builder, "   Close: %.2f\n", candidate.ClosePrice)
		if candidate.HasBacktest {
			fmt.Fprintf(&builder, "   Backtest: Return %.2f%% | Annualized %.2f%% | Benchmark %.2f%% | Excess %.2f%%\n",
				candidate.BacktestReturn*100,
				candidate.BacktestAnnualized*100,
				candidate.BacktestBenchmark*100,
				candidate.BacktestExcess*100,
			)
		}
		fmt.Fprintf(&builder, "   Reason: %s\n", candidate.Reason)
		fmt.Fprintf(&builder, "   Trigger: %s\n", candidate.Trigger)
		fmt.Fprintf(&builder, "   Plan: %s\n\n", candidate.Plan)
	}
	return builder.String()
}

func buildFocusHTML(candidates []scanCandidate) string {
	rows := buildBucketRows(candidates)
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>A-Share Focus</title>
  <style>
    body { margin: 0; font-family: Georgia, "Times New Roman", serif; background: #f4efe6; color: #1f1b16; }
    .wrap { max-width: 1100px; margin: 36px auto; padding: 0 20px; }
    .card { background: #fffaf3; border: 1px solid #d9cfbf; border-radius: 18px; padding: 24px; box-shadow: 0 18px 40px rgba(70, 50, 20, 0.08); }
    h1 { margin: 0 0 16px; font-size: 36px; }
    table { width: 100%%; border-collapse: collapse; font-size: 15px; }
    th, td { text-align: left; padding: 12px 10px; border-bottom: 1px solid #e7dece; vertical-align: top; }
    th { font-size: 12px; text-transform: uppercase; letter-spacing: 0.06em; color: #6d6559; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
	      <h1>A-Share Focus List</h1>
	      <table>
	        <thead>
	          <tr><th>#</th><th>Symbol</th><th>Name</th><th>Action</th><th>Score</th><th>Avg Volume</th><th>Short MA</th><th>Long MA</th><th>Close</th><th>Reason / Trigger</th><th>Backtest</th><th>Plan</th></tr>
	        </thead>
	        <tbody>%s</tbody>
	      </table>
    </div>
  </div>
</body>
</html>`, rows)
}

func filterCandidatesByBucket(candidates []scanCandidate, bucket string) []scanCandidate {
	filtered := make([]scanCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Bucket == bucket {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func writeBucketText(builder *strings.Builder, title string, candidates []scanCandidate) {
	builder.WriteString(title + "\n")
	if len(candidates) == 0 {
		builder.WriteString("  无\n\n")
		return
	}

	for i, candidate := range candidates {
		fmt.Fprintf(builder, "%d. %s %s\n", i+1, candidate.Symbol, candidate.Name)
		if candidate.Industry != "" {
			fmt.Fprintf(builder, "   Industry: %s\n", candidate.Industry)
		}
		fmt.Fprintf(builder, "   Action: %s\n", candidate.Action)
		fmt.Fprintf(builder, "   Market date: %s\n", candidate.MarketDate)
		fmt.Fprintf(builder, "   Score: %.4f\n", candidate.Score)
		fmt.Fprintf(builder, "   Score Breakdown: quality %.4f | risk %.4f | heat_penalty %.4f | reversal %.4f | trend %.4f | liquidity %.4f | structure %.4f | momentum %.4f | persistence %.4f | breakout %.4f | volume_trend %.4f | rotation %.4f | strategy %.4f | model %.4f | risk_penalty %.4f\n",
			candidate.QualityScore,
			candidate.RiskScore,
			candidate.HeatPenalty,
			candidate.ReversalScore,
			candidate.TrendScore,
			candidate.LiquidityScore,
			candidate.StructureScore,
			candidate.MomentumScore,
			candidate.PersistenceScore,
			candidate.BreakoutScore,
			candidate.VolumeTrendScore,
			candidate.RotationScore,
			candidate.StrategyAlignment,
			candidate.ModelScore,
			candidate.RiskPenalty,
		)
		if candidate.StrategyVotes != "" {
			fmt.Fprintf(builder, "   Strategy Votes: %s\n", candidate.StrategyVotes)
		}
		fmt.Fprintf(builder, "   Avg Volume: %.0f\n", candidate.AvgVolume)
		fmt.Fprintf(builder, "   Short/Long MA: %.2f / %.2f\n", candidate.ShortMA, candidate.LongMA)
		fmt.Fprintf(builder, "   Close: %.2f\n", candidate.ClosePrice)
		fmt.Fprintf(builder, "   Reason: %s\n", candidate.Reason)
		fmt.Fprintf(builder, "   Trigger: %s (%.2f)\n", candidate.Trigger, candidate.TriggerPrice)
		if candidate.InPortfolio {
			fmt.Fprintf(builder, "   Portfolio: 当前已纳入组合持仓\n")
		}
		if candidate.HasBacktest {
			fmt.Fprintf(builder, "   Backtest: %s -> %s | Return %.2f%% | Annualized %.2f%% | Benchmark %.2f%% | Excess %.2f%% | Max DD %.2f%% | Win rate %.2f%% | Trades %d\n",
				candidate.BacktestFrom,
				candidate.BacktestTo,
				candidate.BacktestReturn*100,
				candidate.BacktestAnnualized*100,
				candidate.BacktestBenchmark*100,
				candidate.BacktestExcess*100,
				candidate.BacktestDrawdown*100,
				candidate.BacktestWinRate*100,
				candidate.BacktestTrades,
			)
		}
		if len(candidate.AvoidTags) > 0 {
			fmt.Fprintf(builder, "   Avoid Tags: %s\n", strings.Join(candidate.AvoidTags, ", "))
		}
		fmt.Fprintf(builder, "   Plan: %s\n\n", candidate.Plan)
	}
}

func buildBucketRows(candidates []scanCandidate) string {
	if len(candidates) == 0 {
		return `<tr><td colspan="12">No candidates</td></tr>`
	}

	var rows strings.Builder
	for i, candidate := range candidates {
		industry := candidate.Industry
		if industry == "" {
			industry = "-"
		}
		avoidTags := ""
		if len(candidate.AvoidTags) > 0 {
			avoidTags = "<br><small>Tags: " + html.EscapeString(strings.Join(candidate.AvoidTags, ", ")) + "</small>"
		}
		portfolioTag := ""
		if candidate.InPortfolio {
			portfolioTag = "<br><small>Portfolio: in current portfolio</small>"
		}
		backtestCell := `<small>No backtest snapshot</small>`
		if candidate.HasBacktest {
			backtestCell = fmt.Sprintf(`<small>%s to %s</small><br>Return %.2f%%<br>Annualized %.2f%%<br>Benchmark %.2f%%<br>Excess %.2f%%<br>Max DD %.2f%%<br>Win rate %.2f%%<br>Trades %d`,
				html.EscapeString(candidate.BacktestFrom),
				html.EscapeString(candidate.BacktestTo),
				candidate.BacktestReturn*100,
				candidate.BacktestAnnualized*100,
				candidate.BacktestBenchmark*100,
				candidate.BacktestExcess*100,
				candidate.BacktestDrawdown*100,
				candidate.BacktestWinRate*100,
				candidate.BacktestTrades,
			)
		}
		fmt.Fprintf(&rows, `<tr><td>%d</td><td>%s</td><td>%s<br><small>%s</small>%s</td><td>%s</td><td>%.4f<br><small>T %.4f | L %.4f | S %.4f | M %.4f | P %.4f | B %.4f | V %.4f | Rot %.4f | Strat %.4f | Model %.4f | R %.4f</small></td><td>%.0f</td><td>%.2f</td><td>%.2f</td><td>%.2f</td><td>%s<br><small>%s @ %.2f</small><br><small>%s</small>%s</td><td>%s</td><td>%s</td></tr>`,
			i+1,
			html.EscapeString(candidate.Symbol),
			html.EscapeString(candidate.Name),
			html.EscapeString(industry),
			portfolioTag,
			html.EscapeString(candidate.Action),
			candidate.Score,
			candidate.TrendScore,
			candidate.LiquidityScore,
			candidate.StructureScore,
			candidate.MomentumScore,
			candidate.PersistenceScore,
			candidate.BreakoutScore,
			candidate.VolumeTrendScore,
			candidate.RotationScore,
			candidate.StrategyAlignment,
			candidate.ModelScore,
			candidate.RiskPenalty,
			candidate.AvgVolume,
			candidate.ShortMA,
			candidate.LongMA,
			candidate.ClosePrice,
			html.EscapeString(candidate.Reason),
			html.EscapeString(candidate.Trigger),
			candidate.TriggerPrice,
			html.EscapeString(candidate.StrategyVotes),
			avoidTags,
			backtestCell,
			html.EscapeString(candidate.Plan),
		)
	}
	return rows.String()
}

func isAShareSymbol(symbol string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(symbol))
	if strings.Contains(normalized, ".") {
		parts := strings.SplitN(normalized, ".", 2)
		if len(parts) == 2 && isDigits(parts[0]) && len(parts[0]) == 6 {
			switch parts[1] {
			case "SH", "SZ", "BJ":
				return true
			}
		}
	}

	return isDigits(normalized) && len(normalized) == 6
}

func normalizeAShareSymbol(symbol string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(symbol))
	if strings.Contains(normalized, ".") {
		parts := strings.SplitN(normalized, ".", 2)
		if len(parts) != 2 || !isDigits(parts[0]) || len(parts[0]) != 6 {
			return "", fmt.Errorf("invalid A-share symbol %q", symbol)
		}
		switch parts[1] {
		case "SH", "SZ", "BJ":
			return parts[0] + "." + parts[1], nil
		default:
			return "", fmt.Errorf("unsupported A-share exchange suffix %q", parts[1])
		}
	}

	if !isDigits(normalized) || len(normalized) != 6 {
		return "", fmt.Errorf("invalid A-share symbol %q", symbol)
	}

	switch {
	case strings.HasPrefix(normalized, "600"),
		strings.HasPrefix(normalized, "601"),
		strings.HasPrefix(normalized, "603"),
		strings.HasPrefix(normalized, "605"),
		strings.HasPrefix(normalized, "510"),
		strings.HasPrefix(normalized, "511"),
		strings.HasPrefix(normalized, "512"),
		strings.HasPrefix(normalized, "513"),
		strings.HasPrefix(normalized, "515"),
		strings.HasPrefix(normalized, "516"),
		strings.HasPrefix(normalized, "518"),
		strings.HasPrefix(normalized, "588"),
		strings.HasPrefix(normalized, "688"),
		strings.HasPrefix(normalized, "689"):
		return normalized + ".SH", nil
	case strings.HasPrefix(normalized, "000"),
		strings.HasPrefix(normalized, "001"),
		strings.HasPrefix(normalized, "002"),
		strings.HasPrefix(normalized, "003"),
		strings.HasPrefix(normalized, "159"),
		strings.HasPrefix(normalized, "300"),
		strings.HasPrefix(normalized, "301"):
		return normalized + ".SZ", nil
	case strings.HasPrefix(normalized, "430"),
		strings.HasPrefix(normalized, "440"),
		strings.HasPrefix(normalized, "830"),
		strings.HasPrefix(normalized, "831"),
		strings.HasPrefix(normalized, "832"),
		strings.HasPrefix(normalized, "833"),
		strings.HasPrefix(normalized, "834"),
		strings.HasPrefix(normalized, "835"),
		strings.HasPrefix(normalized, "836"),
		strings.HasPrefix(normalized, "837"),
		strings.HasPrefix(normalized, "838"),
		strings.HasPrefix(normalized, "839"),
		strings.HasPrefix(normalized, "870"),
		strings.HasPrefix(normalized, "871"),
		strings.HasPrefix(normalized, "872"),
		strings.HasPrefix(normalized, "873"),
		strings.HasPrefix(normalized, "920"):
		return normalized + ".BJ", nil
	default:
		return "", fmt.Errorf("cannot infer A-share exchange for %q", symbol)
	}
}

func eastmoneySecID(symbol string) (string, error) {
	normalized, err := normalizeAShareSymbol(symbol)
	if err != nil {
		return "", err
	}
	parts := strings.SplitN(normalized, ".", 2)

	switch parts[1] {
	case "SH":
		return "1." + parts[0], nil
	case "SZ":
		return "0." + parts[0], nil
	case "BJ":
		return "0." + parts[0], nil
	default:
		return "", fmt.Errorf("unsupported exchange suffix %q", parts[1])
	}
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func tushareTSCode(symbol string) (string, error) {
	normalized, err := normalizeAShareSymbol(symbol)
	if err != nil {
		return "", err
	}
	parts := strings.SplitN(normalized, ".", 2)
	return parts[0] + "." + parts[1], nil
}

func formatTradeDate(value string) string {
	if len(value) == 8 {
		return value[:4] + "-" + value[4:6] + "-" + value[6:]
	}
	return value
}

func toFloat(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("unexpected numeric type %T", value)
	}
}

func extractBars(rows [][]string) ([]marketBar, error) {
	if len(rows) < 2 {
		return nil, errors.New("market data must include header and at least one row")
	}

	header := make(map[string]int)
	for i, col := range rows[0] {
		header[strings.ToLower(strings.TrimSpace(col))] = i
	}

	required := []string{"timestamp", "open", "high", "low", "close", "volume"}
	if _, ok := header["date"]; ok {
		header["timestamp"] = header["date"]
	}
	for _, col := range required {
		if _, ok := header[col]; !ok {
			return nil, fmt.Errorf("market data is missing %s column", col)
		}
	}

	bars := make([]marketBar, 0, len(rows)-1)
	for rowNumber := 1; rowNumber < len(rows); rowNumber++ {
		row := rows[rowNumber]
		bar, err := parseBar(row, header, rowNumber+1)
		if err != nil {
			return nil, err
		}
		bars = append(bars, bar)
	}

	if len(bars) >= 2 && bars[0].Date > bars[len(bars)-1].Date {
		for left, right := 0, len(bars)-1; left < right; left, right = left+1, right-1 {
			bars[left], bars[right] = bars[right], bars[left]
		}
	}

	return bars, nil
}

func parseBar(row []string, header map[string]int, rowNumber int) (marketBar, error) {
	get := func(name string) (string, error) {
		index := header[name]
		if index >= len(row) {
			return "", fmt.Errorf("row %d missing %s value", rowNumber, name)
		}
		return strings.TrimSpace(row[index]), nil
	}

	parseFloat := func(name string) (float64, error) {
		value, err := get(name)
		if err != nil {
			return 0, err
		}
		number, convErr := strconv.ParseFloat(value, 64)
		if convErr != nil {
			return 0, fmt.Errorf("row %d invalid %s value: %w", rowNumber, name, convErr)
		}
		return number, nil
	}

	date, err := get("timestamp")
	if err != nil {
		return marketBar{}, err
	}
	open, err := parseFloat("open")
	if err != nil {
		return marketBar{}, err
	}
	high, err := parseFloat("high")
	if err != nil {
		return marketBar{}, err
	}
	low, err := parseFloat("low")
	if err != nil {
		return marketBar{}, err
	}
	closePrice, err := parseFloat("close")
	if err != nil {
		return marketBar{}, err
	}
	volume, err := parseFloat("volume")
	if err != nil {
		return marketBar{}, err
	}

	return marketBar{
		Date:   date,
		Open:   open,
		High:   high,
		Low:    low,
		Close:  closePrice,
		Volume: volume,
	}, nil
}

func loadPositionState(dbPath string, symbol string) (positionState, error) {
	query := fmt.Sprintf(
		"SELECT side, quantity, entry_price FROM position_state WHERE symbol = %s LIMIT 1;",
		quoteSQL(symbol),
	)
	output, err := runSQLiteQuery(dbPath, query)
	if err != nil {
		return positionState{}, err
	}
	if strings.TrimSpace(output) == "" {
		return positionState{Side: "FLAT", Quantity: 0, EntryPrice: 0}, nil
	}

	parts := strings.Split(strings.TrimSpace(output), "|")
	if len(parts) != 3 {
		return positionState{}, fmt.Errorf("unexpected position_state result: %s", output)
	}

	quantity, err := strconv.Atoi(parts[1])
	if err != nil {
		return positionState{}, err
	}
	entryPrice, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return positionState{}, err
	}

	return positionState{
		Side:       parts[0],
		Quantity:   quantity,
		EntryPrice: entryPrice,
	}, nil
}

func loadLastSignal(dbPath string, symbol string) (string, error) {
	query := fmt.Sprintf(
		"SELECT signal FROM signal_records WHERE symbol = %s ORDER BY id DESC LIMIT 1;",
		quoteSQL(symbol),
	)
	output, err := runSQLiteQuery(dbPath, query)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

func runSQLiteQuery(path string, sql string) (string, error) {
	cmd := exec.Command("sqlite3", "-noheader", path, sql)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func execSQLite(path string, sql string) error {
	cmd := exec.Command("sqlite3", path, sql)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func quoteSQL(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func average(values []float64) float64 {
	var sum float64
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func nextRunTime(expr string, now time.Time) (time.Time, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return time.Time{}, fmt.Errorf("unsupported cron expression %q", expr)
	}

	minute, err := strconv.Atoi(fields[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid minute: %w", err)
	}
	hour, err := strconv.Atoi(fields[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid hour: %w", err)
	}

	if fields[2] != "*" || fields[3] != "*" {
		return time.Time{}, fmt.Errorf("only day/month '*' is supported: %q", expr)
	}

	weekdays, err := parseWeekdays(fields[4])
	if err != nil {
		return time.Time{}, err
	}

	for dayOffset := 0; dayOffset <= 14; dayOffset++ {
		candidateDate := now.AddDate(0, 0, dayOffset)
		candidate := time.Date(
			candidateDate.Year(),
			candidateDate.Month(),
			candidateDate.Day(),
			hour,
			minute,
			0,
			0,
			now.Location(),
		)

		if !weekdays[candidate.Weekday()] {
			continue
		}
		if !candidate.After(now) {
			continue
		}

		return candidate, nil
	}

	return time.Time{}, fmt.Errorf("could not calculate next run for %q", expr)
}

func parseWeekdays(field string) (map[time.Weekday]bool, error) {
	parts := strings.Split(field, ",")
	weekdays := make(map[time.Weekday]bool)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			if len(bounds) != 2 {
				return nil, fmt.Errorf("invalid weekday range %q", part)
			}
			start, err := strconv.Atoi(bounds[0])
			if err != nil {
				return nil, fmt.Errorf("invalid weekday %q", bounds[0])
			}
			end, err := strconv.Atoi(bounds[1])
			if err != nil {
				return nil, fmt.Errorf("invalid weekday %q", bounds[1])
			}
			if start > end {
				return nil, fmt.Errorf("invalid weekday range %q", part)
			}
			for day := start; day <= end; day++ {
				weekday, err := cronDayToWeekday(day)
				if err != nil {
					return nil, err
				}
				weekdays[weekday] = true
			}
			continue
		}

		day, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid weekday %q", part)
		}
		weekday, err := cronDayToWeekday(day)
		if err != nil {
			return nil, err
		}
		weekdays[weekday] = true
	}

	return weekdays, nil
}

func cronDayToWeekday(day int) (time.Weekday, error) {
	switch day {
	case 0, 7:
		return time.Sunday, nil
	case 1:
		return time.Monday, nil
	case 2:
		return time.Tuesday, nil
	case 3:
		return time.Wednesday, nil
	case 4:
		return time.Thursday, nil
	case 5:
		return time.Friday, nil
	case 6:
		return time.Saturday, nil
	default:
		return time.Sunday, fmt.Errorf("weekday out of range: %d", day)
	}
}
