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
    parser.add_argument("--candidate", default="", help="Candidate strategy version name. Defaults to the latest paper-trial winner when available.")
    parser.add_argument("--reports-dir", default="reports", help="Directory for promotion reports.")
    parser.add_argument("--winner-artifact", default="reports/paper_trial_winner_latest.json", help="Paper-trial winner artifact used to enrich promotion decisions.")
    parser.add_argument("--min-edge", type=float, default=0.0, help="Minimum required equity edge over active.")
    parser.add_argument("--min-observations", type=int, default=1, help="Minimum paper metric observations required for the candidate.")
    parser.add_argument("--min-winner-rank", type=int, default=1, help="Require the winner artifact rank to be at or above this threshold (1 means best rank only).")
    parser.add_argument("--min-winner-equity-delta", type=float, default=0.0, help="Minimum winner equity delta versus the previous batch.")
    parser.add_argument("--min-winner-return-delta", type=float, default=0.0, help="Minimum winner return delta versus the previous batch.")
    parser.add_argument("--allow-regressed-winner", action="store_true", help="Allow promotion even if the latest paper-trial winner regressed versus the previous batch.")
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


def load_winner_artifact(path_str):
    path = Path(path_str)
    if not path.exists():
        return None
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return None


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
        f"Min edge: {summary.get('min_edge')}",
        f"Min observations: {summary.get('min_observations')}",
        f"Winner gate applied: {summary.get('winner_gate_applied')}",
        f"Min winner rank: {summary.get('min_winner_rank')}",
        f"Min winner equity delta: {summary.get('min_winner_equity_delta')}",
        f"Min winner return delta: {summary.get('min_winner_return_delta')}",
    ]
    if summary.get("edge") is not None:
        lines.append(f"Edge: {summary.get('edge'):.6f}")
    active_metrics = summary.get("active_metrics") or {}
    candidate_metrics = summary.get("candidate_metrics") or {}
    winner_artifact = summary.get("winner_artifact") or {}
    if active_metrics:
        lines.append(f"Active metrics: date={active_metrics.get('market_date')} equity={active_metrics.get('equity')} observations={active_metrics.get('observations')}")
    if candidate_metrics:
        lines.append(f"Candidate metrics: date={candidate_metrics.get('market_date')} equity={candidate_metrics.get('equity')} observations={candidate_metrics.get('observations')}")
    if winner_artifact:
        lines.append(
            f"Winner artifact: report={winner_artifact.get('report_tag')} experiment={winner_artifact.get('experiment_id')} "
            f"rank={winner_artifact.get('rank')} equity_delta={winner_artifact.get('equity_delta')} return_delta={winner_artifact.get('return_delta')}"
        )
    text_path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def build_summary(args, market, active_version, candidate_version, active_metrics, candidate_metrics, winner_artifact, edge, promoted, reason, winner_gate_applied):
    return {
        "market": market,
        "active_version": active_version,
        "candidate_version": candidate_version,
        "active_metrics": active_metrics,
        "candidate_metrics": candidate_metrics,
        "winner_artifact": winner_artifact,
        "edge": edge,
        "min_edge": args.min_edge,
        "min_observations": args.min_observations,
        "winner_gate_applied": winner_gate_applied,
        "min_winner_rank": args.min_winner_rank,
        "min_winner_equity_delta": args.min_winner_equity_delta,
        "min_winner_return_delta": args.min_winner_return_delta,
        "promoted": promoted,
        "dry_run": args.dry_run,
        "reason": reason,
    }


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
    winner_artifact = load_winner_artifact(args.winner_artifact)
    candidate_version = args.candidate.strip()
    if not candidate_version and winner_artifact:
        candidate_version = str(winner_artifact.get("candidate_version") or "").strip()
    if not candidate_version:
        raise SystemExit("candidate strategy not provided and no winner artifact candidate available")

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
        (args.market, candidate_version),
    )
    if candidate is None:
        raise SystemExit(f"candidate strategy not found: {candidate_version}")

    if active["version_name"] == candidate_version:
        summary = build_summary(args, args.market, active["version_name"], candidate_version, None, None, winner_artifact, None, False, "candidate is already the active strategy", False)
        write_report(args.reports_dir, summary)
        record_decision(conn, args.market, "noop", active["version_name"], candidate_version, summary["reason"], summary, args.dry_run)
        print(json.dumps(summary, ensure_ascii=False, indent=2))
        return

    active_metrics = fetch_metrics(conn, "live", active["version_name"])
    candidate_metrics = fetch_metrics(conn, f"shadow:{candidate_version}", candidate_version)
    if active_metrics is None or candidate_metrics is None:
        summary = build_summary(args, args.market, active["version_name"], candidate_version, active_metrics, candidate_metrics, winner_artifact, None, False, "missing active or candidate paper metrics", False)
        write_report(args.reports_dir, summary)
        record_decision(conn, args.market, "skip", active["version_name"], candidate_version, summary["reason"], summary, args.dry_run)
        print(json.dumps(summary, ensure_ascii=False, indent=2))
        return
    if active_metrics["market_date"] != candidate_metrics["market_date"]:
        summary = build_summary(
            args,
            args.market,
            active["version_name"],
            candidate_version,
            active_metrics,
            candidate_metrics,
            winner_artifact,
            None,
            False,
            f"market date mismatch: active={active_metrics['market_date']} candidate={candidate_metrics['market_date']}",
            False,
        )
        write_report(args.reports_dir, summary)
        record_decision(conn, args.market, "skip", active["version_name"], candidate_version, summary["reason"], summary, args.dry_run)
        print(json.dumps(summary, ensure_ascii=False, indent=2))
        return
    if candidate_metrics["observations"] < args.min_observations:
        summary = build_summary(
            args,
            args.market,
            active["version_name"],
            candidate_version,
            active_metrics,
            candidate_metrics,
            winner_artifact,
            None,
            False,
            f"candidate observations {candidate_metrics['observations']} < {args.min_observations}",
            False,
        )
        write_report(args.reports_dir, summary)
        record_decision(conn, args.market, "skip", active["version_name"], candidate_version, summary["reason"], summary, args.dry_run)
        print(json.dumps(summary, ensure_ascii=False, indent=2))
        return
    winner_gate_applied = bool(winner_artifact) and str(winner_artifact.get("candidate_version") or "").strip() == candidate_version
    if winner_gate_applied:
        winner_rank = int(winner_artifact.get("rank") or 0)
        if args.min_winner_rank > 0 and winner_rank <= 0:
            summary = build_summary(
                args,
                args.market,
                active["version_name"],
                candidate_version,
                active_metrics,
                candidate_metrics,
                winner_artifact,
                None,
                False,
                "winner artifact rank is missing",
                True,
            )
            write_report(args.reports_dir, summary)
            record_decision(conn, args.market, "skip", active["version_name"], candidate_version, summary["reason"], summary, args.dry_run)
            print(json.dumps(summary, ensure_ascii=False, indent=2))
            return
        if args.min_winner_rank > 0 and winner_rank > args.min_winner_rank:
            summary = build_summary(
                args,
                args.market,
                active["version_name"],
                candidate_version,
                active_metrics,
                candidate_metrics,
                winner_artifact,
                None,
                False,
                f"winner rank {winner_rank} is worse than required {args.min_winner_rank}",
                True,
            )
            write_report(args.reports_dir, summary)
            record_decision(conn, args.market, "skip", active["version_name"], candidate_version, summary["reason"], summary, args.dry_run)
            print(json.dumps(summary, ensure_ascii=False, indent=2))
            return
        equity_delta = float(winner_artifact.get("equity_delta") or 0.0)
        return_delta = float(winner_artifact.get("return_delta") or 0.0)
        has_previous = bool(winner_artifact.get("previous_rank")) or abs(float(winner_artifact.get("previous_equity") or 0.0)) > 1e-9
        if has_previous and not args.allow_regressed_winner and equity_delta < args.min_winner_equity_delta:
            summary = build_summary(
                args,
                args.market,
                active["version_name"],
                candidate_version,
                active_metrics,
                candidate_metrics,
                winner_artifact,
                None,
                False,
                f"winner artifact equity_delta {equity_delta:.2f} < required {args.min_winner_equity_delta:.2f}",
                True,
            )
            write_report(args.reports_dir, summary)
            record_decision(conn, args.market, "skip", active["version_name"], candidate_version, summary["reason"], summary, args.dry_run)
            print(json.dumps(summary, ensure_ascii=False, indent=2))
            return
        if has_previous and not args.allow_regressed_winner and return_delta < args.min_winner_return_delta:
            summary = build_summary(
                args,
                args.market,
                active["version_name"],
                candidate_version,
                active_metrics,
                candidate_metrics,
                winner_artifact,
                None,
                False,
                f"winner artifact return_delta {return_delta:.6f} < required {args.min_winner_return_delta:.6f}",
                True,
            )
            write_report(args.reports_dir, summary)
            record_decision(conn, args.market, "skip", active["version_name"], candidate_version, summary["reason"], summary, args.dry_run)
            print(json.dumps(summary, ensure_ascii=False, indent=2))
            return

    edge = candidate_metrics["equity"] - active_metrics["equity"]
    promoted = edge >= args.min_edge

    summary = build_summary(
        args,
        args.market,
        active["version_name"],
        candidate_version,
        active_metrics,
        candidate_metrics,
        winner_artifact,
        edge,
        promoted,
        f"shadow equity edge {edge:.2f} {'>=' if promoted else '<'} min_edge {args.min_edge:.2f}",
        winner_gate_applied,
    )

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
                (now, args.market, candidate_version),
            )
            conn.execute(
                "INSERT INTO strategy_promotions (event_type, market, from_version, to_version, trigger_reason, metrics_json, recorded_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
                ("promotion", args.market, active["version_name"], candidate_version, summary["reason"], metrics_json, now),
            )
    elif not promoted:
        record_decision(conn, args.market, "skip", active["version_name"], candidate_version, summary["reason"], summary, args.dry_run)

    write_report(args.reports_dir, summary)
    print(json.dumps(summary, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
