# Feature Log

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
