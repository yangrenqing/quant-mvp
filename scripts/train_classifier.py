#!/usr/bin/env python3
import argparse
import csv
import json
import math
from pathlib import Path


DEFAULT_FEATURES = [
    "quality_score",
    "risk_score",
    "heat_penalty",
    "reversal_score",
    "value_score",
    "low_vol_score",
    "crowding_score",
    "fundamental_score",
    "valuation_score",
    "event_score",
    "trend_score",
    "liquidity_score",
    "structure_score",
    "momentum_score",
    "persistence_score",
    "breakout_score",
    "volume_trend_score",
    "short_return_score",
    "medium_return_score",
    "rotation_score",
    "strategy_alignment",
    "breadth",
    "regime_exposure",
]


def parse_args():
    parser = argparse.ArgumentParser(description="Train a minimal zero-dependency logistic classifier.")
    parser.add_argument("--dataset", default="reports/training_dataset.csv")
    parser.add_argument(
        "--label",
        default="beat_benchmark_10d",
        choices=["beat_benchmark_5d", "beat_benchmark_10d", "beat_benchmark_20d"],
    )
    parser.add_argument("--reports-dir", default="reports")
    parser.add_argument("--learning-rate", type=float, default=0.05)
    parser.add_argument("--epochs", type=int, default=500)
    parser.add_argument("--l2", type=float, default=0.001)
    parser.add_argument("--rolling-windows", type=int, default=4)
    return parser.parse_args()


def mean(values):
    return sum(values) / len(values) if values else 0.0


def stdev(values, mu):
    if not values:
        return 1.0
    variance = sum((v - mu) ** 2 for v in values) / len(values)
    sigma = math.sqrt(variance)
    return sigma if sigma > 1e-12 else 1.0


def sigmoid(x):
    x = max(min(x, 30), -30)
    return 1.0 / (1.0 + math.exp(-x))


def read_dataset(path, feature_names, label_name):
    with open(path, newline="", encoding="utf-8") as handle:
        reader = csv.DictReader(handle)
        rows = list(reader)
    dataset = []
    for row in rows:
        try:
            features = [float(row[name]) for name in feature_names]
            label = float(row[label_name])
        except (KeyError, ValueError):
            continue
        dataset.append({"symbol": row.get("symbol", ""), "date": row.get("date", ""), "features": features, "label": label})
    dataset.sort(key=lambda item: item["date"])
    return dataset


def split_dataset(dataset):
    split_idx = max(1, int(len(dataset) * 0.8))
    if split_idx >= len(dataset):
        split_idx = len(dataset) - 1
    return dataset[:split_idx], dataset[split_idx:]


def standardize(train_rows, test_rows):
    feature_count = len(train_rows[0]["features"])
    means = []
    stds = []
    for index in range(feature_count):
        column = [row["features"][index] for row in train_rows]
        mu = mean(column)
        sigma = stdev(column, mu)
        means.append(mu)
        stds.append(sigma)

    def apply(rows):
        transformed = []
        for row in rows:
            normalized = [(value - means[index]) / stds[index] for index, value in enumerate(row["features"])]
            transformed.append({**row, "features": normalized})
        return transformed

    return apply(train_rows), apply(test_rows), means, stds


def dot(weights, features):
    return sum(weight * feature for weight, feature in zip(weights, features))


def train_logistic(train_rows, learning_rate, epochs, l2):
    feature_count = len(train_rows[0]["features"])
    weights = [0.0] * feature_count
    bias = 0.0
    for _ in range(epochs):
        grad_w = [0.0] * feature_count
        grad_b = 0.0
        for row in train_rows:
            prediction = sigmoid(dot(weights, row["features"]) + bias)
            error = prediction - row["label"]
            for index, value in enumerate(row["features"]):
                grad_w[index] += error * value
            grad_b += error
        scale = 1.0 / len(train_rows)
        for index in range(feature_count):
            grad_w[index] = scale * grad_w[index] + 2.0 * l2 * weights[index]
            weights[index] -= learning_rate * grad_w[index]
        bias -= learning_rate * scale * grad_b
    return weights, bias


def evaluate(rows, weights, bias):
    labels = [row["label"] for row in rows]
    predictions = [sigmoid(dot(weights, row["features"]) + bias) for row in rows]
    brier = mean([(pred - label) ** 2 for pred, label in zip(predictions, labels)])
    logloss = mean(
        [-(label * math.log(max(pred, 1e-8)) + (1 - label) * math.log(max(1 - pred, 1e-8))) for pred, label in zip(predictions, labels)]
    )
    accuracy = mean([1.0 if (pred >= 0.5) == (label >= 0.5) else 0.0 for pred, label in zip(predictions, labels)])
    return predictions, {"brier": brier, "log_loss": logloss, "directional_accuracy": accuracy}


def rolling_validate(dataset, learning_rate, epochs, l2, windows):
    if windows <= 1 or len(dataset) < windows * 20:
        return []
    fold_size = len(dataset) // (windows + 1)
    results = []
    for fold in range(1, windows + 1):
        train_end = fold * fold_size
        test_end = min((fold + 1) * fold_size, len(dataset))
        train_rows = dataset[:train_end]
        test_rows = dataset[train_end:test_end]
        if len(train_rows) < 20 or len(test_rows) < 10:
            continue
        train_rows, test_rows, _, _ = standardize(train_rows, test_rows)
        weights, bias = train_logistic(train_rows, learning_rate, epochs, l2)
        _, metrics = evaluate(test_rows, weights, bias)
        results.append(
            {
                "fold": fold,
                "train_rows": len(train_rows),
                "test_rows": len(test_rows),
                "brier": metrics["brier"],
                "log_loss": metrics["log_loss"],
                "directional_accuracy": metrics["directional_accuracy"],
                "train_end_date": train_rows[-1]["date"],
                "test_start_date": test_rows[0]["date"],
                "test_end_date": test_rows[-1]["date"],
            }
        )
    return results


def write_outputs(reports_dir, label_name, feature_names, weights, bias, means, stds, test_rows, predictions, metrics, rolling_results):
    reports_path = Path(reports_dir)
    reports_path.mkdir(parents=True, exist_ok=True)

    model_path = reports_path / "benchmark_classifier.json"
    summary_path = reports_path / "benchmark_classifier.txt"
    predictions_path = reports_path / "benchmark_classifier_predictions.csv"
    rolling_path = reports_path / "benchmark_classifier_rolling.txt"

    feature_weights = [{"feature": feature, "weight": weight, "mean": mu, "std": sigma} for feature, weight, mu, sigma in zip(feature_names, weights, means, stds)]
    feature_weights.sort(key=lambda item: abs(item["weight"]), reverse=True)

    payload = {
        "task": "classification",
        "label": label_name,
        "bias": bias,
        "features": feature_weights,
        "metrics": metrics,
        "rolling_metrics": rolling_results,
    }
    model_path.write_text(json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8")

    lines = [
        "Benchmark Classifier",
        f"Label: {label_name}",
        f"Test rows: {len(test_rows)}",
        f"Brier: {metrics['brier']:.6f}",
        f"Log loss: {metrics['log_loss']:.6f}",
        f"Directional accuracy: {metrics['directional_accuracy'] * 100:.2f}%",
        "",
        "Top Feature Weights",
    ]
    for item in feature_weights[:10]:
        lines.append(f"{item['feature']}: {item['weight']:.6f}")
    summary_path.write_text("\n".join(lines) + "\n", encoding="utf-8")

    rolling_lines = [f"Rolling Validation for {label_name}"]
    if rolling_results:
        for item in rolling_results:
            rolling_lines.append(
                f"fold {item['fold']}: train_rows={item['train_rows']} test_rows={item['test_rows']} "
                f"brier={item['brier']:.6f} log_loss={item['log_loss']:.6f} directional_accuracy={item['directional_accuracy'] * 100:.2f}% "
                f"train_end={item['train_end_date']} test={item['test_start_date']}->{item['test_end_date']}"
            )
    else:
        rolling_lines.append("insufficient rows for rolling validation")
    rolling_path.write_text("\n".join(rolling_lines) + "\n", encoding="utf-8")

    with predictions_path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.writer(handle)
        writer.writerow(["date", "symbol", "actual", "predicted_probability"])
        for row, prediction in zip(test_rows, predictions):
            writer.writerow([row["date"], row["symbol"], f"{row['label']:.0f}", f"{prediction:.6f}"])


def main():
    args = parse_args()
    dataset = read_dataset(args.dataset, DEFAULT_FEATURES, args.label)
    if len(dataset) < 50:
        raise SystemExit("dataset is too small to train a useful classifier")
    train_rows, test_rows = split_dataset(dataset)
    train_rows, test_rows, means, stds = standardize(train_rows, test_rows)
    weights, bias = train_logistic(train_rows, args.learning_rate, args.epochs, args.l2)
    predictions, metrics = evaluate(test_rows, weights, bias)
    rolling_results = rolling_validate(dataset, args.learning_rate, args.epochs, args.l2, args.rolling_windows)
    write_outputs(args.reports_dir, args.label, DEFAULT_FEATURES, weights, bias, means, stds, test_rows, predictions, metrics, rolling_results)
    print(f"trained benchmark classifier for {args.label}")
    print(f"test rows: {len(test_rows)}")
    print(f"brier: {metrics['brier']:.6f}")
    print(f"log_loss: {metrics['log_loss']:.6f}")
    print(f"directional_accuracy: {metrics['directional_accuracy'] * 100:.2f}%")
    if rolling_results:
        avg_direction = mean([item["directional_accuracy"] for item in rolling_results])
        print(f"rolling_directional_accuracy: {avg_direction * 100:.2f}%")


if __name__ == "__main__":
    main()
