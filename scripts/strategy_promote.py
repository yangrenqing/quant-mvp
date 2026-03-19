#!/usr/bin/env python3
import argparse
import json
import sqlite3
from datetime import datetime
from pathlib import Path


def parse_args():
    parser = argparse.ArgumentParser(description="Promote a shadow strategy to active when it beats the current active strategy.")
    parser.add_argument("--db", default="data/quant.db", help="SQLite database path.")
    parser.add_argument("--market", default="a_share", help="Market name.")
    parser.add_argument("--candidate", required=True, help="Candidate strategy version name.")
    parser.add_argument("--reports-dir", default="reports", help="Directory for promotion reports.")
    parser.add_argument("--min-edge", type=float, default=0.0, help="Minimum required equity edge over active.")
    parser.add_argument("--min-observations", type=int, default=1, help="Minimum paper metric observations required for the candidate.")
    parser.add_argument("--dry-run", action="store_true", help="Evaluate promotion without writing changes.")
    return parser.parse_args()


def fetch_one(conn, query, params=()):
    row = conn.execute(query, params).fetchone()
    return row


def fetch_metrics(conn, mode_filter, strategy_version):
    row = fetch_one(
        conn,
        """
        SELECT strategy_version, mode, market_date, equity, cash, holding_count, order_count, fill_count
        FROM paper_daily_metrics
        WHERE strategy_version = ? AND mode LIKE ?
        ORDER BY id DESC
        LIMIT 1
        """,
        (strategy_version, mode_filter),
    )
    if row is None:
        return None
    count_row = fetch_one(
        conn,
        """
        SELECT COUNT(*)
        FROM paper_daily_metrics
        WHERE strategy_version = ? AND mode LIKE ?
        """,
        (strategy_version, mode_filter),
    )
    observations = count_row[0] if count_row is not None else 0
    return {
        "strategy_version": row[0],
        "mode": row[1],
        "market_date": row[2],
        "equity": row[3],
        "cash": row[4],
        "holding_count": row[5],
        "order_count": row[6],
        "fill_count": row[7],
        "observations": observations,
    }


def write_report(reports_dir, summary):
    reports_path = Path(reports_dir)
    reports_path.mkdir(parents=True, exist_ok=True)
    json_path = reports_path / "strategy_promotion_latest.json"
    text_path = reports_path / "strategy_promotion_latest.txt"
    json_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

    lines = [
        "Strategy Promotion Decision",
        "",
        f"Market: {summary.get('market')}",
        f"Active: {summary.get('active_version')}",
        f"Candidate: {summary.get('candidate_version')}",
        f"Promoted: {summary.get('promoted')}",
        f"Dry run: {summary.get('dry_run')}",
        f"Reason: {summary.get('reason', '')}",
    ]
    if summary.get("edge") is not None:
        lines.append(f"Edge: {summary.get('edge'):.6f}")
    active_metrics = summary.get("active_metrics") or {}
    candidate_metrics = summary.get("candidate_metrics") or {}
    if active_metrics:
        lines.append(f"Active metrics: date={active_metrics.get('market_date')} equity={active_metrics.get('equity')} observations={active_metrics.get('observations')}")
    if candidate_metrics:
        lines.append(f"Candidate metrics: date={candidate_metrics.get('market_date')} equity={candidate_metrics.get('equity')} observations={candidate_metrics.get('observations')}")
    text_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def record_decision(conn, market, event_type, from_version, to_version, reason, summary, dry_run):
    if dry_run:
        return
    with conn:
        conn.execute(
            """
            INSERT INTO strategy_promotions (event_type, market, from_version, to_version, trigger_reason, metrics_json, recorded_at)
            VALUES (?, ?, ?, ?, ?, ?, ?)
            """,
            (
                event_type,
                market,
                from_version,
                to_version,
                reason,
                json.dumps(summary, ensure_ascii=False),
                datetime.now().isoformat(timespec="seconds"),
            ),
        )


def main():
    args = parse_args()
    db_path = Path(args.db)
    conn = sqlite3.connect(db_path)
    conn.row_factory = sqlite3.Row

    active = fetch_one(
        conn,
        """
        SELECT version_name, git_commit
        FROM strategy_registry
        WHERE market = ? AND status = 'active'
        ORDER BY id DESC
        LIMIT 1
        """,
        (args.market,),
    )
    if active is None:
        raise SystemExit("no active strategy found")

    candidate = fetch_one(
        conn,
        """
        SELECT version_name, parent_version, git_commit
        FROM strategy_registry
        WHERE market = ? AND version_name = ?
        ORDER BY id DESC
        LIMIT 1
        """,
        (args.market, args.candidate),
    )
    if candidate is None:
        raise SystemExit(f"candidate strategy not found: {args.candidate}")

    if active["version_name"] == args.candidate:
        summary = {
            "market": args.market,
            "active_version": active["version_name"],
            "candidate_version": args.candidate,
            "active_metrics": None,
            "candidate_metrics": None,
            "edge": None,
            "min_edge": args.min_edge,
            "promoted": False,
            "dry_run": args.dry_run,
            "reason": "candidate is already the active strategy",
        }
        write_report(args.reports_dir, summary)
        record_decision(conn, args.market, "noop", active["version_name"], args.candidate, summary["reason"], summary, args.dry_run)
        print(json.dumps(summary, ensure_ascii=False, indent=2))
        return

    active_metrics = fetch_metrics(conn, "live", active["version_name"])
    candidate_metrics = fetch_metrics(conn, f"shadow:{args.candidate}", args.candidate)
    if active_metrics is None or candidate_metrics is None:
        summary = {
            "market": args.market,
            "active_version": active["version_name"],
            "candidate_version": args.candidate,
            "active_metrics": active_metrics,
            "candidate_metrics": candidate_metrics,
            "edge": None,
            "min_edge": args.min_edge,
            "promoted": False,
            "dry_run": args.dry_run,
            "reason": "missing active or candidate paper metrics",
        }
        write_report(args.reports_dir, summary)
        record_decision(conn, args.market, "skip", active["version_name"], args.candidate, summary["reason"], summary, args.dry_run)
        print(json.dumps(summary, ensure_ascii=False, indent=2))
        return
    if active_metrics["market_date"] != candidate_metrics["market_date"]:
        summary = {
            "market": args.market,
            "active_version": active["version_name"],
            "candidate_version": args.candidate,
            "active_metrics": active_metrics,
            "candidate_metrics": candidate_metrics,
            "edge": None,
            "min_edge": args.min_edge,
            "promoted": False,
            "dry_run": args.dry_run,
            "reason": f"market date mismatch: active={active_metrics['market_date']} candidate={candidate_metrics['market_date']}",
        }
        write_report(args.reports_dir, summary)
        record_decision(conn, args.market, "skip", active["version_name"], args.candidate, summary["reason"], summary, args.dry_run)
        print(json.dumps(summary, ensure_ascii=False, indent=2))
        return
    if candidate_metrics["observations"] < args.min_observations:
        summary = {
            "market": args.market,
            "active_version": active["version_name"],
            "candidate_version": args.candidate,
            "active_metrics": active_metrics,
            "candidate_metrics": candidate_metrics,
            "edge": None,
            "min_edge": args.min_edge,
            "promoted": False,
            "dry_run": args.dry_run,
            "reason": f"candidate observations {candidate_metrics['observations']} < {args.min_observations}",
        }
        write_report(args.reports_dir, summary)
        record_decision(conn, args.market, "skip", active["version_name"], args.candidate, summary["reason"], summary, args.dry_run)
        print(json.dumps(summary, ensure_ascii=False, indent=2))
        return

    edge = candidate_metrics["equity"] - active_metrics["equity"]
    promoted = edge >= args.min_edge

    summary = {
        "market": args.market,
        "active_version": active["version_name"],
        "candidate_version": args.candidate,
        "active_metrics": active_metrics,
        "candidate_metrics": candidate_metrics,
        "edge": edge,
        "min_edge": args.min_edge,
        "promoted": promoted,
        "dry_run": args.dry_run,
        "reason": f"shadow equity edge {edge:.2f} {'>=' if promoted else '<'} min_edge {args.min_edge:.2f}",
    }

    if promoted and not args.dry_run:
        now = datetime.now().isoformat(timespec="seconds")
        metrics_json = json.dumps(summary, ensure_ascii=False)
        with conn:
            conn.execute(
                "UPDATE strategy_registry SET status = 'archived', archived_at = ? WHERE market = ? AND status = 'active'",
                (now, args.market),
            )
            conn.execute(
                "UPDATE strategy_registry SET status = 'active', activated_at = ? WHERE market = ? AND version_name = ?",
                (now, args.market, args.candidate),
            )
            conn.execute(
                "INSERT INTO strategy_promotions (event_type, market, from_version, to_version, trigger_reason, metrics_json, recorded_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
                ("promotion", args.market, active["version_name"], args.candidate, summary["reason"], metrics_json, now),
            )
    elif not promoted:
        record_decision(conn, args.market, "skip", active["version_name"], args.candidate, summary["reason"], summary, args.dry_run)

    write_report(args.reports_dir, summary)
    print(json.dumps(summary, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
