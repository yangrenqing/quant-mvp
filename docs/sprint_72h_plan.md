# Quant MVP 72-Hour Sprint Plan

Start time: 2026-03-22 16:35 Asia/Shanghai
Repo: /Users/yangrenqing/Downloads/quant-mvp
Git baseline: 756661a
Owner intent: use the shortest practical time to improve evolution ability, data accuracy, and quant capability.

## Sprint objective
Within 72 hours, move quant-mvp from a "working semi-quant stack" to a more trustworthy and self-improving research/decision engine.

Primary goals:
1. Data trustworthiness
2. Backtest trustworthiness
3. Strategy comparison framework
4. Controlled evolution loop

## Non-goals during this sprint
- pretty UI polish
- adding many new data vendors
- broad feature sprawl
- speculative strategy proliferation without evaluation support

## Hard constraints / operating assumptions
- The Mac may stay on, but network may drop intermittently.
- Token supply/provider routing may change during the sprint.
- Work must remain resumable from local files, not chat memory.
- Prefer artifact-first execution: every major step should leave a file artifact.

## Resilience strategy
1. All sprint state is written to local files under docs/ and reports/.
2. Every major milestone updates `reports/sprint72_status.json`.
3. All recommendations should be reproducible from repo artifacts and commands.
4. If network is unavailable:
   - continue code/doc/config audit
   - continue local backtests / report generation where possible
   - log blocked remote dependencies explicitly
5. If model/token/provider context changes:
   - resume from `docs/sprint_72h_plan.md`
   - resume from `reports/sprint72_status.json`
   - resume from `reports/sprint72_backlog.md`

## Execution phases

### Phase 0 — Kickoff / Baseline Freeze
Deliverables:
- repo baseline hash
- current architecture snapshot
- current output/report map
- sprint backlog

### Phase 1 — Audit and Bottleneck Discovery
Questions to answer:
- where can data integrity fail?
- where can lookahead / leakage occur?
- how is evolution currently gated?
- where are strategy comparisons weak or fragmented?

Deliverables:
- `reports/sprint72_audit.md`
- `reports/sprint72_risks.md`
- `reports/sprint72_baseline.json`

### Phase 2 — Data & Evaluation Trustworthiness
Focus:
- provider fallback correctness
- symbol / calendar / timezone integrity
- data quality summary and failure modes
- backtest realism and leakage checks

Target outputs:
- `reports/data_quality_summary.json`
- `reports/backtest_trust_report.md`

### Phase 3 — Strategy Comparison Layer
Focus:
- compare active / baseline / shadow / latest trial winner
- normalize core metrics
- identify what is truly adding value

Target outputs:
- `reports/strategy_compare_latest.*` reuse/upgrade
- `reports/sprint72_strategy_assessment.md`

### Phase 4 — Controlled Evolution Tightening
Focus:
- champion / challenger / shadow logic clarity
- promotion gate quality
- rollback logic and evidence trail

Target outputs:
- `reports/sprint72_evolution_assessment.md`
- recommendations for minimal high-leverage code changes

## 72-hour priority order
P0:
- audit current architecture and runtime logic
- identify top 3 trustworthiness risks
- identify top 3 alpha / evolution bottlenecks

P1:
- add or improve trustworthiness checks
- improve strategy comparison clarity
- tighten evolution decision rules

P2:
- expand factor/strategy roadmap only after trust/evaluation layer is clearer

## Success criteria
By sprint end, the project should be able to answer:
1. Why current active strategy underperforms or succeeds
2. Whether model/factors add value beyond baseline
3. Whether data for a given run is trustworthy
4. Whether a challenger deserves promotion based on multi-metric evidence

## Commands likely to be reused
- make verify
- make validate-config
- make export-runtime-config
- make daily
- make trial
- bash scripts/daily_run.sh
- bash scripts/trial_run.sh --trial-count 100 --trial-prefix <tag>

## Status discipline
Whenever a milestone completes, update:
- `reports/sprint72_status.json`
- `reports/sprint72_backlog.md`

## Initial next step
Read and map:
- docs/product_design.md
- docs/development_guide.md
- docs/auto_evolution_design.md
- docs/research_platform_plan.md
- docs/research_implementation_plan.md
- configs/*.yaml
- Makefile
- main scheduler entry and related internal modules
