#!/usr/bin/env python3
import argparse
import json
import os
import sqlite3
from datetime import datetime, timezone
from html import escape
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
REPORTS = ROOT / "reports"
DB_PATH = ROOT / "data" / "quant.db"
MODEL_CONFIG = ROOT / "configs" / "model.yaml"


def parse_simple_yaml_section(path: Path, section: str) -> dict:
    values = {}
    current = None
    if not path.exists():
        return values
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.endswith(":"):
            current = line[:-1]
            continue
        if current != section or ":" not in line:
            continue
        key, value = line.split(":", 1)
        values[key.strip()] = value.strip().strip("\"'")
    return values


def parse_time(value: str):
    if not value:
        return None
    try:
        return datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None


def tail_text(path: Path, limit: int = 20) -> str:
    if not path.exists():
        return ""
    lines = path.read_text(encoding="utf-8", errors="ignore").splitlines()
    return "\n".join(lines[-limit:]).strip()


def latest_row(conn, sql: str):
    cur = conn.execute(sql)
    row = cur.fetchone()
    return row


def main() -> int:
    parser = argparse.ArgumentParser(description="Build system health and alert reports.")
    parser.add_argument("--source", default="manual")
    args = parser.parse_args()

    health_cfg = parse_simple_yaml_section(MODEL_CONFIG, "health")
    max_run_age_hours = float(health_cfg.get("max_run_age_hours", "30"))
    shadow_edge_alert = float(health_cfg.get("shadow_edge_alert", "0.01"))
    provider_failure_alert_count = int(float(health_cfg.get("provider_failure_alert_count", "1")))
    min_active_equity_ratio = float(health_cfg.get("min_active_equity_ratio", "0.97"))

    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row

    active_strategy = latest_row(conn, "SELECT version_name, activated_at FROM strategy_registry WHERE status = 'active' ORDER BY id DESC LIMIT 1;")
    latest_run = latest_row(conn, "SELECT run_type, generated_at FROM run_history ORDER BY id DESC LIMIT 1;")
    latest_live = latest_row(conn, "SELECT strategy_version, market_date, equity, order_count FROM paper_daily_metrics WHERE mode = 'live' ORDER BY id DESC LIMIT 1;")
    latest_shadow = latest_row(conn, "SELECT strategy_version, market_date, equity, order_count FROM paper_daily_metrics WHERE mode LIKE 'shadow:%' ORDER BY id DESC LIMIT 1;")
    latest_promotion = latest_row(conn, "SELECT event_type, from_version, to_version, trigger_reason, recorded_at FROM strategy_promotions ORDER BY id DESC LIMIT 1;")

    diagnostics_path = REPORTS / "diagnostics.json"
    diagnostics = {}
    if diagnostics_path.exists():
        diagnostics = json.loads(diagnostics_path.read_text(encoding="utf-8"))

    provider_failures = diagnostics.get("ProviderFailures") or diagnostics.get("provider_failures") or {}
    provider_failure_total = int(sum(provider_failures.values())) if isinstance(provider_failures, dict) else 0

    alerts = []
    warnings = []
    now = datetime.now(timezone.utc).astimezone()

    last_run_age_hours = None
    if latest_run:
        generated_at = parse_time(latest_run["generated_at"])
        if generated_at is not None:
            last_run_age_hours = (now - generated_at.astimezone(now.tzinfo)).total_seconds() / 3600
            if last_run_age_hours > max_run_age_hours:
                alerts.append(f"latest run is stale: {last_run_age_hours:.1f}h since {latest_run['run_type']}")

    if not active_strategy:
        alerts.append("no active strategy is registered")

    if latest_live and latest_live["equity"] <= 0:
        alerts.append("active paper equity is non-positive")

    if latest_live and latest_shadow:
        active_equity = float(latest_live["equity"])
        shadow_equity = float(latest_shadow["equity"])
        diff_ratio = 0.0 if active_equity == 0 else (shadow_equity - active_equity) / active_equity
        if diff_ratio > shadow_edge_alert:
            warnings.append(f"shadow is ahead of active by {diff_ratio:.2%}")
        if active_equity < 100000 * min_active_equity_ratio:
            warnings.append(f"active equity ratio fell below {min_active_equity_ratio:.2f}")

    if provider_failure_total >= provider_failure_alert_count:
        warnings.append(f"provider failures observed: {provider_failure_total}")

    daily_err = tail_text(REPORTS / "launchd_daily.err")
    weekly_err = tail_text(REPORTS / "launchd_weekly.err")
    intraday_err = tail_text(REPORTS / "launchd_intraday.err")
    err_snippets = [text for text in [daily_err, weekly_err, intraday_err] if text]
    if err_snippets:
        warnings.append("launchd stderr has recent output")

    status = "healthy"
    if alerts:
        status = "critical"
    elif warnings:
        status = "warning"

    summary = {
        "status": status,
        "source": args.source,
        "generated_at": now.isoformat(),
        "active_strategy": dict(active_strategy) if active_strategy else None,
        "latest_run": dict(latest_run) if latest_run else None,
        "latest_live": dict(latest_live) if latest_live else None,
        "latest_shadow": dict(latest_shadow) if latest_shadow else None,
        "latest_promotion": dict(latest_promotion) if latest_promotion else None,
        "last_run_age_hours": last_run_age_hours,
        "provider_failure_total": provider_failure_total,
        "alerts": alerts,
        "warnings": warnings,
    }

    REPORTS.mkdir(parents=True, exist_ok=True)
    text_path = REPORTS / "health_monitor.txt"
    json_path = REPORTS / "health_monitor.json"
    html_path = REPORTS / "health_monitor.html"

    lines = [
        "Health Monitor",
        "",
        f"Status: {status}",
        f"Source: {args.source}",
        f"Generated at: {summary['generated_at']}",
        f"Active strategy: {active_strategy['version_name'] if active_strategy else 'none'}",
        f"Latest run: {latest_run['run_type']} @ {latest_run['generated_at']}" if latest_run else "Latest run: none",
        f"Active equity: {float(latest_live['equity']):.2f}" if latest_live else "Active equity: n/a",
        f"Shadow equity: {float(latest_shadow['equity']):.2f}" if latest_shadow else "Shadow equity: n/a",
        f"Provider failures: {provider_failure_total}",
        "",
        "Alerts:",
    ]
    if alerts:
        lines.extend(f"- {item}" for item in alerts)
    else:
        lines.append("- none")
    lines.append("")
    lines.append("Warnings:")
    if warnings:
        lines.extend(f"- {item}" for item in warnings)
    else:
        lines.append("- none")

    text_path.write_text("\n".join(lines) + "\n", encoding="utf-8")
    json_path.write_text(json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8")
    html_path.write_text(
        "<!doctype html><html><head><meta charset='utf-8'><title>Health Monitor</title>"
        "<style>body{font-family:Georgia,serif;background:#f7f1e7;color:#1f1b16;padding:32px;}"
        ".card{max-width:900px;background:#fffaf3;border:1px solid #d8cebe;border-radius:18px;padding:24px;}"
        "pre{white-space:pre-wrap;font:14px/1.6 Menlo,monospace;}</style></head><body>"
        f"<div class='card'><h1>Health Monitor</h1><pre>{escape(text_path.read_text(encoding='utf-8'))}</pre></div></body></html>",
        encoding="utf-8",
    )
    print(f"health status: {status}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
