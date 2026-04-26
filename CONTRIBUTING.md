# Contributing to Animus Datalab

Thank you for your interest in Animus Datalab.

Animus Datalab is an open-core enterprise ML laboratory for reproducible, auditable, Kubernetes-backed machine learning workflows. Contributions should preserve the repository's core engineering principles: clear Control Plane / Data Plane boundaries, database-backed truth, deterministic behavior, project-scoped authorization, and explicit security posture.

## Good contribution areas

Good first contributions usually fit one of these areas:

- documentation improvements that clarify quickstart, deployment, security, or architecture behavior;
- examples for local development, smoke testing, Helm values, SDK usage, or OpenAPI clients;
- tests that improve confidence around reproducibility, authorization, audit, and persistence behavior;
- small service fixes that do not blur Control Plane and Data Plane responsibilities;
- CI and hygiene improvements that make the repository easier to validate.

## Repository truth model

Before changing behavior, identify the authoritative source for that behavior.

The current truth order is:

1. migrations in `closed/migrations/`;
2. deployment values in `deploy/helm/*/values.yaml`;
3. service implementations in `closed/*`;
4. contracts in `core/contracts/*`;
5. operational and security docs in `docs/*`;
6. README-level summaries.

Do not treat legacy compatibility shims such as `open/api`, `open/sdk`, `closed/deploy`, or `closed/scripts` as canonical sources of truth.

## Development workflow

Bootstrap dependencies:

```bash
make bootstrap
```

Run formatting:

```bash
make fmt
```

Run linting:

```bash
make lint
```

Run tests:

```bash
make test
```

Build the codebase:

```bash
make build
```

Start the local development stack:

```bash
make dev
```

Run the local smoke test:

```bash
make dev DEV_ARGS=--smoke
```

Tear the stack down:

```bash
make dev DEV_ARGS=--down
```

## Architecture rules

Preserve these boundaries:

- Control Plane services store authoritative metadata, evaluate policy, govern access, mediate artifact operations, and record audit evidence.
- Data Plane services execute user workloads and report execution evidence back to the Control Plane.
- Control Plane code must not import Data Plane runtime execution internals directly.
- PostgreSQL remains the authoritative metadata and evidence store.
- S3-compatible object storage stores binary objects; PostgreSQL stores references, metadata, and integrity values.
- Internal service-to-service paths such as `/internal/cp/*` and `/internal/dp/*` must not become public API surfaces.

## Security expectations

Contributions must preserve the deny-by-default posture:

- server-side RBAC remains project-scoped;
- protected mutations must remain auditable;
- execution-bound credentials must remain narrower than user or admin identities;
- secrets must not be exposed through UI payloads, logs, unrelated APIs, or documentation examples;
- production examples should use OIDC-oriented session posture and rotated internal auth secrets where applicable.

Report security issues through the process in `SECURITY.md`, not through public issues.

## Pull request checklist

Before opening a PR, verify:

- the change preserves Control Plane / Data Plane boundaries;
- documentation and implementation do not contradict each other;
- relevant OpenAPI contracts or schemas are updated when behavior changes;
- migrations are used for persistence changes;
- tests, linting, and formatting pass where applicable;
- public documentation distinguishes implemented behavior from roadmap or target-only behavior.

## Communication

For engineering or maintenance contact: [rewanderer@proton.me](mailto:rewanderer@proton.me).
