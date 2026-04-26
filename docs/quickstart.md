# Quickstart

This quickstart is the shortest path to running Animus Datalab locally and verifying that the stack is alive.

Animus Datalab is an open-core enterprise ML laboratory for reproducible, auditable, Kubernetes-backed machine learning workflows. The local development path uses Docker Compose, while production deployment is expected to use Kubernetes and Helm.

## Prerequisites

- Go toolchain matching `go.mod`;
- Docker with Docker Compose support;
- Python 3 for SDK and supporting tooling where applicable;
- Git.

## Local development and demo

Clone the repository:

```bash
git clone https://github.com/AnimusHQ/mllaboratory.git
cd mllaboratory
```

Bootstrap dependencies:

```bash
make bootstrap
```

Start the local stack:

```bash
make dev
```

Run the smoke test:

```bash
make dev DEV_ARGS=--smoke
```

Tear the stack down:

```bash
make dev DEV_ARGS=--down
```

`make dev` is the canonical local-development entrypoint. Legacy aliases such as `make demo`, `make demo-smoke`, or `make demo-down` may exist for compatibility, but new documentation and examples should prefer `make dev`.

## Kubernetes quickstart

A minimal Helm-based deployment uses the two production charts under `deploy/helm/`.

```bash
kubectl create namespace animus-system

helm upgrade --install animus-datapilot ./deploy/helm/animus-datapilot \
  --namespace animus-system \
  --values ./deploy/helm/animus-datapilot/values.yaml

helm upgrade --install animus-dataplane ./deploy/helm/animus-dataplane \
  --namespace animus-system \
  --values ./deploy/helm/animus-dataplane/values.yaml
```

Readiness check:

```bash
kubectl -n animus-system get pods
kubectl -n animus-system port-forward svc/animus-datapilot-gateway 8080:8080
curl -fsS http://127.0.0.1:8080/readyz
```

## Production expectations

A production deployment should use:

- external PostgreSQL with tested backup and restore procedures;
- external S3-compatible object storage, or a hardened object-store deployment;
- OIDC mode with secure session configuration;
- a rotated internal auth secret for service-internal CP/DP channels;
- network isolation between public, control, and execution zones;
- image digest pinning and optional signed-image admission policy;
- observability for services, workloads, and request paths.

## Troubleshooting notes

- If local services do not become ready, inspect Docker Compose logs before changing chart or service code.
- If Kubernetes pods are not ready, inspect chart values, pod events, service logs, and readiness probes.
- If behavior differs from README-level summaries, prefer service code, migrations, OpenAPI contracts, Helm values, and detailed docs as the source of truth.
