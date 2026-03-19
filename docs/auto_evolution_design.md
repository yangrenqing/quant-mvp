# 自动进化设计

## 目标

让系统在长期运行中持续改进，但不允许策略在盘中无控制漂移。

设计原则：

- 执行与学习分离
- 新策略先观察，再晋升
- 所有版本可追溯、可回滚
- 晋升必须基于多指标，不基于单次收益

## 核心角色

系统中的策略版本分为四类：

- `active`
  当前正式执行的策略版本。盘中模拟盘只允许执行 `active`。
- `candidate`
  新训练、新调参或新规则生成的候选版本。
- `shadow`
  候选版本进入影子观察期，与 `active` 并行评估，但不接管正式账户。
- `archived`
  历史版本或被淘汰版本。

## 运行节奏

### 盘中

- 仅执行 `active`
- 按固定间隔轮询模拟盘
- 记录订单、成交、权益和持仓快照

### 收盘后

- 归档日报
- 追加样本数据
- 统计 active 与 shadow 的日度表现

### 周度

- 训练新模型
- 运行候选参数搜索
- 生成 `candidate`

### 月度

- 运行更大范围研究
- 评估是否淘汰旧策略
- 更新长期基准和风险阈值

## 晋升门槛

候选版本不能直接替换 `active`，必须先满足：

1. 滚动验证优于当前版本
2. 影子观察期达到最小天数
3. 收益提升达到最小边际
4. 最大回撤未明显恶化
5. 换手未显著放大

建议阈值：

- `candidate_return >= active_return + 0.02`
- `candidate_drawdown <= active_drawdown + 0.01`
- `candidate_turnover <= active_turnover * 1.5`
- `candidate_observation_days >= 10`

## 回滚条件

出现以下情况时允许回滚到上一个稳定版本：

- `active` 连续跑输 `shadow`
- 最大回撤超过预设上限
- 数据源异常导致评估不可信
- 模型文件或配置损坏

## 数据结构

### strategy_registry

记录策略版本本身。

关键字段：

- `strategy_id`
- `version_name`
- `market`
- `status`
- `parent_version`
- `git_commit`
- `config_json`
- `model_path`
- `created_at`
- `activated_at`
- `archived_at`
- `notes`

### strategy_promotions

记录晋升、降级、回滚过程。

关键字段：

- `event_type`
- `market`
- `from_version`
- `to_version`
- `trigger_reason`
- `metrics_json`
- `recorded_at`

### paper_accounts

扩展用法：

- `mode=live` 对应正式 paper account
- `mode=shadow:<version_name>` 对应影子账户

## 第一版实现范围

第一版不做自动切换，只做骨架：

1. 建立 `strategy_registry`
2. 建立 `strategy_promotions`
3. 启动时自动种入当前 `active` 策略版本
4. `paper_accounts` 支持未来 shadow 账户扩展
5. 文档、SQLite、dashboard 口径统一

## 第二版实现范围

1. 新增影子账户运行入口
2. 统计 active vs shadow 日度指标
3. 编写晋升脚本
4. 将晋升结果写入 `strategy_promotions`

## 第三版实现范围

1. 每周自动候选生成
2. 自动进入 shadow
3. 满足门槛后自动晋升
4. 不满足门槛则自动归档

## 对当前项目的直接意义

这套设计会把当前“模型流水线、回测、模拟盘、日报”串成统一策略生命周期：

- 研究产出候选
- 候选进入 shadow
- shadow 观察后晋升
- 晋升失败可回滚

这样系统才是“受控进化”，而不是“随机自我改写”。
