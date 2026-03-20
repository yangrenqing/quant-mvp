#!/usr/bin/env python3
import json
from html import escape
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
REPORTS = ROOT / "reports"


def load_json(name):
    path = REPORTS / name
    if not path.exists():
        return {}
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return {}


def pct(value):
    return f"{value * 100:.2f}%"


def main():
    portfolio = load_json("portfolio_backtest.json")
    paper = load_json("paper_account.json")
    shadow = load_json("paper_shadow.json")
    scan = load_json("a_share_scan.json")
    health = load_json("health_monitor.json")
    overnight = load_json("evolution_report_overnight.json")

    total_return = portfolio.get("TotalReturn", 0.0)
    excess_return = portfolio.get("ExcessReturn", 0.0)
    drawdown = portfolio.get("MaxDrawdown", 0.0)
    rebalance_count = portfolio.get("RebalanceCount", 0)
    regime = portfolio.get("RegimeLabel") or portfolio.get("Regime") or "unknown"
    exposure = portfolio.get("ExposureLevel", 0)

    live_equity = float((paper.get("Equity") or 0.0))
    shadow_equity = float((shadow.get("Equity") or 0.0))
    equity_diff = shadow_equity - live_equity
    holdings = paper.get("Holdings") or []
    watch = scan.get("watch") or []
    strongest = [item.get("Symbol", "") for item in watch[:3] if item.get("Symbol")]

    if excess_return >= 0 and drawdown <= 0.08:
        verdict = "当前 active 策略已达到可继续保留的水平，重点看能否持续跑赢基准。"
    elif total_return > 0 and excess_return < 0:
        verdict = "当前策略能赚钱，但仍明显跑输基准，说明风险控制强于 alpha。"
    else:
        verdict = "当前策略主要价值在稳健和自动化，不在收益领先；需要继续提升选股质量。"

    payload = {
        "portfolio": {
            "total_return": total_return,
            "benchmark_return": portfolio.get("BenchmarkReturn", 0.0),
            "excess_return": excess_return,
            "max_drawdown": drawdown,
            "rebalance_count": rebalance_count,
            "regime": regime,
            "target_exposure": exposure,
        },
        "paper_trading": {
            "active_version": paper.get("Version"),
            "active_equity": live_equity,
            "shadow_equity": shadow_equity,
            "active_shadow_diff": equity_diff,
            "holdings_count": len(holdings),
            "market_date": paper.get("MarketDate"),
        },
        "watch_top3": strongest,
        "health_status": health.get("status", "unknown"),
        "overnight_runs": overnight.get("run_count", 0),
        "verdict": verdict,
    }

    lines = [
        "Strategy Quality",
        "",
        "Portfolio baseline:",
        f"- total return: {pct(total_return)}",
        f"- benchmark return: {pct(portfolio.get('BenchmarkReturn', 0.0))}",
        f"- excess return: {pct(excess_return)}",
        f"- max drawdown: {pct(drawdown)}",
        f"- rebalances: {rebalance_count}",
        f"- regime: {regime}",
        f"- target exposure: {exposure}%",
        "",
        "Paper trading:",
        f"- active version: {paper.get('Version', 'n/a')}",
        f"- active equity: {live_equity:.2f}",
        f"- shadow equity: {shadow_equity:.2f}",
        f"- active vs shadow diff: {equity_diff:.2f}",
        f"- holdings count: {len(holdings)}",
        f"- health: {health.get('status', 'unknown')}",
        "",
        f"Current strongest watchlist names: {', '.join(strongest) or 'n/a'}",
        f"Overnight runs in latest window: {overnight.get('run_count', 0)}",
        "",
        f"Verdict: {verdict}",
    ]

    text = "\n".join(lines) + "\n"
    (REPORTS / "strategy_quality.txt").write_text(text, encoding="utf-8")
    (REPORTS / "strategy_quality.json").write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    (REPORTS / "strategy_quality.html").write_text(
        "<!doctype html><html><head><meta charset='utf-8'><title>Strategy Quality</title>"
        "<style>body{font-family:Georgia,serif;background:#f7f1e7;color:#1f1b16;padding:32px}.card{max-width:1080px;background:#fffaf3;border:1px solid #d8cebe;border-radius:18px;padding:24px}pre{white-space:pre-wrap;font:14px/1.6 Menlo,monospace}</style>"
        f"</head><body><div class='card'><h1>Strategy Quality</h1><pre>{escape(text)}</pre></div></body></html>",
        encoding="utf-8",
    )
    print("strategy quality report generated")


if __name__ == "__main__":
    main()
