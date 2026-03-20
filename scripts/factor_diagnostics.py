#!/usr/bin/env python3
import argparse
import csv
import json
import math
from collections import defaultdict
from html import escape
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
REPORTS = ROOT / "reports"

META_COLUMNS = {"symbol", "name", "industry", "date"}
LABELS = ["label_10d", "excess_10d", "beat_benchmark_10d"]


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
        return 0.0
    ranked = sorted(pairs, key=lambda item: item[0])
    size = max(1, len(ranked) // 5)
    low = ranked[:size]
    high = ranked[-size:]
    return mean([item[1] for item in high]) - mean([item[1] for item in low])


def load_rows(path):
    with path.open("r", encoding="utf-8", newline="") as handle:
        reader = csv.DictReader(handle)
        rows = list(reader)
        headers = reader.fieldnames or []
    return rows, headers


def feature_columns(headers):
    features = []
    for header in headers:
        if header.lower() in META_COLUMNS or header in LABELS or header.startswith("label_") or header.startswith("excess_") or header.startswith("beat_benchmark_"):
            continue
        features.append(header)
    return features


def label_metrics(rows, features, label):
    results = []
    for feature in features:
        xs = []
        ys = []
        pairs = []
        for row in rows:
            try:
                x = float(row[feature])
                y = float(row[label])
            except (KeyError, TypeError, ValueError):
                continue
            xs.append(x)
            ys.append(y)
            pairs.append((x, y))
        corr = pearson(xs, ys)
        spread = quintile_spread(pairs)
        results.append(
            {
                "feature": feature,
                "correlation": corr,
                "abs_correlation": abs(corr),
                "quintile_spread": spread,
                "abs_spread": abs(spread),
                "sample_count": len(xs),
            }
        )
    return sorted(results, key=lambda item: (item["abs_correlation"] + item["abs_spread"]), reverse=True)


def classify_factor(entry):
    if entry["health_score"] >= 0.08:
        return "strong"
    if entry["health_score"] >= 0.04:
        return "watch"
    return "weak"


def main():
    parser = argparse.ArgumentParser(description="Build multi-label factor diagnostics.")
    parser.add_argument("--dataset", default="reports/training_dataset.csv")
    args = parser.parse_args()

    dataset_path = ROOT / args.dataset
    if not dataset_path.exists():
        raise SystemExit(f"dataset not found: {dataset_path}")

    rows, headers = load_rows(dataset_path)
    if not rows:
        raise SystemExit("dataset is empty")
    features = feature_columns(headers)
    by_label = {}
    aggregate = defaultdict(lambda: {"feature": "", "scores": [], "correlations": [], "spreads": [], "sample_count": 0})

    for label in LABELS:
        ranking = label_metrics(rows, features, label)
        by_label[label] = ranking
        for item in ranking:
            slot = aggregate[item["feature"]]
            slot["feature"] = item["feature"]
            slot["scores"].append(item["abs_correlation"] + item["abs_spread"])
            slot["correlations"].append(item["correlation"])
            slot["spreads"].append(item["quintile_spread"])
            slot["sample_count"] = max(slot["sample_count"], item["sample_count"])

    overall = []
    for feature, item in aggregate.items():
        avg_corr = mean([abs(value) for value in item["correlations"]])
        avg_spread = mean([abs(value) for value in item["spreads"]])
        sign_consistency = abs(sum(1 if value >= 0 else -1 for value in item["correlations"])) / max(1, len(item["correlations"]))
        entry = {
            "feature": feature,
            "avg_abs_correlation": avg_corr,
            "avg_abs_spread": avg_spread,
            "sign_consistency": sign_consistency,
            "health_score": mean(item["scores"]),
            "sample_count": item["sample_count"],
        }
        entry["bucket"] = classify_factor(entry)
        overall.append(entry)
    overall.sort(key=lambda item: item["health_score"], reverse=True)

    strong = [item for item in overall if item["bucket"] == "strong"][:5]
    if not strong:
        strong = overall[:5]
    weak = sorted(overall, key=lambda item: item["health_score"])[:5]

    payload = {
        "dataset": str(dataset_path),
        "row_count": len(rows),
        "feature_count": len(features),
        "labels": LABELS,
        "top_by_label": {label: ranking[:8] for label, ranking in by_label.items()},
        "overall": overall,
        "strong_factors": strong,
        "weak_factors": weak,
    }

    lines = [
        "Factor Diagnostics",
        "",
        f"Dataset: {dataset_path}",
        f"Rows: {len(rows)}",
        f"Features: {len(features)}",
        "",
        "Overall strongest factors:",
    ]
    for item in strong:
        lines.append(
            f"- {item['feature']}: health={item['health_score']:.4f} avg_corr={item['avg_abs_correlation']:.4f} avg_spread={item['avg_abs_spread']:.4f} consistency={item['sign_consistency']:.2f}"
        )
    lines.extend(["", "Overall weakest factors:"])
    for item in weak:
        lines.append(
            f"- {item['feature']}: health={item['health_score']:.4f} avg_corr={item['avg_abs_correlation']:.4f} avg_spread={item['avg_abs_spread']:.4f}"
        )
    for label in LABELS:
        lines.extend(["", f"Top factors for {label}:"])
        for item in by_label[label][:5]:
            lines.append(
                f"- {item['feature']}: corr={item['correlation']:.4f} spread={item['quintile_spread']:.4f} samples={item['sample_count']}"
            )

    REPORTS.mkdir(parents=True, exist_ok=True)
    text = "\n".join(lines) + "\n"
    (REPORTS / "factor_diagnostics.txt").write_text(text, encoding="utf-8")
    (REPORTS / "factor_diagnostics.json").write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")
    (REPORTS / "factor_diagnostics.html").write_text(
        "<!doctype html><html><head><meta charset='utf-8'><title>Factor Diagnostics</title>"
        "<style>body{font-family:Georgia,serif;background:#f7f1e7;color:#1f1b16;padding:32px}.card{max-width:1080px;background:#fffaf3;border:1px solid #d8cebe;border-radius:18px;padding:24px}pre{white-space:pre-wrap;font:14px/1.6 Menlo,monospace}</style>"
        f"</head><body><div class='card'><h1>Factor Diagnostics</h1><pre>{escape(text)}</pre></div></body></html>",
        encoding="utf-8",
    )
    print(f"factor diagnostics generated for {len(features)} features")


if __name__ == "__main__":
    main()
