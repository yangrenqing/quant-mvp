# 研究报告落地实施清单

## 1. 目标

基于 `deep-research-report.md`，把当前系统从“趋势驱动 + 轻量模型增强”升级为：

- 主线：稳健多因子长多
- 辅线：事件驱动增强
- 防守线：ETF / 指数趋势

这份文档只关注“如何落到当前代码结构”，不重复论文综述。

## 2. 当前系统与报告建议的映射

### 2.1 当前已有能力

当前系统已经有这些基础，可以直接复用：

- 候选评分框架
  - `rankCandidate`
  - `scoreCandidate`
  - `candidateOverlayScores`
- 组合构建框架
  - `runPortfolioBacktest`
  - `selectPortfolioCandidates`
  - `portfolioSelectionScore`
- 训练样本导出
  - `datasetRow`
  - `writeDatasetReports`
- 模型回灌
  - `predictLinearModel`
- 持久化与归档
  - `run_history`
  - `experiment_history`
  - `dashboard_snapshots`
  - `simulated_account_ledger`

### 2.2 当前不足

和研究报告相比，当前系统还缺：

- 正式的价值因子
- 正式的质量因子
- 正式的低波动因子
- 正式的拥挤度因子
- 基于交易所/板块差异的涨跌停规则
- 税费版本化
- 公告事件数据层
- ETF / 指数趋势子系统

## 3. 第一阶段改造

### 3.1 目标

把主线改成“稳健多因子长多”。

### 3.2 新增因子

优先新增 5 组：

- `value_score`
  - 可先用简化代理值
  - 第一版允许使用“历史回报/价格位置代理”，后续再接财务数据
- `quality_score_v2`
  - 区分当前已有的 `quality_score`
  - 真正表达经营质量、稳定性、回撤修复能力
- `low_vol_score`
  - 用历史波动率和下行波动构造
- `liquidity_score_v2`
  - 保留现在的流动性，但纳入更稳定的成交额/冲击维度
- `crowding_score`
  - 先用现有过热、成交放大、回测拥挤代理构造
  - 后续接两融和北向低频数据

### 3.3 当前代码改造点

1. 扩展 `scanCandidate`
2. 扩展 `datasetRow`
3. 扩展 `predictLinearModel` 特征集合
4. 扩展 `Score Breakdown`
5. 改写 `portfolioSelectionScore`

### 3.4 输出目标

- 扫描页不再明显偏“昨日涨停/强势追涨”
- 组合更偏高质量、低波动、可成交

## 4. 第二阶段改造

### 4.1 目标

把 A 股制度约束做成更正式的交易层。

### 4.2 需要补的规则

- 主板 10%
- 创业板 / 科创板 20%
- ST / 风险警示差异
- 新股前若干日特殊规则
- 税费版本切换

### 4.3 当前代码改造点

- 新增 `market_rules` 配置层
- 在 `isBuyRestricted` / `isSellRestricted` 中按标的类型判断
- 成本模型拆成：
  - 印花税
  - 过户费
  - 经手费
  - 佣金
  - 滑点

### 4.4 输出目标

- 回测可信度进一步提升
- 回测和实盘规则口径更接近

## 5. 第三阶段改造

### 5.1 目标

引入事件驱动增强层，但不替代主模型。

### 5.2 事件优先级

建议按易落地顺序：

1. 业绩预告 / 快报
2. 回购
3. 增持 / 减持
4. 解禁
5. 重大合同

### 5.3 设计原则

- 事件不单独决定买卖
- 事件作为加分 / 减分项
- 强制保留可审计字段：
  - 事件类型
  - 披露日
  - 生效日
  - 原文链接 / 摘要

### 5.4 代码落点

- 新增事件数据表
- 新增事件抓取 / 导入脚本
- 在 `rankCandidate` 中加 `event_score`
- 在 dashboard 中增加事件摘要卡片

## 6. 第四阶段改造

### 6.1 目标

引入 ETF / 指数趋势防守线。

### 6.2 设计原则

- 与个股选股主线分离
- 作为“市场防守 / 风险切换模块”
- 不混入个股评分

### 6.3 实施路径

1. 新增 ETF / 宽基标的池
2. 新增独立 backtest 入口
3. 输出独立报告
4. 在 dashboard 中显示当前防守状态

## 7. 数据工程实施顺序

建议顺序：

1. 先补多因子字段
2. 再补交易规则版本
3. 再接事件数据
4. 最后做 ETF 线

原因：

- 当前系统已经有评分和组合骨架
- 先改主线最容易看到收益变化
- 事件和 ETF 属于增强层，适合后接

## 8. 推荐开发计划

### Sprint 1

- 增加 `value / low_vol / crowding`
- 改写组合排序
- 重跑扫描和组合回测

### Sprint 2

- 板块涨跌停差异规则
- 税费版本化
- 成本模型拆分

### Sprint 3

- 事件数据表
- 事件导入脚本
- `event_score`
- dashboard 事件卡片

### Sprint 4

- ETF / 指数趋势子系统
- 多市场日报整合

## 9. 当前最优先的一步

如果现在只能做一件事，建议先做：

`稳健多因子主线改造`

也就是：

- value
- low_vol
- crowding
- 更正式的 quality / liquidity 结构

这一步对系统收益风格的改变最大。
