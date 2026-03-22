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


def main():
    factor_diag = load_json("factor_diagnostics.json")
    model_cmp = load_json("model_comparison.json")
    strategy = load_json("strategy_quality.json")
    overnight = load_json("evolution_report_overnight.json")
    winner = load_json("paper_trial_winner_latest.json")
    compare = load_json("strategy_compare_latest.json")

    strong = [item.get("feature", "") for item in (factor_diag.get("strong_factors") or [])[:3] if item.get("feature")]
    weak = [item.get("feature", "") for item in (factor_diag.get("weak_factors") or [])[:3] if item.get("feature")]
    classifier_roll = ((model_cmp.get("classifier") or {}).get("rolling_directional_accuracy") or 0.0) * 100
    regression_roll = ((model_cmp.get("regression") or {}).get("rolling_directional_accuracy") or 0.0) * 100
    portfolio = strategy.get("portfolio") or {}
    paper = strategy.get("paper_trading") or {}
    promotions = overnight.get("lifecycle_event_types") or {}
    compare_promotion = compare.get("promotion_gate") or {}
    compare_live = compare.get("latest_live") or {}
    compare_shadow = compare.get("latest_shadow") or {}
    compare_winner = compare.get("winner_metrics") or {}
    winner_sync = compare.get("winner_sync") or "n/a"
    winner_batch = compare.get("winner_batch") or {}
    shadow_active_delta = ((compare.get("comparisons") or {}).get("shadow_vs_active") or {}).get("equity_delta")
    winner_active_delta = ((compare.get("comparisons") or {}).get("winner_vs_active") or {}).get("equity_delta")

    summary_line = (
        f"当前平台以稳健为主：组合收益 {portfolio.get('total_return', 0.0) * 100:.2f}% ，"
        f"超额 {portfolio.get('excess_return', 0.0) * 100:.2f}% ，"
        f"最大回撤 {portfolio.get('max_drawdown', 0.0) * 100:.2f}% 。"
    )
    factor_line = f"当前最有帮助的因子是 {', '.join(strong) or 'n/a'}；最弱的是 {', '.join(weak) or 'n/a'}。"
    model_line = (
        f"模型层目前分类器滚动准确率 {classifier_roll:.2f}% ，回归模型 {regression_roll:.2f}% 。"
    )
    evolution_line = (
        f"最近 overnight 窗口运行 {overnight.get('run_count', 0)} 次，"
        f"promotion {promotions.get('promotion', 0)} 次，rollback {promotions.get('rollback', 0)} 次。"
    )
    paper_line = (
        f"当前 active={compare_live.get('strategy_version') or paper.get('active_version', 'n/a')}，"
        f"shadow={compare_shadow.get('strategy_version', 'n/a')}，"
        f"winner={winner.get('candidate_version', 'n/a')}，"
        f"shadow-active diff={(shadow_active_delta if shadow_active_delta is not None else paper.get('active_shadow_diff', 0.0)):.2f}，"
        f"winner-active diff={(winner_active_delta if winner_active_delta is not None else 0.0):.2f}。"
    )
    winner_line = (
        f"当前实验 winner={winner.get('candidate_version', 'n/a')}，"
        f"exp={winner.get('experiment_id', winner_batch.get('experiment_id', 'n/a'))}，"
        f"sync={winner_sync}，"
        f"rank_delta={winner.get('rank_delta', winner_batch.get('rank_delta', 0))}，"
        f"equity_delta={winner.get('equity_delta', winner_batch.get('equity_delta', 0.0)):.2f}。"
        if winner
        else "当前实验 winner：n/a。"
    )
    promotion_line = (
        f"当前 promotion gate={compare_promotion.get('status', 'n/a')}，"
        f"reason={compare_promotion.get('reason', 'n/a')}。"
        if compare_promotion
        else "当前 promotion gate：n/a。"
    )

    payload = {
        "summary": summary_line,
        "factor_health": factor_line,
        "model_state": model_line,
        "evolution_state": evolution_line,
        "paper_state": paper_line,
        "winner_state": winner_line,
        "promotion_state": promotion_line,
        "verdict": strategy.get("verdict", ""),
    }

    lines = [
        "Research Summary",
        "",
        summary_line,
        factor_line,
        model_line,
        evolution_line,
        paper_line,
        winner_line,
        promotion_line,
        "",
        f"Verdict: {strategy.get('verdict', 'n/a')}",
    ]

    text = "\n".join(lines) + "\n"
    (REPORTS / "research_summary.txt").write_text(text, encoding="utf-8")
    (REPORTS / "research_summary.json").write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    (REPORTS / "research_summary.html").write_text(
        "<!doctype html><html><head><meta charset='utf-8'><title>Research Summary</title>"
        "<style>body{font-family:Georgia,serif;background:#f7f1e7;color:#1f1b16;padding:32px}.card{max-width:1080px;background:#fffaf3;border:1px solid #d8cebe;border-radius:18px;padding:24px}pre{white-space:pre-wrap;font:14px/1.6 Menlo,monospace}</style>"
        f"</head><body><div class='card'><h1>Research Summary</h1><pre>{escape(text)}</pre></div></body></html>",
        encoding="utf-8",
    )
    print("research summary generated")


if __name__ == "__main__":
    main()
