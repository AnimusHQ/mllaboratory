# Animus Datalab

![Animus Datalab](docs/assets/animus-banner.png)

> **Enterprise digital laboratory for machine learning** that organizes the full ML development lifecycle in a managed, reproducible form within a single operational context with common execution, security, and audit rules.

**Language:** Go / Python / SQL  •
**Execution model:** Control Plane + Data Plane  •
**Runtime:** Kubernetes  •
**Source of truth:** PostgreSQL  •
**Artifact storage:** S3-compatible object storage

## Overview

Animus Datalab is a centralized ML platform designed for enterprise environments where **reproducibility**, **auditability**, and **security** are non-negotiable. It manages the complete lifecycle from datasets through experiments, runs, artifacts, and model promotion — all within a single governance context.

The platform exists to solve a specific operational problem:

* keep **dataset versions, code references, environment locks, and execution parameters** centralized, immutable, and authoritative,
* execute **user workloads in isolated, containerized environments** on Kubernetes with enforced resource and network policies,
* produce **deterministic, reproducible runs** defined by the tuple (DatasetVersion, CodeRef commit SHA, EnvironmentLock, parameters, policy snapshot),
* maintain a **complete, append-only audit trail** for every state-changing action, access event, and privilege change,
* enforce **deny-by-default security** with SSO, project-scoped RBAC, and secrets management.

### Intended users

* **Data scientists and ML engineers** who need managed, reproducible experiment workflows with dataset versioning, artifact tracking, and model promotion.
* **Platform operators** who need controlled provisioning, quota enforcement, observability, and production-grade deployment for on-premise, private cloud, or air-gapped environments.
* **Security and compliance teams** who require append-only audit, RBAC enforcement, secrets redaction, and SIEM integration.
* **Maintainers and engineers** who need a clear layered architecture with strict Control Plane / Data Plane separation and predictable operational behavior.

### Core capabilities

* Control Plane / Data Plane separation with strict trust boundaries.
* Project-scoped isolation for all domain entities and operations.
* Dataset and DatasetVersion persistence with immutability enforcement.
* Run execution with deterministic planning and plan hashing.
* PipelineSpec validation with cycle detection and reference verification.
* Artifact storage with presigned URLs, checksums, retention policies, and legal hold.
* Append-only audit with NDJSON export and SIEM integration readiness.
* OIDC (and optional SAML) authentication with session TTL and forced logout.
* RBAC matrix with deny-by-default posture and object-level authorization.
* Model registry with validation/approval workflows and export policy controls.
* Managed developer environments with TTL, quotas, and audited remote access.
* Python SDK for programmatic interaction (git submodule).

---

## Architecture

This repository implements a **Control Plane / Data Plane architecture**. The Control Plane manages governance, policies, metadata, orchestration, and audit — it never executes user code. The Data Plane executes user workloads in isolated, containerized Kubernetes environments.

### Architecture principles

* **Control Plane never executes user code.**
* **Data Plane executes in isolated, containerized environments.**
* **PostgreSQL is the authoritative metadata store.**
* **S3-compatible object storage holds all artifacts.**
* **A production-run is uniquely defined by DatasetVersion + CodeRef commit SHA + EnvironmentLock + parameters + policy snapshot.**
* **All significant actions are recorded as AuditEvent.**
* **All domain entities belong to exactly one Project; cross-Project dependencies are prohibited.**
* **The system has no hidden state that affects execution results.**
* **Deny-by-default security posture; explicit allowlists.**
* **Secrets are ephemeral, minimal in scope, and never exposed in UI, logs, metrics, or artifacts.**

### 1. System context

```mermaid
flowchart LR
  subgraph Users["Users"]
    DS[Data Scientist]
    OPS[Platform Operator]
    SEC[Security / Compliance]
  end

  subgraph ControlPlane["Control Plane"]
    API[CP API Service]
    SCHED[Scheduler]
    WORKER[Background Workers]
    DB[(PostgreSQL)]
    OBJ[(S3-compatible Object Store)]
  end

  subgraph DataPlane["Data Plane (Kubernetes)"]
    EXEC[DP Executor]
    RUN1[Run Pod A]
    RUN2[Run Pod B]
    DEV[Dev Environment Pod]
  end

  subgraph External["External Systems"]
    IDP[Identity Provider - OIDC/SAML]
    VAULT[Secrets Manager]
    SIEM[SIEM / Monitoring]
    GIT[Git Repository]
  end

  DS --> API
  OPS --> API
  SEC --> API

  API <--> DB
  API <--> OBJ
  WORKER <--> DB
  SCHED <--> DB

  API --> EXEC
  SCHED --> EXEC
  WORKER --> EXEC

  EXEC --> RUN1
  EXEC --> RUN2
  EXEC --> DEV

  RUN1 --> OBJ
  RUN2 --> OBJ

  API --> IDP
  EXEC --> VAULT
  API --> SIEM
  API --> GIT
```

The Control Plane is the single decision-making authority. Users interact only with the CP API. The Data Plane receives execution instructions from the Control Plane and reports back status, heartbeats, and terminal states.

### 2. Trust boundary view

```mermaid
flowchart TB
  subgraph Untrusted["Untrusted Zone"]
    CLIENT[User Clients / SDK]
  end

  subgraph Trusted["Trusted Management Zone"]
    CP[Control Plane Services]
    DB[(PostgreSQL)]
    OBJ[(Object Storage)]
  end

  subgraph PartiallyTrusted["Partially Trusted Execution Zone"]
    DP[Data Plane Executor]
    PODS[User Workload Pods]
  end

  subgraph External["External Integration Zone"]
    IDP[Identity Provider]
    VAULT[Secrets Manager]
    SIEM[SIEM]
  end

  CLIENT -->|authn required| CP
  CP <--> DB
  CP <--> OBJ
  CP -->|dispatch + reconcile| DP
  DP --> PODS
  PODS -->|artifacts with checksums| OBJ
  DP -->|heartbeats + terminal states| CP
  CP <-->|SSO| IDP
  DP -->|ephemeral secrets| VAULT
  CP -->|audit export| SIEM
```

Trust boundaries treat user clients as untrusted, Control Plane as trusted management, Data Plane as partially trusted execution, and external systems as separate zones integrated through contractual interfaces.

### 3. Actor interaction view

```mermaid
flowchart TB
  DS[Data Scientist]
  OPS[Platform Operator]
  SEC[Security / Compliance]
  SA[Service Account]

  subgraph DSActions["Data Scientist surface"]
    SDK[Python SDK]
    APIUI[API endpoints]
    DEVENV[Dev Environment]
  end

  subgraph OPSActions["Operator surface"]
    DEPLOY[Install / Upgrade / Rollback]
    MONITOR[Observability dashboards]
    QUOTA[Quota / Policy management]
  end

  subgraph SECActions["Security surface"]
    AUDIT[Audit export / SIEM]
    RBAC[Role / policy configuration]
    SECRETS[Secrets rotation]
  end

  DS --> SDK
  DS --> APIUI
  DS --> DEVENV

  OPS --> DEPLOY
  OPS --> MONITOR
  OPS --> QUOTA

  SEC --> AUDIT
  SEC --> RBAC
  SEC --> SECRETS

  SA --> APIUI
```

Role separation is enforced server-side through project-scoped RBAC with deny-by-default posture. Service accounts are subject to the same RBAC and are audited.

### 4. Use-case view

```mermaid
flowchart LR
  DS[Data Scientist]
  OPS[Operator]
  SEC[Security]

  subgraph DSUseCases["Data Scientist use cases"]
    UC1[Register dataset version]
    UC2[Submit run with full bindings]
    UC3[View run status and artifacts]
    UC4[Promote model version]
    UC5[Launch dev environment]
    UC6[Query lineage graph]
    UC7[Export reproducibility bundle]
  end

  subgraph OPSUseCases["Operator use cases"]
    UO1[Install / upgrade platform]
    UO2[Configure project quotas]
    UO3[Monitor cluster health]
    UO4[Execute backup / DR drills]
    UO5[Manage retention policies]
  end

  subgraph SECUseCases["Security use cases"]
    US1[Configure SSO integration]
    US2[Manage RBAC role bindings]
    US3[Export audit to SIEM]
    US4[Apply legal hold]
    US5[Rotate secrets]
  end

  DS --> UC1
  DS --> UC2
  DS --> UC3
  DS --> UC4
  DS --> UC5
  DS --> UC6
  DS --> UC7

  OPS --> UO1
  OPS --> UO2
  OPS --> UO3
  OPS --> UO4
  OPS --> UO5

  SEC --> US1
  SEC --> US2
  SEC --> US3
  SEC --> US4
  SEC --> US5
```

### 5. High-level container view

```mermaid
flowchart TB
  subgraph Runtime["Runtime containers / processes"]
    API[CP API]
    SCHED[Scheduler]
    WORKER[Workers]
    DPEXEC[DP Executor]
    MIGRATE[Migrate]
    DEMO[Demo / Quickstart]
  end

  subgraph Internal["Internal packages"]
    DOMAIN[Domain layer]
    SERVICE[Service layer]
    REPO[Repository layer]
    STORAGE[Object storage abstraction]
    AUDITPKG[Audit subsystem]
    DSREG[Dataset registry]
    EXPRUNS[Experiments / Runs]
    GATEWAY[Gateway]
    LINEAGE[Lineage]
    QUALITY[Quality]
    RUNTIMEEXEC[Runtime execution]
    PLATFORM[Platform / Config]
  end

  API --> SERVICE
  SCHED --> SERVICE
  WORKER --> SERVICE

  SERVICE --> DOMAIN
  SERVICE --> REPO
  SERVICE --> STORAGE
  SERVICE --> AUDITPKG

  DPEXEC --> RUNTIMEEXEC
  DPEXEC --> STORAGE
```

### Responsibilities

* **CP API**: all business-facing HTTP endpoints, authentication, authorization, reads, mutations, reproducibility bundle export.
* **Scheduler**: queue management, priority handling, quota enforcement, dispatch to Data Plane.
* **Workers**: background reconciliation, health checks, usage collection, quota and expiry enforcement.
* **DP Executor**: Kubernetes workload launcher, heartbeat reporting, terminal state delivery, artifact commit.
* **Migrate**: schema evolution for PostgreSQL.
* **Demo / Quickstart**: containerized local development and smoke testing stack.

### 6. Internal module breakdown

```mermaid
flowchart LR
  API[CP API] --> SVC[Service layer]
  SCHED[Scheduler] --> SVC
  WORKER[Workers] --> SVC

  SVC --> DOMAIN[Domain entities]
  SVC --> PORTS[Service ports]

  PORTS --> PGREPO[PostgreSQL repositories]
  PORTS --> OBJSTORE[Object storage adapter]
  PORTS --> AUDITSTORE[Audit store adapter]
  PORTS --> OIDCADPT[OIDC / SAML adapter]
  PORTS --> SECRETADPT[Secrets manager adapter]
  PORTS --> K8SADPT[Kubernetes adapter]

  DPEXEC[DP Executor] --> K8SADPT
  DPEXEC --> OBJSTORE
  DPEXEC --> SECRETADPT
```

#### Layer responsibilities

* **Domain**
    + domain entities (Project, Dataset, DatasetVersion, Run, Artifact, ModelVersion, AuditEvent, CodeRef, EnvironmentLock, PipelineSpec),
    + status models and state machines,
    + value objects,
    + plan hash computation rules,
    + VLESS export builder rules (from linked submodule, if applicable).
* **Service**
    + orchestrates use cases,
    + owns business transitions,
    + depends only on ports (interfaces),
    + enforces production-run gate requirements.
* **Infrastructure**
    + PostgreSQL persistence,
    + S3-compatible object storage,
    + OIDC/SAML authentication,
    + secrets manager integration,
    + Kubernetes workload management,
    + audit store and export.
* **Transport**
    + HTTP API handlers, middleware, schemas,
    + CP↔DP execution protocol (internal).

### 7. Data flow

```mermaid
flowchart LR
  SDK[Python SDK / API Client] -->|authn + API calls| API[CP API]
  API -->|reads/writes| DB[(PostgreSQL)]
  API -->|presigned URLs / artifact mediation| OBJ[(Object Storage)]
  API -->|dispatch RunExecutionRequest| DP[DP Executor]

  DP -->|launch workloads| K8S[Kubernetes Pods]
  K8S -->|artifacts + checksums| OBJ
  DP -->|heartbeats + terminal states| API

  WORKER[Workers] --> DB
  WORKER --> DP

  API -->|audit events| AUDIT[Audit Store]
  AUDIT -->|NDJSON export| SIEM[SIEM]
```

The platform does not trust Data Plane state as authoritative. The DP provides execution evidence and artifact commits; PostgreSQL remains the canonical source of truth for all metadata.

### 8. Persistence / ER view

```mermaid
erDiagram
  PROJECTS {
    uuid id PK
    text name
    text status
    timestamptz created_at
    timestamptz updated_at
  }

  DATASETS {
    uuid id PK
    uuid project_id FK
    text name
    text description
    timestamptz created_at
    timestamptz updated_at
  }

  DATASET_VERSIONS {
    uuid id PK
    uuid dataset_id FK
    text version_tag
    text storage_uri
    text checksum
    jsonb schema_ref
    jsonb stats_ref
    jsonb lineage_ref
    boolean immutable
    timestamptz created_at
  }

  CODE_REFS {
    uuid id PK
    text repo_url
    text commit_sha
    boolean immutable
    timestamptz created_at
  }

  ENVIRONMENT_LOCKS {
    uuid id PK
    text image_digest
    text sbom_ref
    jsonb packages
    boolean immutable
    timestamptz created_at
  }

  PIPELINE_SPECS {
    uuid id PK
    uuid project_id FK
    text name
    jsonb spec
    text spec_hash
    timestamptz created_at
  }

  RUNS {
    uuid id PK
    uuid project_id FK
    uuid dataset_version_id FK
    uuid code_ref_id FK
    uuid environment_lock_id FK
    uuid pipeline_spec_id FK
    text status
    text plan_hash
    jsonb parameters
    jsonb policy_snapshot
    timestamptz created_at
    timestamptz updated_at
  }

  STEP_ATTEMPTS {
    uuid id PK
    uuid run_id FK
    text step_name
    int attempt_number
    text status
    timestamptz started_at
    timestamptz finished_at
  }

  ARTIFACTS {
    uuid id PK
    uuid project_id FK
    uuid run_id FK
    text storage_uri
    text checksum
    jsonb retention_metadata
    boolean legal_hold
    timestamptz created_at
  }

  MODEL_VERSIONS {
    uuid id PK
    uuid project_id FK
    uuid run_id FK
    uuid artifact_id FK
    text name
    text version
    text status
    timestamptz created_at
    timestamptz updated_at
  }

  AUDIT_EVENTS {
    uuid id PK
    uuid project_id FK
    text actor_type
    text actor_id
    text action
    text target_type
    text target_id
    jsonb metadata_json
    boolean append_only
    timestamptz created_at
  }

  PROJECTS ||--o{ DATASETS : contains
  PROJECTS ||--o{ RUNS : contains
  PROJECTS ||--o{ ARTIFACTS : contains
  PROJECTS ||--o{ PIPELINE_SPECS : defines
  PROJECTS ||--o{ MODEL_VERSIONS : contains
  PROJECTS ||--o{ AUDIT_EVENTS : records
  DATASETS ||--o{ DATASET_VERSIONS : versions
  RUNS }o--|| DATASET_VERSIONS : references
  RUNS }o--|| CODE_REFS : references
  RUNS }o--|| ENVIRONMENT_LOCKS : references
  RUNS }o--o| PIPELINE_SPECS : uses
  RUNS ||--o{ STEP_ATTEMPTS : tracks
  RUNS ||--o{ ARTIFACTS : produces
  MODEL_VERSIONS }o--|| RUNS : sources_from
  MODEL_VERSIONS }o--|| ARTIFACTS : wraps
```

### Persistence notes

* `dataset_versions` are immutable after creation; enforced by database triggers.
* `code_refs` and `environment_locks` are immutable per run.
* `audit_events` are append-only and non-disableable.
* `artifacts` include checksum verification and retention metadata.
* `runs` store a `policy_snapshot` capturing RBAC, retention, network policy, and template restrictions at the time of execution.
* All entities are project-scoped; cross-project queries are prohibited.

### 9. Core sequence: production-run submission

```mermaid
sequenceDiagram
  autonumber
  participant User
  participant SDK as Python SDK
  participant API as CP API
  participant DB as PostgreSQL
  participant SCHED as Scheduler
  participant DP as DP Executor
  participant K8S as Kubernetes

  User->>SDK: Submit run (dataset, code, env, params)
  SDK->>API: POST /v1/runs (RunSpec)
  API->>API: Validate production-run gate
  Note over API: DatasetVersionId + commit_sha + EnvironmentLock required
  API->>DB: Persist Run + plan (immutable)
  API->>DB: Store policy_snapshot
  API-->>SDK: Run ID + status: queued
  SCHED->>DB: Poll queue
  SCHED->>DP: RunExecutionRequest (idempotent)
  DP->>K8S: Create Job/Pod with limits and network policy
  DP-->>API: Heartbeat (periodic)
  K8S-->>DP: Workload completes
  DP->>API: ArtifactCommitted (checksum)
  DP->>API: RunTerminalState (succeeded/failed)
  API->>DB: Update run status
  API->>DB: Emit AuditEvent
```

### 10. Core sequence: deterministic planning

```mermaid
sequenceDiagram
  autonumber
  participant API as CP API
  participant VALIDATOR as Spec Validator
  participant PLANNER as Deterministic Planner
  participant DB as PostgreSQL

  API->>VALIDATOR: Validate PipelineSpec
  VALIDATOR->>VALIDATOR: Check cycles, refs, digests
  alt validation failed
    VALIDATOR-->>API: Validation errors (deterministic)
    API-->>API: Reject run
  else validation passed
    VALIDATOR-->>API: Spec valid
    API->>PLANNER: Generate execution plan
    PLANNER->>PLANNER: Stable topological sort + tie-breaker
    PLANNER->>PLANNER: Compute plan_hash
    Note over PLANNER: plan_hash = f(pipeline_spec_hash + bindings + run_spec + retry_policy)
    PLANNER-->>API: Immutable plan + plan_hash
    API->>DB: Persist plan (append-only)
  end
```

### 11. Core sequence: artifact commit

```mermaid
sequenceDiagram
  autonumber
  participant Pod as Run Pod
  participant DP as DP Executor
  participant OBJ as Object Storage
  participant API as CP API
  participant DB as PostgreSQL

  Pod->>OBJ: Upload artifact (per-project prefix)
  Pod->>Pod: Compute checksum
  Pod->>DP: Artifact ready (URI + checksum)
  DP->>API: ArtifactCommitted event (idempotent)
  API->>DB: Persist artifact metadata + checksum
  API->>DB: Emit AuditEvent
  API-->>DP: Acknowledged
```

### 12. Worker / reconciliation flow

```mermaid
sequenceDiagram
  autonumber
  participant Worker as CP Worker
  participant DB as PostgreSQL
  participant DP as DP Executor
  participant K8S as Kubernetes

  Worker->>DB: List runs with stale heartbeats
  Worker->>DP: Query workload status
  DP->>K8S: Check Pod/Job state
  DP-->>Worker: Current state
  alt orphaned run
    Worker->>DB: Transition to unknown/failed (DB-first)
    Worker->>DP: Cleanup workload
    Worker->>DB: Emit AuditEvent (reconciliation)
  else healthy run
    Worker->>DB: Update last heartbeat
  end

  Worker->>DB: Evaluate quota / expiry policies
  alt violation
    Worker->>DB: Transition state DB-first
    Worker->>DP: Cancel workload
    Worker->>DB: Emit AuditEvent
  end
```

### 13. State models

#### Run lifecycle

```mermaid
stateDiagram-v2
  [*] --> queued
  queued --> running: dispatched to DP
  running --> succeeded: terminal success
  running --> failed: terminal failure
  running --> canceled: user/admin cancel
  running --> unknown: heartbeat timeout
  queued --> canceled: cancel before dispatch
  unknown --> failed: reconciliation resolves
  unknown --> running: heartbeat resumes
  succeeded --> [*]
  failed --> [*]
  canceled --> [*]
```

#### Model version lifecycle

```mermaid
stateDiagram-v2
  [*] --> draft
  draft --> validating: submit for validation
  validating --> validated: passes checks
  validating --> rejected: fails checks
  validated --> approved: authorized approval
  approved --> exported: export triggered
  rejected --> draft: rework
  exported --> [*]
  approved --> [*]
```

#### Project lifecycle

```mermaid
stateDiagram-v2
  [*] --> active
  active --> archived: archive
  archived --> active: unarchive
  archived --> [*]
```

### 14. Core contracts and interfaces

#### CP ↔ DP protocol

```mermaid
classDiagram
  class CPtoDPProtocol {
    <<interface>>
    +RunExecutionRequest(ctx, runSpec) error
    +CancelRun(ctx, runID) error
    +QueryWorkloadStatus(ctx, runID) WorkloadStatus
    +CleanupWorkload(ctx, runID) error
  }

  class DPtoCPProtocol {
    <<interface>>
    +RunHeartbeat(ctx, runID, status) error
    +RunTerminalState(ctx, runID, state, details) error
    +ArtifactCommitted(ctx, runID, artifactMeta) error
  }

  class RunExecutionRequest {
    +RunID string
    +ImageDigest string
    +Resources ResourceSpec
    +NetworkPolicy PolicyRef
    +SecretsRefs []SecretRef
    +Parameters map
    +IdempotencyKey string
  }

  class WorkloadStatus {
    +RunID string
    +Phase string
    +LastHeartbeat time
    +PodStatuses []PodStatus
  }

  CPtoDPProtocol --> RunExecutionRequest
  CPtoDPProtocol --> WorkloadStatus
```

#### Repository boundary

```mermaid
classDiagram
  class ProjectRepository {
    <<interface>>
    +Create(...)
    +GetByID(...)
    +List(...)
    +Archive(...)
  }

  class DatasetRepository {
    <<interface>>
    +Create(...)
    +GetByID(...)
    +ListByProjectID(...)
  }

  class DatasetVersionRepository {
    <<interface>>
    +Create(...)
    +GetByID(...)
    +ListByDatasetID(...)
  }

  class RunRepository {
    <<interface>>
    +Create(...)
    +GetByID(...)
    +ListByProjectID(...)
    +UpdateStatus(...)
  }

  class ArtifactRepository {
    <<interface>>
    +Create(...)
    +GetByID(...)
    +ListByRunID(...)
    +ListByProjectID(...)
  }

  class AuditEventRepository {
    <<interface>>
    +Append(...)
    +ListByProjectID(...)
    +ExportNDJSON(...)
  }

  class ModelVersionRepository {
    <<interface>>
    +Create(...)
    +GetByID(...)
    +UpdateStatus(...)
    +ListByProjectID(...)
  }
```

### 15. Async / subsystem dependency view

```mermaid
flowchart LR
  API[CP API] --> SVC[Service layer]
  SCHED[Scheduler] --> SVC
  WORKER[Workers] --> SVC

  SVC --> REPOS[Repository ports]
  SVC --> OBJPORT[Object storage port]
  SVC --> AUDITPORT[Audit port]
  SVC --> AUTHPORT[Auth port]
  SVC --> SECRETPORT[Secrets port]
  SVC --> DPPORT[DP dispatch port]

  REPOS --> PG[(PostgreSQL adapters)]
  OBJPORT --> S3[S3 adapter]
  AUDITPORT --> AUDITSTORE[Audit store adapter]
  AUTHPORT --> OIDC[OIDC / SAML adapter]
  SECRETPORT --> VAULT[Vault adapter]
  DPPORT --> K8S[Kubernetes adapter]
```

The service layer is the orchestrator. Everything else is either an adapter or a transport.

### 16. Deployment / infrastructure view

```mermaid
flowchart TB
  Internet((Network))

  subgraph Edge["Ingress"]
    LB[Load Balancer / Ingress Controller]
  end

  subgraph CPCluster["Control Plane"]
    API1[CP API replica 1]
    API2[CP API replica 2]
    SCHED[Scheduler]
    WORKER[Workers]
    DB[(PostgreSQL - external)]
    OBJ[(S3-compatible Object Store)]
  end

  subgraph DPCluster["Data Plane (may be separate cluster)"]
    DPEXEC[DP Executor]
    NS1[Project Namespace A]
    NS2[Project Namespace B]
    NS3[Project Namespace N]
  end

  subgraph External["External Services"]
    IDP[Identity Provider]
    VAULT[Secrets Manager]
    SIEM[SIEM / Monitoring]
  end

  Internet --> LB
  LB --> API1
  LB --> API2

  API1 <--> DB
  API2 <--> DB
  SCHED <--> DB
  WORKER <--> DB

  API1 <--> OBJ
  API2 <--> OBJ

  API1 --> DPEXEC
  SCHED --> DPEXEC

  DPEXEC --> NS1
  DPEXEC --> NS2
  DPEXEC --> NS3

  API1 --> IDP
  DPEXEC --> VAULT
  API1 --> SIEM
```

A production deployment uses Helm charts or Kustomize. The CP is stateless (replicas + external DB) for HA. The DP may run on a separate cluster. The platform supports on-premise, private cloud, and air-gapped deployments.

---

## Domain and system internals

### Key entities

#### Project

The isolation boundary and root container for all domain entities. A project may be active or archived. Archived projects become read-only. All queries are project-scoped; cross-project dependencies are prohibited.

#### Dataset / DatasetVersion

A dataset is a named collection within a project. DatasetVersion is an immutable snapshot referencing storage URI, checksum, schema, stats, and lineage. Immutability is enforced by database triggers.

#### CodeRef

An immutable reference to a specific commit SHA and repository URL. Required for production-runs.

#### EnvironmentLock

An immutable specification of the execution environment including image digest, package list, and optional SBOM reference. Required for production-runs.

#### Run

The minimal unit of execution and reproducibility. A Run is defined by its Project, DatasetVersion, CodeRef commit SHA, EnvironmentLock, execution parameters, and policy snapshot. Reproducibility is the ability to re-execute a Run and obtain results consistent with the recorded inputs; determinism is not assumed when bindings are missing or user code introduces non-determinism.

#### PipelineSpec

A validated, hashed specification of a multi-step DAG pipeline. Cycle detection and reference verification are enforced at validation time. Plan hashing is deterministic with stable topological ordering and tie-breaker rules.

#### Artifact

A versioned output of a Run, stored in S3-compatible object storage with checksums. Subject to retention policies and legal hold enforcement. Access is mediated and audited through the Control Plane.

#### ModelVersion

A lifecycle entity referencing a source Run and an Artifact. Subject to validation/approval workflows with RBAC-gated promotion and export policy controls.

#### AuditEvent

An append-only, non-disableable record of significant actions. Covers all state-changing operations, access to protected downloads, authentication events, privilege changes, and secrets access events (metadata only, never values). Exportable to SIEM via webhook/syslog with retries and idempotency.

### Business rules and invariants

#### DB-first orchestration

For run submission, status transitions, artifact commits, and model promotion, central state is persisted before any side effect is attempted.

#### Single authority

The platform never treats Data Plane state as canonical business state. The DP provides execution evidence; PostgreSQL remains authoritative.

#### Production-run gate

A production-run is accepted only when CodeRef commit SHA, EnvironmentLock, DatasetVersion references, and required policy approvals are explicit and recorded. Otherwise the Run is rejected or limitations are recorded in Run metadata and AuditEvent.

#### Deterministic planning

Plan hash is deterministic from (pipeline_spec_hash + bindings + run_spec + retry_policy). Stable topological ordering with tie-breaker rules. Idempotent create operations produce identical resources on duplicate requests.

#### Truthful failure handling

Failure states are explicit. The platform must not represent a partially executed run as fully succeeded. Reconciliation resolves orphaned runs; no manual status edits.

#### Append-only audit

Audit cannot be disabled. All state-changing operations, protected access, and privilege changes are recorded. Export supports SIEM integration with retries, idempotency, and per-project filters.

### Security boundaries

* OIDC (primary) and optional SAML authentication with session TTL, forced logout, and limits on parallel sessions.
* Project-scoped RBAC with deny-by-default; object-level authorization on sensitive reads, downloads, and exports.
* Secrets supplied via external secrets manager, ephemeral and minimal in scope, never exposed in UI, logs, metrics, or artifacts.
* Data Plane executes untrusted user code in containerized environments with restricted privileges; network access and resource limits enforced by policy.
* Control Plane never executes user code; enforced by `depguard` lint rules.
* Service accounts subject to the same RBAC and audited.
* CP/DP boundary enforced at the module level via static analysis.

---

## Repository structure

The repository is organized by visibility boundary (open/closed), with shared API specifications and documentation at the top level.

```
.
├── api/                          # OpenAPI / contract specifications
├── closed/                       # Proprietary implementation
│   ├── audit/                    # Audit subsystem
│   ├── dataset-registry/         # Dataset and DatasetVersion persistence
│   ├── deploy/                   # Docker Compose and deployment assets
│   │   └── docker-compose.yml    # Local dev stack
│   ├── experiments/              # Runs, plans, execution engine
│   ├── gateway/                  # API gateway / ingress
│   ├── internal/
│   │   ├── domain/               # Domain entities and value objects
│   │   ├── repo/postgres/        # PostgreSQL repository implementations
│   │   ├── runtimeexec/          # Data Plane runtime execution
│   │   ├── service/
│   │   │   └── artifacts/        # Artifact service with presigned URLs
│   │   └── storage/
│   │       └── objectstore/      # S3-compatible storage abstraction
│   ├── lineage/                  # Lineage tracking subsystem
│   ├── migrations/               # SQL schema migrations
│   ├── quality/                  # Data quality subsystem
│   └── scripts/
│       └── dev.sh                # Local development helper
├── open/                         # Open-source components
│   ├── demo/
│   │   └── quickstart.sh         # Demo / smoke test quickstart
│   └── sdk/                      # Git submodule
│       └── python/               # Python SDK (submodule)
│           ├── src/              # SDK source
│           └── tests/            # SDK tests
├── docs/
│   ├── assets/                   # Banner and visual assets
│   └── enterprise/               # Normative specification
├── tools/
│   └── cicd/                     # CI/CD tooling
├── .github/
│   └── workflows/                # GitHub Actions CI pipelines
├── .golangci.yml                 # Linter config with CP/DP depguard rules
├── .gitmodules                   # Python SDK submodule reference
├── go.mod                        # Go module definition
├── go.sum                        # Go dependency checksums
├── Makefile                      # Common dev commands
├── roadmap.json                  # Production-grade roadmap (M0-M9)
├── LICENSE                       # Apache-2.0
└── README.md
```

---

## Key flows

### Run submission flow

1. User submits RunSpec via Python SDK or API.
2. CP API validates production-run gate (DatasetVersion + commit SHA + EnvironmentLock).
3. CP persists Run, plan, and policy snapshot (DB-first).
4. Scheduler picks up queued run, dispatches to DP.
5. DP launches Kubernetes workload with resource limits and network policy.
6. DP reports heartbeats; CP updates last heartbeat.
7. Workload completes, artifacts uploaded with checksums.
8. DP reports terminal state; CP finalizes run status.
9. AuditEvents emitted throughout.

### Pipeline execution flow

1. PipelineSpec validated (cycles, refs, digests).
2. Deterministic plan generated with stable topological sort.
3. Plan hash computed and plan stored immutably.
4. Scheduler dispatches node-runs respecting DAG dependencies.
5. Retries/backoff applied per node; partial failure policies enforced.
6. Graph status tracked; all transitions audited.

### Reproducibility flow

1. User requests reproducibility bundle for a completed Run.
2. CP assembles: DatasetVersion, CodeRef (commit SHA + repo URL), EnvironmentLock (image digest + SBOM ref), parameters, policy snapshot.
3. Bundle hash computed (stable).
4. Replay creates a new Run from the snapshot and records the linkage.

### Model promotion flow

1. ModelVersion created from a source Run and Artifact.
2. Validation/approval workflow gated by RBAC.
3. Only authorized roles can approve; approval emits AuditEvent.
4. Export interface controlled by policy; exports audited.

### Dev environment flow

1. User requests dev environment within a project.
2. DP controller creates isolated Pod with TTL and quotas.
3. Network policies and template restrictions applied from project policy.
4. Remote access (terminal/IDE) mediated and audited.
5. Transition path from dev work to production-run submission.

### Background enforcement flow

1. Workers check for stale heartbeats and orphaned runs.
2. Reconciliation resolves to terminal states or resumes.
3. Quota and expiry policies evaluated.
4. Violations trigger DB-first state transitions and DP cleanup.
5. All reconciliation decisions audited.

---

## Technology stack

### Languages

* Go (91.4%)
* Python (SDK and tooling)
* SQL / PLpgSQL (migrations and triggers)
* Shell (scripts and CI)

### Control Plane

* Go HTTP API with structured logging and request ID propagation
* Deterministic planner with stable topological sort
* PostgreSQL for authoritative metadata
* S3-compatible object storage for artifacts

### Data Plane

* Kubernetes (Jobs/Pods) for workload execution
* Container isolation with resource limits and network policies
* Ephemeral secrets injection at execution time

### Identity and Security

* OIDC (primary) and SAML (optional) via `coreos/go-oidc`
* OAuth2 via `golang.org/x/oauth2`
* Vault-like external secrets manager
* Project-scoped RBAC with deny-by-default

### Persistence

* PostgreSQL (external supported) via `jackc/pgx`
* S3-compatible object storage via `minio/minio-go`

### Observability

* Prometheus + OpenTelemetry + structured logs
* Correlation IDs across CP and DP

### Packaging and Delivery

* Helm charts (preferred) and/or Kustomize
* Docker and Docker Compose for local development
* Air-gapped bundle support with integrity verification
* SBOM generation and vulnerability scanning in CI

### Tooling

* `golangci-lint` with `depguard` for CP/DP boundary enforcement
* GitHub Actions CI pipelines
* Makefile-based task runner
* Python `unittest` for SDK tests

---

## Setup, development, and operation

### Prerequisites

* Go 1.25+
* Docker and Docker Compose
* Python 3 (for SDK development and linting)
* PostgreSQL access for non-container workflows
* Git (with submodule support for Python SDK)

### Local development

Initialize the repository and dependencies:

```
make bootstrap
```

Initialize the Python SDK submodule:

```
git submodule update --init --recursive open/sdk
```

Start the local development stack:

```
make dev
```

Run the demo quickstart:

```
make demo
```

Run a smoke test:

```
make demo-smoke
```

Tear down the demo stack:

```
make demo-down
```

### Code quality

Format code:

```
make fmt
```

Run all linters (gofmt, go vet, golangci-lint, Python compileall):

```
make lint
```

Run all tests (Go + Python SDK):

```
make test
```

Build all packages:

```
make build
```

### Development notes

* `closed/deploy/docker-compose.yml` defines the local development stack.
* The Python SDK lives in `open/sdk/python/` as a git submodule.
* `api/` contains OpenAPI/contract specifications; breaking changes require a major version bump.
* `.golangci.yml` enforces CP/DP boundary: control plane packages (`audit`, `dataset-registry`, `gateway`, `lineage`, `quality`) are prohibited from importing `runtimeexec`.
* `docs/enterprise/` contains the normative specification; this README is an entry point and not the normative source.

### Production operation notes

Production deployment uses Helm charts or Kustomize and typically requires:

* Kubernetes cluster with namespace isolation per project.
* External PostgreSQL (non-public, backed up, with RPO/RTO targets).
* S3-compatible object storage with per-project prefix separation.
* OIDC-compatible identity provider (and optional SAML).
* External secrets manager (Vault-like) for ephemeral credential injection.
* SIEM integration for audit export (webhook/syslog).
* Prometheus + OpenTelemetry endpoints for observability.

Operational requirements:

* CP is stateless and supports horizontal scaling (replicas + external DB).
* DP may run on a separate cluster; multi-cluster-ready.
* Network policies must isolate user workload namespaces.
* All secrets must be ephemeral and minimal in scope.
* Backup/restore procedures must be validated; runbooks executed in drills.
* Air-gapped deployments require pre-built bundles with integrity verification and SBOM gates.

---

## Roadmap

The production-grade roadmap is tracked in `roadmap.json` and structured across 10 milestones (M0–M9).

### Milestone summary

* **M0 — Foundations & Architecture Baseline** *(done)*: CP/DP boundary, repo boundaries, Kubernetes baseline, object storage abstraction.
* **M1 — Domain Model & Metadata Core** *(done)*: Project/Dataset/DatasetVersion/Artifact/AuditEvent persistence, immutability enforcement, audit export MVP.
* **M2 — Execution Contracts, Runs & Deterministic Planning** *(in progress)*: PipelineSpec validation, RunSpec bindings, plan persistence, deterministic hashing, dry-run executor.
* **M3 — Data Plane Runtime & Real Execution**: DP executor on Kubernetes, CP↔DP protocol, artifacts commit, reconciliation.
* **M4 — Scheduling, Queues, Quotas, Cancellation**: Production-grade scheduling, retry/backoff, cancellation end-to-end.
* **M5 — Security & Governance Hardening**: SSO, RBAC matrix, secrets TTL, SIEM-grade audit export, retention policies.
* **M6 — Pipelines (DAG) & Orchestration**: PipelineRun with node-runs, DAG engine, graph query APIs.
* **M7 — Developer Environments**: Managed dev environments with audited remote access, TTL, templates.
* **M8 — Model Registry & Promotion**: Model lifecycle, validation/approval, export hooks, lineage integration.
* **M9 — Operability, HA/DR, Packaging, Supply Chain, E2E Acceptance & Release**: HA, observability, backups/DR, Helm packaging, air-gapped/SBOM gates, automated E2E acceptance.

### Production-grade definition

The platform is production-grade when all mandatory acceptance criteria are satisfied and verified on a working installation with security and audit policies enabled:

* End-to-end ML lifecycle runs within a Project without external stitching.
* Any production-run reproducible from DatasetVersion + commit SHA + EnvironmentLock.
* All state-changing actions audited, append-only, exportable.
* SSO + RBAC enforced end-to-end with object-level authorization.
* Install/upgrade/rollback automated and documented.
* No implicit state; every result explainable via domain graph.
* HA control plane, observability, backup/DR validated.

---

## Explicit non-goals

* Built-in Git hosting or full IDE replacement.
* A full inference platform (export to external systems is supported).
* A standalone Feature Store product (interfaces may be integrated).

---

## Documentation

The authoritative specification is in `docs/enterprise/`. This README is an entry point and is not the normative source.

---

## License

Apache-2.0. See [LICENSE](LICENSE).

---

## Contact

For engineering, maintenance, or operational contact:

**[rewanderer@proton.me](mailto:rewanderer@proton.me)**