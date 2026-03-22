# Feature Log

## 2026-03-23

### feature/backtest-trust-execution-metadata · execution assumption metadata
- 背景：next-bar 行为重构前，先把 backtest / portfolio backtest 的执行假设显式写进工件，避免 same-bar 假设继续隐身。
- 已做：
  - 给 `backtestResult` 增加：
    - `signal_date_basis`
    - `execution_date_basis`
    - `same_bar_execution`
    - `degraded_execution_assumption`
  - 给 `portfolioBacktestResult` 增加同样字段
  - 给 `backtestTrade` 预留：
    - `signal_date`
    - `execution_date`
  - 更新 backtest / portfolio backtest 文本与 HTML 报告，把执行假设直接展示出来
- 主要文件：
  - `cmd/scheduler/main.go`
- 验证：
  - `/usr/local/go/bin/gofmt -w cmd/scheduler/main.go`
  - `/usr/local/go/bin/go test ./...`
- 当前状态：已验证，待提交
- 合并状态：未合并进 `master`
- 下一步：
  - commit metadata slice
  - 再进入 single-name next-bar slice

## 2026-03-23

### docs/backtest-next-bar-handoff · next-bar execution handoff
- 背景：compare / promotion gate 已收紧，当前最高杠杆风险转为 backtest 的 same-bar decision/execution coupling。
- 已做：
  - 梳理 `runPortfolioBacktest` 与 `simulateBacktest` 的 same-bar 触发点
  - 形成 `docs/backtest_next_bar_handoff.md`
  - 明确最安全的实施顺序：先执行假设元数据，再单标的 next-bar，再组合 next-bar
- 主要文件：
  - `docs/backtest_next_bar_handoff.md`
- 当前状态：handoff ready
- 合并状态：未合并进 `master`
- 下一步：
  - 按 handoff 从 metadata slice 开新分支实施

## 2026-03-23

### feature/evolution-gates-compare-artifact-sync · compare artifact sync gate
- 背景：promotion gate 之前只看 candidate shadow 指标与 winner 工件，未强制 compare 工件已经对齐候选视图，存在 compare/promotion 口径短暂失步的风险。
- 已做：
  - 给 `scripts/strategy_promote.py` 增加 compare artifact 读取与对齐校验
  - 新增 `--require-compare-candidate` 与 `--require-compare-metrics`
  - 在 promotion 报告中记录 `compare_candidate_version` 与 `compare_metric_sources`
  - 调整 `cmd/scheduler/main.go`，在 daily/weekly promotion 前先重新生成 `strategy_compare.py`
  - scheduler 调 promotion 时显式带上 compare gate 参数
- 主要文件：
  - `scripts/strategy_promote.py`
  - `cmd/scheduler/main.go`
  - `reports/sprint72_status.json`
  - `reports/cc_autopilot_status.json`
- 验证：
  - `python3 -m py_compile scripts/strategy_promote.py`
  - `/usr/local/go/bin/go test ./...`
  - `python3 scripts/strategy_compare.py`
  - `python3 scripts/strategy_promote.py --candidate candidate_trial_opt-20260322-a_exp001 --require-compare-candidate --require-compare-metrics --min-observations 3 --dry-run`
- 当前状态：已验证，待提交
- 合并状态：未合并进 `master`
- 下一步：
  - commit 本次 compare gate sync patch
  - 再进入 next-bar execution backtest refactor

## 2026-03-22

### master · sprint72 trust/evolution tightening
- 背景：72 小时冲刺目标是提升数据可信度、回测可信度、策略比较清晰度、受控进化质量。
- 已做：
  - 建立 sprint 计划、status、backlog、handoff 工件
  - 输出 audit / risks / baseline / trust report
  - patch promotion / compare / health 三处脚本
  - 建立 `quant-mvp` autopilot 持续优化协议
  - 建立 branch / feature / upgrade 记录机制
- 主要文件：
  - `docs/sprint_72h_plan.md`
  - `docs/cc_quant_autopilot.md`
  - `docs/branch_strategy.md`
  - `docs/now.md`
  - `docs/feature_log.md`
  - `docs/upgrade_log.jsonl`
  - `docs/autopilot_status.json`
  - `scripts/strategy_promote.py`
  - `scripts/strategy_compare.py`
  - `scripts/health_monitor.py`
  - `reports/sprint72_*`
- 当前状态：进行中
- 合并状态：准备收口进 `master`
- 下一步：
  - freshness / fallback verdict patch
  - next-bar backtest refactor
  - compare / promotion gate tightening
