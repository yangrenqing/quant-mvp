package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const reportsDir = "reports"

type Config struct {
	AppName   string           `yaml:"-" json:"app_name"`
	App       AppConfig        `yaml:"app" json:"app"`
	DB        DBConfig         `yaml:"db" json:"db"`
	Schedule  ScheduleConfig   `yaml:"schedule" json:"schedule"`
	Strategy  StrategyConfig   `yaml:"strategy" json:"strategy"`
	Risk      RiskConfig       `yaml:"risk" json:"risk"`
	Portfolio PortfolioConfig  `yaml:"portfolio" json:"portfolio"`
	Regime    RegimeConfig     `yaml:"regime" json:"regime"`
	Model     ModelConfig      `yaml:"model" json:"model"`
	Health    HealthConfig     `yaml:"health" json:"health"`
	Report    ReportConfig     `yaml:"report" json:"report"`
	Market    MarketRuleConfig `yaml:"market" json:"market"`
}

type AppConfig struct {
	Name string `yaml:"name" json:"name"`
}

type DBConfig struct {
	Path string `yaml:"path" json:"path"`
}

type ScheduleConfig struct {
	DailyRun string `yaml:"daily_run" json:"daily_run"`
	CacheTTL string `yaml:"cache_ttl" json:"cache_ttl"`
}

type ModelConfig struct {
	DefaultLabel             string  `yaml:"default_label" json:"default_label"`
	BenchmarkLabel           string  `yaml:"benchmark_label" json:"benchmark_label"`
	ModelPath                string  `yaml:"model_path" json:"model_path"`
	BenchmarkModelPath       string  `yaml:"benchmark_model_path" json:"benchmark_model_path"`
	PromotionMetric          string  `yaml:"promotion_metric" json:"promotion_metric"`
	BenchmarkPromotionMetric string  `yaml:"benchmark_promotion_metric" json:"benchmark_promotion_metric"`
	MinPromotionEdge         float64 `yaml:"min_promotion_edge" json:"min_promotion_edge"`
	BenchmarkMinImprovement  float64 `yaml:"benchmark_min_improvement" json:"benchmark_min_improvement"`
	MinShadowObservations    int     `yaml:"min_shadow_observations" json:"min_shadow_observations"`
	ShadowVersion            string  `yaml:"shadow_version" json:"shadow_version"`
}

type HealthConfig struct {
	MaxRunAgeHours            float64 `yaml:"max_run_age_hours" json:"max_run_age_hours"`
	ShadowEdgeAlert           float64 `yaml:"shadow_edge_alert" json:"shadow_edge_alert"`
	ProviderFailureAlertCount int     `yaml:"provider_failure_alert_count" json:"provider_failure_alert_count"`
	MinActiveEquityRatio      float64 `yaml:"min_active_equity_ratio" json:"min_active_equity_ratio"`
	NotifyOnCritical          bool    `yaml:"notify_on_critical" json:"notify_on_critical"`
}

type ReportConfig struct {
	HistoryRoot      string `yaml:"history_root" json:"history_root"`
	ExportJSON       bool   `yaml:"export_json" json:"export_json"`
	CleanupKeepDays  int    `yaml:"cleanup_keep_days" json:"cleanup_keep_days"`
	ExperimentLedger string `yaml:"experiment_ledger" json:"experiment_ledger"`
	RunIndexPath     string `yaml:"run_index_path" json:"run_index_path"`
}

type MarketRuleConfig struct {
	AShareT1               bool    `yaml:"a_share_t1" json:"a_share_t1"`
	MainBoardLimit         float64 `yaml:"main_board_limit" json:"main_board_limit"`
	ChiNextLimit           float64 `yaml:"chinext_limit" json:"chinext_limit"`
	STARLimit              float64 `yaml:"star_limit" json:"star_limit"`
	RiskWarningLimit       float64 `yaml:"risk_warning_limit" json:"risk_warning_limit"`
	StampDutySellBps       float64 `yaml:"stamp_duty_sell_bps" json:"stamp_duty_sell_bps"`
	TransferFeeBps         float64 `yaml:"transfer_fee_bps" json:"transfer_fee_bps"`
	HandlingFeeBps         float64 `yaml:"handling_fee_bps" json:"handling_fee_bps"`
	CommissionBps          float64 `yaml:"commission_bps" json:"commission_bps"`
	IPOUncappedTradingDays int     `yaml:"ipo_uncapped_trading_days" json:"ipo_uncapped_trading_days"`
}

type StrategyConfig struct {
	Name        string `yaml:"name" json:"name"`
	Symbol      string `yaml:"symbol" json:"symbol"`
	DataPath    string `yaml:"data_path" json:"data_path"`
	DataSource  string `yaml:"data_source" json:"data_source"`
	APIKeyEnv   string `yaml:"api_key_env" json:"api_key_env"`
	ShortWindow int    `yaml:"short_window" json:"short_window"`
	LongWindow  int    `yaml:"long_window" json:"long_window"`
}

type RiskConfig struct {
	MaxPosition      int     `yaml:"max_position" json:"max_position"`
	StopLossPct      float64 `yaml:"stop_loss_pct" json:"stop_loss_pct"`
	SkipRepeatSignal bool    `yaml:"skip_repeat_signal" json:"skip_repeat_signal"`
}

type PortfolioConfig struct {
	RebalanceIntervalDays  int     `yaml:"rebalance_interval_days" json:"rebalance_interval_days"`
	WeightDriftThreshold   float64 `yaml:"weight_drift_threshold" json:"weight_drift_threshold"`
	MinHoldings            int     `yaml:"min_holdings" json:"min_holdings"`
	MaxPositionWeight      float64 `yaml:"max_position_weight" json:"max_position_weight"`
	MaxCashShare           float64 `yaml:"max_cash_share" json:"max_cash_share"`
	MaxVolatility          float64 `yaml:"max_volatility" json:"max_volatility"`
	MinAverageTurnover     float64 `yaml:"min_average_turnover" json:"min_average_turnover"`
	OverheatThreshold      float64 `yaml:"overheat_threshold" json:"overheat_threshold"`
	MaxHoldingDrawdown     float64 `yaml:"max_holding_drawdown" json:"max_holding_drawdown"`
	MinTrendGap            float64 `yaml:"min_trend_gap" json:"min_trend_gap"`
	StopCooldownDays       int     `yaml:"stop_cooldown_days" json:"stop_cooldown_days"`
	ExitCooldownDays       int     `yaml:"exit_cooldown_days" json:"exit_cooldown_days"`
	TrendBreakCooldownDays int     `yaml:"trend_break_cooldown_days" json:"trend_break_cooldown_days"`
	MomentumWeight         float64 `yaml:"momentum_weight" json:"momentum_weight"`
	PersistenceWeight      float64 `yaml:"persistence_weight" json:"persistence_weight"`
	BacktestExcessWeight   float64 `yaml:"backtest_excess_weight" json:"backtest_excess_weight"`
	BacktestReturnWeight   float64 `yaml:"backtest_return_weight" json:"backtest_return_weight"`
	BacktestDrawdownWeight float64 `yaml:"backtest_drawdown_weight" json:"backtest_drawdown_weight"`
	WatchPenalty           float64 `yaml:"watch_penalty" json:"watch_penalty"`
	TrendStrategyEnabled   bool    `yaml:"trend_strategy_enabled" json:"trend_strategy_enabled"`
	BreakoutEnabled        bool    `yaml:"breakout_enabled" json:"breakout_enabled"`
	PullbackEnabled        bool    `yaml:"pullback_enabled" json:"pullback_enabled"`
	TrendStrategyWeight    float64 `yaml:"trend_strategy_weight" json:"trend_strategy_weight"`
	BreakoutStrategyWeight float64 `yaml:"breakout_strategy_weight" json:"breakout_strategy_weight"`
	PullbackStrategyWeight float64 `yaml:"pullback_strategy_weight" json:"pullback_strategy_weight"`
	MinPrice               float64 `yaml:"min_price" json:"min_price"`
	MinBacktestExcess      float64 `yaml:"min_backtest_excess" json:"min_backtest_excess"`
	MaxBacktestDrawdown    float64 `yaml:"max_backtest_drawdown" json:"max_backtest_drawdown"`
	LimitMoveThreshold     float64 `yaml:"limit_move_threshold" json:"limit_move_threshold"`
	QualityWeight          float64 `yaml:"quality_weight" json:"quality_weight"`
	RiskWeight             float64 `yaml:"risk_weight" json:"risk_weight"`
	HeatPenaltyWeight      float64 `yaml:"heat_penalty_weight" json:"heat_penalty_weight"`
	ReversalWeight         float64 `yaml:"reversal_weight" json:"reversal_weight"`
	IndustryMaxPositions   int     `yaml:"industry_max_positions" json:"industry_max_positions"`
	VolatilityTarget       float64 `yaml:"volatility_target" json:"volatility_target"`
	CapacityTurnoverShare  float64 `yaml:"capacity_turnover_share" json:"capacity_turnover_share"`
	GapOpenThreshold       float64 `yaml:"gap_open_threshold" json:"gap_open_threshold"`
	ReserveCandidates      int     `yaml:"reserve_candidates" json:"reserve_candidates"`
}

type RegimeConfig struct {
	CautiousExposure float64 `yaml:"cautious_exposure" json:"cautious_exposure"`
	RiskOffExposure  float64 `yaml:"risk_off_exposure" json:"risk_off_exposure"`
	RiskOffDrawdown  float64 `yaml:"risk_off_drawdown" json:"risk_off_drawdown"`
	CautiousDrawdown float64 `yaml:"cautious_drawdown" json:"cautious_drawdown"`
	BreadthRiskOff   float64 `yaml:"breadth_risk_off" json:"breadth_risk_off"`
	BreadthCautious  float64 `yaml:"breadth_cautious" json:"breadth_cautious"`
}

func Load(path string) (Config, error) {
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

	cfg := Config{
		Market: MarketRuleConfig{
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

	for _, candidatePath := range paths {
		content, err := os.ReadFile(candidatePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return Config{}, err
		}
		section := ""
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
				return Config{}, fmt.Errorf("parse %s line %d: invalid config line", candidatePath, lineNumber+1)
			}
			key := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")
			if err := applyValue(&cfg, section, key, value); err != nil {
				return Config{}, fmt.Errorf("parse %s line %d: %w", candidatePath, lineNumber+1, err)
			}
		}
	}

	cfg.AppName = cfg.App.Name
	applyDefaults(&cfg)
	return cfg, validate(cfg)
}

func applyValue(cfg *Config, section string, key string, value string) error {
	switch section {
	case "app":
		if key == "name" {
			cfg.AppName = value
			cfg.App.Name = value
		}
	case "db":
		if key == "path" {
			cfg.DB.Path = value
		}
	case "schedule":
		switch key {
		case "daily_run":
			cfg.Schedule.DailyRun = value
		case "cache_ttl":
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
			v, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid strategy.short_window: %w", err)
			}
			cfg.Strategy.ShortWindow = v
		case "long_window":
			v, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid strategy.long_window: %w", err)
			}
			cfg.Strategy.LongWindow = v
		}
	case "risk":
		switch key {
		case "max_position":
			v, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid risk.max_position: %w", err)
			}
			cfg.Risk.MaxPosition = v
		case "stop_loss_pct":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("invalid risk.stop_loss_pct: %w", err)
			}
			cfg.Risk.StopLossPct = v
		case "skip_repeat_signal":
			v, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid risk.skip_repeat_signal: %w", err)
			}
			cfg.Risk.SkipRepeatSignal = v
		}
	case "portfolio":
		return applyPortfolioValue(cfg, key, value)
	case "regime":
		switch key {
		case "cautious_exposure":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("invalid regime.cautious_exposure: %w", err)
			}
			cfg.Regime.CautiousExposure = v
		case "risk_off_exposure":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("invalid regime.risk_off_exposure: %w", err)
			}
			cfg.Regime.RiskOffExposure = v
		case "risk_off_drawdown":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("invalid regime.risk_off_drawdown: %w", err)
			}
			cfg.Regime.RiskOffDrawdown = v
		case "cautious_drawdown":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("invalid regime.cautious_drawdown: %w", err)
			}
			cfg.Regime.CautiousDrawdown = v
		case "breadth_risk_off":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("invalid regime.breadth_risk_off: %w", err)
			}
			cfg.Regime.BreadthRiskOff = v
		case "breadth_cautious":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("invalid regime.breadth_cautious: %w", err)
			}
			cfg.Regime.BreadthCautious = v
		}
	case "model":
		switch key {
		case "default_label":
			cfg.Model.DefaultLabel = value
		case "benchmark_label":
			cfg.Model.BenchmarkLabel = value
		case "model_path":
			cfg.Model.ModelPath = value
		case "benchmark_model_path":
			cfg.Model.BenchmarkModelPath = value
		case "promotion_metric":
			cfg.Model.PromotionMetric = value
		case "benchmark_promotion_metric":
			cfg.Model.BenchmarkPromotionMetric = value
		case "min_promotion_edge":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("invalid model.min_promotion_edge: %w", err)
			}
			cfg.Model.MinPromotionEdge = v
		case "benchmark_min_improvement":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("invalid model.benchmark_min_improvement: %w", err)
			}
			cfg.Model.BenchmarkMinImprovement = v
		case "min_shadow_observations":
			v, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid model.min_shadow_observations: %w", err)
			}
			cfg.Model.MinShadowObservations = v
		case "shadow_version":
			cfg.Model.ShadowVersion = value
		}
	case "health":
		switch key {
		case "max_run_age_hours":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("invalid health.max_run_age_hours: %w", err)
			}
			cfg.Health.MaxRunAgeHours = v
		case "shadow_edge_alert":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("invalid health.shadow_edge_alert: %w", err)
			}
			cfg.Health.ShadowEdgeAlert = v
		case "provider_failure_alert_count":
			v, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid health.provider_failure_alert_count: %w", err)
			}
			cfg.Health.ProviderFailureAlertCount = v
		case "min_active_equity_ratio":
			v, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("invalid health.min_active_equity_ratio: %w", err)
			}
			cfg.Health.MinActiveEquityRatio = v
		case "notify_on_critical":
			v, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid health.notify_on_critical: %w", err)
			}
			cfg.Health.NotifyOnCritical = v
		}
	case "report":
		switch key {
		case "history_root":
			cfg.Report.HistoryRoot = value
		case "export_json":
			v, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("invalid report.export_json: %w", err)
			}
			cfg.Report.ExportJSON = v
		case "cleanup_keep_days":
			v, err := strconv.Atoi(value)
			if err != nil {
				return fmt.Errorf("invalid report.cleanup_keep_days: %w", err)
			}
			cfg.Report.CleanupKeepDays = v
		case "experiment_ledger":
			cfg.Report.ExperimentLedger = value
		case "run_index_path":
			cfg.Report.RunIndexPath = value
		}
	case "market":
		return applyMarketValue(cfg, key, value)
	}
	return nil
}

func applyPortfolioValue(cfg *Config, key string, value string) error {
	switch key {
	case "rebalance_interval_days":
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid portfolio.rebalance_interval_days: %w", err)
		}
		cfg.Portfolio.RebalanceIntervalDays = v
	case "weight_drift_threshold":
		return assignFloat(value, "portfolio.weight_drift_threshold", &cfg.Portfolio.WeightDriftThreshold)
	case "min_holdings":
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid portfolio.min_holdings: %w", err)
		}
		cfg.Portfolio.MinHoldings = v
	case "max_position_weight":
		return assignFloat(value, "portfolio.max_position_weight", &cfg.Portfolio.MaxPositionWeight)
	case "max_cash_share":
		return assignFloat(value, "portfolio.max_cash_share", &cfg.Portfolio.MaxCashShare)
	case "max_volatility":
		return assignFloat(value, "portfolio.max_volatility", &cfg.Portfolio.MaxVolatility)
	case "min_average_turnover":
		return assignFloat(value, "portfolio.min_average_turnover", &cfg.Portfolio.MinAverageTurnover)
	case "overheat_threshold":
		return assignFloat(value, "portfolio.overheat_threshold", &cfg.Portfolio.OverheatThreshold)
	case "max_holding_drawdown":
		return assignFloat(value, "portfolio.max_holding_drawdown", &cfg.Portfolio.MaxHoldingDrawdown)
	case "min_trend_gap":
		return assignFloat(value, "portfolio.min_trend_gap", &cfg.Portfolio.MinTrendGap)
	case "stop_cooldown_days":
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid portfolio.stop_cooldown_days: %w", err)
		}
		cfg.Portfolio.StopCooldownDays = v
	case "exit_cooldown_days":
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid portfolio.exit_cooldown_days: %w", err)
		}
		cfg.Portfolio.ExitCooldownDays = v
	case "trend_break_cooldown_days":
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid portfolio.trend_break_cooldown_days: %w", err)
		}
		cfg.Portfolio.TrendBreakCooldownDays = v
	case "momentum_weight":
		return assignFloat(value, "portfolio.momentum_weight", &cfg.Portfolio.MomentumWeight)
	case "persistence_weight":
		return assignFloat(value, "portfolio.persistence_weight", &cfg.Portfolio.PersistenceWeight)
	case "backtest_excess_weight":
		return assignFloat(value, "portfolio.backtest_excess_weight", &cfg.Portfolio.BacktestExcessWeight)
	case "backtest_return_weight":
		return assignFloat(value, "portfolio.backtest_return_weight", &cfg.Portfolio.BacktestReturnWeight)
	case "backtest_drawdown_weight":
		return assignFloat(value, "portfolio.backtest_drawdown_weight", &cfg.Portfolio.BacktestDrawdownWeight)
	case "watch_penalty":
		return assignFloat(value, "portfolio.watch_penalty", &cfg.Portfolio.WatchPenalty)
	case "trend_strategy_enabled":
		return assignBool(value, "portfolio.trend_strategy_enabled", &cfg.Portfolio.TrendStrategyEnabled)
	case "breakout_enabled":
		return assignBool(value, "portfolio.breakout_enabled", &cfg.Portfolio.BreakoutEnabled)
	case "pullback_enabled":
		return assignBool(value, "portfolio.pullback_enabled", &cfg.Portfolio.PullbackEnabled)
	case "trend_strategy_weight":
		return assignFloat(value, "portfolio.trend_strategy_weight", &cfg.Portfolio.TrendStrategyWeight)
	case "breakout_strategy_weight":
		return assignFloat(value, "portfolio.breakout_strategy_weight", &cfg.Portfolio.BreakoutStrategyWeight)
	case "pullback_strategy_weight":
		return assignFloat(value, "portfolio.pullback_strategy_weight", &cfg.Portfolio.PullbackStrategyWeight)
	case "min_price":
		return assignFloat(value, "portfolio.min_price", &cfg.Portfolio.MinPrice)
	case "min_backtest_excess":
		return assignFloat(value, "portfolio.min_backtest_excess", &cfg.Portfolio.MinBacktestExcess)
	case "max_backtest_drawdown":
		return assignFloat(value, "portfolio.max_backtest_drawdown", &cfg.Portfolio.MaxBacktestDrawdown)
	case "limit_move_threshold":
		return assignFloat(value, "portfolio.limit_move_threshold", &cfg.Portfolio.LimitMoveThreshold)
	case "quality_weight":
		return assignFloat(value, "portfolio.quality_weight", &cfg.Portfolio.QualityWeight)
	case "risk_weight":
		return assignFloat(value, "portfolio.risk_weight", &cfg.Portfolio.RiskWeight)
	case "heat_penalty_weight":
		return assignFloat(value, "portfolio.heat_penalty_weight", &cfg.Portfolio.HeatPenaltyWeight)
	case "reversal_weight":
		return assignFloat(value, "portfolio.reversal_weight", &cfg.Portfolio.ReversalWeight)
	case "industry_max_positions":
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid portfolio.industry_max_positions: %w", err)
		}
		cfg.Portfolio.IndustryMaxPositions = v
	case "volatility_target":
		return assignFloat(value, "portfolio.volatility_target", &cfg.Portfolio.VolatilityTarget)
	case "capacity_turnover_share":
		return assignFloat(value, "portfolio.capacity_turnover_share", &cfg.Portfolio.CapacityTurnoverShare)
	case "gap_open_threshold":
		return assignFloat(value, "portfolio.gap_open_threshold", &cfg.Portfolio.GapOpenThreshold)
	case "reserve_candidates":
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid portfolio.reserve_candidates: %w", err)
		}
		cfg.Portfolio.ReserveCandidates = v
	}
	return nil
}

func applyMarketValue(cfg *Config, key string, value string) error {
	switch key {
	case "a_share_t1":
		return assignBool(value, "market.a_share_t1", &cfg.Market.AShareT1)
	case "main_board_limit":
		return assignFloat(value, "market.main_board_limit", &cfg.Market.MainBoardLimit)
	case "chinext_limit":
		return assignFloat(value, "market.chinext_limit", &cfg.Market.ChiNextLimit)
	case "star_limit":
		return assignFloat(value, "market.star_limit", &cfg.Market.STARLimit)
	case "risk_warning_limit":
		return assignFloat(value, "market.risk_warning_limit", &cfg.Market.RiskWarningLimit)
	case "stamp_duty_sell_bps":
		return assignFloat(value, "market.stamp_duty_sell_bps", &cfg.Market.StampDutySellBps)
	case "transfer_fee_bps":
		return assignFloat(value, "market.transfer_fee_bps", &cfg.Market.TransferFeeBps)
	case "handling_fee_bps":
		return assignFloat(value, "market.handling_fee_bps", &cfg.Market.HandlingFeeBps)
	case "commission_bps":
		return assignFloat(value, "market.commission_bps", &cfg.Market.CommissionBps)
	case "ipo_uncapped_trading_days":
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid market.ipo_uncapped_trading_days: %w", err)
		}
		cfg.Market.IPOUncappedTradingDays = v
	}
	return nil
}

func assignFloat(value string, name string, target *float64) error {
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("invalid %s: %w", name, err)
	}
	*target = v
	return nil
}

func assignBool(value string, name string, target *bool) error {
	v, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("invalid %s: %w", name, err)
	}
	*target = v
	return nil
}

func WriteRuntimeSnapshot(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(path, payload, 0o644)
}

func applyDefaults(cfg *Config) {
	if cfg.AppName == "" {
		cfg.AppName = "quant-mvp"
		cfg.App.Name = cfg.AppName
	}
	if cfg.DB.Path == "" {
		cfg.DB.Path = "data/quant.db"
	}
	if cfg.Schedule.CacheTTL == "" {
		cfg.Schedule.CacheTTL = "4h"
	}
	if cfg.Model.DefaultLabel == "" {
		cfg.Model.DefaultLabel = "label_10d"
	}
	if cfg.Model.BenchmarkLabel == "" {
		cfg.Model.BenchmarkLabel = "beat_benchmark_10d"
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
	if cfg.Model.BenchmarkPromotionMetric == "" {
		cfg.Model.BenchmarkPromotionMetric = "rolling_directional_accuracy"
	}
	if cfg.Model.MinShadowObservations <= 0 {
		cfg.Model.MinShadowObservations = 1
	}
	if cfg.Model.ShadowVersion == "" {
		cfg.Model.ShadowVersion = "candidate_auto_v1"
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
	if cfg.Health.MaxRunAgeHours <= 0 {
		cfg.Health.MaxRunAgeHours = 30
	}
	if cfg.Health.ShadowEdgeAlert <= 0 {
		cfg.Health.ShadowEdgeAlert = 0.01
	}
	if cfg.Health.ProviderFailureAlertCount <= 0 {
		cfg.Health.ProviderFailureAlertCount = 1
	}
	if cfg.Health.MinActiveEquityRatio <= 0 {
		cfg.Health.MinActiveEquityRatio = 0.97
	}
}

func validate(cfg Config) error {
	if cfg.Schedule.DailyRun == "" {
		return errors.New("schedule.daily_run is required")
	}
	if cfg.Strategy.ShortWindow >= cfg.Strategy.LongWindow {
		return errors.New("strategy.short_window must be smaller than strategy.long_window")
	}
	return nil
}
