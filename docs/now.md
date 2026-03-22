# Quant MVP NOW

Updated: 2026-03-22 21:10 Asia/Shanghai

## 当前状态
- repo: `/Users/yangrenqing/Downloads/quant-mvp`
- branch: `master`
- baseline commit: `756661a`
- mode: `autopilot active`
- sprint: `quant-mvp-72h`
- phase: `phase2-promotion-compare-health-patched`

## 当前主线 feature
### `master` · sprint72 trust/evolution tightening
目标：
- data trustworthiness
- backtest trustworthiness
- strategy comparison clarity
- controlled evolution tightening

## 最近已完成
- 建立 sprint 计划 / 状态 / backlog / handoff 工件
- 输出 audit / risks / baseline / trust report
- patch `strategy_promote.py`
- patch `strategy_compare.py`
- patch `health_monitor.py`
- 建立 `quant-mvp` autopilot 持续优化协议
- 建立 branch / feature / upgrade 记录机制

## 当前将收口进 git 的内容
- `scripts/health_monitor.py`
- `scripts/strategy_compare.py`
- `scripts/strategy_promote.py`
- `docs/sprint_72h_plan.md`
- `docs/cc_quant_autopilot.md`
- `docs/branch_strategy.md`
- `docs/feature_log.md`
- `docs/upgrade_log.jsonl`
- `docs/autopilot_status.json`
- `docs/now.md`

## 下一步动作
1. patch runtime freshness / fallback verdict fields into major artifacts
2. review whether `strategy_promote` should enforce candidate metric presence before promotion
3. begin next-bar execution backtest refactor
4. rerun compare / health after freshness verdict patch

## 关键入口
- sprint 计划：`docs/sprint_72h_plan.md`
- autopilot 协议：`docs/cc_quant_autopilot.md`
- 分支策略：`docs/branch_strategy.md`
- autopilot 状态：`docs/autopilot_status.json`
- 当前驾驶舱：`docs/now.md`
- feature 台账：`docs/feature_log.md`
- 升级流水：`docs/upgrade_log.jsonl`
- sprint 状态：`reports/sprint72_status.json`
- handoff：`reports/sprint72_handoff.md`

## 一句话判断
项目当前不是缺功能，而是在把“可信度、回测真实性、比较口径、进化门槛”做硬，并且把整个推进过程纳入可追踪 git 历史。
