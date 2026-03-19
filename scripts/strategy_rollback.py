#!/usr/bin/env python3
import argparse
import json
import sqlite3
from datetime import datetime
from pathlib import Path


def parse_args():
    parser = argparse.ArgumentParser(description="Rollback the active strategy to a previous archived version.")
    parser.add_argument("--db", default="data/quant.db", help="SQLite database path.")
    parser.add_argument("--market", default="a_share", help="Market name.")
    parser.add_argument("--to-version", required=True, help="Archived strategy version to restore as active.")
    parser.add_argument("--reason", default="manual rollback", help="Rollback reason.")
    parser.add_argument("--dry-run", action="store_true", help="Evaluate without writing changes.")
    return parser.parse_args()


def fetch_one(conn, query, params=()):
    return conn.execute(query, params).fetchone()


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
        "SELECT version_name, status FROM strategy_registry WHERE market = ? AND version_name = ? ORDER BY id DESC LIMIT 1",
        (args.market, args.to_version),
    )
    if target is None:
        raise SystemExit(f"target version not found: {args.to_version}")

    summary = {
        "market": args.market,
        "active_version": active["version_name"],
        "rollback_to": args.to_version,
        "target_status": target["status"],
        "dry_run": args.dry_run,
        "rolled_back": False,
        "reason": args.reason,
    }

    if active["version_name"] == args.to_version:
        summary["reason"] = "target version is already active"
        print(json.dumps(summary, ensure_ascii=False, indent=2))
        return

    if target["status"] not in {"archived", "shadow"}:
        summary["reason"] = f"target version status not eligible: {target['status']}"
        print(json.dumps(summary, ensure_ascii=False, indent=2))
        return

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
                (now, args.market, args.to_version),
            )
            conn.execute(
                """
                INSERT INTO strategy_promotions (event_type, market, from_version, to_version, trigger_reason, metrics_json, recorded_at)
                VALUES (?, ?, ?, ?, ?, ?, ?)
                """,
                ("rollback", args.market, active["version_name"], args.to_version, args.reason, metrics_json, now),
            )
        summary["rolled_back"] = True

    print(json.dumps(summary, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
