#!/usr/bin/env python3
import argparse
import json
import shutil
import subprocess
from datetime import datetime
from pathlib import Path


def parse_args():
    parser = argparse.ArgumentParser(description="Run the quant-mvp model learning pipeline.")
    parser.add_argument("--from", dest="from_date", required=True, help="Dataset start date in YYYY-MM-DD.")
    parser.add_argument("--to", dest="to_date", required=True, help="Dataset end date in YYYY-MM-DD.")
    parser.add_argument(
        "--label",
        default="label_10d",
        choices=[
            "label_5d",
            "label_10d",
            "label_20d",
            "excess_5d",
            "excess_10d",
            "excess_20d",
            "beat_benchmark_5d",
            "beat_benchmark_10d",
            "beat_benchmark_20d",
        ],
        help="Training label.",
    )
    parser.add_argument(
        "--benchmark-label",
        default="beat_benchmark_10d",
        choices=["beat_benchmark_5d", "beat_benchmark_10d", "beat_benchmark_20d"],
        help="Benchmark-beating classifier label.",
    )
    parser.add_argument("--rolling-windows", type=int, default=4, help="Rolling validation windows.")
    parser.add_argument("--go-bin", default="/usr/local/go/bin/go", help="Go binary path.")
    parser.add_argument("--python-bin", default="python3", help="Python binary path.")
    parser.add_argument("--promote-on", choices=["directional_accuracy", "rolling_directional_accuracy"], default="rolling_directional_accuracy", help="Metric used for promotion.")
    parser.add_argument("--benchmark-promote-on", choices=["directional_accuracy", "rolling_directional_accuracy"], default="rolling_directional_accuracy", help="Metric used for classifier promotion.")
    parser.add_argument("--min-improvement", type=float, default=0.0, help="Required minimum improvement for promotion.")
    parser.add_argument("--benchmark-min-improvement", type=float, default=0.0, help="Required minimum improvement for classifier promotion.")
    return parser.parse_args()


def run(cmd, cwd):
    completed = subprocess.run(cmd, cwd=cwd, text=True, capture_output=True)
    if completed.returncode != 0:
        raise RuntimeError(f"command failed: {' '.join(cmd)}\nstdout:\n{completed.stdout}\nstderr:\n{completed.stderr}")
    return completed.stdout.strip()


def read_json(path):
    if not path.exists():
        return None
    return json.loads(path.read_text(encoding="utf-8"))


def metric_value(model_payload, metric_name):
    if not model_payload:
        return None
    if metric_name == "rolling_directional_accuracy":
        rolling = model_payload.get("rolling_metrics") or []
        if not rolling:
            return None
        return sum(item.get("directional_accuracy", 0.0) for item in rolling) / len(rolling)
    return (model_payload.get("metrics") or {}).get(metric_name)


def copy_if_exists(src, dst):
    if src.exists():
        dst.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(src, dst)


def train_regression_candidate(args, repo, dataset_path, candidate_dir):
    run(
        [
            args.python_bin,
            "scripts/train_model.py",
            "--dataset",
            str(dataset_path),
            "--label",
            args.label,
            "--reports-dir",
            str(candidate_dir),
            "--rolling-windows",
            str(args.rolling_windows),
        ],
        cwd=str(repo),
    )
    candidate_model = read_json(candidate_dir / "linear_model.json")
    current_model = read_json(repo / "reports" / "linear_model.json")
    candidate_metric = metric_value(candidate_model, args.promote_on)
    current_metric = metric_value(current_model, args.promote_on)
    promoted = False
    if candidate_metric is not None and (current_metric is None or candidate_metric >= current_metric + args.min_improvement):
        promoted = True
    if promoted:
        copy_if_exists(candidate_dir / "linear_model.json", repo / "reports" / "linear_model.json")
        copy_if_exists(candidate_dir / "model_train.txt", repo / "reports" / "model_train.txt")
        copy_if_exists(candidate_dir / "model_predictions.csv", repo / "reports" / "model_predictions.csv")
        copy_if_exists(candidate_dir / "model_rolling.txt", repo / "reports" / "model_rolling.txt")
    return {
        "label": args.label,
        "metric_name": args.promote_on,
        "candidate_metric": candidate_metric,
        "current_metric_before": current_metric,
        "promoted": promoted,
    }


def train_classifier_candidate(args, repo, dataset_path, candidate_dir):
    run(
        [
            args.python_bin,
            "scripts/train_classifier.py",
            "--dataset",
            str(dataset_path),
            "--label",
            args.benchmark_label,
            "--reports-dir",
            str(candidate_dir),
            "--rolling-windows",
            str(args.rolling_windows),
        ],
        cwd=str(repo),
    )
    candidate_model = read_json(candidate_dir / "benchmark_classifier.json")
    current_model = read_json(repo / "reports" / "benchmark_classifier.json")
    candidate_metric = metric_value(candidate_model, args.benchmark_promote_on)
    current_metric = metric_value(current_model, args.benchmark_promote_on)
    promoted = False
    if candidate_metric is not None and (current_metric is None or candidate_metric >= current_metric + args.benchmark_min_improvement):
        promoted = True
    if promoted:
        copy_if_exists(candidate_dir / "benchmark_classifier.json", repo / "reports" / "benchmark_classifier.json")
        copy_if_exists(candidate_dir / "benchmark_classifier.txt", repo / "reports" / "benchmark_classifier.txt")
        copy_if_exists(candidate_dir / "benchmark_classifier_predictions.csv", repo / "reports" / "benchmark_classifier_predictions.csv")
        copy_if_exists(candidate_dir / "benchmark_classifier_rolling.txt", repo / "reports" / "benchmark_classifier_rolling.txt")
    return {
        "label": args.benchmark_label,
        "metric_name": args.benchmark_promote_on,
        "candidate_metric": candidate_metric,
        "current_metric_before": current_metric,
        "promoted": promoted,
    }


def main():
    args = parse_args()
    repo = Path(__file__).resolve().parents[1]
    reports_dir = repo / "reports"
    versions_dir = reports_dir / "model_versions"
    versions_dir.mkdir(parents=True, exist_ok=True)

    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    version_dir = versions_dir / timestamp
    candidate_dir = version_dir / "candidate"
    candidate_dir.mkdir(parents=True, exist_ok=True)

    run(
        [
            args.go_bin,
            "run",
            "./cmd/scheduler",
            "--export-dataset",
            "--from",
            args.from_date,
            "--to",
            args.to_date,
        ],
        cwd=str(repo),
    )

    dataset_path = reports_dir / "training_dataset.csv"
    dataset_summary_path = reports_dir / "training_dataset.txt"
    copy_if_exists(dataset_path, version_dir / "training_dataset.csv")
    copy_if_exists(dataset_summary_path, version_dir / "training_dataset.txt")

    regression = train_regression_candidate(args, repo, dataset_path, candidate_dir)
    classifier = train_classifier_candidate(args, repo, dataset_path, candidate_dir)

    registry_entry = {
        "timestamp": timestamp,
        "from_date": args.from_date,
        "to_date": args.to_date,
        "regression": regression,
        "classifier": classifier,
        "version_dir": str(version_dir.relative_to(repo)),
    }
    with (reports_dir / "model_registry.jsonl").open("a", encoding="utf-8") as handle:
        handle.write(json.dumps(registry_entry, ensure_ascii=False) + "\n")

    summary_lines = [
        "Model Pipeline",
        f"Timestamp: {timestamp}",
        f"Dataset range: {args.from_date} -> {args.to_date}",
        f"Regression label: {regression['label']}",
        f"Regression promotion metric: {regression['metric_name']}",
        f"Regression candidate metric: {regression['candidate_metric']}",
        f"Regression current metric before: {regression['current_metric_before']}",
        f"Regression promoted: {regression['promoted']}",
        f"Classifier label: {classifier['label']}",
        f"Classifier promotion metric: {classifier['metric_name']}",
        f"Classifier candidate metric: {classifier['candidate_metric']}",
        f"Classifier current metric before: {classifier['current_metric_before']}",
        f"Classifier promoted: {classifier['promoted']}",
        f"Version dir: {version_dir.relative_to(repo)}",
    ]
    (version_dir / "pipeline_summary.txt").write_text("\n".join(summary_lines) + "\n", encoding="utf-8")
    (reports_dir / "model_pipeline_latest.txt").write_text("\n".join(summary_lines) + "\n", encoding="utf-8")

    print("\n".join(summary_lines))


if __name__ == "__main__":
    main()
