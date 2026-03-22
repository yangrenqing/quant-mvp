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
  - 不再直接承载日常开发改动；日常改动应先在 feature/bugfix/docs/chore 分支完成后再回到主线

### 2. feature branches
命名建议：
- `feature/data-trust-*`
- `feature/backtest-trust-*`
- `feature/strategy-compare-*`
- `feature/evolution-gates-*`
- `feature/automation-*`
- `feature/docs-*`
- `bugfix/*`
- `docs/*`
- `chore/*`

用途：
- 承载一组相对独立的升级
- 便于回看某次优化到底改了什么
- 便于并行试验和回滚

## 强制规则

从现在开始：
- 任何改动，无论是小 feature、小 bug、文档修正、脚本调整，**都必须先开新分支**
- 每个分支至少对应一个明确 commit
- 完成后必须留下升级记录
- 未经收口，不应直接在 `master` 上持续堆改动

推荐流程：
1. 从 `master` 切出新分支
2. 完成单一目标改动
3. 提交 commit
4. 更新 `docs/feature_log.md` / `docs/upgrade_log.jsonl` / `docs/now.md`
5. 再决定是否合并回 `master`

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
- 当前 feature 台账：`docs/feature_log.md`
- 当前升级流水：`docs/upgrade_log.jsonl`
- 当前分支策略：`docs/branch_strategy.md`
- 当前驾驶舱：`docs/now.md`

## 当前已知工作上下文
- current sprint baseline: `756661a`
- current sprint docs: `docs/sprint_72h_plan.md`
- autopilot control: `docs/cc_quant_autopilot.md`

## 规则
- 小修也必须开分支，不再默认直接进主线
- 中等以上改动优先走 feature / bugfix 分支，再收口回主线
- 所有升级记录应尽量写清：为什么做、改了什么、怎么验证、下一步是什么
