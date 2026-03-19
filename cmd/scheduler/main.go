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
const fundamentalsPath = "data/fundamentals.csv"
const eventsPath = "data/events.csv"

var cachedLinearModel *linearModel
var linearModelLoaded bool
var cachedBenchmarkModel *linearModel
var benchmarkModelLoaded bool
var cachedFundamentals map[string]fundamentalSnapshot
var fundamentalsLoaded bool
var cachedEvents map[string]eventSnapshot
var eventsLoaded bool
var runtimeConfig config
var diagnosticsState = runtimeDiagnostics{
	ProviderFailures: map[string]int{},
	FallbackReasons:  []string{},
}

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
	Model     modelConfig
	Report    reportConfig
	Market    marketRuleConfig
}

type dbConfig struct {
	Path string
}

type scheduleConfig struct {
	DailyRun string
	CacheTTL string
}

type modelConfig struct {
	DefaultLabel          string
	ModelPath             string
	BenchmarkModelPath    string
	PromotionMetric       string
	MinPromotionEdge      float64
	MinShadowObservations int
	ShadowVersion         string
}

type reportConfig struct {
	HistoryRoot      string
	ExportJSON       bool
	CleanupKeepDays  int
	ExperimentLedger string
	RunIndexPath     string
}

type marketRuleConfig struct {
	AShareT1               bool
	MainBoardLimit         float64
	ChiNextLimit           float64
	STARLimit              float64
	RiskWarningLimit       float64
	StampDutySellBps       float64
	TransferFeeBps         float64
	HandlingFeeBps         float64
	CommissionBps          float64
	IPOUncappedTradingDays int
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
	IndustryMaxPositions   int
	VolatilityTarget       float64
	CapacityTurnoverShare  float64
	GapOpenThreshold       float64
	ReserveCandidates      int
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
	Symbol              string
	Name                string
	Industry            string
	Action              string
	Bucket              string
	Score               float64
	QualityScore        float64
	RiskScore           float64
	HeatPenalty         float64
	ReversalScore       float64
	ValueScore          float64
	LowVolScore         float64
	CrowdingScore       float64
	FundamentalScore    float64
	ValuationScore      float64
	EventScore          float64
	TrendScore          float64
	LiquidityScore      float64
	StructureScore      float64
	MomentumScore       float64
	PersistenceScore    float64
	BreakoutScore       float64
	VolumeTrendScore    float64
	ShortReturnScore    float64
	MediumReturnScore   float64
	IndustryStrength    float64
	RotationScore       float64
	StrategyAlignment   float64
	StrategyVotes       string
	ModelScore          float64
	BenchmarkModelScore float64
	RiskPenalty         float64
	AvgVolume           float64
	Trigger             string
	TriggerPrice        float64
	AvoidTags           []string
	ShortMA             float64
	LongMA              float64
	ClosePrice          float64
	MarketDate          string
	Reason              string
	Plan                string
	HasBacktest         bool
	BacktestMode        string
	BacktestFrom        string
	BacktestTo          string
	BacktestReturn      float64
	BacktestAnnualized  float64
	BacktestBenchmark   float64
	BacktestExcess      float64
	BacktestDrawdown    float64
	BacktestWinRate     float64
	BacktestTrades      int
	InPortfolio         bool
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

type paperPosition struct {
	Symbol     string
	Name       string
	Shares     int
	EntryPrice float64
	EntryDate  string
}

type paperOrder struct {
	PlacedAt string
	Symbol   string
	Name     string
	Side     string
	Quantity int
	Price    float64
	Status   string
	Note     string
}

type paperFill struct {
	FilledAt string
	Symbol   string
	Name     string
	Side     string
	Quantity int
	Price    float64
	Fee      float64
	Note     string
}

type paperAccountResult struct {
	AccountID  int
	Version    string
	Market     string
	Mode       string
	Session    string
	MarketDate string
	Status     string
	UpdatedAt  string
	Cash       float64
	Equity     float64
	Targets    []scanCandidate
	Holdings   []paperPosition
	Orders     []paperOrder
	Fills      []paperFill
	Note       string
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
	ValueScore        float64
	LowVolScore       float64
	CrowdingScore     float64
	FundamentalScore  float64
	ValuationScore    float64
	EventScore        float64
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
	Excess5D          float64
	Excess10D         float64
	Excess20D         float64
	BeatBenchmark5D   float64
	BeatBenchmark10D  float64
	BeatBenchmark20D  float64
}

type linearModelFeature struct {
	Feature string  `json:"feature"`
	Weight  float64 `json:"weight"`
	Mean    float64 `json:"mean"`
	Std     float64 `json:"std"`
}

type linearModel struct {
	Task           string               `json:"task"`
	Label          string               `json:"label"`
	Bias           float64              `json:"bias"`
	Features       []linearModelFeature `json:"features"`
	RollingMetrics []map[string]any     `json:"rolling_metrics"`
}

type marketSeries struct {
	meta aShareSymbol
	bars []marketBar
}

type fundamentalSnapshot struct {
	Symbol        string
	ROE           float64
	ProfitGrowth  float64
	CashflowRatio float64
	DebtRatio     float64
	PEPercentile  float64
	PBPercentile  float64
	PSPercentile  float64
	UpdatedAt     string
}

type eventSnapshot struct {
	Symbol       string
	EarningsFlag float64
	BuybackFlag  float64
	UnlockFlag   float64
	InsiderFlag  float64
	UpdatedAt    string
}

type runtimeDiagnostics struct {
	CacheHits        int
	CacheMisses      int
	CacheStaleLoads  int
	ProviderFailures map[string]int
	FallbackReasons  []string
	LastUpdated      string
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
	paperRun := flag.Bool("paper-run", false, "Run the simulated paper-trading account")
	paperShadowRun := flag.Bool("paper-shadow-run", false, "Run a shadow paper-trading account")
	gridSearch := flag.Bool("grid-search", false, "Run a portfolio parameter grid search across short/long windows")
	exportDataset := flag.Bool("export-dataset", false, "Export a training dataset with factor features and forward-return labels")
	dashboardOnly := flag.Bool("dashboard-only", false, "Rebuild dashboard and overview reports from the latest report files")
	fromDate := flag.String("from", "", "Backtest start date in YYYY-MM-DD")
	toDate := flag.String("to", "", "Backtest end date in YYYY-MM-DD")
	initialCash := flag.Float64("cash", 100000, "Backtest initial cash")
	feeBps := flag.Float64("fee-bps", 10, "Backtest transaction fee in basis points")
	slippageBps := flag.Float64("slippage-bps", 5, "Backtest slippage in basis points")
	shortMin := flag.Int("short-min", 3, "Grid search minimum short window")
	shortMax := flag.Int("short-max", 8, "Grid search maximum short window")
	longMin := flag.Int("long-min", 8, "Grid search minimum long window")
	longMax := flag.Int("long-max", 20, "Grid search maximum long window")
	paperInterval := flag.String("paper-interval", "60s", "Paper-trading polling interval")
	paperMarket := flag.String("paper-market", "a_share", "Paper-trading market")
	paperMode := flag.String("paper-mode", "live", "Paper-trading mode")
	shadowVersion := flag.String("shadow-version", "", "Shadow strategy version name")
	flag.Parse()

	cfg, err := loadConfig(configPath)
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}
	if strings.TrimSpace(*symbolOverride) != "" {
		cfg.Strategy.Symbol = strings.TrimSpace(*symbolOverride)
	}
	runtimeConfig = cfg

	if err := ensureSQLiteDB(cfg.DB.Path); err != nil {
		logger.Fatalf("ensure sqlite db: %v", err)
	}
	if err := ensureStrategyRegistrySeed(cfg); err != nil {
		logger.Fatalf("ensure strategy registry: %v", err)
	}
	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		logger.Fatalf("ensure reports dir: %v", err)
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		logger.Fatalf("ensure cache dir: %v", err)
	}
	if err := os.MkdirAll(cfg.Report.HistoryRoot, 0o755); err != nil {
		logger.Fatalf("ensure history dir: %v", err)
	}
	if err := cleanupOldArtifacts(); err != nil {
		logger.Fatalf("cleanup artifacts: %v", err)
	}
	if *dashboardOnly {
		if err := writeDashboardReports(); err != nil {
			logger.Fatalf("write dashboard reports: %v", err)
		}
		fmt.Printf("dashboard rebuilt: %s\n", filepath.Join(reportsDir, "dashboard.html"))
		return
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
	if *paperRun {
		runCycle := func() {
			activeCfg, versionName, err := resolveActiveStrategyConfig(runtimeConfig, *paperMarket)
			if err != nil {
				logger.Printf("resolve active strategy config: %v", err)
				return
			}
			result, err := runPaperTrading(activeCfg.Strategy, activeCfg.Portfolio, activeCfg.Regime, *topN, *initialCash, *feeBps, *slippageBps, *paperMarket, *paperMode, versionName)
			if err != nil {
				logger.Printf("paper trading failed: %v", err)
				return
			}
			if err := writePaperTradingReports(result); err != nil {
				logger.Printf("write paper trading reports: %v", err)
				return
			}
			printPaperTradingSummary(result)
		}
		runCycle()
		if *once {
			return
		}
		interval, err := time.ParseDuration(*paperInterval)
		if err != nil {
			logger.Fatalf("invalid paper interval: %v", err)
		}
		for {
			logger.Printf("next paper run in %s", interval)
			time.Sleep(interval)
			runCycle()
		}
	}
	if *paperShadowRun {
		versionName := strings.TrimSpace(*shadowVersion)
		if versionName == "" {
			versionName = currentStrategyVersionName(cfg) + "_shadow"
		}
		if err := ensureStrategyVersion(runtimeConfig.DB.Path, "a_share", versionName, "shadow", currentStrategyVersionName(cfg), runtimeConfig); err != nil {
			logger.Fatalf("ensure shadow strategy version: %v", err)
		}
		runCycle := func() {
			shadowCfg, err := loadStrategyVersionConfig(runtimeConfig.DB.Path, versionName, runtimeConfig)
			if err != nil {
				logger.Printf("load shadow strategy config: %v", err)
				return
			}
			result, err := runPaperTrading(shadowCfg.Strategy, shadowCfg.Portfolio, shadowCfg.Regime, *topN, *initialCash, *feeBps, *slippageBps, *paperMarket, "shadow:"+versionName, versionName)
			if err != nil {
				logger.Printf("shadow paper trading failed: %v", err)
				return
			}
			if err := writePaperTradingReports(result); err != nil {
				logger.Printf("write shadow paper trading reports: %v", err)
				return
			}
			printPaperTradingSummary(result)
		}
		runCycle()
		if *once {
			return
		}
		interval, err := time.ParseDuration(*paperInterval)
		if err != nil {
			logger.Fatalf("invalid paper interval: %v", err)
		}
		for {
			logger.Printf("next shadow paper run in %s", interval)
			time.Sleep(interval)
			runCycle()
		}
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
	selected, err := loadSelectedAShareCandidates(strategy, portfolio, runtimeConfig.Regime, topN)
	if err != nil {
		return err
	}
	if err := writeAShareScanReports(selected); err != nil {
		return err
	}

	fmt.Printf("A-share scan complete. Top %d candidates written to %s and %s\n", len(selected), filepath.Join(reportsDir, "a_share_scan.txt"), filepath.Join(reportsDir, "a_share_scan.html"))
	for i, candidate := range selected {
		fmt.Printf("%d. [%s] %s %s %s score=%.4f close=%.2f\n", i+1, candidate.Bucket, candidate.Symbol, candidate.Name, candidate.Action, candidate.Score, candidate.ClosePrice)
	}
	fmt.Println()

	return nil
}

func loadSelectedAShareCandidates(strategy strategyConfig, portfolio portfolioConfig, regime regimeConfig, topN int) ([]scanCandidate, error) {
	if topN <= 0 {
		topN = 10
	}

	symbols, err := loadAShareUniverse()
	if err != nil {
		return nil, err
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
		return nil, errors.New("no A-share candidates were generated")
	}

	regimeLabel := "cautious"
	if benchmarkBars, err := loadAShareBenchmarkBars(); err == nil && len(benchmarkBars) >= strategy.LongWindow {
		regimeLabel, _ = benchmarkMarketRegime(benchmarkBars, marketBreadth(candidates), strategy.LongWindow, regime)
	}

	sort.Slice(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if bucketPriority(left.Bucket) != bucketPriority(right.Bucket) {
			return bucketPriority(left.Bucket) < bucketPriority(right.Bucket)
		}
		return portfolioSelectionScore(left, portfolio, regimeLabel) > portfolioSelectionScore(right, portfolio, regimeLabel)
	})

	selected := selectPortfolioCandidates(candidates, topN, portfolio.MinHoldings, portfolio, regimeLabel)
	if len(selected) == 0 {
		selected = candidates
	}
	if topN > len(selected) {
		topN = len(selected)
	}
	return selected[:topN], nil
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

	feeRateBuy := effectiveFeeRate(false)
	feeRateSell := effectiveFeeRate(true)
	slippageRate := slippageBps / 10000
	cash := initialCash
	holdings := map[string]int{}
	entryPrices := map[string]float64{}
	entryDates := map[string]string{}
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

		rankedCandidates := append([]scanCandidate(nil), candidates...)
		candidates = selectPortfolioCandidates(candidates, topN, portfolio.MinHoldings, portfolio, regimeLabel)
		reserveCandidates := reservePortfolioCandidates(rankedCandidates, candidates, portfolio.ReserveCandidates)
		if len(candidates) > 0 {
			latestSelection = append([]scanCandidate(nil), candidates...)
			latestSelection = append(latestSelection, reserveCandidates...)
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
			prevBar := bar
			if prior, ok := barBySymbolDate[symbol][previousTradingDate(dates, dayIdx)]; ok {
				prevBar = prior
			}
			if bar.Close > holdingPeaks[symbol] {
				holdingPeaks[symbol] = bar.Close
			}
			if entryPrice := entryPrices[symbol]; entryPrice > 0 && bar.Close <= entryPrice*(1-risk.StopLossPct) {
				if (runtimeConfig.Market.AShareT1 && entryDates[symbol] == date) || isSellRestricted(symbol, targetSet[symbol].Name, bar) || gapOpenMove(prevBar.Close, bar) <= -portfolio.GapOpenThreshold {
					continue
				}
				execPrice := bar.Close * (1 - slippageRate)
				fee := float64(shares) * execPrice * feeRateSell
				cash += float64(shares)*execPrice - fee
				holdings[symbol] = 0
				delete(entryPrices, symbol)
				delete(entryDates, symbol)
				delete(holdingPeaks, symbol)
				cooldownUntil[symbol] = portfolioCooldownDate(date, portfolio.StopCooldownDays)
				rebalanceCount++
				continue
			}
			if peak := holdingPeaks[symbol]; peak > 0 && bar.Close <= peak*(1-portfolio.MaxHoldingDrawdown) {
				if (runtimeConfig.Market.AShareT1 && entryDates[symbol] == date) || isSellRestricted(symbol, targetSet[symbol].Name, bar) || gapOpenMove(prevBar.Close, bar) <= -portfolio.GapOpenThreshold {
					continue
				}
				execPrice := bar.Close * (1 - slippageRate)
				fee := float64(shares) * execPrice * feeRateSell
				cash += float64(shares)*execPrice - fee
				holdings[symbol] = 0
				delete(entryPrices, symbol)
				delete(entryDates, symbol)
				delete(holdingPeaks, symbol)
				cooldownUntil[symbol] = portfolioCooldownDate(date, portfolio.StopCooldownDays)
				rebalanceCount++
				continue
			}
			if candidate, keep := targetSet[symbol]; keep && candidate.Action == "SELL" {
				if (runtimeConfig.Market.AShareT1 && entryDates[symbol] == date) || isSellRestricted(symbol, candidate.Name, bar) || gapOpenMove(prevBar.Close, bar) <= -portfolio.GapOpenThreshold {
					continue
				}
				execPrice := bar.Close * (1 - slippageRate)
				fee := float64(shares) * execPrice * feeRateSell
				cash += float64(shares)*execPrice - fee
				holdings[symbol] = 0
				delete(entryPrices, symbol)
				delete(entryDates, symbol)
				delete(holdingPeaks, symbol)
				cooldownUntil[symbol] = portfolioCooldownDate(date, portfolio.TrendBreakCooldownDays)
				rebalanceCount++
				continue
			}
			if _, keep := targetSet[symbol]; keep {
				continue
			}
			if (runtimeConfig.Market.AShareT1 && entryDates[symbol] == date) || isSellRestricted(symbol, "", bar) || gapOpenMove(prevBar.Close, bar) <= -portfolio.GapOpenThreshold {
				continue
			}
			execPrice := bar.Close * (1 - slippageRate)
			fee := float64(shares) * execPrice * feeRateSell
			cash += float64(shares)*execPrice - fee
			holdings[symbol] = 0
			delete(entryPrices, symbol)
			delete(entryDates, symbol)
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
				prevBar := bar
				if prior, ok := barBySymbolDate[candidate.Symbol][previousTradingDate(dates, dayIdx)]; ok {
					prevBar = prior
				}
				currentShares := holdings[candidate.Symbol]
				currentValue := float64(currentShares) * bar.Close
				targetSlotValue := slotValue
				history := barsUpToDate(seriesBarsForSymbol(series, candidate.Symbol), date)
				nameVol := recentVolatility(history, min(10, len(history)-1))
				if nameVol > 0 && portfolio.VolatilityTarget > 0 {
					targetSlotValue *= clampFloat(portfolio.VolatilityTarget/nameVol, 0.45, 1.15)
				}
				targetShares := int(targetSlotValue / (bar.Close * (1 + feeRateBuy + slippageRate)))
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
					if (runtimeConfig.Market.AShareT1 && entryDates[candidate.Symbol] == date) || isSellRestricted(candidate.Symbol, candidate.Name, bar) || gapOpenMove(prevBar.Close, bar) <= -portfolio.GapOpenThreshold {
						continue
					}
					capacity := capacityLimitedShares(bar, portfolio.CapacityTurnoverShare)
					if capacity > 0 && sellShares > capacity {
						sellShares = capacity
					}
					if sellShares <= 0 {
						continue
					}
					execPrice := bar.Close * (1 - slippageRate)
					fee := float64(sellShares) * execPrice * feeRateSell
					cash += float64(sellShares)*execPrice - fee
					holdings[candidate.Symbol] = currentShares - sellShares
					if holdings[candidate.Symbol] <= 0 {
						delete(entryPrices, candidate.Symbol)
						delete(entryDates, candidate.Symbol)
						delete(holdingPeaks, candidate.Symbol)
					}
					rebalanceCount++
					continue
				}

				execPrice := bar.Close * (1 + slippageRate)
				if isBuyRestricted(candidate.Symbol, candidate.Name, bar) || gapOpenMove(prevBar.Close, bar) >= portfolio.GapOpenThreshold {
					continue
				}
				cost := float64(diff) * execPrice
				fee := cost * feeRateBuy
				capacity := capacityLimitedShares(bar, portfolio.CapacityTurnoverShare)
				if capacity > 0 && diff > capacity {
					diff = capacity
					if diff <= 0 {
						continue
					}
					cost = float64(diff) * execPrice
					fee = cost * feeRateBuy
				}
				if cost+fee > cash {
					maxAffordable := int(cash / (execPrice * (1 + feeRateBuy)))
					diff = maxAffordable
					if diff <= 0 {
						continue
					}
					cost = float64(diff) * execPrice
					fee = cost * feeRateBuy
				}
				cash -= cost + fee
				holdings[candidate.Symbol] = currentShares + diff
				if currentShares == 0 {
					entryPrices[candidate.Symbol] = execPrice
					entryDates[candidate.Symbol] = date
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
		benchmarkCurve = buildBenchmarkCurve(aShareBenchmarkSymbol, "CSI300", benchmarkBars, fromDate, toDate, initialCash, feeRateBuy, feeRateSell, slippageRate)
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

func runPaperTrading(strategy strategyConfig, portfolio portfolioConfig, regime regimeConfig, topN int, initialCash float64, feeBps float64, slippageBps float64, market string, mode string, strategyVersion string) (paperAccountResult, error) {
	if market != "a_share" {
		return paperAccountResult{}, fmt.Errorf("unsupported paper market: %s", market)
	}

	selected, err := loadSelectedAShareCandidates(strategy, portfolio, runtimeConfig.Regime, topN)
	if err != nil {
		return paperAccountResult{}, err
	}
	if err := writeAShareScanReports(selected); err != nil {
		return paperAccountResult{}, err
	}

	targets := make([]scanCandidate, 0, len(selected))
	for _, candidate := range selected {
		if candidate.Bucket == "建议关注" && candidate.Action == "BUY" {
			targets = append(targets, candidate)
		}
	}
	if len(targets) > topN {
		targets = targets[:topN]
	}

	now := time.Now()
	session := paperSessionForMarket(market, now)
	accountID, cash, lastMarketDate, status, _, _, err := ensurePaperAccount(runtimeConfig.DB.Path, market, mode, initialCash, strategyVersion)
	if err != nil {
		return paperAccountResult{}, err
	}
	positions, err := loadPaperPositions(runtimeConfig.DB.Path, accountID)
	if err != nil {
		return paperAccountResult{}, err
	}

	symbolMeta := make(map[string]scanCandidate, len(targets))
	for _, candidate := range targets {
		symbolMeta[candidate.Symbol] = candidate
	}
	latestBars := make(map[string]marketBar, len(targets)+len(positions))
	for _, candidate := range selected {
		if _, ok := latestBars[candidate.Symbol]; !ok {
			latestBars[candidate.Symbol] = marketBar{
				Date:   candidate.MarketDate,
				Open:   candidate.ClosePrice,
				High:   candidate.ClosePrice,
				Low:    candidate.ClosePrice,
				Close:  candidate.ClosePrice,
				Volume: candidate.AvgVolume,
			}
		}
	}
	for _, position := range positions {
		if _, ok := latestBars[position.Symbol]; ok {
			continue
		}
		bars, _, _, err := loadSymbolBars(position.Symbol, "auto", "", "ALPHAVANTAGE_API_KEY", false)
		if err != nil || len(bars) == 0 {
			continue
		}
		latestBars[position.Symbol] = bars[len(bars)-1]
	}

	marketDate := ""
	for _, bar := range latestBars {
		if bar.Date > marketDate {
			marketDate = bar.Date
		}
	}
	if marketDate == "" {
		marketDate = now.Format("2006-01-02")
	}

	orders := make([]paperOrder, 0)
	fills := make([]paperFill, 0)
	noteParts := make([]string, 0, 3)
	feeRateBuy := effectiveFeeRate(false)
	feeRateSell := effectiveFeeRate(true)
	slippageRate := slippageBps / 10000
	positionMap := make(map[string]paperPosition, len(positions))
	for _, position := range positions {
		positionMap[position.Symbol] = position
	}

	if lastMarketDate == marketDate {
		noteParts = append(noteParts, "latest market date already processed; snapshot refreshed only")
	} else {
		targetSet := make(map[string]scanCandidate, len(targets))
		for _, candidate := range targets {
			targetSet[candidate.Symbol] = candidate
		}

		for symbol, position := range positionMap {
			bar, ok := latestBars[symbol]
			if !ok {
				continue
			}
			candidate, keep := targetSet[symbol]
			if keep {
				continue
			}
			order := paperOrder{
				PlacedAt: now.Format(time.RFC3339),
				Symbol:   symbol,
				Name:     position.Name,
				Side:     "SELL",
				Quantity: position.Shares,
				Price:    bar.Close,
				Status:   "skipped",
				Note:     "not in current target list",
			}
			if runtimeConfig.Market.AShareT1 && position.EntryDate == marketDate {
				order.Note = "blocked by T+1 rule"
				orders = append(orders, order)
				continue
			}
			if isSellRestricted(symbol, candidate.Name, bar) {
				order.Note = "sell restricted by market rule"
				orders = append(orders, order)
				continue
			}
			execPrice := bar.Close * (1 - slippageRate)
			fee := float64(position.Shares) * execPrice * feeRateSell
			cash += float64(position.Shares)*execPrice - fee
			order.Price = execPrice
			order.Status = "filled"
			order.Note = "rebalanced out of target set"
			orders = append(orders, order)
			fills = append(fills, paperFill{
				FilledAt: now.Format(time.RFC3339),
				Symbol:   symbol,
				Name:     position.Name,
				Side:     "SELL",
				Quantity: position.Shares,
				Price:    execPrice,
				Fee:      fee,
				Note:     order.Note,
			})
			delete(positionMap, symbol)
		}

		equity := cash
		for symbol, position := range positionMap {
			if bar, ok := latestBars[symbol]; ok {
				equity += float64(position.Shares) * bar.Close
			}
		}
		targetSlots := len(targets)
		if targetSlots < portfolio.MinHoldings {
			targetSlots = portfolio.MinHoldings
		}
		deployableCapital := equity * (1 - portfolio.MaxCashShare)
		slotValue := 0.0
		if targetSlots > 0 {
			slotValue = deployableCapital / float64(targetSlots)
		}
		maxSlotValue := equity * portfolio.MaxPositionWeight
		if slotValue > maxSlotValue {
			slotValue = maxSlotValue
		}

		for _, candidate := range targets {
			if _, ok := positionMap[candidate.Symbol]; ok {
				continue
			}
			bar, ok := latestBars[candidate.Symbol]
			if !ok {
				continue
			}
			order := paperOrder{
				PlacedAt: now.Format(time.RFC3339),
				Symbol:   candidate.Symbol,
				Name:     candidate.Name,
				Side:     "BUY",
				Price:    bar.Close,
				Status:   "skipped",
				Note:     "waiting for capital allocation",
			}
			if isBuyRestricted(candidate.Symbol, candidate.Name, bar) {
				order.Note = "buy restricted by market rule"
				orders = append(orders, order)
				continue
			}
			execPrice := bar.Close * (1 + slippageRate)
			targetBudget := math.Min(slotValue, cash)
			shares := int(targetBudget / (execPrice * (1 + feeRateBuy)))
			capacity := capacityLimitedShares(bar, portfolio.CapacityTurnoverShare)
			if capacity > 0 && shares > capacity {
				shares = capacity
			}
			if shares <= 0 {
				order.Note = "not enough cash or capacity"
				orders = append(orders, order)
				continue
			}
			fee := float64(shares) * execPrice * feeRateBuy
			cost := float64(shares)*execPrice + fee
			if cost > cash {
				order.Note = "not enough cash after fees"
				orders = append(orders, order)
				continue
			}
			cash -= cost
			order.Quantity = shares
			order.Price = execPrice
			order.Status = "filled"
			order.Note = "entered paper target list"
			orders = append(orders, order)
			fills = append(fills, paperFill{
				FilledAt: now.Format(time.RFC3339),
				Symbol:   candidate.Symbol,
				Name:     candidate.Name,
				Side:     "BUY",
				Quantity: shares,
				Price:    execPrice,
				Fee:      fee,
				Note:     order.Note,
			})
			positionMap[candidate.Symbol] = paperPosition{
				Symbol:     candidate.Symbol,
				Name:       candidate.Name,
				Shares:     shares,
				EntryPrice: execPrice,
				EntryDate:  marketDate,
			}
		}
	}

	holdings := make([]paperPosition, 0, len(positionMap))
	equity := cash
	for _, position := range positionMap {
		holdings = append(holdings, position)
		if bar, ok := latestBars[position.Symbol]; ok {
			equity += float64(position.Shares) * bar.Close
		}
	}
	sort.Slice(holdings, func(i, j int) bool { return holdings[i].Symbol < holdings[j].Symbol })

	if len(noteParts) == 0 {
		if session == "open" {
			noteParts = append(noteParts, "market session is open; paper account polled with live candidates")
		} else {
			noteParts = append(noteParts, "market closed; paper account refreshed with latest available bars")
		}
	}
	finalNote := strings.Join(noteParts, " | ")
	if err := savePaperAccountState(runtimeConfig.DB.Path, accountID, market, mode, strategyVersion, marketDate, status, cash, equity, holdings, orders, fills, finalNote, now); err != nil {
		return paperAccountResult{}, err
	}

	return paperAccountResult{
		AccountID:  accountID,
		Version:    strategyVersion,
		Market:     market,
		Mode:       mode,
		Session:    session,
		MarketDate: marketDate,
		Status:     status,
		UpdatedAt:  now.Format(time.RFC3339),
		Cash:       cash,
		Equity:     equity,
		Targets:    targets,
		Holdings:   holdings,
		Orders:     orders,
		Fills:      fills,
		Note:       finalNote,
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
			benchmark5 := forwardReturn(benchmarkBars, date, 5)
			benchmark10 := forwardReturn(benchmarkBars, date, 10)
			benchmark20 := forwardReturn(benchmarkBars, date, 20)
			if len(benchmarkBars) == 0 || math.IsNaN(benchmark5) || math.IsNaN(benchmark10) || math.IsNaN(benchmark20) {
				benchmark5 = 0
				benchmark10 = 0
				benchmark20 = 0
			}
			excess5 := label5 - benchmark5
			excess10 := label10 - benchmark10
			excess20 := label20 - benchmark20
			fundamentalScore, valuationScore, eventScore := fundamentalOverlayScores(candidate.Symbol)
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
				ValueScore:        candidate.ValueScore,
				LowVolScore:       candidate.LowVolScore,
				CrowdingScore:     candidate.CrowdingScore,
				FundamentalScore:  fundamentalScore,
				ValuationScore:    valuationScore,
				EventScore:        eventScore,
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
				Excess5D:          excess5,
				Excess10D:         excess10,
				Excess20D:         excess20,
				BeatBenchmark5D:   boolToFloat(excess5 > 0),
				BeatBenchmark10D:  boolToFloat(excess10 > 0),
				BeatBenchmark20D:  boolToFloat(excess20 > 0),
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

func boolToFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
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
	feeRateBuy := effectiveFeeRate(false)
	feeRateSell := effectiveFeeRate(true)
	slippageRate := slippageBps / 10000
	entryDate := ""

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

		prevClose := bar.Close
		if i > 0 {
			prevClose = filtered[i-1].Close
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
			if isBuyRestricted(symbol, name, bar) || gapOpenMove(prevClose, bar) >= runtimeConfig.Portfolio.GapOpenThreshold {
				continue
			}
			execPrice := math.Max(bar.Open, bar.Close) * (1 + slippageRate)
			buyShares := int(cash / (execPrice * (1 + feeRateBuy)))
			capacity := capacityLimitedShares(bar, runtimeConfig.Portfolio.CapacityTurnoverShare)
			if capacity > 0 && buyShares > capacity {
				buyShares = capacity
			}
			if buyShares <= 0 {
				continue
			}
			fee := float64(buyShares) * execPrice * feeRateBuy
			cash -= float64(buyShares)*execPrice + fee
			totalFees += fee
			shares = buyShares
			entryPrice = execPrice
			entryDate = bar.Date
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
			if runtimeConfig.Market.AShareT1 && entryDate == bar.Date {
				continue
			}
			if isSellRestricted(symbol, name, bar) || gapOpenMove(prevClose, bar) <= -runtimeConfig.Portfolio.GapOpenThreshold {
				continue
			}
			execPrice := math.Min(bar.Open, bar.Close) * (1 - slippageRate)
			capacity := capacityLimitedShares(bar, runtimeConfig.Portfolio.CapacityTurnoverShare)
			sellShares := shares
			if capacity > 0 && sellShares > capacity {
				sellShares = capacity
			}
			if sellShares <= 0 {
				continue
			}
			fee := float64(sellShares) * execPrice * feeRateSell
			proceeds := float64(sellShares)*execPrice - fee
			cash += proceeds
			totalFees += fee
			pnl := (execPrice - entryPrice) * float64(sellShares)
			if sellShares == shares {
				completedTrades++
				if pnl > 0 {
					winningTrades++
				}
				entryDate = ""
			}
			trades = append(trades, backtestTrade{
				Date:   bar.Date,
				Action: action,
				Price:  execPrice,
				Shares: sellShares,
				Fee:    fee,
				Cash:   cash,
				Equity: cash + float64(shares-sellShares)*bar.Close,
				Reason: reason,
			})
			shares -= sellShares
			if shares <= 0 {
				shares = 0
				entryPrice = 0
			}
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
	benchmarkEquity, benchmarkReturn, benchmarkDrawdown := simulateBuyAndHoldBenchmark(symbol, name, filtered, initialCash, feeRateBuy, feeRateSell, slippageRate)
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

func simulateBuyAndHoldBenchmark(symbol string, name string, bars []marketBar, initialCash float64, feeRateBuy float64, feeRateSell float64, slippageRate float64) (float64, float64, float64) {
	if len(bars) == 0 {
		return initialCash, 0, 0
	}

	buyPrice := bars[0].Close * (1 + slippageRate)
	shares := int(initialCash / (buyPrice * (1 + feeRateBuy)))
	if shares <= 0 {
		return initialCash, 0, 0
	}

	buyFee := float64(shares) * buyPrice * feeRateBuy
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

	finalBar := bars[len(bars)-1]
	sellPrice := finalBar.Close * (1 - slippageRate)
	if runtimeConfig.Market.AShareT1 && bars[0].Date == finalBar.Date {
		return cash + float64(shares)*finalBar.Close, (cash + float64(shares)*finalBar.Close - initialCash) / initialCash, maxDrawdown
	}
	if isSellRestricted(symbol, name, finalBar) {
		return cash + float64(shares)*finalBar.Close, (cash + float64(shares)*finalBar.Close - initialCash) / initialCash, maxDrawdown
	}
	sellFee := float64(shares) * sellPrice * feeRateSell
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

func averageBenchmarkReturn(selection []scanCandidate, fromDate string, toDate string, series []marketSeries, initialCash float64, feeRateBuy float64, feeRateSell float64, slippageRate float64) float64 {
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
		_, ret, _ := simulateBuyAndHoldBenchmark(candidate.Symbol, candidate.Name, filtered, initialCash, feeRateBuy, feeRateSell, slippageRate)
		values = append(values, ret)
	}
	if len(values) == 0 {
		return 0
	}
	return average(values)
}

func buildBenchmarkCurve(symbol string, name string, bars []marketBar, fromDate string, toDate string, initialCash float64, feeRateBuy float64, feeRateSell float64, slippageRate float64) []backtestTrade {
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
	shares := int(initialCash / (buyPrice * (1 + feeRateBuy)))
	if shares <= 0 {
		return nil
	}
	buyFee := float64(shares) * buyPrice * feeRateBuy
	cash := initialCash - float64(shares)*buyPrice - buyFee

	curve := make([]backtestTrade, 0, len(filtered))
	for _, bar := range filtered {
		equity := cash + float64(shares)*bar.Close
		if isSellRestricted(symbol, name, bar) {
			equity = cash + float64(shares)*bar.Close
		}
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
	if candidate.CrowdingScore > 0.16 {
		return false
	}
	if candidate.TrendScore < minTrendGap {
		return false
	}
	if candidate.LowVolScore < -0.03 {
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

func recentVolatility(history []marketBar, lookback int) float64 {
	if len(history) < 2 {
		return 0
	}
	if lookback <= 0 || lookback >= len(history) {
		lookback = len(history) - 1
	}
	returns := make([]float64, 0, lookback)
	for i := len(history) - lookback; i < len(history); i++ {
		if i == 0 || history[i-1].Close <= 0 {
			continue
		}
		returns = append(returns, history[i].Close/history[i-1].Close-1)
	}
	return standardDeviation(returns)
}

func previousTradingDate(dates []string, idx int) string {
	if idx <= 0 || idx >= len(dates) {
		return ""
	}
	return dates[idx-1]
}

func seriesBarsForSymbol(series []marketSeries, symbol string) []marketBar {
	for _, item := range series {
		if item.meta.Symbol == symbol {
			return item.bars
		}
	}
	return nil
}

func averageTurnover(bars []marketBar, lookback int) float64 {
	if len(bars) == 0 {
		return 0
	}
	if lookback <= 0 || lookback > len(bars) {
		lookback = len(bars)
	}
	window := bars[len(bars)-lookback:]
	total := 0.0
	for _, bar := range window {
		total += bar.Close * bar.Volume
	}
	return total / float64(len(window))
}

func amihudIlliquidity(bars []marketBar, lookback int) float64 {
	if len(bars) < 2 {
		return 0
	}
	if lookback <= 0 || lookback >= len(bars) {
		lookback = len(bars) - 1
	}
	window := bars[len(bars)-lookback:]
	total := 0.0
	count := 0.0
	for i := 1; i < len(window); i++ {
		prevClose := window[i-1].Close
		turnover := window[i].Close * window[i].Volume
		if prevClose <= 0 || turnover <= 0 {
			continue
		}
		ret := math.Abs(window[i].Close/prevClose - 1)
		total += ret / turnover
		count++
	}
	if count == 0 {
		return 0
	}
	return total / count
}

func rollingMaxDrawdown(closes []float64, lookback int) float64 {
	if len(closes) == 0 {
		return 0
	}
	if lookback <= 0 || lookback > len(closes) {
		lookback = len(closes)
	}
	window := closes[len(closes)-lookback:]
	peak := window[0]
	maxDrawdown := 0.0
	for _, value := range window {
		if value > peak {
			peak = value
		}
		if peak > 0 {
			drawdown := (peak - value) / peak
			if drawdown > maxDrawdown {
				maxDrawdown = drawdown
			}
		}
	}
	return maxDrawdown
}

func dailyMove(bar marketBar) float64 {
	if bar.Open <= 0 {
		return 0
	}
	return bar.Close/bar.Open - 1
}

func gapOpenMove(prevClose float64, bar marketBar) float64 {
	if prevClose <= 0 {
		return 0
	}
	return bar.Open/prevClose - 1
}

func isSuspendedBar(bar marketBar) bool {
	return bar.Volume <= 0 || (bar.Open == 0 && bar.High == 0 && bar.Low == 0 && bar.Close == 0)
}

func boardLimitForSymbol(symbol string, name string) float64 {
	if isSTName(name) {
		return runtimeConfig.Market.RiskWarningLimit
	}
	if strings.HasPrefix(symbol, "300") {
		return runtimeConfig.Market.ChiNextLimit
	}
	if strings.HasPrefix(symbol, "688") {
		return runtimeConfig.Market.STARLimit
	}
	return runtimeConfig.Market.MainBoardLimit
}

func isOnePriceLimitBar(bar marketBar, threshold float64) bool {
	if bar.Open <= 0 {
		return false
	}
	intradayRange := math.Abs(bar.High/bar.Open - 1)
	return intradayRange < 0.001 && math.Abs(dailyMove(bar)) >= threshold
}

func isBuyRestricted(symbol string, name string, bar marketBar) bool {
	threshold := boardLimitForSymbol(symbol, name)
	return isSuspendedBar(bar) || isOnePriceLimitBar(bar, threshold) || dailyMove(bar) >= threshold
}

func isSellRestricted(symbol string, name string, bar marketBar) bool {
	threshold := boardLimitForSymbol(symbol, name)
	return isSuspendedBar(bar) || isOnePriceLimitBar(bar, threshold) || dailyMove(bar) <= -threshold
}

func capacityLimitedShares(bar marketBar, capacityShare float64) int {
	if bar.Volume <= 0 || capacityShare <= 0 {
		return 0
	}
	return int(bar.Volume * capacityShare)
}

func effectiveFeeRate(isSell bool) float64 {
	rate := runtimeConfig.Market.CommissionBps + runtimeConfig.Market.TransferFeeBps + runtimeConfig.Market.HandlingFeeBps
	if isSell {
		rate += runtimeConfig.Market.StampDutySellBps
	}
	return rate / 10000
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

func selectPortfolioCandidates(candidates []scanCandidate, topN int, minHoldings int, portfolio portfolioConfig, regimeLabel string) []scanCandidate {
	if len(candidates) == 0 {
		return candidates
	}

	sort.Slice(candidates, func(i, j int) bool {
		leftScore := portfolioSelectionScore(candidates[i], portfolio, regimeLabel)
		rightScore := portfolioSelectionScore(candidates[j], portfolio, regimeLabel)
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
	industryLimit := portfolio.IndustryMaxPositions
	if industryLimit <= 0 {
		industryLimit = 1
	}
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
		if industryKey != "" && industryCount[industryKey] >= industryLimit {
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

func reservePortfolioCandidates(candidates []scanCandidate, selected []scanCandidate, reserveCount int) []scanCandidate {
	if reserveCount <= 0 {
		return nil
	}
	reserve := make([]scanCandidate, 0, reserveCount)
	for _, candidate := range candidates {
		if containsCandidate(selected, candidate.Symbol) {
			continue
		}
		reserve = append(reserve, candidate)
		if len(reserve) >= reserveCount {
			break
		}
	}
	return reserve
}

func portfolioSelectionScore(candidate scanCandidate, portfolio portfolioConfig, regimeLabel string) float64 {
	score := candidate.Score
	score += candidate.QualityScore * portfolio.QualityWeight
	score += candidate.RiskScore * portfolio.RiskWeight
	score += candidate.ReversalScore * portfolio.ReversalWeight
	score += candidate.ValueScore * 0.95
	score += candidate.LowVolScore * 1.10
	score -= candidate.CrowdingScore * 0.95
	score += candidate.FundamentalScore * 1.05
	score += candidate.ValuationScore * 0.90
	score += candidate.EventScore * 0.80
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
	score += (candidate.BenchmarkModelScore - 0.50) * 0.60
	switch regimeLabel {
	case "risk_on":
		score += candidate.MomentumScore * 0.40
		score += candidate.BreakoutScore * 0.35
		score += candidate.ModelScore * 0.25
		score -= candidate.ValuationScore * 0.15
	case "risk_off":
		score += candidate.FundamentalScore * 0.35
		score += candidate.ValuationScore * 0.30
		score += candidate.LowVolScore * 0.30
		score += (candidate.BenchmarkModelScore - 0.50) * 0.40
		score -= candidate.MomentumScore * 0.45
		score -= candidate.BreakoutScore * 0.35
		score -= candidate.HeatPenalty * 0.40
	default:
		score += candidate.FundamentalScore * 0.15
		score += candidate.ValuationScore * 0.10
		score -= candidate.CrowdingScore * 0.10
	}
	if candidate.RiskPenalty > 0 {
		score -= candidate.RiskPenalty
	}
	if candidate.Bucket == "观望" {
		score -= portfolio.WatchPenalty
	}
	return score
}

func candidateOverlayScores(bars []marketBar, shortMA float64, longMA float64, avgVolume float64, shortReturn float64, mediumReturn float64, trendScore float64, liquidityScore float64, persistenceScore float64, breakoutScore float64, volumeTrendScore float64, riskPenalty float64) (float64, float64, float64, float64, float64, float64, float64) {
	if len(bars) == 0 {
		return 0, 0, 0, 0, 0, 0, 0
	}
	latest := bars[len(bars)-1]
	closes := make([]float64, 0, len(bars))
	for _, bar := range bars {
		closes = append(closes, bar.Close)
	}

	valueScore := 0.0
	lookbackHigh := latest.Close
	start := max(0, len(closes)-min(30, len(closes)))
	for _, price := range closes[start:] {
		if price > lookbackHigh {
			lookbackHigh = price
		}
	}
	if lookbackHigh > 0 && latest.Close >= longMA {
		pullbackGap := 1 - latest.Close/lookbackHigh
		valueScore = clampFloat(pullbackGap, 0, 0.18) * 0.70
		if shortMA > 0 {
			valueScore += clampFloat(shortMA/latest.Close-1, -0.03, 0.05) * 0.60
		}
	}

	stabilityDrawdown := rollingMaxDrawdown(closes, min(20, len(closes)))
	aboveLongRatio := 0.0
	if len(bars) > 0 && longMA > 0 {
		count := 0.0
		for _, bar := range bars[max(0, len(bars)-min(20, len(bars))):] {
			count++
			if bar.Close >= longMA {
				aboveLongRatio++
			}
		}
		if count > 0 {
			aboveLongRatio /= count
		}
	}

	qualityScore := 0.0
	qualityScore += trendScore * 1.15
	qualityScore += persistenceScore * 1.35
	qualityScore += liquidityScore * 5.50
	qualityScore += valueScore * 0.85
	qualityScore += clampFloat(aboveLongRatio-0.5, -0.2, 0.3) * 0.22
	qualityScore += clampFloat(0.18-stabilityDrawdown, -0.10, 0.18) * 0.60
	qualityScore += volumeTrendScore * 0.45
	qualityScore = clampFloat(qualityScore, -0.20, 0.40)

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

	lowVolScore := 0.0
	if volatilityPenalty > 0 {
		lowVolScore = clampFloat(0.06-volatilityPenalty, -0.06, 0.06) * 1.10
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
	riskScore += lowVolScore
	riskScore -= volatilityPenalty * 0.70
	riskScore -= riskPenalty * 1.50
	riskScore = clampFloat(riskScore, -0.15, 0.18)

	crowdingScore := 0.0
	if shortReturn > 0.04 {
		crowdingScore += clampFloat(shortReturn-0.04, 0, 0.08) * 1.70
	}
	if mediumReturn > 0.12 {
		crowdingScore += clampFloat(mediumReturn-0.12, 0, 0.15) * 1.20
	}
	if avgVolume > 0 {
		recentVolumeWindow := min(3, len(bars))
		recentAvgVolume := 0.0
		for _, bar := range bars[len(bars)-recentVolumeWindow:] {
			recentAvgVolume += bar.Volume
		}
		recentAvgVolume /= float64(recentVolumeWindow)
		crowdingScore += clampFloat(recentAvgVolume/avgVolume-1, 0, 0.80) * 0.10
	}
	crowdingScore += clampFloat(breakoutScore, 0, 0.03) * 1.40
	crowdingScore = clampFloat(crowdingScore, 0, 0.24)

	heatPenalty := 0.0
	if shortReturn > 0.05 {
		heatPenalty += (shortReturn - 0.05) * 1.60
	}
	if mediumReturn > 0.15 {
		heatPenalty += (mediumReturn - 0.15) * 1.00
	}
	heatPenalty += clampFloat(breakoutScore-0.01, 0, 0.05) * 3.0
	heatPenalty += clampFloat(volumeTrendScore-0.02, 0, 0.06) * 2.0
	heatPenalty += crowdingScore * 0.85
	heatPenalty = clampFloat(heatPenalty, 0, 0.25)

	reversalScore := 0.0
	threeDayReturn := trailingReturn(closes, min(3, len(closes)-1))
	if mediumReturn > 0 && latest.Close >= longMA && latest.Close <= shortMA*1.01 {
		reversalScore += clampFloat(-threeDayReturn, 0, 0.06) * 1.50
		reversalScore += clampFloat(shortMA/latest.Close-1, 0, 0.04) * 1.20
		reversalScore += clampFloat(0.08-heatPenalty, 0, 0.08) * 0.60
	}
	reversalScore = clampFloat(reversalScore, 0, 0.18)

	return valueScore, lowVolScore, crowdingScore, qualityScore, riskScore, heatPenalty, reversalScore
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
	jsonPath := reportJSONPath("backtest_latest")
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
	if err := writeJSONFile(jsonPath, result); err != nil {
		return err
	}
	_ = appendExperimentRecord("backtest", map[string]any{
		"symbol":       result.Symbol,
		"from_date":    result.FromDate,
		"to_date":      result.ToDate,
		"fee_bps":      result.FeeBps,
		"slippage_bps": result.SlippageBps,
		"initial_cash": result.InitialCash,
	}, map[string]any{
		"final_equity":      result.FinalEquity,
		"total_return":      result.TotalReturn,
		"annualized_return": result.AnnualizedReturn,
		"excess_return":     result.ExcessReturn,
		"max_drawdown":      result.MaxDrawdown,
		"win_rate":          result.WinRate,
	})
	return persistRunRecord("backtest_latest", map[string]any{
		"symbol":            result.Symbol,
		"mode":              result.Mode,
		"total_return":      result.TotalReturn,
		"max_drawdown":      result.MaxDrawdown,
		"annualized_return": result.AnnualizedReturn,
	}, []string{textPath, htmlPath, jsonPath})
}

func writeBatchBacktestReports(results []backtestResult, fromDate string, toDate string, initialCash float64, feeBps float64, slippageBps float64) error {
	textPath := filepath.Join(reportsDir, "backtest_scan.txt")
	htmlPath := filepath.Join(reportsDir, "backtest_scan.html")
	csvPath := filepath.Join(reportsDir, "backtest_scan.csv")
	jsonPath := reportJSONPath("backtest_scan")

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
	if err := writeJSONFile(jsonPath, results); err != nil {
		return err
	}
	_ = appendExperimentRecord("batch_backtest", map[string]any{
		"from_date": fromDate,
		"to_date":   toDate,
		"count":     len(results),
	}, map[string]any{
		"top_symbol": func() string {
			if len(results) == 0 {
				return ""
			}
			return results[0].Symbol
		}(),
	})
	return persistRunRecord("backtest_scan", map[string]any{
		"from_date": fromDate,
		"to_date":   toDate,
		"count":     len(results),
	}, []string{textPath, htmlPath, csvPath, jsonPath})
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
	jsonPath := reportJSONPath("portfolio_backtest")

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
	if err := writeJSONFile(jsonPath, result); err != nil {
		return err
	}
	if runtimeConfig.DB.Path != "" && len(result.Snapshots) > 0 {
		holdingsJSON, _ := json.Marshal(result.CurrentHoldings)
		last := result.Snapshots[len(result.Snapshots)-1]
		sql := fmt.Sprintf(
			"INSERT INTO simulated_account_ledger (snapshot_date, market, regime, exposure, equity, cash, holdings_json, note) VALUES (%s, %s, %s, %.6f, %.6f, %.6f, %s, %s);",
			quoteSQL(last.Date),
			quoteSQL("a_share"),
			quoteSQL(result.RegimeLabel),
			result.ExposureLevel,
			result.FinalEquity,
			last.Cash,
			quoteSQL(string(holdingsJSON)),
			quoteSQL("portfolio_backtest_snapshot"),
		)
		_ = execSQLite(runtimeConfig.DB.Path, sql)
	}
	_ = appendExperimentRecord("portfolio_backtest", map[string]any{
		"from_date": result.FromDate,
		"to_date":   result.ToDate,
		"positions": result.Positions,
		"regime":    result.RegimeLabel,
	}, map[string]any{
		"final_equity":      result.FinalEquity,
		"total_return":      result.TotalReturn,
		"annualized_return": result.AnnualizedReturn,
		"benchmark_return":  result.BenchmarkReturn,
		"excess_return":     result.ExcessReturn,
		"max_drawdown":      result.MaxDrawdown,
		"rebalances":        result.RebalanceCount,
	})
	if err := persistRunRecord("portfolio_backtest", map[string]any{
		"from_date":       result.FromDate,
		"to_date":         result.ToDate,
		"regime":          result.RegimeLabel,
		"target_exposure": result.ExposureLevel,
		"total_return":    result.TotalReturn,
		"max_drawdown":    result.MaxDrawdown,
	}, []string{textPath, htmlPath, csvPath, jsonPath}); err != nil {
		return err
	}
	return writeDashboardReports()
}

func writePaperTradingReports(result paperAccountResult) error {
	baseName := paperReportBaseName(result.Mode)
	textPath := filepath.Join(reportsDir, baseName+".txt")
	htmlPath := filepath.Join(reportsDir, baseName+".html")
	jsonPath := reportJSONPath(baseName)

	var textBuilder strings.Builder
	fmt.Fprintf(&textBuilder, "Paper Trading Account\n\n")
	fmt.Fprintf(&textBuilder, "Account ID: %d\n", result.AccountID)
	fmt.Fprintf(&textBuilder, "Strategy version: %s\n", result.Version)
	fmt.Fprintf(&textBuilder, "Market: %s\n", result.Market)
	fmt.Fprintf(&textBuilder, "Mode: %s\n", result.Mode)
	fmt.Fprintf(&textBuilder, "Session: %s\n", result.Session)
	fmt.Fprintf(&textBuilder, "Market date: %s\n", result.MarketDate)
	fmt.Fprintf(&textBuilder, "Cash: %.2f\n", result.Cash)
	fmt.Fprintf(&textBuilder, "Equity: %.2f\n", result.Equity)
	fmt.Fprintf(&textBuilder, "Note: %s\n\n", result.Note)
	textBuilder.WriteString("Current holdings\n")
	if len(result.Holdings) == 0 {
		textBuilder.WriteString("No holdings.\n")
	} else {
		for _, holding := range result.Holdings {
			fmt.Fprintf(&textBuilder, "- %s %s shares=%d entry=%.2f entry_date=%s\n", holding.Symbol, holding.Name, holding.Shares, holding.EntryPrice, holding.EntryDate)
		}
	}
	textBuilder.WriteString("\nOrders this cycle\n")
	if len(result.Orders) == 0 {
		textBuilder.WriteString("No orders.\n")
	} else {
		for _, order := range result.Orders {
			fmt.Fprintf(&textBuilder, "- %s %s %s qty=%d price=%.2f status=%s note=%s\n", order.PlacedAt, order.Symbol, order.Side, order.Quantity, order.Price, order.Status, order.Note)
		}
	}
	textBuilder.WriteString("\nTargets\n")
	if len(result.Targets) == 0 {
		textBuilder.WriteString("No current targets.\n")
	} else {
		for _, candidate := range result.Targets {
			fmt.Fprintf(&textBuilder, "- %s %s score=%.4f plan=%s\n", candidate.Symbol, candidate.Name, candidate.Score, candidate.Plan)
		}
	}

	var holdingsHTML strings.Builder
	for _, holding := range result.Holdings {
		fmt.Fprintf(&holdingsHTML, "<tr><td>%s</td><td>%s</td><td>%d</td><td>%.2f</td><td>%s</td></tr>",
			html.EscapeString(holding.Symbol),
			html.EscapeString(holding.Name),
			holding.Shares,
			holding.EntryPrice,
			html.EscapeString(holding.EntryDate),
		)
	}
	var ordersHTML strings.Builder
	for _, order := range result.Orders {
		fmt.Fprintf(&ordersHTML, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%.2f</td><td>%s</td><td>%s</td></tr>",
			html.EscapeString(order.PlacedAt),
			html.EscapeString(order.Symbol),
			html.EscapeString(order.Side),
			order.Quantity,
			order.Price,
			html.EscapeString(order.Status),
			html.EscapeString(order.Note),
		)
	}
	htmlContent := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Paper Trading</title>
  <style>
    body { margin: 0; font-family: Georgia, "Times New Roman", serif; background: #f4efe6; color: #1f1b16; }
    .wrap { max-width: 1100px; margin: 36px auto; padding: 0 20px; }
    .card { background: #fffaf3; border: 1px solid #d9cfbf; border-radius: 18px; padding: 24px; box-shadow: 0 18px 40px rgba(70, 50, 20, 0.08); }
    h1, h2 { margin-top: 0; }
    .meta { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 12px; margin-bottom: 24px; }
    .meta div { background: #f8f1e6; border: 1px solid #eadfcd; border-radius: 14px; padding: 12px 14px; }
    table { width: 100%%; border-collapse: collapse; font-size: 15px; }
    th, td { text-align: left; padding: 10px 8px; border-bottom: 1px solid #e7dece; vertical-align: top; }
    th { font-size: 12px; text-transform: uppercase; color: #6d6559; }
    p { line-height: 1.6; }
  </style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <h1>Paper Trading Account</h1>
      <div class="meta">
        <div><strong>Account</strong><br>%d</div>
        <div><strong>Version</strong><br>%s</div>
        <div><strong>Market</strong><br>%s</div>
        <div><strong>Session</strong><br>%s</div>
        <div><strong>Market Date</strong><br>%s</div>
        <div><strong>Cash</strong><br>%.2f</div>
        <div><strong>Equity</strong><br>%.2f</div>
      </div>
      <p>%s</p>
      <h2>Current Holdings</h2>
      <table><thead><tr><th>Symbol</th><th>Name</th><th>Shares</th><th>Entry</th><th>Entry Date</th></tr></thead><tbody>%s</tbody></table>
      <h2>Orders This Cycle</h2>
      <table><thead><tr><th>Placed</th><th>Symbol</th><th>Side</th><th>Qty</th><th>Price</th><th>Status</th><th>Note</th></tr></thead><tbody>%s</tbody></table>
    </div>
  </div>
</body>
</html>`,
		result.AccountID,
		html.EscapeString(result.Version),
		html.EscapeString(result.Market),
		html.EscapeString(result.Session),
		html.EscapeString(result.MarketDate),
		result.Cash,
		result.Equity,
		html.EscapeString(result.Note),
		holdingsHTML.String(),
		ordersHTML.String(),
	)

	if err := os.WriteFile(textPath, []byte(textBuilder.String()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0o644); err != nil {
		return err
	}
	if err := writeJSONFile(jsonPath, result); err != nil {
		return err
	}
	runType := "paper_trading"
	if strings.HasPrefix(result.Mode, "shadow:") {
		runType = "paper_shadow"
	}
	if err := persistRunRecord(runType, map[string]any{
		"account_id":    result.AccountID,
		"version":       result.Version,
		"market":        result.Market,
		"session":       result.Session,
		"market_date":   result.MarketDate,
		"cash":          result.Cash,
		"equity":        result.Equity,
		"order_count":   len(result.Orders),
		"holding_count": len(result.Holdings),
	}, []string{textPath, htmlPath, jsonPath}); err != nil {
		return err
	}
	return writeDashboardReports()
}

func paperReportBaseName(mode string) string {
	if strings.HasPrefix(mode, "shadow:") {
		return "paper_shadow"
	}
	return "paper_account"
}

func writeGridSearchReports(results []gridSearchResult, fromDate string, toDate string) error {
	textPath := filepath.Join(reportsDir, "grid_search.txt")
	htmlPath := filepath.Join(reportsDir, "grid_search.html")
	csvPath := filepath.Join(reportsDir, "grid_search.csv")
	jsonPath := reportJSONPath("grid_search")

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
	if err := writeJSONFile(jsonPath, results); err != nil {
		return err
	}
	if len(results) > 0 {
		_ = appendExperimentRecord("grid_search", map[string]any{
			"from_date": fromDate,
			"to_date":   toDate,
		}, map[string]any{
			"best_short_window": results[0].ShortWindow,
			"best_long_window":  results[0].LongWindow,
			"best_total_return": results[0].TotalReturn,
			"best_max_drawdown": results[0].MaxDrawdown,
		})
	}
	if err := persistRunRecord("grid_search", map[string]any{
		"from_date": fromDate,
		"to_date":   toDate,
		"count":     len(results),
	}, []string{textPath, htmlPath, csvPath, jsonPath}); err != nil {
		return err
	}
	return writeDashboardReports()
}

func writeDatasetReports(rows []datasetRow, fromDate string, toDate string) error {
	csvPath := filepath.Join(reportsDir, "training_dataset.csv")
	textPath := filepath.Join(reportsDir, "training_dataset.txt")
	jsonPath := reportJSONPath("training_dataset")

	var csvBuilder strings.Builder
	csvBuilder.WriteString("symbol,name,industry,date,close,avg_volume,short_ma,long_ma,score,quality_score,risk_score,heat_penalty,reversal_score,value_score,low_vol_score,crowding_score,fundamental_score,valuation_score,event_score,trend_score,liquidity_score,structure_score,momentum_score,persistence_score,breakout_score,volume_trend_score,short_return_score,medium_return_score,rotation_score,strategy_alignment,breadth,regime_exposure,label_5d,label_10d,label_20d,excess_5d,excess_10d,excess_20d,beat_benchmark_5d,beat_benchmark_10d,beat_benchmark_20d\n")
	for _, row := range rows {
		fmt.Fprintf(&csvBuilder, "%s,%s,%s,%s,%.4f,%.0f,%.4f,%.4f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.6f,%.0f,%.0f,%.0f\n",
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
			row.ValueScore,
			row.LowVolScore,
			row.CrowdingScore,
			row.FundamentalScore,
			row.ValuationScore,
			row.EventScore,
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
			row.Excess5D,
			row.Excess10D,
			row.Excess20D,
			row.BeatBenchmark5D,
			row.BeatBenchmark10D,
			row.BeatBenchmark20D,
		)
	}

	var textBuilder strings.Builder
	fmt.Fprintf(&textBuilder, "Training Dataset %s -> %s\n\nRows: %d\nFeatures: 24\nLabels: label_5d, label_10d, label_20d, excess_5d, excess_10d, excess_20d, beat_benchmark_5d, beat_benchmark_10d, beat_benchmark_20d\nCSV: %s\n\nSample Rows\n",
		fromDate,
		toDate,
		len(rows),
		csvPath,
	)
	sampleCount := min(5, len(rows))
	for i := 0; i < sampleCount; i++ {
		row := rows[i]
		fmt.Fprintf(&textBuilder, "%d. %s %s %s score=%.4f label5=%.2f%% label10=%.2f%% label20=%.2f%% excess10=%.2f%% beat10=%.0f\n",
			i+1,
			row.Date,
			row.Symbol,
			row.Name,
			row.Score,
			row.Label5D*100,
			row.Label10D*100,
			row.Label20D*100,
			row.Excess10D*100,
			row.BeatBenchmark10D,
		)
	}

	if err := os.WriteFile(csvPath, []byte(csvBuilder.String()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(textPath, []byte(textBuilder.String()), 0o644); err != nil {
		return err
	}
	if err := writeJSONFile(jsonPath, map[string]any{
		"from_date": fromDate,
		"to_date":   toDate,
		"rows":      len(rows),
		"sample":    rows[:min(10, len(rows))],
	}); err != nil {
		return err
	}
	_ = appendExperimentRecord("dataset_export", map[string]any{
		"from_date": fromDate,
		"to_date":   toDate,
	}, map[string]any{
		"rows": len(rows),
	})
	if err := persistRunRecord("training_dataset", map[string]any{
		"from_date": fromDate,
		"to_date":   toDate,
		"rows":      len(rows),
	}, []string{textPath, csvPath, jsonPath}); err != nil {
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

	path := runtimeConfig.Model.ModelPath
	if strings.TrimSpace(path) == "" {
		path = filepath.Join(reportsDir, "linear_model.json")
	}
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

func getBenchmarkModel() *linearModel {
	if benchmarkModelLoaded {
		return cachedBenchmarkModel
	}
	benchmarkModelLoaded = true

	path := runtimeConfig.Model.BenchmarkModelPath
	if strings.TrimSpace(path) == "" {
		path = filepath.Join(reportsDir, "benchmark_classifier.json")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var model linearModel
	if err := json.Unmarshal(content, &model); err != nil {
		return nil
	}
	cachedBenchmarkModel = &model
	return cachedBenchmarkModel
}

func predictWithModel(model *linearModel, candidate scanCandidate) float64 {
	if model == nil {
		return 0
	}

	featureValues := map[string]float64{
		"score":               candidate.Score,
		"quality_score":       candidate.QualityScore,
		"risk_score":          candidate.RiskScore,
		"heat_penalty":        candidate.HeatPenalty,
		"reversal_score":      candidate.ReversalScore,
		"value_score":         candidate.ValueScore,
		"low_vol_score":       candidate.LowVolScore,
		"crowding_score":      candidate.CrowdingScore,
		"fundamental_score":   candidate.FundamentalScore,
		"valuation_score":     candidate.ValuationScore,
		"event_score":         candidate.EventScore,
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
	if model.Task == "classification" || strings.HasPrefix(model.Label, "beat_benchmark_") {
		return 1.0 / (1.0 + math.Exp(-score))
	}
	return score
}

func predictLinearModel(candidate scanCandidate) float64 {
	return predictWithModel(getLinearModel(), candidate)
}

func predictBenchmarkModel(candidate scanCandidate) float64 {
	return predictWithModel(getBenchmarkModel(), candidate)
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
	jsonPath := reportJSONPath("dashboard")
	historyTextPath := filepath.Join(reportsDir, "history_compare.txt")
	historyHTMLPath := filepath.Join(reportsDir, "history_compare.html")
	historyJSONPath := reportJSONPath("history_compare")
	marketTextPath := filepath.Join(reportsDir, "market_overview.txt")
	marketHTMLPath := filepath.Join(reportsDir, "market_overview.html")
	marketJSONPath := reportJSONPath("market_overview")
	lifecycleTextPath := filepath.Join(reportsDir, "strategy_lifecycle.txt")
	lifecycleHTMLPath := filepath.Join(reportsDir, "strategy_lifecycle.html")
	lifecycleJSONPath := reportJSONPath("strategy_lifecycle")

	if err := writeDiagnosticsReports(); err != nil {
		return err
	}

	sections := []struct {
		title string
		path  string
	}{
		{title: "Latest Plan", path: filepath.Join(reportsDir, "latest_plan.txt")},
		{title: "A-Share Focus", path: filepath.Join(reportsDir, "a_share_focus.txt")},
		{title: "A-Share Scan", path: filepath.Join(reportsDir, "a_share_scan.txt")},
		{title: "Paper Trading", path: filepath.Join(reportsDir, "paper_account.txt")},
		{title: "Shadow Trading", path: filepath.Join(reportsDir, "paper_shadow.txt")},
		{title: "Health Monitor", path: filepath.Join(reportsDir, "health_monitor.txt")},
		{title: "Factor Research", path: filepath.Join(reportsDir, "factor_research.txt")},
		{title: "Promotion Decision", path: filepath.Join(reportsDir, "strategy_promotion_latest.txt")},
		{title: "Rollback Decision", path: filepath.Join(reportsDir, "strategy_rollback_latest.txt")},
		{title: "Portfolio Backtest", path: filepath.Join(reportsDir, "portfolio_backtest.txt")},
		{title: "Diagnostics", path: filepath.Join(reportsDir, "diagnostics.txt")},
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

	todayCard := buildTodayConclusionCard()
	riskCard := buildRiskAlertCard()
	changeCard := buildChangeCard()
	strongWeakCard := buildStrengthCard()
	holdingCard := buildHoldingCard()
	evolutionCard := buildStrategyEvolutionCard()
	lifecycleCard := buildLifecycleSummaryCard()
	healthCard := buildHealthSummaryCard()
	factorCard := buildFactorSummaryCard()

	var textBuilder strings.Builder
	textBuilder.WriteString("Quant MVP Dashboard\n\n")
	textBuilder.WriteString("Today Conclusion\n" + todayCard + "\n\n")
	textBuilder.WriteString("Risk Alerts\n" + riskCard + "\n\n")
	textBuilder.WriteString("Changes vs Yesterday\n" + changeCard + "\n\n")
	textBuilder.WriteString("Strongest / Weakest\n" + strongWeakCard + "\n\n")
	textBuilder.WriteString("Current Holdings\n" + holdingCard + "\n\n")
	textBuilder.WriteString("Strategy Evolution\n" + evolutionCard + "\n\n")
	textBuilder.WriteString("Lifecycle Summary\n" + lifecycleCard + "\n\n")
	textBuilder.WriteString("System Health\n" + healthCard + "\n\n")
	textBuilder.WriteString("Factor Research\n" + factorCard + "\n\n")
	for _, section := range rendered {
		textBuilder.WriteString(section.Title + "\n")
		if section.Stamp != "" {
			textBuilder.WriteString("Updated: " + section.Stamp + "\n")
		}
		textBuilder.WriteString(section.Content + "\n\n")
	}

	var summaryCards strings.Builder
	for _, item := range []struct {
		Title string
		Body  string
	}{
		{Title: "Today Conclusion", Body: todayCard},
		{Title: "Risk Alerts", Body: riskCard},
		{Title: "Changes vs Yesterday", Body: changeCard},
		{Title: "Strongest / Weakest", Body: strongWeakCard},
		{Title: "Current Holdings", Body: holdingCard},
		{Title: "Strategy Evolution", Body: evolutionCard},
		{Title: "Lifecycle Summary", Body: lifecycleCard},
		{Title: "System Health", Body: healthCard},
		{Title: "Factor Research", Body: factorCard},
	} {
		fmt.Fprintf(&summaryCards, `<section class="summary"><h2>%s</h2><p>%s</p></section>`,
			html.EscapeString(item.Title),
			html.EscapeString(item.Body),
		)
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
    .summary-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 18px; margin-bottom: 18px; }
    .summary { background: linear-gradient(180deg, #fff8ef, #f5ead8); border: 1px solid var(--border); border-radius: 18px; padding: 18px 20px; box-shadow: 0 14px 34px rgba(70, 50, 20, 0.08); }
    .summary p { margin: 0; font-size: 15px; line-height: 1.55; color: #3a3128; }
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
    <div class="summary-grid">%s</div>
    <div class="grid">%s</div>
  </div>
</body>
</html>`, summaryCards.String(), cards.String())

	if err := os.WriteFile(textPath, []byte(textBuilder.String()), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0o644); err != nil {
		return err
	}
	payload := map[string]any{
		"today_conclusion":   todayCard,
		"risk_alerts":        riskCard,
		"changes":            changeCard,
		"strong_weak":        strongWeakCard,
		"current_holdings":   holdingCard,
		"strategy_evolution": evolutionCard,
		"lifecycle_summary":  lifecycleCard,
		"system_health":      healthCard,
		"factor_research":    factorCard,
		"sections":           rendered,
	}
	if err := writeJSONFile(jsonPath, payload); err != nil {
		return err
	}
	historyPayload, historyText, historyHTML := buildHistoryCompareReport()
	if err := os.WriteFile(historyTextPath, []byte(historyText), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(historyHTMLPath, []byte(historyHTML), 0o644); err != nil {
		return err
	}
	if err := writeJSONFile(historyJSONPath, historyPayload); err != nil {
		return err
	}
	marketPayload, marketText, marketHTML := buildMarketOverviewReport()
	if err := os.WriteFile(marketTextPath, []byte(marketText), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(marketHTMLPath, []byte(marketHTML), 0o644); err != nil {
		return err
	}
	if err := writeJSONFile(marketJSONPath, marketPayload); err != nil {
		return err
	}
	lifecyclePayload, lifecycleText, lifecycleHTML := buildStrategyLifecycleReport()
	if err := os.WriteFile(lifecycleTextPath, []byte(lifecycleText), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(lifecycleHTMLPath, []byte(lifecycleHTML), 0o644); err != nil {
		return err
	}
	if err := writeJSONFile(lifecycleJSONPath, lifecyclePayload); err != nil {
		return err
	}
	if runtimeConfig.DB.Path != "" {
		summaryJSON, _ := json.Marshal(payload)
		sql := fmt.Sprintf("INSERT INTO dashboard_snapshots (generated_at, summary_json) VALUES (%s, %s);",
			quoteSQL(time.Now().Format(time.RFC3339)),
			quoteSQL(string(summaryJSON)),
		)
		_ = execSQLite(runtimeConfig.DB.Path, sql)
	}
	return persistRunRecord("dashboard", payload, []string{textPath, htmlPath, jsonPath})
}

func reportJSONPath(baseName string) string {
	return filepath.Join(reportsDir, baseName+".json")
}

func writeJSONFile(path string, payload any) error {
	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func reportHistoryRoot() string {
	root := runtimeConfig.Report.HistoryRoot
	if strings.TrimSpace(root) == "" {
		root = filepath.Join(reportsDir, "history")
	}
	return root
}

func appendJSONLine(path string, payload any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(content, '\n')); err != nil {
		return err
	}
	return nil
}

func copyFile(src string, dst string) error {
	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, content, 0o644)
}

func archiveFiles(runType string, files []string) (string, []string, error) {
	dateDir := time.Now().Format("2006-01-02")
	targetDir := filepath.Join(reportHistoryRoot(), dateDir, runType)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", nil, err
	}
	archived := make([]string, 0, len(files))
	for _, src := range files {
		if strings.TrimSpace(src) == "" {
			continue
		}
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(targetDir, filepath.Base(src))
		if err := copyFile(src, dst); err != nil {
			return "", nil, err
		}
		archived = append(archived, dst)
	}
	return targetDir, archived, nil
}

func currentGitCommit() string {
	cmd := exec.Command("git", "-C", ".", "rev-parse", "--short", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

func currentStrategyVersionName(cfg config) string {
	return fmt.Sprintf("%s_%s_%d_%d", cfg.Strategy.Name, cfg.Strategy.Symbol, cfg.Strategy.ShortWindow, cfg.Strategy.LongWindow)
}

func ensureStrategyRegistrySeed(cfg config) error {
	if strings.TrimSpace(cfg.DB.Path) == "" {
		return nil
	}
	versionName := currentStrategyVersionName(cfg)
	return ensureStrategyVersion(cfg.DB.Path, "a_share", versionName, "active", "", cfg)
}

func ensureStrategyVersion(dbPath string, market string, versionName string, status string, parentVersion string, cfg config) error {
	query := fmt.Sprintf("SELECT COUNT(*) FROM strategy_registry WHERE version_name = %s;", quoteSQL(versionName))
	output, err := runSQLiteQuery(dbPath, query)
	if err != nil {
		return err
	}
	if strings.TrimSpace(output) != "" && strings.TrimSpace(output) != "0" {
		return nil
	}
	configJSON, err := json.Marshal(map[string]any{
		"strategy":  cfg.Strategy,
		"risk":      cfg.Risk,
		"portfolio": cfg.Portfolio,
		"regime":    cfg.Regime,
		"market":    cfg.Market,
	})
	if err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339)
	activatedAt := ""
	if status == "active" {
		activatedAt = now
	}
	insert := fmt.Sprintf(
		"INSERT INTO strategy_registry (market, version_name, status, parent_version, git_commit, config_json, model_path, created_at, activated_at, archived_at, notes) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s, %s);",
		quoteSQL(market),
		quoteSQL(versionName),
		quoteSQL(status),
		quoteSQL(parentVersion),
		quoteSQL(currentGitCommit()),
		quoteSQL(string(configJSON)),
		quoteSQL(cfg.Model.ModelPath),
		quoteSQL(now),
		quoteSQL(activatedAt),
		quoteSQL(""),
		quoteSQL("seeded from runtime configuration"),
	)
	return execSQLite(dbPath, insert)
}

func loadStrategyVersionConfig(dbPath string, versionName string, fallback config) (config, error) {
	if strings.TrimSpace(dbPath) == "" || strings.TrimSpace(versionName) == "" {
		return fallback, nil
	}
	query := fmt.Sprintf("SELECT config_json FROM strategy_registry WHERE version_name = %s ORDER BY id DESC LIMIT 1;", quoteSQL(versionName))
	output, err := runSQLiteQuery(dbPath, query)
	if err != nil {
		return fallback, err
	}
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return fallback, nil
	}
	cfg := fallback
	if err := json.Unmarshal([]byte(trimmed), &cfg); err != nil {
		return fallback, err
	}
	return cfg, nil
}

func resolveActiveStrategyConfig(base config, market string) (config, string, error) {
	if strings.TrimSpace(base.DB.Path) == "" {
		return base, currentStrategyVersionName(base), nil
	}
	query := fmt.Sprintf("SELECT version_name FROM strategy_registry WHERE market = %s AND status = 'active' ORDER BY id DESC LIMIT 1;", quoteSQL(market))
	output, err := runSQLiteQuery(base.DB.Path, query)
	if err != nil {
		return base, currentStrategyVersionName(base), err
	}
	versionName := strings.TrimSpace(output)
	if versionName == "" {
		versionName = currentStrategyVersionName(base)
		return base, versionName, nil
	}
	cfg, err := loadStrategyVersionConfig(base.DB.Path, versionName, base)
	if err != nil {
		return base, versionName, err
	}
	return cfg, versionName, nil
}

func persistRunRecord(runType string, summary map[string]any, files []string) error {
	indexPath := runtimeConfig.Report.RunIndexPath
	if strings.TrimSpace(indexPath) == "" {
		indexPath = filepath.Join(reportsDir, "run_index.jsonl")
	}
	historyDir, archivedFiles, err := archiveFiles(runType, files)
	if err != nil {
		return err
	}
	record := map[string]any{
		"run_type":     runType,
		"generated_at": time.Now().Format(time.RFC3339),
		"git_commit":   currentGitCommit(),
		"history_dir":  historyDir,
		"files":        archivedFiles,
		"summary":      summary,
	}
	if err := appendJSONLine(indexPath, record); err != nil {
		return err
	}
	if runtimeConfig.DB.Path != "" {
		content, _ := json.Marshal(summary)
		sql := fmt.Sprintf(
			"INSERT INTO run_history (run_type, git_commit, generated_at, history_dir, summary_json) VALUES (%s, %s, %s, %s, %s);",
			quoteSQL(runType),
			quoteSQL(currentGitCommit()),
			quoteSQL(time.Now().Format(time.RFC3339)),
			quoteSQL(historyDir),
			quoteSQL(string(content)),
		)
		_ = execSQLite(runtimeConfig.DB.Path, sql)
	}
	return nil
}

func appendExperimentRecord(experimentType string, configValues map[string]any, metrics map[string]any) error {
	basePath := runtimeConfig.Report.ExperimentLedger
	if strings.TrimSpace(basePath) == "" {
		basePath = filepath.Join(reportsDir, "experiments")
	}
	jsonlPath := basePath + ".jsonl"
	csvPath := basePath + ".csv"
	record := map[string]any{
		"experiment_type": experimentType,
		"recorded_at":     time.Now().Format(time.RFC3339),
		"git_commit":      currentGitCommit(),
		"config":          configValues,
		"metrics":         metrics,
	}
	configJSON, _ := json.Marshal(configValues)
	metricsJSON, _ := json.Marshal(metrics)
	if err := appendJSONLine(jsonlPath, record); err != nil {
		return err
	}
	if runtimeConfig.DB.Path != "" {
		sql := fmt.Sprintf(
			"INSERT INTO experiment_history (experiment_type, git_commit, recorded_at, config_json, metrics_json) VALUES (%s, %s, %s, %s, %s);",
			quoteSQL(experimentType),
			quoteSQL(currentGitCommit()),
			quoteSQL(time.Now().Format(time.RFC3339)),
			quoteSQL(string(configJSON)),
			quoteSQL(string(metricsJSON)),
		)
		_ = execSQLite(runtimeConfig.DB.Path, sql)
	}

	header := []string{"recorded_at", "experiment_type", "git_commit", "config_json", "metrics_json"}
	if _, err := os.Stat(csvPath); errors.Is(err, os.ErrNotExist) {
		if err := writeCSVFile(csvPath, [][]string{header, {time.Now().Format(time.RFC3339), experimentType, currentGitCommit(), string(configJSON), string(metricsJSON)}}); err != nil {
			return err
		}
		return nil
	}
	file, err := os.OpenFile(csvPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	if err := writer.Write([]string{time.Now().Format(time.RFC3339), experimentType, currentGitCommit(), string(configJSON), string(metricsJSON)}); err != nil {
		return err
	}
	writer.Flush()
	return writer.Error()
}

func noteProviderFailure(provider string, reason string) {
	if diagnosticsState.ProviderFailures == nil {
		diagnosticsState.ProviderFailures = map[string]int{}
	}
	diagnosticsState.ProviderFailures[provider]++
	if reason != "" {
		diagnosticsState.FallbackReasons = append(diagnosticsState.FallbackReasons, fmt.Sprintf("%s: %s", provider, reason))
	}
	diagnosticsState.LastUpdated = time.Now().Format(time.RFC3339)
}

func noteCacheHit() {
	diagnosticsState.CacheHits++
	diagnosticsState.LastUpdated = time.Now().Format(time.RFC3339)
}

func noteCacheMiss() {
	diagnosticsState.CacheMisses++
	diagnosticsState.LastUpdated = time.Now().Format(time.RFC3339)
}

func noteCacheStaleLoad() {
	diagnosticsState.CacheStaleLoads++
	diagnosticsState.LastUpdated = time.Now().Format(time.RFC3339)
}

func writeDiagnosticsReports() error {
	textPath := filepath.Join(reportsDir, "diagnostics.txt")
	jsonPath := filepath.Join(reportsDir, "diagnostics.json")
	var builder strings.Builder
	builder.WriteString("Diagnostics\n\n")
	fmt.Fprintf(&builder, "Cache hits: %d\n", diagnosticsState.CacheHits)
	fmt.Fprintf(&builder, "Cache misses: %d\n", diagnosticsState.CacheMisses)
	fmt.Fprintf(&builder, "Stale cache loads: %d\n", diagnosticsState.CacheStaleLoads)
	if len(diagnosticsState.ProviderFailures) == 0 {
		builder.WriteString("Provider failures: none\n")
	} else {
		builder.WriteString("Provider failures:\n")
		keys := make([]string, 0, len(diagnosticsState.ProviderFailures))
		for key := range diagnosticsState.ProviderFailures {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&builder, "- %s: %d\n", key, diagnosticsState.ProviderFailures[key])
		}
	}
	if len(diagnosticsState.FallbackReasons) > 0 {
		builder.WriteString("Fallback reasons:\n")
		limit := min(10, len(diagnosticsState.FallbackReasons))
		for _, item := range diagnosticsState.FallbackReasons[len(diagnosticsState.FallbackReasons)-limit:] {
			fmt.Fprintf(&builder, "- %s\n", item)
		}
	}
	if err := os.WriteFile(textPath, []byte(builder.String()), 0o644); err != nil {
		return err
	}
	return writeJSONFile(jsonPath, diagnosticsState)
}

func cleanupOldArtifacts() error {
	keepDays := runtimeConfig.Report.CleanupKeepDays
	if keepDays <= 0 {
		return nil
	}
	cutoff := time.Now().AddDate(0, 0, -keepDays)
	for _, root := range []string{reportHistoryRoot(), filepath.Join(reportsDir, "model_versions")} {
		entries, err := os.ReadDir(root)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				_ = os.RemoveAll(filepath.Join(root, entry.Name()))
			}
		}
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

func paperSessionForMarket(market string, now time.Time) string {
	if market != "a_share" {
		return "closed"
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	current := now.In(loc)
	switch current.Weekday() {
	case time.Saturday, time.Sunday:
		return "closed"
	}
	minutes := current.Hour()*60 + current.Minute()
	morning := minutes >= 9*60+30 && minutes < 11*60+30
	afternoon := minutes >= 13*60 && minutes < 15*60
	if morning || afternoon {
		return "open"
	}
	return "closed"
}

func buildTodayConclusionCard() string {
	focus, _ := readDashboardSection(filepath.Join(reportsDir, "a_share_focus.txt"))
	portfolio, _ := readDashboardSection(filepath.Join(reportsDir, "portfolio_backtest.txt"))
	firstFocus := firstMatchingLine(focus, []string{"1."})
	regimeLine := firstMatchingLine(portfolio, []string{"Regime:", "Target exposure:", "Total return:"})
	if firstFocus == "" && regimeLine == "" {
		return "今天还没有完整日报，先运行 scan 和 portfolio backtest。"
	}
	return strings.TrimSpace(firstFocus + " " + regimeLine)
}

func buildRiskAlertCard() string {
	portfolio, _ := readDashboardSection(filepath.Join(reportsDir, "portfolio_backtest.txt"))
	diagnostics, _ := readDashboardSection(filepath.Join(reportsDir, "diagnostics.txt"))
	alerts := make([]string, 0, 3)
	for _, prefix := range []string{"Regime:", "Excess return:", "Max drawdown:"} {
		if line := firstMatchingLine(portfolio, []string{prefix}); line != "" {
			alerts = append(alerts, line)
		}
	}
	if line := firstMatchingLine(diagnostics, []string{"Provider failures:", "-"}); line != "" {
		alerts = append(alerts, "Diagnostics: "+line)
	}
	if len(alerts) == 0 {
		return "当前没有明显的系统级风险告警。"
	}
	return strings.Join(alerts, " | ")
}

func buildChangeCard() string {
	current, _ := readDashboardSection(filepath.Join(reportsDir, "a_share_focus.txt"))
	previousPath := latestHistoricalFileBeforeToday("a_share_scan", "a_share_focus.txt")
	if previousPath == "" {
		return "还没有昨日快照，今天起会自动归档。"
	}
	previousBytes, err := os.ReadFile(previousPath)
	if err != nil {
		return "昨日快照读取失败。"
	}
	currentSymbols := extractLeadingSymbols(current)
	previousSymbols := extractLeadingSymbols(string(previousBytes))
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

func buildStrengthCard() string {
	scan, _ := readDashboardSection(filepath.Join(reportsDir, "a_share_scan.txt"))
	symbols := extractLeadingSymbols(scan)
	if len(symbols) == 0 {
		return "还没有扫描结果。"
	}
	strong := symbols[0]
	weak := symbols[len(symbols)-1]
	return fmt.Sprintf("最强候选: %s | 最弱候选: %s", strong, weak)
}

func buildHoldingCard() string {
	portfolio, _ := readDashboardSection(filepath.Join(reportsDir, "portfolio_backtest.txt"))
	if line := firstMatchingLine(portfolio, []string{"Current holdings:"}); line != "" {
		return strings.TrimPrefix(line, "Current holdings: ")
	}
	return "当前没有组合持仓快照。"
}

func buildStrategyEvolutionCard() string {
	if strings.TrimSpace(runtimeConfig.DB.Path) == "" {
		return "策略演进数据库未启用。"
	}
	query := `SELECT strategy_version, mode, market_date, equity, order_count FROM paper_daily_metrics WHERE mode = 'live' OR mode LIKE 'shadow:%' ORDER BY id DESC LIMIT 20;`
	output, err := runSQLiteQuery(runtimeConfig.DB.Path, query)
	if err != nil || strings.TrimSpace(output) == "" {
		return "还没有 active / shadow 对比数据。"
	}
	type metric struct {
		Version string
		Mode    string
		Date    string
		Equity  float64
		Orders  int
	}
	metrics := make([]metric, 0)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		parts := strings.Split(line, "|")
		if len(parts) != 5 {
			continue
		}
		equity, _ := strconv.ParseFloat(parts[3], 64)
		orders, _ := strconv.Atoi(parts[4])
		metrics = append(metrics, metric{Version: parts[0], Mode: parts[1], Date: parts[2], Equity: equity, Orders: orders})
	}
	var active *metric
	var shadow *metric
	for _, item := range metrics {
		if active == nil && item.Mode == "live" {
			copy := item
			active = &copy
		}
		if shadow == nil && strings.HasPrefix(item.Mode, "shadow:") {
			copy := item
			shadow = &copy
		}
	}
	if active == nil && shadow == nil {
		return "还没有 active / shadow 对比数据。"
	}
	if active != nil && shadow == nil {
		return fmt.Sprintf("当前 active=%s equity=%.2f，尚无 shadow 账户。", active.Version, active.Equity)
	}
	if active == nil && shadow != nil {
		return fmt.Sprintf("当前 shadow=%s equity=%.2f，尚无 active 对比。", shadow.Version, shadow.Equity)
	}
	diff := shadow.Equity - active.Equity
	return fmt.Sprintf("active=%s equity=%.2f | shadow=%s equity=%.2f | diff=%.2f | market_date=%s", active.Version, active.Equity, shadow.Version, shadow.Equity, diff, active.Date)
}

func buildLifecycleSummaryCard() string {
	if strings.TrimSpace(runtimeConfig.DB.Path) == "" {
		return "生命周期数据库未启用。"
	}
	activeOutput, _ := runSQLiteQuery(runtimeConfig.DB.Path, "SELECT version_name FROM strategy_registry WHERE status = 'active' ORDER BY id DESC LIMIT 1;")
	activeVersion := strings.TrimSpace(activeOutput)
	countOutput, _ := runSQLiteQuery(runtimeConfig.DB.Path, "SELECT COUNT(*) FROM strategy_registry;")
	eventOutput, _ := runSQLiteQuery(runtimeConfig.DB.Path, "SELECT event_type, from_version, to_version FROM strategy_promotions ORDER BY id DESC LIMIT 1;")
	if activeVersion == "" {
		activeVersion = "none"
	}
	versionCount := strings.TrimSpace(countOutput)
	lastEvent := "no events"
	if strings.TrimSpace(eventOutput) != "" {
		parts := strings.Split(strings.TrimSpace(eventOutput), "|")
		if len(parts) == 3 {
			lastEvent = fmt.Sprintf("%s %s -> %s", parts[0], parts[1], parts[2])
		}
	}
	return fmt.Sprintf("active=%s | versions=%s | last_event=%s", activeVersion, versionCount, lastEvent)
}

func buildHealthSummaryCard() string {
	health, _ := readDashboardSection(filepath.Join(reportsDir, "health_monitor.txt"))
	statusLine := firstMatchingLine(health, []string{"Status:", "Active strategy:", "Latest run:"})
	if statusLine == "" {
		return "系统健康报告尚未生成。"
	}
	providerLine := firstMatchingLine(health, []string{"Provider failures:", "Shadow equity:", "Active equity:"})
	if providerLine == "" || providerLine == statusLine {
		return statusLine
	}
	return statusLine + " | " + providerLine
}

func buildFactorSummaryCard() string {
	factors, _ := readDashboardSection(filepath.Join(reportsDir, "factor_research.txt"))
	rowLine := firstMatchingLine(factors, []string{"Rows:", "Features:", "Top factor correlations:"})
	if rowLine == "" {
		return "因子研究报告尚未生成。"
	}
	bestLine := firstMatchingLine(factors, []string{"- "})
	if bestLine == "" {
		return rowLine
	}
	return rowLine + " | " + bestLine
}

func latestHistoricalFileBeforeToday(runType string, fileName string) string {
	root := reportHistoryRoot()
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	today := time.Now().Format("2006-01-02")
	dates := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() < today {
			dates = append(dates, entry.Name())
		}
	}
	sort.Strings(dates)
	for i := len(dates) - 1; i >= 0; i-- {
		path := filepath.Join(root, dates[i], runType, fileName)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func extractLeadingSymbols(content string) []string {
	lines := strings.Split(content, "\n")
	symbols := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "1.") && !strings.HasPrefix(line, "2.") && !strings.HasPrefix(line, "3.") && !strings.HasPrefix(line, "4.") && !strings.HasPrefix(line, "5.") && !strings.HasPrefix(line, "6.") && !strings.HasPrefix(line, "7.") && !strings.HasPrefix(line, "8.") && !strings.HasPrefix(line, "9.") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			symbols = append(symbols, fields[1])
		}
	}
	return symbols
}

func diffStrings(left []string, right []string) []string {
	rightSet := map[string]struct{}{}
	for _, item := range right {
		rightSet[item] = struct{}{}
	}
	var diff []string
	for _, item := range left {
		if _, ok := rightSet[item]; !ok {
			diff = append(diff, item)
		}
	}
	return diff
}

func firstMatchingLine(content string, prefixes []string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, prefix := range prefixes {
			if strings.HasPrefix(trimmed, prefix) {
				return trimmed
			}
		}
	}
	return ""
}

func buildHistoryCompareReport() (map[string]any, string, string) {
	lines := make([]string, 0)
	rows := make([]string, 0)
	payload := []map[string]any{}
	runIndexPath := runtimeConfig.Report.RunIndexPath
	content, err := os.ReadFile(runIndexPath)
	if err != nil {
		text := "History Compare\n\nNo run history yet.\n"
		html := `<!doctype html><html><body><pre>No run history yet.</pre></body></html>`
		return map[string]any{"entries": payload}, text, html
	}
	type historyEntry struct {
		RunType string         `json:"run_type"`
		When    string         `json:"generated_at"`
		Summary map[string]any `json:"summary"`
	}
	entries := make([]historyEntry, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry historyEntry
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].When > entries[j].When })
	limit := min(12, len(entries))
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

func buildMarketOverviewReport() (map[string]any, string, string) {
	usPlan, _ := readDashboardSection(filepath.Join(reportsDir, "latest_plan.txt"))
	aShareFocus, _ := readDashboardSection(filepath.Join(reportsDir, "a_share_focus.txt"))
	payload := map[string]any{
		"a_share_focus": aShareFocus,
		"us_plan":       usPlan,
	}
	text := "Market Overview\n\nUS / Single-symbol Plan\n" + usPlan + "\n\nA-Share Focus\n" + aShareFocus + "\n"
	htmlContent := fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><title>Market Overview</title><style>body{font-family:Georgia,serif;background:#f4efe6;color:#1f1b16}.wrap{max-width:1100px;margin:36px auto;padding:0 20px}.grid{display:grid;grid-template-columns:1fr 1fr;gap:18px}.card{background:#fffaf3;border:1px solid #d9cfbf;border-radius:18px;padding:24px}pre{white-space:pre-wrap}</style></head><body><div class="wrap"><h1>Market Overview</h1><div class="grid"><div class="card"><h2>US / Single-symbol</h2><pre>%s</pre></div><div class="card"><h2>A-Share Focus</h2><pre>%s</pre></div></div></div></body></html>`, html.EscapeString(usPlan), html.EscapeString(aShareFocus))
	return payload, text, htmlContent
}

func buildStrategyLifecycleReport() (map[string]any, string, string) {
	if strings.TrimSpace(runtimeConfig.DB.Path) == "" {
		payload := map[string]any{"rows": []map[string]any{}, "events": []map[string]any{}}
		return payload, "Strategy Lifecycle\n\nDatabase disabled.\n", "<html><body><pre>Database disabled.</pre></body></html>"
	}

	registryOutput, _ := runSQLiteQuery(runtimeConfig.DB.Path, "SELECT version_name, status, parent_version, activated_at, archived_at FROM strategy_registry ORDER BY id DESC LIMIT 20;")
	eventOutput, _ := runSQLiteQuery(runtimeConfig.DB.Path, "SELECT event_type, from_version, to_version, trigger_reason, recorded_at FROM strategy_promotions ORDER BY id DESC LIMIT 20;")

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

	rows := make([]lifecycleRow, 0)
	if strings.TrimSpace(registryOutput) != "" {
		for _, line := range strings.Split(strings.TrimSpace(registryOutput), "\n") {
			parts := strings.Split(line, "|")
			if len(parts) != 5 {
				continue
			}
			rows = append(rows, lifecycleRow{
				Version: parts[0], Status: parts[1], ParentVersion: parts[2], ActivatedAt: parts[3], ArchivedAt: parts[4],
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
				EventType: parts[0], From: parts[1], To: parts[2], Reason: parts[3], Recorded: parts[4],
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
	configDir := filepath.Dir(path)
	paths := []string{
		path,
		filepath.Join(configDir, "data.yaml"),
		filepath.Join(configDir, "portfolio.yaml"),
		filepath.Join(configDir, "model.yaml"),
		filepath.Join(configDir, "market.yaml"),
		filepath.Join(configDir, "report.yaml"),
		filepath.Join(configDir, "local.yaml"),
	}
	var merged strings.Builder
	for _, candidatePath := range paths {
		content, err := os.ReadFile(candidatePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return config{}, err
		}
		merged.Write(content)
		merged.WriteString("\n")
	}

	var cfg = config{
		Market: marketRuleConfig{
			AShareT1:               true,
			MainBoardLimit:         0.10,
			ChiNextLimit:           0.20,
			STARLimit:              0.20,
			RiskWarningLimit:       0.05,
			StampDutySellBps:       5.0,
			TransferFeeBps:         0.1,
			HandlingFeeBps:         0.341,
			CommissionBps:          3.0,
			IPOUncappedTradingDays: 5,
		},
	}
	var section string

	for lineNumber, raw := range strings.Split(merged.String(), "\n") {
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
		case "model":
			switch key {
			case "default_label":
				cfg.Model.DefaultLabel = value
			case "model_path":
				cfg.Model.ModelPath = value
			case "benchmark_model_path":
				cfg.Model.BenchmarkModelPath = value
			case "promotion_metric":
				cfg.Model.PromotionMetric = value
			case "min_promotion_edge":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid model.min_promotion_edge: %w", convErr)
				}
				cfg.Model.MinPromotionEdge = v
			case "min_shadow_observations":
				v, convErr := strconv.Atoi(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid model.min_shadow_observations: %w", convErr)
				}
				cfg.Model.MinShadowObservations = v
			case "shadow_version":
				cfg.Model.ShadowVersion = value
			}
		case "report":
			switch key {
			case "history_root":
				cfg.Report.HistoryRoot = value
			case "export_json":
				v, convErr := strconv.ParseBool(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid report.export_json: %w", convErr)
				}
				cfg.Report.ExportJSON = v
			case "cleanup_keep_days":
				v, convErr := strconv.Atoi(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid report.cleanup_keep_days: %w", convErr)
				}
				cfg.Report.CleanupKeepDays = v
			case "experiment_ledger":
				cfg.Report.ExperimentLedger = value
			case "run_index_path":
				cfg.Report.RunIndexPath = value
			}
		case "market":
			switch key {
			case "a_share_t1":
				v, convErr := strconv.ParseBool(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid market.a_share_t1: %w", convErr)
				}
				cfg.Market.AShareT1 = v
			case "main_board_limit":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid market.main_board_limit: %w", convErr)
				}
				cfg.Market.MainBoardLimit = v
			case "chinext_limit":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid market.chinext_limit: %w", convErr)
				}
				cfg.Market.ChiNextLimit = v
			case "star_limit":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid market.star_limit: %w", convErr)
				}
				cfg.Market.STARLimit = v
			case "risk_warning_limit":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid market.risk_warning_limit: %w", convErr)
				}
				cfg.Market.RiskWarningLimit = v
			case "stamp_duty_sell_bps":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid market.stamp_duty_sell_bps: %w", convErr)
				}
				cfg.Market.StampDutySellBps = v
			case "transfer_fee_bps":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid market.transfer_fee_bps: %w", convErr)
				}
				cfg.Market.TransferFeeBps = v
			case "handling_fee_bps":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid market.handling_fee_bps: %w", convErr)
				}
				cfg.Market.HandlingFeeBps = v
			case "commission_bps":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid market.commission_bps: %w", convErr)
				}
				cfg.Market.CommissionBps = v
			case "ipo_uncapped_trading_days":
				v, convErr := strconv.Atoi(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid market.ipo_uncapped_trading_days: %w", convErr)
				}
				cfg.Market.IPOUncappedTradingDays = v
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
			case "industry_max_positions":
				v, convErr := strconv.Atoi(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.industry_max_positions: %w", convErr)
				}
				cfg.Portfolio.IndustryMaxPositions = v
			case "volatility_target":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.volatility_target: %w", convErr)
				}
				cfg.Portfolio.VolatilityTarget = v
			case "capacity_turnover_share":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.capacity_turnover_share: %w", convErr)
				}
				cfg.Portfolio.CapacityTurnoverShare = v
			case "gap_open_threshold":
				v, convErr := strconv.ParseFloat(value, 64)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.gap_open_threshold: %w", convErr)
				}
				cfg.Portfolio.GapOpenThreshold = v
			case "reserve_candidates":
				v, convErr := strconv.Atoi(value)
				if convErr != nil {
					return config{}, fmt.Errorf("invalid portfolio.reserve_candidates: %w", convErr)
				}
				cfg.Portfolio.ReserveCandidates = v
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
	if cfg.Model.DefaultLabel == "" {
		cfg.Model.DefaultLabel = "label_10d"
	}
	if cfg.Model.ModelPath == "" {
		cfg.Model.ModelPath = filepath.Join(reportsDir, "linear_model.json")
	}
	if cfg.Model.BenchmarkModelPath == "" {
		cfg.Model.BenchmarkModelPath = filepath.Join(reportsDir, "benchmark_classifier.json")
	}
	if cfg.Model.PromotionMetric == "" {
		cfg.Model.PromotionMetric = "rolling_directional_accuracy"
	}
	if cfg.Model.MinShadowObservations <= 0 {
		cfg.Model.MinShadowObservations = 1
	}
	if strings.TrimSpace(cfg.Model.ShadowVersion) == "" {
		cfg.Model.ShadowVersion = "candidate_auto_v1"
	}
	if cfg.Market.MainBoardLimit <= 0 {
		cfg.Market.MainBoardLimit = 0.10
	}
	if cfg.Market.ChiNextLimit <= 0 {
		cfg.Market.ChiNextLimit = 0.20
	}
	if cfg.Market.STARLimit <= 0 {
		cfg.Market.STARLimit = 0.20
	}
	if cfg.Market.RiskWarningLimit <= 0 {
		cfg.Market.RiskWarningLimit = 0.05
	}
	if cfg.Market.StampDutySellBps <= 0 {
		cfg.Market.StampDutySellBps = 5.0
	}
	if cfg.Market.TransferFeeBps <= 0 {
		cfg.Market.TransferFeeBps = 0.1
	}
	if cfg.Market.HandlingFeeBps <= 0 {
		cfg.Market.HandlingFeeBps = 0.341
	}
	if cfg.Market.CommissionBps <= 0 {
		cfg.Market.CommissionBps = 3.0
	}
	if cfg.Market.IPOUncappedTradingDays <= 0 {
		cfg.Market.IPOUncappedTradingDays = 5
	}
	if cfg.Report.HistoryRoot == "" {
		cfg.Report.HistoryRoot = filepath.Join(reportsDir, "history")
	}
	if cfg.Report.ExperimentLedger == "" {
		cfg.Report.ExperimentLedger = filepath.Join(reportsDir, "experiments")
	}
	if cfg.Report.RunIndexPath == "" {
		cfg.Report.RunIndexPath = filepath.Join(reportsDir, "run_index.jsonl")
	}
	if cfg.Report.CleanupKeepDays <= 0 {
		cfg.Report.CleanupKeepDays = 45
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
	if cfg.Portfolio.IndustryMaxPositions <= 0 {
		cfg.Portfolio.IndustryMaxPositions = 1
	}
	if cfg.Portfolio.VolatilityTarget <= 0 {
		cfg.Portfolio.VolatilityTarget = 0.18
	}
	if cfg.Portfolio.CapacityTurnoverShare <= 0 {
		cfg.Portfolio.CapacityTurnoverShare = 0.08
	}
	if cfg.Portfolio.GapOpenThreshold <= 0 {
		cfg.Portfolio.GapOpenThreshold = 0.04
	}
	if cfg.Portfolio.ReserveCandidates <= 0 {
		cfg.Portfolio.ReserveCandidates = 2
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
			noteProviderFailure("a_share", err.Error())
			if allowCSVFallback && dataPath != "" {
				csvBars, csvErr := loadBarsFromCSV(dataPath)
				if csvErr == nil {
					noteProviderFailure("csv_fallback", err.Error())
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
		noteProviderFailure("alphavantage", err.Error())
		if allowCSVFallback && dataPath != "" {
			csvBars, csvErr := loadBarsFromCSV(dataPath)
			if csvErr == nil {
				noteProviderFailure("csv_fallback", err.Error())
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
		noteProviderFailure("tushare", err.Error())
		if allowCSVFallback && dataPath != "" {
			csvBars, csvErr := loadBarsFromCSV(dataPath)
			if csvErr == nil {
				noteProviderFailure("csv_fallback", err.Error())
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
		noteProviderFailure("baostock", err.Error())
		if allowCSVFallback && dataPath != "" {
			csvBars, csvErr := loadBarsFromCSV(dataPath)
			if csvErr == nil {
				noteProviderFailure("csv_fallback", err.Error())
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
		noteProviderFailure("alphavantage", err.Error())
		if allowCSVFallback && dataPath != "" {
			csvBars, csvErr := loadBarsFromCSV(dataPath)
			if csvErr == nil {
				noteProviderFailure("csv_fallback", err.Error())
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
		noteCacheHit()
		return bars, nil
	}
	noteCacheMiss()

	bars, err := fetch()
	if err == nil {
		if writeErr := writeBarsCache(path, bars); writeErr != nil {
			return bars, nil
		}
		return bars, nil
	}

	if bars, _, cacheErr := loadCachedBars(path, 365*24*time.Hour); cacheErr == nil && len(bars) > 0 {
		noteCacheStaleLoad()
		noteProviderFailure(provider, err.Error())
		return bars, nil
	}
	noteProviderFailure(provider, err.Error())
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
	jsonPath := reportJSONPath("latest_plan")

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
	payload := map[string]any{
		"mode":        signal.Mode,
		"market_date": signal.MarketDate,
		"signal":      signal.Action,
		"reason":      signal.Reason,
		"plan_date":   reportDate,
		"plan":        signal.Plan,
		"data_source": signal.DataSource,
	}
	if err := writeJSONFile(jsonPath, payload); err != nil {
		return err
	}
	if err := persistRunRecord("latest_plan", payload, []string{textPath, htmlPath, jsonPath}); err != nil {
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

func loadFundamentalSnapshots() map[string]fundamentalSnapshot {
	if fundamentalsLoaded {
		return cachedFundamentals
	}
	fundamentalsLoaded = true
	cachedFundamentals = map[string]fundamentalSnapshot{}

	file, err := os.Open(fundamentalsPath)
	if err != nil {
		return cachedFundamentals
	}
	defer file.Close()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil || len(rows) < 2 {
		return cachedFundamentals
	}
	index := map[string]int{}
	for i, header := range rows[0] {
		index[strings.TrimSpace(strings.ToLower(header))] = i
	}
	for _, row := range rows[1:] {
		symbol := csvValue(row, index, "symbol")
		if symbol == "" {
			continue
		}
		cachedFundamentals[symbol] = fundamentalSnapshot{
			Symbol:        symbol,
			ROE:           parseCSVFloat(csvValue(row, index, "roe")),
			ProfitGrowth:  parseCSVFloat(csvValue(row, index, "profit_growth")),
			CashflowRatio: parseCSVFloat(csvValue(row, index, "cashflow_ratio")),
			DebtRatio:     parseCSVFloat(csvValue(row, index, "debt_ratio")),
			PEPercentile:  parseCSVFloat(csvValue(row, index, "pe_percentile")),
			PBPercentile:  parseCSVFloat(csvValue(row, index, "pb_percentile")),
			PSPercentile:  parseCSVFloat(csvValue(row, index, "ps_percentile")),
			UpdatedAt:     csvValue(row, index, "updated_at"),
		}
	}
	return cachedFundamentals
}

func loadEventSnapshots() map[string]eventSnapshot {
	if eventsLoaded {
		return cachedEvents
	}
	eventsLoaded = true
	cachedEvents = map[string]eventSnapshot{}

	file, err := os.Open(eventsPath)
	if err != nil {
		return cachedEvents
	}
	defer file.Close()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil || len(rows) < 2 {
		return cachedEvents
	}
	index := map[string]int{}
	for i, header := range rows[0] {
		index[strings.TrimSpace(strings.ToLower(header))] = i
	}
	for _, row := range rows[1:] {
		symbol := csvValue(row, index, "symbol")
		if symbol == "" {
			continue
		}
		cachedEvents[symbol] = eventSnapshot{
			Symbol:       symbol,
			EarningsFlag: parseCSVFloat(csvValue(row, index, "earnings_flag")),
			BuybackFlag:  parseCSVFloat(csvValue(row, index, "buyback_flag")),
			UnlockFlag:   parseCSVFloat(csvValue(row, index, "unlock_flag")),
			InsiderFlag:  parseCSVFloat(csvValue(row, index, "insider_flag")),
			UpdatedAt:    csvValue(row, index, "updated_at"),
		}
	}
	return cachedEvents
}

func csvValue(row []string, index map[string]int, key string) string {
	col, ok := index[key]
	if !ok || col < 0 || col >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[col])
}

func parseCSVFloat(value string) float64 {
	if strings.TrimSpace(value) == "" {
		return 0
	}
	number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0
	}
	return number
}

func fundamentalOverlayScores(symbol string) (float64, float64, float64) {
	fundamentals := loadFundamentalSnapshots()
	events := loadEventSnapshots()

	fundamental := 0.0
	valuation := 0.0
	event := 0.0

	if snapshot, ok := fundamentals[symbol]; ok {
		fundamental += clampFloat(snapshot.ROE/20.0, -0.20, 0.20) * 0.40
		fundamental += clampFloat(snapshot.ProfitGrowth/30.0, -0.20, 0.20) * 0.30
		fundamental += clampFloat(snapshot.CashflowRatio-1.0, -0.20, 0.20) * 0.20
		fundamental -= clampFloat(snapshot.DebtRatio-0.50, -0.20, 0.20) * 0.20

		avgPercentile := 0.0
		count := 0.0
		for _, value := range []float64{snapshot.PEPercentile, snapshot.PBPercentile, snapshot.PSPercentile} {
			if value > 0 {
				avgPercentile += value
				count++
			}
		}
		if count > 0 {
			avgPercentile /= count
			valuation = clampFloat((0.50-avgPercentile)*0.40, -0.20, 0.20)
		}
	}

	if snapshot, ok := events[symbol]; ok {
		event += snapshot.EarningsFlag * 0.06
		event += snapshot.BuybackFlag * 0.05
		event += snapshot.InsiderFlag * 0.04
		event -= snapshot.UnlockFlag * 0.08
		event = clampFloat(event, -0.12, 0.12)
	}

	return clampFloat(fundamental, -0.20, 0.20), clampFloat(valuation, -0.20, 0.20), clampFloat(event, -0.12, 0.12)
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
	valueScore, lowVolScore, crowdingScore, qualityScore, riskScore, heatPenalty, reversalScore := candidateOverlayScores(bars, shortMA, longMA, avgVolume, shortReturnScore, mediumReturnScore, trendScore, liquidityScore, persistenceScore, breakoutScore, volumeTrendScore, riskPenalty)
	fundamentalScore, valuationScore, eventScore := fundamentalOverlayScores(symbol)
	action, strategyAlignment, strategyVotes, reason, trigger := evaluateStrategyEnsemble(bars, shortMA, longMA, avgVolume, shortReturnScore, mediumReturnScore, dataSource, sourceErr, portfolio)
	score = score*0.35 +
		qualityScore*portfolio.QualityWeight +
		riskScore*portfolio.RiskWeight +
		fundamentalScore*1.05 +
		valuationScore*0.90 +
		eventScore*0.80 +
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
		ValueScore:        valueScore,
		LowVolScore:       lowVolScore,
		CrowdingScore:     crowdingScore,
		FundamentalScore:  fundamentalScore,
		ValuationScore:    valuationScore,
		EventScore:        eventScore,
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
	candidate.BenchmarkModelScore = predictBenchmarkModel(candidate)
	if candidate.ModelScore != 0 {
		candidate.Score = candidate.Score*0.70 + candidate.ModelScore*0.30
	}
	if candidate.BenchmarkModelScore != 0 {
		candidate.Score += (candidate.BenchmarkModelScore - 0.50) * 0.20
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
	avgTurnover := averageTurnover(bars, longWindow)
	illiquidity := amihudIlliquidity(bars, min(10, len(bars)-1))

	trendScore := 0.0
	if longMA > 0 {
		trendScore = (shortMA - longMA) / longMA
	}

	liquidityScore := 0.0
	if avgTurnover > 0 {
		liquidityScore += math.Min(math.Log10(avgTurnover+1)/10.0, 0.04)
	}
	if illiquidity > 0 {
		liquidityScore -= clampFloat(illiquidity*1e8, 0, 0.03)
	}
	recentVolumeWindow := min(5, len(volumes))
	if recentVolumeWindow > 1 {
		recentVolatility := standardDeviation(volumes[len(volumes)-recentVolumeWindow:])
		recentAverage := average(volumes[len(volumes)-recentVolumeWindow:])
		if recentAverage > 0 {
			liquidityScore += clampFloat(0.8-recentVolatility/recentAverage, -0.3, 0.3) * 0.03
		}
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
	recentVolumeWindow = min(3, len(volumes))
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
	jsonPath := reportJSONPath("a_share_scan")
	focusJSONPath := reportJSONPath("a_share_focus")
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
	if err := writeJSONFile(jsonPath, map[string]any{
		"watch":   watch,
		"observe": observe,
		"avoid":   avoid,
	}); err != nil {
		return err
	}
	if err := writeJSONFile(focusJSONPath, watch); err != nil {
		return err
	}
	if err := persistRunRecord("a_share_scan", map[string]any{
		"watch_count":   len(watch),
		"observe_count": len(observe),
		"avoid_count":   len(avoid),
	}, []string{textPath, htmlPath, focusTextPath, focusHTMLPath, jsonPath, focusJSONPath}); err != nil {
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
		fmt.Fprintf(builder, "   Score Breakdown: quality %.4f | risk %.4f | heat_penalty %.4f | reversal %.4f | value %.4f | low_vol %.4f | crowding %.4f | trend %.4f | liquidity %.4f | structure %.4f | momentum %.4f | persistence %.4f | breakout %.4f | volume_trend %.4f | rotation %.4f | strategy %.4f | model %.4f | risk_penalty %.4f\n",
			candidate.QualityScore,
			candidate.RiskScore,
			candidate.HeatPenalty,
			candidate.ReversalScore,
			candidate.ValueScore,
			candidate.LowVolScore,
			candidate.CrowdingScore,
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

func ensurePaperAccount(dbPath string, market string, mode string, initialCash float64, strategyName string) (int, float64, string, string, string, string, error) {
	query := fmt.Sprintf("SELECT id, cash, last_market_date, status, updated_at, note FROM paper_accounts WHERE market = %s AND mode = %s ORDER BY id DESC LIMIT 1;",
		quoteSQL(market),
		quoteSQL(mode),
	)
	output, err := runSQLiteQuery(dbPath, query)
	if err != nil {
		return 0, 0, "", "", "", "", err
	}
	trimmed := strings.TrimSpace(output)
	if trimmed != "" {
		parts := strings.Split(trimmed, "|")
		if len(parts) == 6 {
			accountID, convErr := strconv.Atoi(parts[0])
			if convErr != nil {
				return 0, 0, "", "", "", "", convErr
			}
			cash, convErr := strconv.ParseFloat(parts[1], 64)
			if convErr != nil {
				return 0, 0, "", "", "", "", convErr
			}
			return accountID, cash, parts[2], parts[3], parts[4], parts[5], nil
		}
	}

	now := time.Now().Format(time.RFC3339)
	insert := fmt.Sprintf(
		"INSERT INTO paper_accounts (market, mode, status, active_strategy, cash, equity, last_market_date, created_at, updated_at, note) VALUES (%s, %s, %s, %s, %.6f, %.6f, %s, %s, %s, %s);",
		quoteSQL(market),
		quoteSQL(mode),
		quoteSQL("active"),
		quoteSQL(strategyName),
		initialCash,
		initialCash,
		quoteSQL(""),
		quoteSQL(now),
		quoteSQL(now),
		quoteSQL("paper account initialized"),
	)
	if err := execSQLite(dbPath, insert); err != nil {
		return 0, 0, "", "", "", "", err
	}
	return ensurePaperAccount(dbPath, market, mode, initialCash, strategyName)
}

func loadPaperPositions(dbPath string, accountID int) ([]paperPosition, error) {
	query := fmt.Sprintf("SELECT symbol, name, shares, entry_price, entry_date FROM paper_positions WHERE account_id = %d ORDER BY symbol;", accountID)
	output, err := runSQLiteQuery(dbPath, query)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil, nil
	}
	lines := strings.Split(trimmed, "\n")
	positions := make([]paperPosition, 0, len(lines))
	for _, line := range lines {
		parts := strings.Split(line, "|")
		if len(parts) != 5 {
			continue
		}
		shares, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, err
		}
		entryPrice, err := strconv.ParseFloat(parts[3], 64)
		if err != nil {
			return nil, err
		}
		positions = append(positions, paperPosition{
			Symbol:     parts[0],
			Name:       parts[1],
			Shares:     shares,
			EntryPrice: entryPrice,
			EntryDate:  parts[4],
		})
	}
	return positions, nil
}

func savePaperAccountState(dbPath string, accountID int, market string, mode string, strategyName string, marketDate string, status string, cash float64, equity float64, holdings []paperPosition, orders []paperOrder, fills []paperFill, note string, now time.Time) error {
	holdingsJSON, _ := json.Marshal(holdings)
	statements := []string{
		fmt.Sprintf(
			"UPDATE paper_accounts SET status = %s, active_strategy = %s, cash = %.6f, equity = %.6f, last_market_date = %s, updated_at = %s, note = %s WHERE id = %d;",
			quoteSQL(status),
			quoteSQL(strategyName),
			cash,
			equity,
			quoteSQL(marketDate),
			quoteSQL(now.Format(time.RFC3339)),
			quoteSQL(note),
			accountID,
		),
		fmt.Sprintf("DELETE FROM paper_positions WHERE account_id = %d;", accountID),
	}
	for _, holding := range holdings {
		statements = append(statements, fmt.Sprintf(
			"INSERT INTO paper_positions (account_id, symbol, name, shares, entry_price, entry_date, updated_at) VALUES (%d, %s, %s, %d, %.6f, %s, %s);",
			accountID,
			quoteSQL(holding.Symbol),
			quoteSQL(holding.Name),
			holding.Shares,
			holding.EntryPrice,
			quoteSQL(holding.EntryDate),
			quoteSQL(now.Format(time.RFC3339)),
		))
	}
	for _, order := range orders {
		statements = append(statements, fmt.Sprintf(
			"INSERT INTO paper_orders (account_id, symbol, name, side, order_type, quantity, order_price, status, placed_at, note) VALUES (%d, %s, %s, %s, %s, %d, %.6f, %s, %s, %s);",
			accountID,
			quoteSQL(order.Symbol),
			quoteSQL(order.Name),
			quoteSQL(order.Side),
			quoteSQL("market"),
			order.Quantity,
			order.Price,
			quoteSQL(order.Status),
			quoteSQL(order.PlacedAt),
			quoteSQL(order.Note),
		))
	}
	for _, fill := range fills {
		statements = append(statements, fmt.Sprintf(
			"INSERT INTO paper_fills (order_id, account_id, symbol, name, side, quantity, fill_price, fee, filled_at, note) VALUES (NULL, %d, %s, %s, %s, %d, %.6f, %.6f, %s, %s);",
			accountID,
			quoteSQL(fill.Symbol),
			quoteSQL(fill.Name),
			quoteSQL(fill.Side),
			fill.Quantity,
			fill.Price,
			fill.Fee,
			quoteSQL(fill.FilledAt),
			quoteSQL(fill.Note),
		))
	}
	statements = append(statements, fmt.Sprintf(
		"INSERT INTO paper_equity_curve (account_id, snapshot_time, market_date, market, equity, cash, holdings_json, note) VALUES (%d, %s, %s, %s, %.6f, %.6f, %s, %s);",
		accountID,
		quoteSQL(now.Format(time.RFC3339)),
		quoteSQL(marketDate),
		quoteSQL(market),
		equity,
		cash,
		quoteSQL(string(holdingsJSON)),
		quoteSQL(note),
	))
	statements = append(statements, fmt.Sprintf(
		"INSERT INTO paper_daily_metrics (account_id, strategy_version, mode, market, market_date, equity, cash, holding_count, order_count, fill_count, session, recorded_at, note) VALUES (%d, %s, %s, %s, %s, %.6f, %.6f, %d, %d, %d, %s, %s, %s);",
		accountID,
		quoteSQL(strategyName),
		quoteSQL(mode),
		quoteSQL(market),
		quoteSQL(marketDate),
		equity,
		cash,
		len(holdings),
		len(orders),
		len(fills),
		quoteSQL(paperSessionForMarket(market, now)),
		quoteSQL(now.Format(time.RFC3339)),
		quoteSQL(note),
	))
	return execSQLite(dbPath, strings.Join(statements, "\n"))
}

func printPaperTradingSummary(result paperAccountResult) {
	fmt.Printf("Paper Trading %s %s\n", result.Market, result.MarketDate)
	fmt.Printf("Session: %s\n", result.Session)
	fmt.Printf("Cash: %.2f\n", result.Cash)
	fmt.Printf("Equity: %.2f\n", result.Equity)
	fmt.Printf("Targets: %d\n", len(result.Targets))
	fmt.Printf("Holdings: %d\n", len(result.Holdings))
	fmt.Printf("Orders this cycle: %d\n", len(result.Orders))
	fmt.Printf("Note: %s\n\n", result.Note)
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
