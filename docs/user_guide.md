# 使用文档

## 1. 快速开始

进入项目目录：

```bash
cd /Users/yangrenqing/Downloads/quant-mvp
```

最常用的一键运行：

```bash
bash scripts/daily_run.sh
```

运行后重点查看：

```bash
open reports/dashboard.html
```

## 2. 常用页面

- `reports/dashboard.html`
  日常总览
- `reports/a_share_focus.html`
  今日重点关注名单
- `reports/a_share_scan.html`
  全量扫描结果
- `reports/portfolio_backtest.html`
  组合回测结果
- `reports/history_compare.html`
  历史对比
- `reports/market_overview.html`
  多市场总览

## 3. 常用命令

### 3.1 扫描 A 股

```bash
go run ./cmd/scheduler --scan-a-share --top 10
```

### 3.2 跑组合回测

```bash
go run ./cmd/scheduler --portfolio-backtest --from 2025-01-01 --to 2026-03-19 --cash 100000 --fee-bps 10 --slippage-bps 5 --top 3
```

### 3.3 导出训练数据

```bash
go run ./cmd/scheduler --export-dataset --from 2025-01-01 --to 2026-03-19
```

### 3.4 训练模型

```bash
python3 scripts/train_model.py --dataset reports/training_dataset.csv --label label_10d
```

### 3.5 运行模型流水线

```bash
python3 scripts/model_pipeline.py --from 2025-01-01 --to 2026-03-19 --label label_10d
```

## 4. 日常脚本

### 4.1 全流程

```bash
bash scripts/daily_run.sh
```

### 4.2 指定日期区间

```bash
bash scripts/daily_run.sh --from 2025-01-01 --to 2026-03-19
```

### 4.3 跳过模型

```bash
bash scripts/daily_run.sh --skip-model
```

### 4.4 只做扫描

```bash
bash scripts/daily_run.sh --scan-only
```

## 5. Makefile 入口

```bash
make help
make scan
make portfolio
make dataset
make model
make daily
```

## 6. 配置说明

配置位于：

- `configs/config.yaml`
- `configs/data.yaml`
- `configs/portfolio.yaml`
- `configs/model.yaml`
- `configs/report.yaml`

如果你想做本机私有覆盖，建议新建：

```text
configs/local.yaml
```

## 7. 结果在哪里看

### 7.1 报告

所有最新报告都在：

```text
reports/
```

### 7.2 历史快照

```text
reports/history/YYYY-MM-DD/
```

### 7.3 数据库

```text
data/quant.db
```

### 7.4 运行索引和实验记录

```text
reports/run_index.jsonl
reports/experiments.csv
reports/experiments.jsonl
```

## 8. 研究工作台

初始化研究目录：

```bash
bash scripts/research_run.sh
```

目录用途：

- `research/papers/`
  论文笔记
- `research/factors/`
  因子草稿
- `research/experiments/`
  实验记录

## 9. 常见问题

### 9.1 为什么是 test 模式

说明远程行情没有成功获取，系统回退到了本地 CSV。

先看：

```text
reports/diagnostics.txt
```

### 9.2 为什么报告每天会变化

因为系统会覆盖 `reports/` 下的最新文件。  
历史版本在：

```text
reports/history/
```

### 9.3 怎么看系统到底做了什么

优先看：

- `dashboard.html`
- `history_compare.html`
- `diagnostics.txt`
- `run_index.jsonl`
