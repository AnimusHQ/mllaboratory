# Image release pipeline (deterministic + signed)

## Scope

Pipeline covers container release integrity for production profile:

- deterministic `docker buildx` builds (`linux/amd64`)
- captured image digests (`artifacts/images.json`)
- SBOM generation and validation
- cosign signature + provenance attestation
- digest-only production Helm values validation

## Local commands

```bash
make images-build
make sbom
make sbom-check
make sign-images
make verify-images
make repro-check
```

Notes:

- `make images-build` writes:
  - `artifacts/images.json`
  - `artifacts/images.txt`
  - `artifacts/helm/animus-datapilot-values-production.generated.yaml`
  - `artifacts/helm/animus-dataplane-values-production.generated.yaml`
- For local key-based signing set:
  - `COSIGN_KEY=/path/to/cosign.key`
  - `COSIGN_PUBLIC_KEY=/path/to/cosign.pub`

## CI workflow

`/.github/workflows/release-images.yml` performs:

1. build images + digest capture
2. SBOM generation
3. SBOM integrity check
4. cosign sign + provenance attestation
5. cosign verify + attestation verify
6. reproducibility checks (including manifest-to-values digest consistency)

## Policy

- `latest` tag is forbidden for release builds.
- Production Helm values must be digest-based.
- `profile=production` chart render fails when digest is missing.
- Tool auto-install in scripts requires explicit `ANIMUS_INSTALL_TOOLS=1`.
