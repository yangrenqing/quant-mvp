# Quant MVP Branch & Feature Tracking

## 当前主分支
- branch: `master`
- role: stable integration branch

## 分支策略

### 1. `master`
- 用途：稳定集成主线
- 要求：
  - 只放已经收口、可解释、可恢复的改动
  - 重要升级进入主线前应留下对应报告/状态/文档工件

### 2. feature branches
命名建议：
- `feature/data-trust-*`
- `feature/backtest-trust-*`
- `feature/strategy-compare-*`
- `feature/evolution-gates-*`
- `feature/automation-*`
- `feature/docs-*`

用途：
- 承载一组相对独立的升级
- 便于回看某次优化到底改了什么
- 便于并行试验和回滚

## feature 记录规则

每次有明确升级时，至少记录：
- 日期时间
- 分支名
- feature 名称
- 背景问题
- 做了什么
- 影响文件
- 产出工件
- 风险/后续
- 是否已合并进 `master`

## 升级记录口径

统一分成五类：
1. data trustworthiness
2. backtest trustworthiness
3. strategy comparison
4. controlled evolution
5. automation / docs / ops

## 记录位置
- 当前 feature 台账：`reports/feature_log.md`
- 当前升级流水：`reports/upgrade_log.jsonl`
- 当前分支策略：`docs/branch_strategy.md`

## 当前已知工作上下文
- current sprint baseline: `756661a`
- current sprint docs: `docs/sprint_72h_plan.md`
- autopilot control: `docs/cc_quant_autopilot.md`

## 规则
- 小修可以直接进主线，但仍应写升级记录
- 中等以上改动优先走 feature 分支，再收口回主线
- 所有升级记录应尽量写清：为什么做、改了什么、怎么验证、下一步是什么
