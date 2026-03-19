# 开发文档

## 1. 项目结构

```text
cmd/scheduler/main.go        主程序入口和核心逻辑
configs/                     分层配置
data/                        本地数据和股票池
scripts/                     日常脚本、训练脚本、模型流水线
research/                    研究工作台
reports/                     运行产物和历史快照
```

## 2. 运行环境

建议环境：

- Go
- Python 3
- SQLite3

当前脚本假设：

- Go 在 `/usr/local/go/bin/go`
- Python 可通过 `python3` 调用

## 3. 配置加载

系统会按顺序合并以下配置：

1. `configs/config.yaml`
2. `configs/data.yaml`
3. `configs/portfolio.yaml`
4. `configs/model.yaml`
5. `configs/report.yaml`
6. `configs/local.yaml`（如果存在）

建议：

- 公共默认值放分层配置里
- 本机私有配置放 `local.yaml`

## 4. 主要模块

### 4.1 数据加载

核心入口：

- `loadBars`
- `loadSymbolBars`
- `loadAShareBarsWithPrimary`
- `loadCachedProviderBars`

职责：

- 选择数据源
- 读取缓存
- 远程失败时回退
- 记录 diagnostics

### 4.2 信号与候选打分

核心入口：

- `rankCandidate`
- `scoreCandidate`
- `candidateOverlayScores`
- `evaluateStrategyEnsemble`

当前评分体系包括：

- `quality_score`
- `risk_score`
- `heat_penalty`
- `reversal_score`
- 趋势、结构、动量、持续性、突破、量能等细分项

### 4.3 组合构建

核心入口：

- `runPortfolioBacktest`
- `selectPortfolioCandidates`
- `portfolioSelectionScore`

当前组合逻辑包含：

- 行业上限
- 波动率目标缩放
- 市场 regime 控制
- 容量限制
- 替补候选
- 持仓止损 / 回撤退出 / 冷却期

### 4.4 报告输出

核心入口：

- `writePlanReports`
- `writeAShareScanReports`
- `writeBacktestReports`
- `writePortfolioBacktestReports`
- `writeGridSearchReports`
- `writeDatasetReports`
- `writeDashboardReports`

输出格式：

- `.txt`
- `.html`
- `.json`
- `.csv`（适合表格结果）

### 4.5 持久化与归档

核心入口：

- `persistRunRecord`
- `appendExperimentRecord`
- `archiveFiles`
- `writeDiagnosticsReports`
- `cleanupOldArtifacts`

SQLite 表：

- `execution_records`
- `signal_records`
- `position_state`
- `run_history`
- `experiment_history`
- `dashboard_snapshots`
- `simulated_account_ledger`

## 5. 模型部分

训练脚本：

- `scripts/train_model.py`

自动流水线：

- `scripts/model_pipeline.py`

Go 侧回灌：

- `getLinearModel`
- `predictLinearModel`

如果要替换成更强模型，建议保持以下契约不变：

- 输出 `linear_model.json` 风格的特征和权重结构
- 或在 Go 端新增一个并行模型加载器

## 6. 新功能开发建议

### 6.1 加新因子

建议路径：

1. 在 `rankCandidate` / `scoreCandidate` 中新增字段
2. 补 `scanCandidate`
3. 补 `datasetRow`
4. 补数据集导出
5. 补模型特征
6. 补报告展示

### 6.2 加新报告

建议路径：

1. 在 `reports/` 下定义文件名
2. 写 `txt/html/json`
3. 接入 `persistRunRecord`
4. 决定是否接入 dashboard

### 6.3 改 daily 流程

入口：

- `scripts/daily_run.sh`
- `Makefile`

原则：

- 日常流程保持稳定
- 研究流程和生产流程不要混在一个脚本里

## 7. 调试建议

优先看这些文件：

- `reports/diagnostics.txt`
- `reports/run_index.jsonl`
- `reports/experiments.jsonl`
- `reports/history_compare.html`

优先查这些路径：

- `reports/history/`
- `data/cache/`
- `data/quant.db`

## 8. 提交规范建议

建议按改动类型拆提交：

- `strategy`
- `portfolio`
- `reporting`
- `model`
- `ops`

这样以后回看历史更清楚。
