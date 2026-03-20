# Quant MVP Research Platform Plan

## Current Baseline

As of 2026-03-20, the platform has moved beyond a simple strategy runner and now operates as a research-oriented quant workflow with:

- A-share scan, focus list, and portfolio backtest
- Active vs shadow paper trading
- Promotion / rollback lifecycle
- Daily / weekly / intraday automation
- Factor research, health monitoring, and evolution reports

Current baseline results:

- Portfolio total return: about `2.97%`
- Benchmark return: about `19.75%`
- Excess return: about `-16.78%`
- Max drawdown: about `0.53%`
- Rebalances: `8`
- Overnight evolution: many runs, but limited effective upgrades

This means the platform is already operational and disciplined, but the main alpha engine remains weak. The next step is to improve research quality and decision explainability before pushing harder on strategy complexity.

## Core Problem

The system can run, compare, and evolve, but it still cannot answer these questions in one place with enough rigor:

- Why is the current active strategy underperforming the benchmark?
- Which factors are actually helping, and which have decayed?
- Is the benchmark-aware classifier adding value relative to the regression model?
- Did overnight evolution create a real upgrade, or just more activity?

The current phase therefore focuses on building the research-evaluation layer rather than immediately chasing higher returns.

## Phase 1: Research Evaluation Layer

### Objective

Turn the project into a research platform that can evaluate itself clearly and consistently.

### Deliverables

- `reports/research_summary.*`
- `reports/factor_diagnostics.*`
- `reports/model_comparison.*`
- `reports/strategy_quality.*`

### Required Capabilities

- Factor diagnostics across multiple labels:
  - `label_10d`
  - `excess_10d`
  - `beat_benchmark_10d`
- Unified model comparison:
  - regression model
  - benchmark classifier
- Strategy quality assessment:
  - total return
  - benchmark return
  - excess return
  - max drawdown
  - rebalance count
  - active vs shadow account state
- Overnight explanation:
  - number of runs
  - effective promotions
  - model replacements
  - whether evolution actually improved anything

### Acceptance Criteria

- One page explains current strategy quality
- One page explains current factor health
- One page explains model comparison
- One page explains whether overnight evolution was meaningful

## Phase 2: Data and Evaluation Persistence

### Objective

Make research results historical and queryable instead of ephemeral.

### Deliverables

- Persist research summaries into report artifacts and existing ledgers
- Extend experiment bookkeeping with baseline comparisons
- Reuse JSON outputs in dashboard and future SQLite extensions

### Acceptance Criteria

- Research outputs are reproducible from local artifacts
- Dashboard can summarize current research state without rerunning the whole stack manually

## Phase 3: Research-Driven Strategy Iteration

### Objective

Use the research layer to make better changes to factors, labels, and models.

### Priority Directions

1. benchmark-aware ranking improvements
2. richer factor sets:
   - fundamentals
   - valuation
   - events
   - crowding
3. better candidate generation and promotion criteria

### Acceptance Criteria

- Factor additions are justified by diagnostics, not guesswork
- Promotion decisions include research evidence, not only raw automation metrics

## Evaluation Framework

All future changes should be judged on a unified scorecard.

### Portfolio Layer

- total return
- benchmark return
- excess return
- max drawdown
- rebalance count
- target exposure
- regime

### Model Layer

- regression rolling directional accuracy
- classifier rolling directional accuracy
- test metrics
- promoted / not promoted

### Factor Layer

- correlation / IC proxy
- quintile spread
- sample count
- stability across labels

### Evolution Layer

- run count
- lifecycle events
- promotions
- rollbacks
- active vs shadow equity diff

## Implementation Order

1. write this plan into `docs/`
2. generate factor diagnostics
3. generate model comparison
4. generate strategy quality report
5. generate research summary
6. connect the outputs into dashboard and automation

## Notes

- This phase does not require a new paid data source.
- This phase does not rewrite promotion / rollback rules.
- This phase is deliberately focused on research quality, explainability, and comparability.
