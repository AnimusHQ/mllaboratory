# Comparison positioning

This document gives precise positioning language for comparing Animus Datalab with adjacent ML and MLOps tools.

The goal is not to claim that Animus Datalab replaces every tool in the ML platform ecosystem. The goal is to explain where the project sits: governed ML execution, reproducibility evidence, auditability, policy, project-scoped metadata, and Control Plane / Data Plane separation.

## Short positioning

Animus Datalab is an open-core enterprise ML laboratory for reproducible, auditable, Kubernetes-backed machine learning workflows.

It is strongest when teams need:

- authoritative dataset, run, policy, and artifact metadata;
- immutable execution plans and stable specification hashing;
- project-scoped RBAC and deny-by-default access;
- append-only audit evidence;
- Kubernetes-backed isolated workload execution;
- explicit governance-plane and execution-plane separation.

## Compared with MLflow

MLflow is widely associated with experiment tracking, model registry, and lifecycle workflows.

Animus Datalab is not positioned as a direct one-for-one MLflow replacement. Its emphasis is enterprise-governed execution and evidence: authoritative PostgreSQL-backed metadata, immutable dataset and execution records, policy decisions, audit trails, object-store mediation, and isolated Kubernetes workload execution.

Use Animus Datalab language when the comparison is about:

- reproducible execution evidence;
- policy and approval records;
- project-scoped authorization;
- Control Plane / Data Plane separation;
- audit export and compliance-oriented operations.

## Compared with Kubeflow

Kubeflow is a Kubernetes-native ML toolkit and ecosystem.

Animus Datalab uses Kubernetes as the production execution substrate, but its primary abstraction is not a general Kubernetes ML toolkit. It is an enterprise ML laboratory that keeps governance, metadata, policy, audit, artifacts, and execution evidence in a controlled operational model.

Use Animus Datalab language when the comparison is about:

- database-backed operational truth;
- governed workload execution;
- immutable execution plans;
- auditable policy decisions;
- internal-only CP/DP channels.

## Compared with experiment trackers

Experiment trackers often focus on metrics, parameters, artifacts, and model lifecycle visibility.

Animus Datalab covers run and artifact metadata as part of a broader governed-execution system. It should be described as an ML laboratory or ML platform control/execution architecture rather than only a tracking UI.

## Compared with feature stores

Animus Datalab is not a standalone feature-store product. Feature storage and serving are not the repository's explicit product center. The platform focuses on datasets, runs, artifacts, execution plans, policy decisions, and audit evidence.

## Compared with standalone inference platforms

Animus Datalab is not positioned as a standalone inference platform. The repository emphasizes ML development lifecycle governance, execution, reproducibility, and auditability rather than production model serving as the sole product surface.

## Safe public wording

Use:

> Animus Datalab is an open-core enterprise ML laboratory for reproducible, auditable, Kubernetes-backed machine learning workflows.

Use:

> Animus Datalab separates governance from execution through a Control Plane / Data Plane architecture, with PostgreSQL-backed metadata, S3-compatible artifact storage, OpenAPI contracts, project-scoped RBAC, and append-only audit evidence.

Avoid:

- "MLflow killer";
- "Kubeflow replacement";
- "fully production hardened in every environment";
- "zero-trust" unless a specific threat model and implementation details are cited;
- claims that roadmap items are implemented unless code, migrations, deployment assets, and documentation support them.
