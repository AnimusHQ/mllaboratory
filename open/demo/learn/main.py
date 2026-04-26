#!/usr/bin/env python3

from __future__ import annotations

import json
import math
import mimetypes
import os
import sys
import tempfile
import time
import zipfile
from dataclasses import dataclass
from pathlib import Path

try:
    import numpy as np
    from PIL import Image
except Exception as e:  # noqa: BLE001
    raise SystemExit("missing dependency: install numpy and pillow") from e

from animus_sdk import DatasetRegistryClient, ExperimentsClient, RunTelemetryLogger
from animus_sdk.errors import AnimusAPIError


IMAGE_EXTS = {".jpg", ".jpeg", ".png", ".bmp", ".webp", ".tif", ".tiff"}
PAD_TOKEN = "<pad>"
MODEL_SCHEMA = "demo.car_plate_ocr.model.v2"


def _require_env(name: str) -> str:
    v = (os.environ.get(name) or "").strip()
    if not v:
        raise ValueError(f"missing required env: {name}")
    return v


def _clamp_int(value: int, *, lo: int, hi: int) -> int:
    return max(lo, min(hi, int(value)))


def _as_int(v: object, default: int) -> int:
    if isinstance(v, bool):
        return default
    if isinstance(v, int):
        return v
    if isinstance(v, float):
        return int(v)
    if isinstance(v, str):
        try:
            return int(v.strip())
        except Exception:
            return default
    return default


def _as_float(v: object, default: float) -> float:
    if isinstance(v, bool):
        return default
    if isinstance(v, (int, float)):
        return float(v)
    if isinstance(v, str):
        try:
            return float(v.strip())
        except Exception:
            return default
    return default


def _normalize_split(split: str) -> str:
    v = (split or "").strip().lower()
    if v in {"val", "valid", "validation"}:
        return "validation"
    if v in {"eval", "evalute", "evaluate"}:
        return "evaluate"
    if v in {"train", "test"}:
        return v
    return v or "evaluate"


def _resolve_dataset_root(extracted_dir: Path) -> Path:
    """
    Some dataset zips contain a single top-level folder; normalize to the folder that contains split dirs.
    """
    cur = extracted_dir
    for _ in range(2):
        if (
            (cur / "train").is_dir()
            or (cur / "validation").is_dir()
            or (cur / "test").is_dir()
            or (cur / "evaluate").is_dir()
            or (cur / "evalute").is_dir()
        ):
            return cur
        children = [p for p in cur.iterdir() if p.is_dir()]
        files = [p for p in cur.iterdir() if p.is_file()]
        if len(children) == 1 and len(files) == 0:
            cur = children[0]
            continue
        break
    return extracted_dir


def _resolve_split_dir(root: Path, split: str) -> Path | None:
    normalized = _normalize_split(split)
    candidates = [normalized]
    if normalized == "evaluate":
        candidates = ["evalute", "evaluate"]

    for name in candidates:
        d = root / name
        if d.is_dir():
            return d
    return None


def _iter_images(root: Path, *, limit: int) -> list[Path]:
    out: list[Path] = []
    for p in sorted(root.rglob("*")):
        if not p.is_file():
            continue
        if p.suffix.lower() not in IMAGE_EXTS:
            continue
        out.append(p)
        if len(out) >= limit:
            break
    return out


def _label_from_filename(path: Path) -> str:
    return path.stem.strip()


def _guess_content_type(path: Path) -> str:
    guessed, _ = mimetypes.guess_type(str(path))
    return (guessed or "application/octet-stream").strip()


@dataclass(frozen=True)
class TrainConfig:
    epochs: int = 40
    batch_size: int = 32
    learning_rate: float = 0.2
    hidden_size: int = 128
    min_epoch_seconds: float = 0.5
    image_width: int = 96
    image_height: int = 24
    max_train_samples: int = 200
    max_validation_samples: int = 64
    seed: int = 17


@dataclass(frozen=True)
class TrainedModel:
    config: dict[str, object]
    w1: np.ndarray
    b1: np.ndarray
    w2: np.ndarray
    b2: np.ndarray


def _clamp_float(value: float, *, lo: float, hi: float) -> float:
    return max(lo, min(hi, float(value)))


def _parse_train_config(params: dict[str, object]) -> TrainConfig:
    epochs = _as_int(params.get("train_epochs"), 0)
    if epochs <= 0:
        epochs = _as_int(params.get("train_steps"), 40)  # legacy key

    min_epoch_seconds = _as_float(params.get("train_min_epoch_seconds"), -1.0)
    if min_epoch_seconds < 0:
        min_epoch_seconds = _as_float(params.get("min_epoch_seconds"), 0.5)

    return TrainConfig(
        epochs=_clamp_int(epochs, lo=1, hi=5000),
        batch_size=_clamp_int(_as_int(params.get("train_batch_size"), 32), lo=4, hi=512),
        learning_rate=_clamp_float(_as_float(params.get("train_lr"), 0.2), lo=0.0001, hi=5.0),
        hidden_size=_clamp_int(_as_int(params.get("train_hidden_size"), 128), lo=16, hi=2048),
        min_epoch_seconds=_clamp_float(min_epoch_seconds, lo=0.0, hi=60.0),
        image_width=_clamp_int(_as_int(params.get("image_width"), 96), lo=16, hi=512),
        image_height=_clamp_int(_as_int(params.get("image_height"), 24), lo=8, hi=256),
        max_train_samples=_clamp_int(_as_int(params.get("max_train_samples"), 200), lo=10, hi=50000),
        max_validation_samples=_clamp_int(_as_int(params.get("max_validation_samples"), 64), lo=10, hi=50000),
        seed=_clamp_int(_as_int(params.get("seed"), 17), lo=0, hi=2**31 - 1),
    )


def _label_from_path(path: Path) -> str:
    return path.stem.strip()


def _build_vocab(labels: list[str]) -> list[str]:
    chars = sorted({ch for label in labels for ch in label})
    if PAD_TOKEN not in chars:
        chars.append(PAD_TOKEN)
    return chars


def _encode_labels(labels: list[str], *, vocab: list[str], max_len: int) -> np.ndarray:
    tok_to_idx = {t: i for i, t in enumerate(vocab)}
    pad_idx = tok_to_idx[PAD_TOKEN]
    y = np.full((len(labels), max_len), pad_idx, dtype=np.int64)
    for i, label in enumerate(labels):
        for j, ch in enumerate(label[:max_len]):
            y[i, j] = tok_to_idx.get(ch, pad_idx)
    return y


def _load_features(paths: list[Path], *, width: int, height: int) -> np.ndarray:
    features: list[np.ndarray] = []
    for p in paths:
        img = Image.open(p).convert("L")
        img = img.resize((width, height), resample=Image.BILINEAR)
        arr = np.asarray(img, dtype=np.float32) / 255.0
        features.append(arr.reshape(-1))
    return np.stack(features, axis=0)


def _relu(x: np.ndarray) -> np.ndarray:
    return np.maximum(x, 0.0)


def _softmax(logits: np.ndarray) -> np.ndarray:
    z = logits - logits.max(axis=1, keepdims=True)
    exp = np.exp(z)
    return exp / exp.sum(axis=1, keepdims=True)


def _eval_model(
    x: np.ndarray,
    y: np.ndarray,
    *,
    w1: np.ndarray,
    b1: np.ndarray,
    w2: np.ndarray,
    b2: np.ndarray,
) -> tuple[float, float, float]:
    if x.size == 0 or y.size == 0:
        return 0.0, 0.0, 0.0

    z1 = x @ w1.T + b1
    h = _relu(z1)

    seq_len, vocab_size, _ = w2.shape
    loss_total = 0.0
    preds = np.empty_like(y)
    char_correct = 0
    for pos in range(seq_len):
        logits = h @ w2[pos].T + b2[pos]
        probs = _softmax(logits)
        idx = y[:, pos]
        loss_total += float(-np.log(probs[np.arange(len(idx)), idx] + 1e-9).mean())
        pred = probs.argmax(axis=1)
        preds[:, pos] = pred
        char_correct += int((pred == idx).sum())

    loss = loss_total / float(seq_len)
    full_acc = float((preds == y).all(axis=1).mean())
    char_acc = float(char_correct / float(len(y) * seq_len))
    return loss, full_acc, char_acc


def _train_mlp(
    x_train: np.ndarray,
    y_train: np.ndarray,
    x_val: np.ndarray,
    y_val: np.ndarray,
    *,
    cfg: TrainConfig,
    vocab: list[str],
    logger: RunTelemetryLogger,
) -> tuple[TrainedModel, dict[str, float]]:
    if x_train.size == 0:
        raise RuntimeError("empty_train_set")

    n_train, feat_dim = x_train.shape
    seq_len = y_train.shape[1]
    vocab_size = len(vocab)

    rng = np.random.default_rng(cfg.seed)

    w1 = (rng.standard_normal((cfg.hidden_size, feat_dim)) * (1.0 / math.sqrt(max(1, feat_dim)))).astype(np.float32)
    b1 = np.zeros((cfg.hidden_size,), dtype=np.float32)
    w2 = (rng.standard_normal((seq_len, vocab_size, cfg.hidden_size)) * (1.0 / math.sqrt(max(1, cfg.hidden_size)))).astype(np.float32)
    b2 = np.zeros((seq_len, vocab_size), dtype=np.float32)

    best_val_loss = float("inf")
    best_snapshot: tuple[np.ndarray, np.ndarray, np.ndarray, np.ndarray] | None = None

    batches_per_epoch = int(math.ceil(n_train / float(cfg.batch_size)))
    total_steps = cfg.epochs

    for epoch in range(1, cfg.epochs + 1):
        epoch_started = time.monotonic()
        perm = rng.permutation(n_train)
        epoch_loss = 0.0

        for batch_idx in range(batches_per_epoch):
            start = batch_idx * cfg.batch_size
            idx = perm[start : start + cfg.batch_size]
            xb = x_train[idx]
            yb = y_train[idx]

            z1 = xb @ w1.T + b1
            h = _relu(z1)

            dW2 = np.zeros_like(w2)
            db2 = np.zeros_like(b2)
            dh = np.zeros_like(h)
            batch_loss = 0.0

            for pos in range(seq_len):
                logits = h @ w2[pos].T + b2[pos]
                probs = _softmax(logits)
                tgt = yb[:, pos]

                batch_loss += float(-np.log(probs[np.arange(len(tgt)), tgt] + 1e-9).mean())

                dlogits = probs
                dlogits[np.arange(len(tgt)), tgt] -= 1.0
                dlogits /= float(len(tgt))

                dW2[pos] = dlogits.T @ h
                db2[pos] = dlogits.sum(axis=0)
                dh += dlogits @ w2[pos]

            batch_loss /= float(seq_len)
            epoch_loss += batch_loss

            dh[z1 <= 0] = 0.0
            dW1 = dh.T @ xb
            db1 = dh.sum(axis=0)

            lr = float(cfg.learning_rate)
            w1 -= (lr * dW1).astype(np.float32)
            b1 -= (lr * db1).astype(np.float32)
            w2 -= (lr * dW2).astype(np.float32)
            b2 -= (lr * db2).astype(np.float32)

        train_loss, train_full_acc, train_char_acc = _eval_model(x_train, y_train, w1=w1, b1=b1, w2=w2, b2=b2)
        val_loss, val_full_acc, val_char_acc = _eval_model(x_val, y_val, w1=w1, b1=b1, w2=w2, b2=b2)

        if val_loss < best_val_loss:
            best_val_loss = val_loss
            best_snapshot = (w1.copy(), b1.copy(), w2.copy(), b2.copy())

        logger.log_metrics(
            step=epoch,
            metrics={
                "loss": float(train_loss),
                "val_loss": float(val_loss),
                "mAP": float(val_full_acc),
            },
            metadata={
                "train_full_acc": float(train_full_acc),
                "train_char_acc": float(train_char_acc),
                "val_char_acc": float(val_char_acc),
                "epoch": epoch,
                "epochs": cfg.epochs,
            },
        )
        logger.log_progress(step=epoch, total_steps=total_steps, percent=epoch / float(total_steps), message="training")

        if cfg.min_epoch_seconds > 0:
            elapsed = time.monotonic() - epoch_started
            remaining = float(cfg.min_epoch_seconds) - float(elapsed)
            if remaining > 0:
                time.sleep(remaining)

    if best_snapshot is not None:
        w1, b1, w2, b2 = best_snapshot

    metrics = {
        "val_loss": float(best_val_loss if best_val_loss != float("inf") else 0.0),
    }

    model_cfg: dict[str, object] = {
        "schema": MODEL_SCHEMA,
        "image_width": cfg.image_width,
        "image_height": cfg.image_height,
        "hidden_size": cfg.hidden_size,
        "max_label_len": int(seq_len),
        "pad_token": PAD_TOKEN,
        "vocab": vocab,
        "seed": int(cfg.seed),
    }

    model = TrainedModel(config=model_cfg, w1=w1, b1=b1, w2=w2, b2=b2)
    return model, metrics


def _save_model_npz(path: Path, model: TrainedModel, *, mean: float, std: float) -> None:
    cfg = dict(model.config)
    cfg["pixel_mean"] = float(mean)
    cfg["pixel_std"] = float(std)

    path.parent.mkdir(parents=True, exist_ok=True)
    np.savez_compressed(
        path,
        config_json=np.array(json.dumps(cfg, ensure_ascii=False, sort_keys=True)),
        w1=model.w1.astype(np.float32),
        b1=model.b1.astype(np.float32),
        w2=model.w2.astype(np.float32),
        b2=model.b2.astype(np.float32),
    )


def _load_model_npz(path: Path) -> tuple[TrainedModel, float, float]:
    with np.load(path, allow_pickle=False) as data:
        cfg = json.loads(str(data["config_json"].item()))
        mean = float(cfg.get("pixel_mean", 0.0))
        std = float(cfg.get("pixel_std", 1.0))
        model = TrainedModel(
            config=cfg,
            w1=data["w1"].astype(np.float32),
            b1=data["b1"].astype(np.float32),
            w2=data["w2"].astype(np.float32),
            b2=data["b2"].astype(np.float32),
        )
        return model, mean, std


def _predict_text(model: TrainedModel, x: np.ndarray, *, mean: float, std: float) -> tuple[str, float]:
    cfg = model.config
    vocab = cfg.get("vocab") if isinstance(cfg, dict) else None
    if not isinstance(vocab, list) or not vocab:
        raise RuntimeError("model_invalid_vocab")
    pad_token = str(cfg.get("pad_token") or PAD_TOKEN)

    x = x.astype(np.float32)
    x = (x - float(mean)) / max(1e-6, float(std))

    z1 = x @ model.w1.T + model.b1
    h = _relu(z1)

    seq_len, vocab_size, _ = model.w2.shape
    tokens: list[str] = []
    confs: list[float] = []

    for pos in range(seq_len):
        logits = h @ model.w2[pos].T + model.b2[pos]
        probs = _softmax(logits)
        idx = int(probs.argmax(axis=1)[0])
        idx = max(0, min(vocab_size - 1, idx))
        tokens.append(str(vocab[idx]))
        confs.append(float(probs[0, idx]))

    while tokens and tokens[-1] == pad_token:
        tokens.pop()
        confs.pop()

    text = "".join(tokens)
    confidence = float(min(confs)) if confs else 0.0
    return text, confidence


def run_training() -> None:
    datapilot_url = _require_env("DATAPILOT_URL")
    token = _require_env("TOKEN")
    run_id = _require_env("RUN_ID")
    dataset_version_id = _require_env("DATASET_VERSION_ID")

    experiments = ExperimentsClient(gateway_url=datapilot_url, auth_token=token, timeout_seconds=30.0)
    datasets = DatasetRegistryClient(gateway_url=datapilot_url, auth_token=token, timeout_seconds=60.0)

    logger = RunTelemetryLogger.from_env(timeout_seconds=2.0)
    logger.log_status(status="starting", message="training starting", metadata={"job_kind": "training"})

    run = experiments.get_run(run_id=run_id)
    params = run.get("params") if isinstance(run, dict) else None
    params = params if isinstance(params, dict) else {}

    cfg = _parse_train_config(params)
    logger.log_event(
        level="info",
        message="training config resolved",
        metadata={
            "train_epochs": cfg.epochs,
            "train_batch_size": cfg.batch_size,
            "train_lr": cfg.learning_rate,
            "train_hidden_size": cfg.hidden_size,
            "train_min_epoch_seconds": cfg.min_epoch_seconds,
            "image_width": cfg.image_width,
            "image_height": cfg.image_height,
            "max_train_samples": cfg.max_train_samples,
            "max_validation_samples": cfg.max_validation_samples,
        },
    )

    with tempfile.TemporaryDirectory(prefix="animus-train-") as td:
        work = Path(td)
        dataset_zip = work / "dataset.zip"
        extracted = work / "dataset"
        extracted.mkdir(parents=True, exist_ok=True)

        logger.log_status(status="downloading_dataset", message="downloading dataset version")
        ds_meta = datasets.download_dataset_version(dataset_version_id=dataset_version_id, dest_path=str(dataset_zip))
        logger.log_event(level="info", message="dataset downloaded", metadata={"dataset_meta": ds_meta})

        logger.log_status(status="extracting_dataset", message="extracting dataset zip")
        with zipfile.ZipFile(dataset_zip, "r") as zf:
            zf.extractall(extracted)
        root = _resolve_dataset_root(extracted)
        train_dir = root / "train"
        val_dir = root / "validation"
        if not train_dir.is_dir():
            raise RuntimeError("dataset_missing_train_split")

        train_images = _iter_images(train_dir, limit=cfg.max_train_samples)
        if not train_images:
            raise RuntimeError("empty_train_split")

        val_source = "validation"
        if not val_dir.is_dir():
            val_source = "train"
            val_dir = train_dir

        val_images = _iter_images(val_dir, limit=cfg.max_validation_samples)
        if not val_images:
            val_source = "train"
            val_images = train_images[: min(len(train_images), cfg.max_validation_samples)]

        logger.log_event(
            level="info",
            message="splits scanned",
            metadata={"train_samples": len(train_images), "val_samples": len(val_images), "val_source": val_source},
        )

        train_labels = [_label_from_path(p) for p in train_images]
        val_labels = [_label_from_path(p) for p in val_images]
        max_label_len = max(len(s) for s in (train_labels + val_labels))
        max_label_len = _clamp_int(max_label_len, lo=4, hi=16)

        vocab = _build_vocab(train_labels + val_labels)
        y_train = _encode_labels(train_labels, vocab=vocab, max_len=max_label_len)
        y_val = _encode_labels(val_labels, vocab=vocab, max_len=max_label_len)

        logger.log_status(status="loading_features", message="loading image features")
        x_train = _load_features(train_images, width=cfg.image_width, height=cfg.image_height)
        x_val = _load_features(val_images, width=cfg.image_width, height=cfg.image_height)

        mean = float(x_train.mean())
        std = float(x_train.std())
        if std < 1e-6:
            std = 1.0
        x_train = ((x_train - mean) / std).astype(np.float32)
        x_val = ((x_val - mean) / std).astype(np.float32)

        logger.log_status(status="training", message="training model")
        model, summary = _train_mlp(x_train, y_train, x_val, y_val, cfg=cfg, vocab=vocab, logger=logger)

        model_path = work / "model.npz"
        _save_model_npz(model_path, model, mean=mean, std=std)

        logger.log_status(status="uploading_model", message="uploading model artifact")
        experiments.upload_run_artifact(
            run_id=run_id,
            kind="model",
            file_path=str(model_path),
            name="demo-car-plate-ocr-model",
            metadata={
                "format": "npz",
                "schema": MODEL_SCHEMA,
                "train_samples": len(train_images),
                "val_samples": len(val_images),
                "val_source": val_source,
                "summary": summary,
            },
            filename="model.npz",
            content_type="application/octet-stream",
        )

    logger.log_status(status="finished", message="training finished", metadata={"job_kind": "training"})
    logger.close(flush=True, timeout_seconds=5.0)


def run_evaluation() -> None:
    datapilot_url = _require_env("DATAPILOT_URL")
    token = _require_env("TOKEN")
    run_id = _require_env("RUN_ID")
    dataset_version_id = _require_env("DATASET_VERSION_ID")

    evaluation_id = (os.environ.get("ANIMUS_EVALUATION_ID") or "").strip()
    split = _normalize_split(os.environ.get("ANIMUS_EVAL_SPLIT") or "evaluate")
    preview_samples = _clamp_int(_as_int(os.environ.get("ANIMUS_EVAL_PREVIEW_SAMPLES"), 16), lo=1, hi=128)

    experiments = ExperimentsClient(gateway_url=datapilot_url, auth_token=token, timeout_seconds=30.0)
    datasets = DatasetRegistryClient(gateway_url=datapilot_url, auth_token=token, timeout_seconds=60.0)

    logger = RunTelemetryLogger.from_env(timeout_seconds=2.0)
    logger.log_status(
        status="starting_evaluation",
        message="evaluation starting",
        metadata={"job_kind": "evaluation", "evaluation_id": evaluation_id, "split": split},
    )

    with tempfile.TemporaryDirectory(prefix="animus-eval-") as td:
        work = Path(td)

        # 1) Download model artifact
        artifacts = experiments.list_run_artifacts(run_id=run_id, kind="model", limit=1)
        items = artifacts.get("artifacts") if isinstance(artifacts, dict) else None
        if not isinstance(items, list) or len(items) == 0:
            raise RuntimeError("model_artifact_not_found")
        model_artifact_id = str(items[0].get("artifact_id") or "").strip()
        if not model_artifact_id:
            raise RuntimeError("model_artifact_not_found")

        model_filename = str(items[0].get("filename") or "").strip().lower()
        use_trained_model = model_filename.endswith(".npz")

        model: TrainedModel | None = None
        mean = 0.0
        std = 1.0
        model_cfg: dict[str, object] = {}

        if use_trained_model:
            model_path = work / "model.npz"
            experiments.download_run_artifact(run_id=run_id, artifact_id=model_artifact_id, dest_path=str(model_path))
            model, mean, std = _load_model_npz(model_path)
            model_cfg = model.config if isinstance(model.config, dict) else {}
        else:
            logger.log_event(
                level="warn",
                message="model artifact is not an .npz; falling back to filename-stem predictor",
                metadata={"filename": model_filename or "(missing)"},
            )
            model_path = work / (model_filename or "model.bin")
            experiments.download_run_artifact(run_id=run_id, artifact_id=model_artifact_id, dest_path=str(model_path))

        # 2) Download dataset
        dataset_zip = work / "dataset.zip"
        extracted = work / "dataset"
        extracted.mkdir(parents=True, exist_ok=True)
        datasets.download_dataset_version(dataset_version_id=dataset_version_id, dest_path=str(dataset_zip))

        with zipfile.ZipFile(dataset_zip, "r") as zf:
            zf.extractall(extracted)
        root = _resolve_dataset_root(extracted)
        split_dir = _resolve_split_dir(root, split)
        if split_dir is None:
            raise RuntimeError("dataset_missing_split")

        images = _iter_images(split_dir, limit=preview_samples)
        if not images:
            raise RuntimeError("no_images_for_preview")

        for idx, img in enumerate(images):
            group = f"{evaluation_id or run_id}:{idx}"
            label = _label_from_path(img)

            logger.log_progress(step=idx + 1, total_steps=len(images), percent=(idx + 1) / float(len(images)), message="preview")

            if use_trained_model and model is not None:
                feats = _load_features(
                    [img],
                    width=_clamp_int(_as_int(model_cfg.get("image_width"), 96), lo=16, hi=512),
                    height=_clamp_int(_as_int(model_cfg.get("image_height"), 24), lo=8, hi=256),
                )
                predicted, confidence = _predict_text(model, feats, mean=mean, std=std)
            else:
                predicted, confidence = label, 1.0

            experiments.upload_run_artifact(
                run_id=run_id,
                kind="preview",
                file_path=str(img),
                name="input",
                metadata={
                    "preview_group": group,
                    "preview_role": "input",
                    "preview_index": idx,
                    "label": label,
                    "split": split,
                },
                filename=f"{label}_input{img.suffix.lower()}",
                content_type=_guess_content_type(img),
            )
            experiments.upload_run_artifact(
                run_id=run_id,
                kind="preview",
                file_path=str(img),
                name="prediction",
                metadata={
                    "preview_group": group,
                    "preview_role": "prediction",
                    "preview_index": idx,
                    "predicted_class": predicted,
                    "confidence": float(confidence),
                    "split": split,
                    "is_correct": bool(predicted == label),
                },
                filename=f"{label}_prediction{img.suffix.lower()}",
                content_type=_guess_content_type(img),
            )

    logger.log_metric(step=0, name="eval_preview_samples", value=float(preview_samples))
    logger.log_status(status="finished_evaluation", message="evaluation finished", metadata={"job_kind": "evaluation"})
    logger.close(flush=True, timeout_seconds=5.0)


def main() -> int:
    kind = (os.environ.get("ANIMUS_JOB_KIND") or "training").strip().lower()
    try:
        if kind == "evaluation":
            run_evaluation()
        else:
            run_training()
        return 0
    except AnimusAPIError as e:
        print(f"animus_api_error: {e}", file=sys.stderr)
        return 2
    except Exception as e:
        print(f"job_failed: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
