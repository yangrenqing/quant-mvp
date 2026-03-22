#!/usr/bin/env python3
import json
import sqlite3
from datetime import datetime, timezone
from html import escape
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
REPORTS = ROOT / "reports"
DB_PATH = ROOT / "data" / "quant.db"


def load_json(name):
    path = REPORTS / name
    if not path.exists():
        return {}
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return {}


def latest_row(conn, sql, params=()):
    row = conn.execute(sql, params).fetchone()
    return dict(row) if row is not None else None


def count_rows(conn, sql, params=()):
    row = conn.execute(sql, params).fetchone()
    if row is None:
        return 0
    if isinstance(row, sqlite3.Row):
        return int(row[0])
    return int(row[0])


def safe_float(value):
    try:
        return float(value)
    except (TypeError, ValueError):
        return 0.0


def safe_int(value, default=0):
    try:
        return int(value)
    except (TypeError, ValueError):
        return default


def equity_delta(lhs, rhs):
    if lhs is None or rhs is None:
        return None
    return safe_float(lhs.get("equity")) - safe_float(rhs.get("equity"))


def equity_ratio(lhs, rhs):
    if lhs is None or rhs is None:
        return None
    rhs_equity = safe_float(rhs.get("equity"))
    if abs(rhs_equity) < 1e-9:
        return None
    return (safe_float(lhs.get("equity")) - rhs_equity) / rhs_equity


def observation_counts(conn, strategy_version, mode_filter):
    if not strategy_version:
        return {"raw": 0, "distinct_market_days": 0}
    row = conn.execute(
        """
        SELECT COUNT(*), COUNT(DISTINCT market_date)
        FROM paper_daily_metrics
        WHERE strategy_version = ? AND mode LIKE ?
        """,
        (strategy_version, mode_filter),
    ).fetchone()
    if row is None:
        return {"raw": 0, "distinct_market_days": 0}
    return {
        "raw": safe_int(row[0], 0),
        "distinct_market_days": safe_int(row[1], 0),
    }


def format_metric(label, payload):
    if not payload:
        return f"{label}: n/a"
    return (
        f"{label}: version={payload.get('strategy_version', 'n/a')} "
        f"mode={payload.get('mode', 'n/a')} "
        f"date={payload.get('market_date', 'n/a')} "
        f"equity={safe_float(payload.get('equity')):.2f} "
        f"obs_raw={safe_int(payload.get('observations_raw'), 0)} "
        f"obs_distinct_days={safe_int(payload.get('observations_distinct_market_days'), 0)}"
    )


def format_delta(label, absolute_value, ratio_value):
    if absolute_value is None:
        return f"{label}: n/a"
    if ratio_value is None:
        return f"{label}: {absolute_value:.2f}"
    return f"{label}: {absolute_value:.2f} ({ratio_value:.2%})"


def attach_observations(conn, payload, strategy_version, mode_filter):
    if not payload or not strategy_version:
        return payload
    counts = observation_counts(conn, strategy_version, mode_filter)
    payload["observations"] = counts["distinct_market_days"]
    payload["observations_raw"] = counts["raw"]
    payload["observations_distinct_market_days"] = counts["distinct_market_days"]
    return payload


def canonical_challenger_state(winner_candidate, shadow_candidate, promotion_candidate):
    winner_candidate = (winner_candidate or "").strip()
    shadow_candidate = (shadow_candidate or "").strip()
    promotion_candidate = (promotion_candidate or "").strip()

    if promotion_candidate and winner_candidate and promotion_candidate != winner_candidate:
        return {
            "winner_candidate_version": winner_candidate,
            "shadow_candidate_version": shadow_candidate,
            "promotion_candidate_version": promotion_candidate,
            "canonical_candidate_version": promotion_candidate,
            "sync_status": "promotion-stale",
            "sync_reason": f"promotion report evaluated {promotion_candidate}, latest winner is {winner_candidate}",
        }
    if winner_candidate and shadow_candidate and winner_candidate != shadow_candidate:
        return {
            "winner_candidate_version": winner_candidate,
            "shadow_candidate_version": shadow_candidate,
            "promotion_candidate_version": promotion_candidate,
            "canonical_candidate_version": winner_candidate,
            "sync_status": "shadow-mismatch",
            "sync_reason": f"latest shadow version {shadow_candidate} does not match latest winner {winner_candidate}",
        }
    if promotion_candidate and shadow_candidate and not winner_candidate and promotion_candidate != shadow_candidate:
        return {
            "winner_candidate_version": winner_candidate,
            "shadow_candidate_version": shadow_candidate,
            "promotion_candidate_version": promotion_candidate,
            "canonical_candidate_version": promotion_candidate,
            "sync_status": "shadow-mismatch",
            "sync_reason": f"promotion candidate {promotion_candidate} does not match latest shadow {shadow_candidate}",
        }
    if winner_candidate and not shadow_candidate and not promotion_candidate:
        return {
            "winner_candidate_version": winner_candidate,
            "shadow_candidate_version": shadow_candidate,
            "promotion_candidate_version": promotion_candidate,
            "canonical_candidate_version": winner_candidate,
            "sync_status": "winner-only",
            "sync_reason": "latest challenger exists only in winner artifact",
        }
    if promotion_candidate and not shadow_candidate and not winner_candidate:
        return {
            "winner_candidate_version": winner_candidate,
            "shadow_candidate_version": shadow_candidate,
            "promotion_candidate_version": promotion_candidate,
            "canonical_candidate_version": promotion_candidate,
            "sync_status": "promotion-only",
            "sync_reason": "promotion report has a challenger but no winner/shadow challenger is available",
        }
    if shadow_candidate and not winner_candidate and not promotion_candidate:
        return {
            "winner_candidate_version": winner_candidate,
            "shadow_candidate_version": shadow_candidate,
            "promotion_candidate_version": promotion_candidate,
            "canonical_candidate_version": shadow_candidate,
            "sync_status": "shadow-only",
            "sync_reason": "latest challenger exists only as shadow account state",
        }
    if winner_candidate or shadow_candidate or promotion_candidate:
        canonical = promotion_candidate or winner_candidate or shadow_candidate
        return {
            "winner_candidate_version": winner_candidate,
            "shadow_candidate_version": shadow_candidate,
            "promotion_candidate_version": promotion_candidate,
            "canonical_candidate_version": canonical,
            "sync_status": "aligned",
            "sync_reason": "winner, shadow, and promotion views are aligned or compatible",
        }
    return {
        "winner_candidate_version": "",
        "shadow_candidate_version": "",
        "promotion_candidate_version": "",
        "canonical_candidate_version": "",
        "sync_status": "none",
        "sync_reason": "no challenger candidate is currently available",
    }


def main():
    REPORTS.mkdir(parents=True, exist_ok=True)

    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row

    active_strategy = latest_row(
        conn,
        "SELECT version_name, activated_at FROM strategy_registry WHERE status = 'active' ORDER BY id DESC LIMIT 1",
    )
    latest_live = latest_row(
        conn,
        """
        SELECT strategy_version, mode, market_date, equity, cash, holding_count, order_count, fill_count
        FROM paper_daily_metrics
        WHERE mode = 'live'
        ORDER BY id DESC
        LIMIT 1
        """,
    )
    latest_shadow = latest_row(
        conn,
        """
        SELECT strategy_version, mode, market_date, equity, cash, holding_count, order_count, fill_count
        FROM paper_daily_metrics
        WHERE mode LIKE 'shadow:%'
        ORDER BY id DESC
        LIMIT 1
        """,
    )
    winner_artifact = load_json("paper_trial_winner_latest.json")
    promotion = load_json("strategy_promotion_latest.json")

    if latest_live:
        attach_observations(conn, latest_live, latest_live.get("strategy_version"), "live")
    if latest_shadow:
        attach_observations(conn, latest_shadow, latest_shadow.get("strategy_version"), "shadow:%")

    winner_candidate = str(
        winner_artifact.get("candidate_version")
        or promotion.get("candidate_version")
        or ""
    ).strip()
    promotion_candidate = str(promotion.get("candidate_version") or "").strip()
    shadow_candidate = str((latest_shadow or {}).get("strategy_version") or "").strip()

    winner_metrics = None
    if winner_candidate:
        winner_metrics = latest_row(
            conn,
            """
            SELECT strategy_version, mode, market_date, equity, cash, holding_count, order_count, fill_count
            FROM paper_daily_metrics
            WHERE strategy_version = ? AND mode LIKE 'shadow:%'
            ORDER BY id DESC
            LIMIT 1
            """,
            (winner_candidate,),
        )

    if winner_metrics is None and latest_shadow and latest_shadow.get("strategy_version") == winner_candidate:
        winner_metrics = latest_shadow

    if winner_metrics:
        attach_observations(conn, winner_metrics, winner_candidate, "shadow:%")

    winner_rank = winner_artifact.get("rank")

    active_shadow_delta = equity_delta(latest_shadow, latest_live)
    active_shadow_ratio = equity_ratio(latest_shadow, latest_live)
    active_winner_delta = equity_delta(winner_metrics, latest_live)
    active_winner_ratio = equity_ratio(winner_metrics, latest_live)
    shadow_winner_delta = equity_delta(winner_metrics, latest_shadow)
    shadow_winner_ratio = equity_ratio(winner_metrics, latest_shadow)

    challenger_state = canonical_challenger_state(winner_candidate, shadow_candidate, promotion_candidate)

    gate_status = "missing"
    gate_reason = "promotion report missing"
    if promotion:
        if challenger_state["sync_status"] == "promotion-stale":
            gate_status = "stale"
            gate_reason = challenger_state["sync_reason"]
        elif bool(promotion.get("promoted")):
            gate_status = "promoted"
            gate_reason = str(promotion.get("reason") or "candidate promoted")
        else:
            gate_status = "blocked"
            gate_reason = str(promotion.get("reason") or "candidate did not pass promotion gate")

    payload = {
        "generated_at": datetime.now(timezone.utc).astimezone().isoformat(),
        "active_strategy": active_strategy,
        "latest_live": latest_live,
        "latest_shadow": latest_shadow,
        "winner_artifact": winner_artifact or None,
        "winner_metrics": winner_metrics,
        "comparisons": {
            "shadow_vs_active": {
                "equity_delta": active_shadow_delta,
                "equity_delta_ratio": active_shadow_ratio,
            },
            "winner_vs_active": {
                "equity_delta": active_winner_delta,
                "equity_delta_ratio": active_winner_ratio,
            },
            "winner_vs_shadow": {
                "equity_delta": shadow_winner_delta,
                "equity_delta_ratio": shadow_winner_ratio,
            },
        },
        "promotion_gate": {
            "status": gate_status,
            "reason": gate_reason,
            "candidate_version": promotion_candidate or winner_candidate,
            "promoted": bool(promotion.get("promoted")) if promotion else False,
            "min_edge": safe_float(promotion.get("min_edge")) if promotion else None,
            "min_observations": safe_int(promotion.get("min_observations"), 0) if promotion else None,
            "min_observations_basis": str(promotion.get("min_observations_basis") or "distinct_market_days") if promotion else None,
            "min_winner_rank": safe_int(promotion.get("min_winner_rank"), 0) if promotion else None,
            "min_winner_equity_delta": safe_float(promotion.get("min_winner_equity_delta")) if promotion else None,
            "min_winner_return_delta": safe_float(promotion.get("min_winner_return_delta")) if promotion else None,
        },
        "winner_sync": challenger_state["sync_status"],
        "challenger_state": challenger_state,
        "winner_batch": {
            "report_tag": winner_artifact.get("report_tag"),
            "experiment_id": winner_artifact.get("experiment_id"),
            "rank": winner_rank,
            "rank_delta": safe_int(winner_artifact.get("rank_delta"), 0),
            "equity_delta": safe_float(winner_artifact.get("equity_delta")),
            "return_delta": safe_float(winner_artifact.get("return_delta")),
            "parameter_summary": winner_artifact.get("parameter_summary", ""),
        }
        if winner_artifact
        else None,
    }

    lines = [
        "Strategy Compare",
        "",
        f"Generated at: {payload['generated_at']}",
        f"Active registry version: {(active_strategy or {}).get('version_name', 'n/a')}",
        format_metric("Active live", latest_live),
        format_metric("Latest shadow", latest_shadow),
        format_metric("Winner shadow", winner_metrics),
        format_delta("Shadow vs active", active_shadow_delta, active_shadow_ratio),
        format_delta("Winner vs active", active_winner_delta, active_winner_ratio),
        format_delta("Winner vs shadow", shadow_winner_delta, shadow_winner_ratio),
        f"Challenger sync: {challenger_state['sync_status']}",
        f"Challenger reason: {challenger_state['sync_reason']}",
        f"Canonical challenger: {challenger_state['canonical_candidate_version'] or 'n/a'}",
        "",
        f"Promotion gate: {gate_status}",
        f"Promotion reason: {gate_reason}",
    ]

    gate = payload["promotion_gate"]
    if gate.get("candidate_version"):
        lines.append(f"Promotion candidate: {gate['candidate_version']}")
    if gate.get("min_observations") is not None:
        lines.append(
            "Promotion thresholds: "
            f"min_edge={safe_float(gate.get('min_edge')):.2f} "
            f"min_observations={safe_int(gate.get('min_observations'), 0)} ({gate.get('min_observations_basis')}) "
            f"winner_rank<={safe_int(gate.get('min_winner_rank'), 0)} "
            f"winner_equity_delta>={safe_float(gate.get('min_winner_equity_delta')):.2f} "
            f"winner_return_delta>={safe_float(gate.get('min_winner_return_delta')):.6f}"
        )

    if winner_artifact:
        lines.extend(
            [
                "",
                "Winner batch:",
                f"- report: {winner_artifact.get('report_tag', 'n/a')}",
                f"- experiment: {winner_artifact.get('experiment_id', 'n/a')}",
                f"- candidate: {winner_candidate or 'n/a'}",
                f"- rank: {winner_rank if winner_rank is not None else 'n/a'}",
                f"- rank delta: {safe_int(winner_artifact.get('rank_delta'), 0)}",
                f"- equity delta: {safe_float(winner_artifact.get('equity_delta')):.2f}",
                f"- return delta: {safe_float(winner_artifact.get('return_delta')):.6f}",
                f"- params: {winner_artifact.get('parameter_summary', 'n/a')}",
            ]
        )

    text = "\n".join(lines) + "\n"
    (REPORTS / "strategy_compare_latest.txt").write_text(text, encoding="utf-8")
    (REPORTS / "strategy_compare_latest.json").write_text(
        json.dumps(payload, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    (REPORTS / "strategy_compare_latest.html").write_text(
        "<!doctype html><html><head><meta charset='utf-8'><title>Strategy Compare</title>"
        "<style>body{font-family:Georgia,serif;background:#f7f1e7;color:#1f1b16;padding:32px}"
        ".card{max-width:1080px;background:#fffaf3;border:1px solid #d8cebe;border-radius:18px;padding:24px}"
        "pre{white-space:pre-wrap;font:14px/1.6 Menlo,monospace}</style>"
        f"</head><body><div class='card'><h1>Strategy Compare</h1><pre>{escape(text)}</pre></div></body></html>",
        encoding="utf-8",
    )
    print("strategy compare report generated")


if __name__ == "__main__":
    main()
