#!/usr/bin/env python3
import json
from html import escape
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
REPORTS = ROOT / "reports"


def load_json(path: Path):
    if not path.exists():
        return {}
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return {}


def rolling_avg(metrics):
    if not metrics:
        return 0.0
    values = [item.get("directional_accuracy", 0.0) for item in metrics]
    return sum(values) / len(values) if values else 0.0


def top_features(payload, limit=5):
    return [item.get("feature", "") for item in (payload.get("features") or [])[:limit] if item.get("feature")]


def main():
    regression = load_json(REPORTS / "linear_model.json")
    classifier = load_json(REPORTS / "benchmark_classifier.json")
    pipeline_text = (REPORTS / "model_pipeline_latest.txt").read_text(encoding="utf-8") if (REPORTS / "model_pipeline_latest.txt").exists() else ""

    reg_metrics = regression.get("metrics") or {}
    cls_metrics = classifier.get("metrics") or {}
    reg_rolling = rolling_avg(regression.get("rolling_metrics") or [])
    cls_rolling = rolling_avg(classifier.get("rolling_metrics") or [])

    if cls_rolling > reg_rolling:
        verdict = "当前分类器更接近候选池目标，适合继续作为跑赢基准辅助分。"
    elif reg_rolling > cls_rolling:
        verdict = "当前回归模型更稳，但还不够证明其对超额收益有明显优势。"
    else:
        verdict = "两条模型线目前都较弱，更多是在提供辅助排序，而不是主导 alpha。"

    payload = {
        "regression": {
            "label": regression.get("label"),
            "metrics": reg_metrics,
            "rolling_directional_accuracy": reg_rolling,
            "top_features": top_features(regression),
        },
        "classifier": {
            "label": classifier.get("label"),
            "metrics": cls_metrics,
            "rolling_directional_accuracy": cls_rolling,
            "top_features": top_features(classifier),
        },
        "pipeline_excerpt": pipeline_text.strip().splitlines()[:12],
        "verdict": verdict,
    }

    lines = [
        "Model Comparison",
        "",
        "Regression model:",
        f"- label: {payload['regression']['label']}",
        f"- test directional accuracy: {reg_metrics.get('directional_accuracy', 0.0) * 100:.2f}%",
        f"- rolling directional accuracy: {reg_rolling * 100:.2f}%",
        f"- top features: {', '.join(payload['regression']['top_features']) or 'n/a'}",
        "",
        "Benchmark classifier:",
        f"- label: {payload['classifier']['label']}",
        f"- test directional accuracy: {cls_metrics.get('directional_accuracy', 0.0) * 100:.2f}%",
        f"- rolling directional accuracy: {cls_rolling * 100:.2f}%",
        f"- brier: {cls_metrics.get('brier', 0.0):.6f}",
        f"- log loss: {cls_metrics.get('log_loss', 0.0):.6f}",
        f"- top features: {', '.join(payload['classifier']['top_features']) or 'n/a'}",
        "",
        f"Verdict: {verdict}",
    ]
    if payload["pipeline_excerpt"]:
        lines.extend(["", "Latest pipeline excerpt:"])
        lines.extend(payload["pipeline_excerpt"])

    text = "\n".join(lines) + "\n"
    (REPORTS / "model_comparison.txt").write_text(text, encoding="utf-8")
    (REPORTS / "model_comparison.json").write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    (REPORTS / "model_comparison.html").write_text(
        "<!doctype html><html><head><meta charset='utf-8'><title>Model Comparison</title>"
        "<style>body{font-family:Georgia,serif;background:#f7f1e7;color:#1f1b16;padding:32px}.card{max-width:1080px;background:#fffaf3;border:1px solid #d8cebe;border-radius:18px;padding:24px}pre{white-space:pre-wrap;font:14px/1.6 Menlo,monospace}</style>"
        f"</head><body><div class='card'><h1>Model Comparison</h1><pre>{escape(text)}</pre></div></body></html>",
        encoding="utf-8",
    )
    print("model comparison generated")


if __name__ == "__main__":
    main()
