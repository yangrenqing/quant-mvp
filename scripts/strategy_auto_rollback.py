#!/usr/bin/env python3
import argparse
import json
import sqlite3
from datetime import datetime
from pathlib import Path


def parse_args():
    parser = argparse.ArgumentParser(description="Automatically rollback the active strategy when a recent archived strategy is stronger.")
    parser.add_argument("--db", default="data/quant.db", help="SQLite database path.")
    parser.add_argument("--market", default="a_share", help="Market name.")
    parser.add_argument("--reports-dir", default="reports", help="Directory for rollback reports.")
    parser.add_argument("--min-edge", type=float, default=0.0, help="Minimum equity edge required for rollback.")
    parser.add_argument("--dry-run", action="store_true", help="Evaluate without writing changes.")
    return parser.parse_args()


def fetch_one(conn, query, params=()):
    return conn.execute(query, params).fetchone()


def fetch_metrics(conn, strategy_version, mode_filter="live"):
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
    return {
        "strategy_version": row[0],
        "mode": row[1],
        "market_date": row[2],
        "equity": row[3],
        "cash": row[4],
        "holding_count": row[5],
        "order_count": row[6],
        "fill_count": row[7],
    }


def write_report(reports_dir, summary):
    reports_path = Path(reports_dir)
    reports_path.mkdir(parents=True, exist_ok=True)
    json_path = reports_path / "strategy_rollback_latest.json"
    text_path = reports_path / "strategy_rollback_latest.txt"
    json_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

    lines = [
        "Strategy Rollback Decision",
        "",
        f"Market: {summary.get('market')}",
        f"Active: {summary.get('active_version')}",
        f"Rollback target: {summary.get('rollback_to')}",
        f"Rolled back: {summary.get('rolled_back')}",
        f"Dry run: {summary.get('dry_run')}",
        f"Reason: {summary.get('reason')}",
    ]
    if summary.get("edge") is not None:
        lines.append(f"Edge: {summary.get('edge'):.6f}")
    if summary.get("active_metrics"):
        lines.append(
            f"Active metrics: date={summary['active_metrics'].get('market_date')} equity={summary['active_metrics'].get('equity')}"
        )
    if summary.get("target_metrics"):
        lines.append(
            f"Target metrics: date={summary['target_metrics'].get('market_date')} equity={summary['target_metrics'].get('equity')}"
        )
    text_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main():
    args = parse_args()
    conn = sqlite3.connect(Path(args.db))
    conn.row_factory = sqlite3.Row

    active = fetch_one(
        conn,
        "SELECT version_name FROM strategy_registry WHERE market = ? AND status = 'active' ORDER BY id DESC LIMIT 1",
        (args.market,),
    )
    if active is None:
        raise SystemExit("no active strategy found")

    target = fetch_one(
        conn,
        """
        SELECT version_name, parent_version, archived_at
        FROM strategy_registry
        WHERE market = ? AND status = 'archived' AND version_name != ?
        ORDER BY archived_at DESC, id DESC
        LIMIT 1
        """,
        (args.market, active["version_name"]),
    )

    active_metrics = fetch_metrics(conn, active["version_name"], "live")
    target_metrics = fetch_metrics(conn, target["version_name"], "live") if target is not None else None

    summary = {
        "market": args.market,
        "active_version": active["version_name"],
        "rollback_to": target["version_name"] if target is not None else "",
        "active_metrics": active_metrics,
        "target_metrics": target_metrics,
        "edge": None,
        "min_edge": args.min_edge,
        "rolled_back": False,
        "dry_run": args.dry_run,
        "reason": "",
    }

    if target is None:
        summary["reason"] = "no archived strategy available for rollback"
        write_report(args.reports_dir, summary)
        print(json.dumps(summary, ensure_ascii=False, indent=2))
        return

    if active_metrics is None or target_metrics is None:
        summary["reason"] = "missing active or rollback-target live metrics"
        write_report(args.reports_dir, summary)
        print(json.dumps(summary, ensure_ascii=False, indent=2))
        return

    if active_metrics["market_date"] != target_metrics["market_date"]:
        summary["reason"] = (
            f"market date mismatch: active={active_metrics['market_date']} target={target_metrics['market_date']}"
        )
        write_report(args.reports_dir, summary)
        print(json.dumps(summary, ensure_ascii=False, indent=2))
        return

    edge = target_metrics["equity"] - active_metrics["equity"]
    summary["edge"] = edge
    if edge < args.min_edge:
        summary["reason"] = f"rollback target edge {edge:.2f} < min_edge {args.min_edge:.2f}"
        write_report(args.reports_dir, summary)
        print(json.dumps(summary, ensure_ascii=False, indent=2))
        return

    summary["reason"] = f"rollback target edge {edge:.2f} >= min_edge {args.min_edge:.2f}"
    if not args.dry_run:
        now = datetime.now().isoformat(timespec="seconds")
        metrics_json = json.dumps(summary, ensure_ascii=False)
        with conn:
            conn.execute(
                "UPDATE strategy_registry SET status = 'archived', archived_at = ? WHERE market = ? AND status = 'active'",
                (now, args.market),
            )
            conn.execute(
                "UPDATE strategy_registry SET status = 'active', activated_at = ?, archived_at = '' WHERE market = ? AND version_name = ?",
                (now, args.market, target["version_name"]),
            )
            conn.execute(
                """
                INSERT INTO strategy_promotions (event_type, market, from_version, to_version, trigger_reason, metrics_json, recorded_at)
                VALUES (?, ?, ?, ?, ?, ?, ?)
                """,
                ("rollback", args.market, active["version_name"], target["version_name"], summary["reason"], metrics_json, now),
            )
        summary["rolled_back"] = True

    write_report(args.reports_dir, summary)
    print(json.dumps(summary, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
