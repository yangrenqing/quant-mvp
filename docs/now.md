# Quant MVP NOW

Updated: 2026-03-23 08:10 Asia/Shanghai

## 当前状态
- repo: `/Users/yangrenqing/Downloads/quant-mvp`
- branch: `feature/backtest-trust-next-bar-single`
- baseline commit: `756661a`
- mode: `autopilot active`
- sprint: `quant-mvp-72h`
- phase: `phase4-next-bar-single-committed`

## 当前主线 feature
### `feature/backtest-trust-next-bar-single` · single-name next-bar execution
目标：
- backtest trustworthiness
- remove same-bar decision/execution coupling in single-symbol backtests

## 最近已完成
- `simulateBacktest` 改为信号日 `close_t`、执行日 `open_t_plus_1`
- 买卖执行统一切到 next bar open，并把 T+1 / gap / capacity 判断对齐到执行日
- trade / equity 记录补齐 `signal_date` / `execution_date`
- single-name next-bar slice 已提交

## 当前将收口进 git 的内容
- 当前切片已收口；`reports/` 继续作为本地恢复点，不随本次 commit 入库

## 下一步动作
1. start portfolio next-bar slice on a separate branch
2. validate the portfolio slice with gofmt and `/usr/local/go/bin/go test ./...`
3. align reports/docs resume points after the next committed slice

## 关键入口
- 当前交接：`docs/backtest_next_bar_handoff.md`
- trust 报告：`reports/backtest_trust_report.md`
- patch 计划：`reports/sprint72_patch_plan.md`
- 当前驾驶舱：`docs/now.md`
- feature 台账：`docs/feature_log.md`

## 一句话判断
单标的 backtest 已从“看见当天、当天成交”切到“收盘形成信号、下一交易日开盘执行”，时序上更像真实交易。
