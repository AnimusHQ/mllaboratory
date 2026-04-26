# FAQ

## What is Animus Datalab?

Animus Datalab is an open-core enterprise ML laboratory for reproducible, auditable, Kubernetes-backed machine learning workflows. It organizes datasets, run specifications, execution plans, artifacts, policy decisions, and audit evidence in one operational context.

## Is Animus Datalab an experiment tracker?

Not only. Experiment tracking is part of the broader problem space, but Animus Datalab focuses on governed ML execution, reproducibility evidence, policy, auditability, project scoping, and Control Plane / Data Plane separation.

## Is Animus Datalab a Kubeflow replacement?

No direct one-to-one replacement claim is made. Animus Datalab uses Kubernetes as the production execution substrate, but its emphasis is an enterprise-governed ML laboratory: authoritative metadata, immutable execution plans, audit evidence, RBAC, artifact mediation, and isolated workload execution.

## What is the Control Plane?

The Control Plane, Animus DataPilot, stores authoritative metadata, evaluates policy, governs access, mediates artifact operations, and records immutable audit evidence. Control Plane services do not execute untrusted user code.

## What is the Data Plane?

The Data Plane, Animus DataPlane, launches and reconciles user workloads in Kubernetes. It reports execution evidence back to the Control Plane but is not authoritative for business state.

## What is the source of truth?

PostgreSQL is the authoritative metadata and evidence store. S3-compatible object storage stores dataset payloads, run outputs, artifacts, and related binary material. PostgreSQL stores object references, metadata, integrity values, policy decisions, audit events, and execution evidence metadata.

## How is reproducibility represented?

Animus Datalab uses immutable dataset version metadata, persisted run specifications, immutable execution plans, stable specification hashing, integrity fields, and evidence records. These mechanisms are designed to make runs explainable and reconstructable.

## How is security enforced?

The default posture is deny-by-default. RBAC is project-scoped and enforced server-side. Internal CP/DP paths are separate from public API surfaces. Production deployments are expected to use OIDC, secure session settings, rotated internal auth secrets, network isolation, and image integrity controls where required.

## Can I run it locally?

Yes. The local development and demo path uses Docker Compose:

```bash
make bootstrap
make dev
make dev DEV_ARGS=--smoke
```

## How is production deployed?

Production deployment is Kubernetes-backed and uses Helm charts under `deploy/helm/` for Animus DataPilot and Animus DataPlane. Production environments should use external PostgreSQL, S3-compatible object storage, OIDC, isolated network zones, and tested backup/restore procedures.

## Where are API contracts?

Canonical API contracts live in `core/contracts/openapi/*.yaml`. Compatibility baselines and schema assets live under `core/contracts/*`.

## Where should contributors start?

Start with `CONTRIBUTING.md`, `docs/quickstart.md`, and GitHub issues labeled `good first issue` once available. Contributions should preserve Control Plane / Data Plane separation and the repository truth model.
