# Next-Bar Execution Handoff

Updated: 2026-03-24 09:19 Asia/Shanghai
Repo: `/Users/yangrenqing/Downloads/quant-mvp`
Branch: `feature/backtest-trust-next-bar-seam-audit`
Resume baseline: follow `reports/sprint72_status.json` + `reports/cc_autopilot_status.json` for the latest pushed checkpoint on this branch

## Purpose

Provide a minimal, precise handoff for the next high-leverage patch:
**remove same-bar decision/execution coupling** from backtests, while keeping the patch scoped and testable.

This document is intentionally narrower than the trust report: it maps the exact code seams and a safest-first implementation order.

---

## 1. Current leakage points

### A. `runPortfolioBacktest` same-bar coupling

Code path: `cmd/scheduler/main.go:1283-1545`

Current behavior:
1. On trading date `t`, build `history := barsUpToDate(..., date)`
2. Rank/select candidates using data that includes bar `t`
3. Build `targetSet` immediately for the same `date`
4. Execute sells/buys using the same day's bar (`bar.Close`, sometimes with gap checks using same-day context)

Leakage consequence:
- strategy observes bar `t`
- then trades on bar `t`
- ranking and execution are not separated by one executable bar

### B. `simulateBacktest` same-bar coupling

Code path: `cmd/scheduler/main.go:2372-2497`

Current behavior:
1. Append current `bar.Close` into `closes`
2. Compute short/long MA using arrays that include current bar `t`
3. Decide `BUY`/`SELL` on bar `t`
4. Execute on the same bar via `bar.Open`/`bar.Close` proxy

Leakage consequence:
- signal knowledge includes close of `t`
- but execution still happens on `t`
- open/close mix is not temporally coherent

---

## 2. Safest implementation target

### Rule to enforce

For all backtest paths:
- signal basis: `close_t`
- earliest execution basis: `open_t_plus_1`
- if `t+1` does not exist, do not execute

### Minimal patch scope

Only touch:
- `cmd/scheduler/main.go`
- report payload fields emitted by existing backtest writers

Do **not** widen into paper/live execution yet.
Do **not** change promotion logic in the same patch.
Do **not** refactor unrelated ranking code in the same patch.

---

## 3. Recommended patch slicing

### Slice 1 — explicit metadata first

Before behavior change, add explicit metadata fields to backtest artifacts:
- `signal_date_basis: close_t`
- `execution_date_basis: same_bar_proxy` (current)
- `same_bar_execution: true`
- `degraded_execution_assumption: true`

Targets:
- `backtest_latest.json`
- `portfolio_backtest.json`
- any persisted backtest run payload using the same structs

Why first:
- lowest-risk truthfulness win
- creates before/after comparison anchor
- reduces ambiguity even before the full next-bar refactor lands

### Slice 2 — `simulateBacktest` next-bar conversion

This is the smaller behavioral patch.

Suggested implementation model:
1. iterate signal bar index `i`
2. compute MA / action from bars through `filtered[i]`
3. if action exists and `i+1 < len(filtered)`, use `filtered[i+1]` as execution bar
4. record:
   - `signal_date = filtered[i].Date`
   - `execution_date = filtered[i+1].Date`
5. mark trade price from next bar, ideally `nextBar.Open` adjusted for slippage
6. keep T+1 restriction logic aligned to execution date rather than signal date

Acceptance for this slice:
- no trade record date equals the bar whose close formed the signal
- before/after output can explain the assumption change

### Slice 3 — `runPortfolioBacktest` pending-target rebalance

This is the larger patch.

Suggested implementation model:
1. on date `t`, compute ranked candidates and store a `pendingTargetSet`
2. on next trading date `t+1`, execute sells/buys against that pending set using `t+1` bars
3. snapshot should distinguish:
   - signal date
   - execution date
   - holdings after execution
4. stop-loss / drawdown-trigger exits should also follow the same convention:
   - trigger observed on `t`
   - earliest executable exit on `t+1`
   - unless the design explicitly documents a different assumption

Acceptance for this slice:
- no same-date rank-and-fill path remains
- rebalance count and holdings updates happen on execution day, not signal day

---

## 4. Concrete code seams

### `simulateBacktest`

Current decision/execution block:
- signal logic starts around `2405-2419`
- execution switch starts around `2421`
- current buy execution uses same-bar price at `2426`
- current sell execution uses same-bar price at `2461`

Refactor hint:
- split into two variables:
  - `signalBar := filtered[i]`
  - `execBar := filtered[i+1]`
- keep `equityCurve` marking daily mark-to-market on current loop date
- but append trade log entries with `execution_date`

### `runPortfolioBacktest`

Current coupling points:
- candidate ranking on `1284-1325`
- same-day target set creation on `1331-1334` **(already removed via pending target carry)**
- same-day liquidation logic on `1350-1426` **(still same-day execution on current bar)**
- same-day rebalance logic on `1428-1545` **(still same-day execution on current bar)**
- snapshot signal/execution fields near `1588-1601` **(already split, but only reporting-level split for the remaining liquidation/rebalance paths)**

Refactor hint:
- existing state already has:
  - `pendingTargetSet map[string]scanCandidate`
  - `pendingSignalDate string`
- first day now builds pending targets with no immediate fill
- each loop day now executes prior pending targets, then computes the next pending set
- **remaining trust gap** is narrower now: stop-loss / max-drawdown exits and rebalance buy/sell sizing still execute against the current loop day's bar, so behavior is not yet fully `signal_t -> execution_t+1`
- residual seam anchors now confirmed in `cmd/scheduler/main.go`:
  - stop-loss exit: `1391-1399`
  - max-holding-drawdown exit: `1406-1414`
  - target-set SELL exit: `1421-1429`
  - drop-out liquidation exit: `1437-1446`
  - rebalance trim sell path: `1517-1536`
  - rebalance add/buy path: `1542-1563`
- safest next slice is to convert exactly these residual execution points in one tightly scoped patch, without widening into paper/live execution

### Recommended residual conversion model

To finish the portfolio path without reopening broader design questions, prefer a minimal state-machine extension over another in-place same-day patch.

Suggested additions:
- keep existing `pendingTargetSet` / `pendingSignalDate`
- add `pendingExitSet map[string]string` keyed by symbol with exit reason (`stop_loss`, `max_drawdown`, `trend_break`, `drop_out`)
- treat all residual liquidation/rebalance actions as **signals formed on close_t, executed on t+1**

Suggested loop order per trading date `t`:
1. execute prior `pendingExitSet` against bar `t`
2. execute prior `pendingTargetSet` rebalance against bar `t`
3. mark holdings / equity snapshot for execution date `t`
4. evaluate fresh close-based exit signals from bar `t` and populate next `pendingExitSet`
5. evaluate ranking / target weights from bar `t` and populate next `pendingTargetSet`
6. store `pendingSignalDate = t`

This keeps the patch narrow because:
- stop-loss / drawdown / trend-break / drop-out all share the same pending-exit mechanism
- trim-sell / add-buy continue to share the existing pending-target rebalance mechanism
- reporting semantics stay aligned: `signal_date` comes from the pending set origin, `execution_date` is the current loop date

---

## 5. Reporting fields to add

At minimum, extend backtest JSON payloads with:
- `signal_date_basis`
- `execution_date_basis`
- `same_bar_execution`
- `degraded_execution_assumption`

If the patch budget allows, also add:
- `signal_date`
- `execution_date`
per trade entry

This will make trust comparisons much easier than inferring from code.

---

## 6. Validation checklist

### For metadata-only slice
- regenerate `backtest_latest.json`
- regenerate `portfolio_backtest.json`
- confirm fields appear and describe current assumption correctly

### For `simulateBacktest` next-bar slice
- `python3 -m py_compile scripts/strategy_promote.py` (sanity for repo flow)
- `/usr/local/go/bin/go test ./...`
- run a single-symbol backtest before/after
- verify no trade executes on the same signal bar

### For portfolio next-bar slice
- `/usr/local/go/bin/go test ./...`
- rerun portfolio backtest
- inspect first several snapshot/trade transitions manually
- compare return / drawdown delta against prior artifact
- explicitly verify these semantics after the pending-exit conversion:
  - no sell/buy triggered by close on day `t` is executed with day `t` pricing
  - a stop-loss / drawdown / trend-break / drop-out detected on `t` first appears as an executed exit on `t+1`
  - rebalance trim/add actions consume the carried pending sets rather than same-day ranking output
  - snapshot `signal_date` stays at the prior decision day while `execution_date` equals the current loop day
  - terminal day with no `t+1` does not create a phantom execution

---

## 7. Recommended next operator step

Do **Slice 1 first** in a new branch, e.g.:
- `feature/backtest-trust-execution-metadata`

If that lands cleanly, then:
- `feature/backtest-trust-next-bar-single`
- `feature/backtest-trust-next-bar-portfolio`

This keeps metric semantics changes isolated and reviewable.

---

## Bottom line

The next-bar problem was the top backtest trust issue, and the portfolio residual conversion is now implemented, pushed, and first-pass validated on `feature/backtest-trust-next-bar-seam-audit`.

Current status:
1. execution assumption is exposed in artifacts
2. single-name backtest next-bar conversion is done
3. portfolio residual next-bar conversion is pushed and first-pass validated on remote checkpoint `10ac6c6`

Validation evidence already captured:
- `go test ./...` passed
- fresh `portfolio_backtest` artifacts report `SignalDateBasis=close_t` and `ExecutionDateBasis=open_t_plus_1`
- holding-change samples show explicit `t -> t+1` transitions such as `2025-01-23 -> 2025-01-24` and `2025-05-15 -> 2025-05-16`

Immediate next step:
- use `10ac6c6` as the validated resume checkpoint for this next-bar line
- do not confuse later branch-head commits (for example `335ca1e`, a separate quality-pullback trial slice) with this validated checkpoint unless they are explicitly revalidated for next-bar semantics
- only reopen this area if new evidence shows the restricted/gap-blocked pending-exit caveat is materially harmful

Remaining caveat captured in code-path behavior:
- if an exit is signaled on `t` but `t+1` is restricted / suspended / gap-blocked, the exit remains pending until a later executable trading day rather than forcing a phantom fill
