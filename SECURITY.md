# Security Policy

Animus Datalab is an enterprise ML laboratory that treats reproducibility, auditability, and security as platform properties. Please report suspected vulnerabilities privately.

## Reporting a vulnerability

Do not open a public GitHub issue for security-sensitive reports.

Send reports to:

**[rewanderer@proton.me](mailto:rewanderer@proton.me)**

Please include, when possible:

- affected component or path;
- affected version, branch, or commit;
- reproduction steps;
- observed impact;
- logs or traces with secrets removed;
- whether the issue affects local development, Kubernetes deployment, Control Plane services, Data Plane execution, authentication, RBAC, audit, object storage, or supply-chain controls.

## Security-sensitive areas

The following areas are especially sensitive:

- Gateway authentication and session handling;
- project-scoped RBAC and role-gated API surfaces;
- internal-only Control Plane / Data Plane channels;
- execution-bound run tokens and workload credentials;
- secret injection and secret-provider integrations;
- audit event integrity and append-only behavior;
- immutable dataset, run, policy, and execution-plan records;
- object-store references, content hashes, and artifact mediation;
- Helm values, image pinning, and signed-image policy controls;
- boundaries between Control Plane governance code and Data Plane runtime execution internals.

## Security posture

The expected platform posture is deny-by-default:

- unauthenticated or insufficiently scoped requests should fail closed;
- mutating operations should require editor-grade or stronger authorization;
- admin-oriented audit, lineage, and quality surfaces should remain restricted;
- internal CP/DP APIs must not be exposed as public API surfaces;
- secrets must not be logged, returned to UI clients, or persisted as general application state;
- workload state is not authoritative until reconciled into PostgreSQL-backed Control Plane state.

## Supported versions

Until the project publishes versioned releases, security assessment should target the current `main` branch and any tagged releases once available.

## Disclosure

Animus will prioritize triage for reports that include a clear impact path. Coordinated disclosure timing should be agreed before public discussion.
