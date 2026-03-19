#!/usr/bin/env python3
import argparse
import json
import sqlite3
from collections import Counter
from datetime import datetime, timedelta
from html import escape
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
REPORTS = ROOT / "reports"
DB_PATH = ROOT / "data" / "quant.db"
MODEL_REGISTRY = REPORTS / "model_registry.jsonl"


def read_jsonl(path: Path):
    if not path.exists():
        return []
    records = []
    for line in path.read_text(encoding="utf-8").splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            records.append(json.loads(line))
        except json.JSONDecodeError:
            continue
    return records


def parse_when(value: str):
    if not value:
        return None
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None


def main():
    parser = argparse.ArgumentParser(description="Build an evolution report for recent strategy/model changes.")
    parser.add_argument("--hours", type=int, default=24, help="Lookback window in hours.")
    parser.add_argument("--preset", choices=["rolling", "overnight"], default="rolling", help="Named reporting window.")
    args = parser.parse_args()

    now = datetime.now().astimezone()
    if args.preset == "overnight":
        start = now.replace(hour=15, minute=0, second=0, microsecond=0) - timedelta(days=1)
        end = now.replace(hour=9, minute=30, second=0, microsecond=0)
        if now < end:
            end = now
    else:
        start = now - timedelta(hours=args.hours)
        end = now

    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row

    run_rows = conn.execute(
        "SELECT run_type, generated_at FROM run_history WHERE generated_at >= ? AND generated_at <= ? ORDER BY generated_at DESC",
        (start.isoformat(), end.isoformat()),
    ).fetchall()
    promo_rows = conn.execute(
        "SELECT event_type, from_version, to_version, trigger_reason, recorded_at FROM strategy_promotions WHERE recorded_at >= ? AND recorded_at <= ? ORDER BY recorded_at DESC",
        (start.isoformat(), end.isoformat()),
    ).fetchall()
    metric_rows = conn.execute(
        "SELECT strategy_version, mode, market_date, equity, order_count, recorded_at FROM paper_daily_metrics WHERE recorded_at >= ? AND recorded_at <= ? ORDER BY recorded_at DESC",
        (start.isoformat(), end.isoformat()),
    ).fetchall()
    registry_rows = conn.execute(
        "SELECT version_name, status, activated_at, archived_at, created_at FROM strategy_registry ORDER BY id DESC LIMIT 20"
    ).fetchall()

    run_counter = Counter(row["run_type"] for row in run_rows)
    promo_counter = Counter(row["event_type"] for row in promo_rows)

    active_latest = next((row for row in metric_rows if row["mode"] == "live"), None)
    shadow_latest = next((row for row in metric_rows if str(row["mode"]).startswith("shadow:")), None)
    equity_diff = None
    if active_latest and shadow_latest:
        equity_diff = float(shadow_latest["equity"]) - float(active_latest["equity"])

    model_records = []
    for record in read_jsonl(MODEL_REGISTRY):
        version_dir = record.get("version_dir", "")
        timestamp = record.get("timestamp", "")
        record_time = None
        if timestamp:
            try:
                record_time = datetime.strptime(timestamp, "%Y%m%d_%H%M%S").astimezone()
            except ValueError:
                record_time = None
        if record_time and start <= record_time <= end:
            model_records.append(record)

    model_promotions = 0
    classifier_promotions = 0
    for record in model_records:
        regression = record.get("regression") or {}
        classifier = record.get("classifier") or {}
        if regression.get("promoted"):
            model_promotions += 1
        if classifier.get("promoted"):
            classifier_promotions += 1

    lines = [
        "Evolution Report",
        "",
        f"Preset: {args.preset}",
        f"Window start: {start.isoformat()}",
        f"Window end: {end.isoformat()}",
        f"Generated at: {now.isoformat()}",
        "",
        f"Run count: {len(run_rows)}",
        "Runs by type:",
    ]
    if run_counter:
        for run_type, count in sorted(run_counter.items()):
            lines.append(f"- {run_type}: {count}")
    else:
        lines.append("- none")

    lines.extend(
        [
            "",
            f"Lifecycle events: {len(promo_rows)}",
            "Lifecycle by type:",
        ]
    )
    if promo_counter:
        for event_type, count in sorted(promo_counter.items()):
            lines.append(f"- {event_type}: {count}")
    else:
        lines.append("- none")

    lines.extend(
        [
            "",
            f"Model versions built: {len(model_records)}",
            f"Regression promotions: {model_promotions}",
            f"Classifier promotions: {classifier_promotions}",
        ]
    )

    lines.append("")
    if active_latest:
        lines.append(
            f"Active latest: {active_latest['strategy_version']} equity={float(active_latest['equity']):.2f} orders={active_latest['order_count']} market_date={active_latest['market_date']}"
        )
    else:
        lines.append("Active latest: none")
    if shadow_latest:
        lines.append(
            f"Shadow latest: {shadow_latest['strategy_version']} equity={float(shadow_latest['equity']):.2f} orders={shadow_latest['order_count']} market_date={shadow_latest['market_date']}"
        )
    else:
        lines.append("Shadow latest: none")
    if equity_diff is not None:
        lines.append(f"Active vs shadow equity diff: {equity_diff:.2f}")

    lines.extend(["", "Recent lifecycle events:"])
    if promo_rows:
        for row in promo_rows[:10]:
            lines.append(
                f"- {row['recorded_at']} {row['event_type']} {row['from_version']} -> {row['to_version']} ({row['trigger_reason']})"
            )
    else:
        lines.append("- none")

    lines.extend(["", "Recent strategy registry:"])
    for row in registry_rows[:10]:
        lines.append(
            f"- {row['version_name']} status={row['status']} created={row['created_at']} activated={row['activated_at'] or '-'} archived={row['archived_at'] or '-'}"
        )

    summary = {
        "preset": args.preset,
        "window_hours": args.hours,
        "window_start": start.isoformat(),
        "window_end": end.isoformat(),
        "generated_at": now.isoformat(),
        "run_count": len(run_rows),
        "run_types": dict(run_counter),
        "lifecycle_event_count": len(promo_rows),
        "lifecycle_event_types": dict(promo_counter),
        "model_versions_built": len(model_records),
        "regression_promotions": model_promotions,
        "classifier_promotions": classifier_promotions,
        "active_latest": dict(active_latest) if active_latest else None,
        "shadow_latest": dict(shadow_latest) if shadow_latest else None,
        "active_shadow_equity_diff": equity_diff,
        "recent_events": [dict(row) for row in promo_rows[:10]],
    }

    REPORTS.mkdir(parents=True, exist_ok=True)
    suffix = "" if args.preset == "rolling" else "_overnight"
    text_path = REPORTS / f"evolution_report{suffix}.txt"
    html_path = REPORTS / f"evolution_report{suffix}.html"
    json_path = REPORTS / f"evolution_report{suffix}.json"

    text = "\n".join(lines) + "\n"
    text_path.write_text(text, encoding="utf-8")
    json_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    html_path.write_text(
        "<!doctype html><html><head><meta charset='utf-8'><title>Evolution Report</title>"
        "<style>body{font-family:Georgia,serif;background:#f7f1e7;color:#1f1b16;padding:32px;}"
        ".card{max-width:980px;background:#fffaf3;border:1px solid #d8cebe;border-radius:18px;padding:24px;}"
        "pre{white-space:pre-wrap;font:14px/1.6 Menlo,monospace;}</style></head><body>"
        f"<div class='card'><h1>Evolution Report</h1><pre>{escape(text)}</pre></div></body></html>",
        encoding="utf-8",
    )
    print(f"evolution report generated for preset={args.preset}")


if __name__ == "__main__":
    main()
