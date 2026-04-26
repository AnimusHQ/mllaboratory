# Demo training/evaluation container

This directory contains a **user-owned** training/evaluation container implementation for Animus DataPilot.
It follows the runtime contract expected by the Animus gateway and is intended for end-to-end integration testing (SDK + executor).

## What it does

- `training` (`ANIMUS_JOB_KIND=training`, default)
  - Downloads dataset bytes for `DATASET_VERSION_ID`
  - Emits live metrics + progress via `animus_sdk.RunTelemetryLogger`
  - Trains a small OCR baseline model (MLP on downsampled pixels)
  - Uploads a `model.npz` artifact (`kind=model`) used by evaluation
- `evaluation` (`ANIMUS_JOB_KIND=evaluation`)
  - Downloads the latest `kind=model` artifact
  - Downloads dataset bytes and selects `ANIMUS_EVAL_PREVIEW_SAMPLES` images from `ANIMUS_EVAL_SPLIT`
  - Runs inference with the trained model and uploads preview artifacts (`kind=preview`) with `preview_group` / `preview_role` metadata

## Dataset format (expected by this demo)

Upload a ZIP that contains:

```
train/...
validation/...
test/...
evalute/...
```

This repo includes a helper that creates a small sample ZIP from the extracted dataset:

```bash
python3 open/demo/data/package_car_plate_ocr_sample.py --overwrite
```

## Build image

Build from the repo root (the Docker build context must include `open/sdk/python`):

```bash
docker build -f open/demo/learn/Dockerfile -t animus/demo-car-plate-ocr:dev .
```

## Run locally (manual)

The container requires `DATAPILOT_URL`, `RUN_ID`, `DATASET_VERSION_ID`, and `TOKEN` (injected by Animus during execution).
For local debugging you can run it manually after you have a valid run-scoped token:

```bash
docker run --rm \
  -e DATAPILOT_URL=http://localhost:8080 \
  -e RUN_ID=... \
  -e DATASET_VERSION_ID=... \
  -e TOKEN=... \
  -e ANIMUS_JOB_KIND=training \
  animus/demo-car-plate-ocr:dev

## Training params (run `params`)

The demo training job reads these keys from the run `params` JSON:

- `train_epochs` (int) or `train_steps` (int, legacy): epochs to train (default `40`)
- `train_batch_size` (int): batch size (default `32`)
- `train_lr` (float): learning rate (default `0.2`)
- `train_hidden_size` (int): MLP hidden size (default `128`)
- `train_min_epoch_seconds` (float): minimum wall time per epoch (default `0.5`, set to `0` to disable)
- `image_width` / `image_height` (int): resize before training/inference (default `96x24`)
- `max_train_samples` / `max_validation_samples` (int): caps per-split samples (default `200` / `64`)
- `seed` (int): RNG seed (default `17`)
```
