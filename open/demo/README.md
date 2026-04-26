# Full-surface demo

Control, not a demo. This directory contains the deterministic dataset and userspace runner used by the full-surface demo.
Requirements: docker compose (or docker-compose), curl (preferred) or python3.

Quickstart:

```bash
make demo
```

Smoke check:

```bash
make demo-smoke
```

No-docker smoke check (requires running services):

```bash
DEMO_NO_DOCKER=1 DEMO_BASE_URL=http://localhost:8080 make demo-smoke
```

Golden demo transcript:

```bash
DEMO_GOLDEN=1 make demo
```

Regeneration requires Docker daemon access. In restricted environments (no Docker), the transcript cannot be regenerated.

Optional smoke transcript (requires running services):

```bash
DEMO_GOLDEN=1 DEMO_NO_DOCKER=1 DEMO_BASE_URL=http://localhost:8080 make demo-smoke
```

Excerpt:

```
==> starting demo stack
==> waiting for gateway <gateway_url>/healthz
==> waiting for userspace <userspace_url>/healthz
=== Create run ===
==> create run
run_id=<run_id>
spec_hash=<spec_hash>
=== Audit export ===
==> audit export (first 3 lines)
```

Full transcript: `open/demo/golden/demo-transcript.txt`

Components:
- `docker-compose.yml`: control plane + userspace demo stack.
- `data/`: deterministic demo dataset.
- `userspace/`: allowlisted data plane simulation container.

The demo runs:
- Project creation and scoping
- Dataset registry + dataset version upload
- Artifact presign + upload
- Run create -> plan -> dry-run -> derived state
- Audit export
- Userspace execution simulation (no user code in control plane)
