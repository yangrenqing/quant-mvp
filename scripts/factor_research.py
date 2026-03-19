#!/usr/bin/env python3
import argparse
import csv
import json
import math
from html import escape
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
REPORTS = ROOT / "reports"


META_COLUMNS = {"symbol", "name", "industry", "date"}
LABEL_COLUMNS = {
    "label_5d",
    "label_10d",
    "label_20d",
    "excess_5d",
    "excess_10d",
    "excess_20d",
    "beat_benchmark_5d",
    "beat_benchmark_10d",
    "beat_benchmark_20d",
}


def mean(values):
    return sum(values) / len(values) if values else 0.0


def pearson(xs, ys):
    if len(xs) != len(ys) or len(xs) < 2:
        return 0.0
    mx = mean(xs)
    my = mean(ys)
    num = sum((x - mx) * (y - my) for x, y in zip(xs, ys))
    denx = math.sqrt(sum((x - mx) ** 2 for x in xs))
    deny = math.sqrt(sum((y - my) ** 2 for y in ys))
    if denx == 0 or deny == 0:
        return 0.0
    return num / (denx * deny)


def quintile_spread(pairs):
    if len(pairs) < 10:
        return 0.0, 0.0, 0.0
    ranked = sorted(pairs, key=lambda item: item[0])
    size = max(1, len(ranked) // 5)
    low = ranked[:size]
    high = ranked[-size:]
    low_mean = mean([item[1] for item in low])
    high_mean = mean([item[1] for item in high])
    return high_mean - low_mean, high_mean, low_mean


def main():
    parser = argparse.ArgumentParser(description="Generate factor research diagnostics from the exported dataset.")
    parser.add_argument("--dataset", default="reports/training_dataset.csv")
    parser.add_argument("--label", default="label_10d")
    args = parser.parse_args()

    dataset_path = ROOT / args.dataset
    if not dataset_path.exists():
        raise SystemExit(f"dataset not found: {dataset_path}")

    with dataset_path.open("r", encoding="utf-8", newline="") as handle:
        reader = csv.DictReader(handle)
        rows = list(reader)
        headers = reader.fieldnames or []

    if not rows:
        raise SystemExit("dataset is empty")

    features = []
    for header in headers:
        lower = header.lower()
        if lower in META_COLUMNS or lower in LABEL_COLUMNS:
            continue
        features.append(header)

    label_values = []
    filtered_rows = []
    for row in rows:
        try:
            label = float(row[args.label])
        except (KeyError, TypeError, ValueError):
            continue
        filtered_rows.append(row)
        label_values.append(label)

    rankings = []
    for feature in features:
        pairs = []
        xs = []
        ys = []
        for row, label in zip(filtered_rows, label_values):
            try:
                x = float(row[feature])
            except (TypeError, ValueError):
                continue
            xs.append(x)
            ys.append(label)
            pairs.append((x, label))
        corr = pearson(xs, ys)
        spread, high_mean, low_mean = quintile_spread(pairs)
        rankings.append(
            {
                "feature": feature,
                "correlation": corr,
                "abs_correlation": abs(corr),
                "quintile_spread": spread,
                "top_quintile_mean": high_mean,
                "bottom_quintile_mean": low_mean,
                "sample_count": len(xs),
            }
        )

    top_corr = sorted(rankings, key=lambda item: item["abs_correlation"], reverse=True)[:8]
    top_spread = sorted(rankings, key=lambda item: abs(item["quintile_spread"]), reverse=True)[:8]

    payload = {
        "dataset": str(dataset_path),
        "label": args.label,
        "row_count": len(filtered_rows),
        "feature_count": len(features),
        "top_correlations": top_corr,
        "top_spreads": top_spread,
    }

    REPORTS.mkdir(parents=True, exist_ok=True)
    text_path = REPORTS / "factor_research.txt"
    json_path = REPORTS / "factor_research.json"
    html_path = REPORTS / "factor_research.html"

    lines = [
        "Factor Research",
        "",
        f"Dataset: {dataset_path}",
        f"Label: {args.label}",
        f"Rows: {len(filtered_rows)}",
        f"Features: {len(features)}",
        "",
        "Top factor correlations:",
    ]
    for item in top_corr:
        lines.append(
            f"- {item['feature']}: corr={item['correlation']:.4f} spread={item['quintile_spread']:.4f} samples={item['sample_count']}"
        )
    lines.append("")
    lines.append("Top quintile spreads:")
    for item in top_spread:
        lines.append(
            f"- {item['feature']}: spread={item['quintile_spread']:.4f} top={item['top_quintile_mean']:.4f} bottom={item['bottom_quintile_mean']:.4f}"
        )

    text = "\n".join(lines) + "\n"
    text_path.write_text(text, encoding="utf-8")
    json_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    html_path.write_text(
        "<!doctype html><html><head><meta charset='utf-8'><title>Factor Research</title>"
        "<style>body{font-family:Georgia,serif;background:#f7f1e7;color:#1f1b16;padding:32px;}"
        ".card{max-width:980px;background:#fffaf3;border:1px solid #d8cebe;border-radius:18px;padding:24px;}"
        "pre{white-space:pre-wrap;font:14px/1.6 Menlo,monospace;}</style></head><body>"
        f"<div class='card'><h1>Factor Research</h1><pre>{escape(text)}</pre></div></body></html>",
        encoding="utf-8",
    )
    print(f"factor research generated for {len(features)} features")


if __name__ == "__main__":
    main()
