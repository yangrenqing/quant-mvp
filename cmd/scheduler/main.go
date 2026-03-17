package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const configPath = "configs/config.yaml"

type config struct {
	AppName  string
	DB       dbConfig
	Schedule scheduleConfig
	Strategy strategyConfig
	Risk     riskConfig
}

type dbConfig struct {
	Path string
}

type scheduleConfig struct {
	DailyRun string
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
	ShortMA      float64
	LongMA       float64
	ClosePrice   float64
	OpenPrice    float64
	HighPrice    float64
	LowPrice     float64
	Volume       float64
	Reason       string
	PositionSize int
}

type positionState struct {
	Side       string
	Quantity   int
	EntryPrice float64
}

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	cfg, err := loadConfig(configPath)
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}

	if err := ensureSQLiteDB(cfg.DB.Path); err != nil {
		logger.Fatalf("ensure sqlite db: %v", err)
	}

	logger.Printf("scheduler started for %s", cfg.AppName)
	if err := runStrategy(cfg.DB.Path, cfg.Strategy, cfg.Risk); err != nil {
		logger.Printf("initial run failed: %v", err)
	} else {
		logger.Printf("initial run finished")
	}

	if len(os.Args) > 1 && os.Args[1] == "--once" {
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
		cfg.Strategy.DataSource = "alphavantage"
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
		sql := fmt.Sprintf(
			"INSERT INTO execution_records (strategy_name, status, note, executed_at) VALUES (%s, %s, %s, %s);",
			quoteSQL(strategy.Name),
			quoteSQL("skipped"),
			quoteSQL(signal.Reason),
			quoteSQL(now.Format(time.RFC3339)),
		)
		return execSQLite(path, sql)
	}

	sql := buildPersistSQL(strategy.Name, signal, now)
	return execSQLite(path, sql)
}

func buildPersistSQL(strategyName string, signal strategySignal, now time.Time) string {
	note := fmt.Sprintf(
		"signal=%s symbol=%s reason=%s short_ma=%.2f long_ma=%.2f close=%.2f position=%d",
		signal.Action,
		signal.Symbol,
		signal.Reason,
		signal.ShortMA,
		signal.LongMA,
		signal.ClosePrice,
		signal.PositionSize,
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
	bars, err := loadBars(strategy)
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
			ShortMA:      shortMA,
			LongMA:       longMA,
			ClosePrice:   latest.Close,
			OpenPrice:    latest.Open,
			HighPrice:    latest.High,
			LowPrice:     latest.Low,
			Volume:       latest.Volume,
			Reason:       fmt.Sprintf("repeat signal filtered: %s", action),
			PositionSize: state.Quantity,
		}, nil
	}

	return strategySignal{
		Symbol:       strategy.Symbol,
		Action:       action,
		ShortMA:      shortMA,
		LongMA:       longMA,
		ClosePrice:   latest.Close,
		OpenPrice:    latest.Open,
		HighPrice:    latest.High,
		LowPrice:     latest.Low,
		Volume:       latest.Volume,
		Reason:       reason,
		PositionSize: positionSize,
	}, nil
}

func loadBars(strategy strategyConfig) ([]marketBar, error) {
	if strings.EqualFold(strategy.DataSource, "alphavantage") {
		bars, err := loadBarsFromAlphaVantage(strategy.Symbol, os.Getenv(strategy.APIKeyEnv))
		if err == nil {
			return bars, nil
		}
		if strategy.DataPath == "" {
			return nil, err
		}
	}

	return loadBarsFromCSV(strategy.DataPath)
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
