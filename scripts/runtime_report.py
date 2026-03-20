#!/usr/bin/env python3
import json
import sqlite3
from collections import Counter
from datetime import datetime
from html import escape
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
REPORTS = ROOT / "reports"
DB_PATH = ROOT / "data" / "quant.db"


def parse_time(value):
    if not value:
        return None
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None


def fmt_duration(start, end):
    if not start or not end or end < start:
        return "0m"
    delta = end - start
    total_seconds = int(delta.total_seconds())
    days = total_seconds // 86400
    hours = (total_seconds % 86400) // 3600
    minutes = (total_seconds % 3600) // 60
    parts = []
    if days:
        parts.append(f"{days}d")
    if hours or days:
        parts.append(f"{hours}h")
    parts.append(f"{minutes}m")
    return " ".join(parts)


def main():
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row

    rows = conn.execute(
        "SELECT run_type, generated_at FROM run_history ORDER BY generated_at ASC"
    ).fetchall()
    if not rows:
        raise SystemExit("run_history is empty")

    first = parse_time(rows[0]["generated_at"])
    last = parse_time(rows[-1]["generated_at"])
    now = datetime.now().astimezone()
    run_counter = Counter(row["run_type"] for row in rows)

    last24_rows = conn.execute(
        "SELECT run_type FROM run_history WHERE generated_at >= datetime('now', '-24 hours')"
    ).fetchall()
    last24_counter = Counter(row["run_type"] for row in last24_rows)

    payload = {
        "started_at": rows[0]["generated_at"],
        "latest_at": rows[-1]["generated_at"],
        "generated_at": now.isoformat(),
        "runtime": fmt_duration(first, last),
        "total_runs": len(rows),
        "runs_by_type": dict(run_counter),
        "last_24h_runs": len(last24_rows),
        "last_24h_by_type": dict(last24_counter),
    }

    lines = [
        "Runtime Report",
        "",
        f"Started at: {payload['started_at']}",
        f"Latest run at: {payload['latest_at']}",
        f"Generated at: {payload['generated_at']}",
        f"Runtime: {payload['runtime']}",
        f"Total runs: {payload['total_runs']}",
        "",
        "Runs by type:",
    ]
    for run_type, count in sorted(run_counter.items()):
        lines.append(f"- {run_type}: {count}")
    lines.extend(["", f"Last 24h runs: {payload['last_24h_runs']}", "Last 24h by type:"])
    for run_type, count in sorted(last24_counter.items()):
        lines.append(f"- {run_type}: {count}")

    REPORTS.mkdir(parents=True, exist_ok=True)
    text = "\n".join(lines) + "\n"
    (REPORTS / "runtime_report.txt").write_text(text, encoding="utf-8")
    (REPORTS / "runtime_report.json").write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    (REPORTS / "runtime_report.html").write_text(
        "<!doctype html><html><head><meta charset='utf-8'><title>Runtime Report</title>"
        "<style>body{font-family:Georgia,serif;background:#f7f1e7;color:#1f1b16;padding:32px}.card{max-width:980px;background:#fffaf3;border:1px solid #d8cebe;border-radius:18px;padding:24px}pre{white-space:pre-wrap;font:14px/1.6 Menlo,monospace}</style>"
        f"</head><body><div class='card'><h1>Runtime Report</h1><pre>{escape(text)}</pre></div></body></html>",
        encoding="utf-8",
    )
    print("runtime report generated")


if __name__ == "__main__":
    main()
