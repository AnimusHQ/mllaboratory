## Summary

Describe the change and the component it affects.

## Type of change

- [ ] Documentation
- [ ] Bug fix
- [ ] Feature
- [ ] Refactor
- [ ] Tests / CI / hygiene
- [ ] Security hardening

## Repository truth model

- [ ] I checked the canonical source for the behavior I changed.
- [ ] I updated migrations, OpenAPI contracts, Helm values, docs, or service code where applicable.
- [ ] I did not treat legacy compatibility shims as canonical sources of truth.

## Architecture and security

- [ ] Control Plane / Data Plane boundaries are preserved.
- [ ] Internal-only surfaces remain internal-only.
- [ ] Project-scoped RBAC and deny-by-default posture are preserved.
- [ ] No secrets, credentials, private endpoints, or sensitive object references are included.

## Validation

- [ ] `make fmt`
- [ ] `make lint`
- [ ] `make test`
- [ ] `make build`
- [ ] `make dev DEV_ARGS=--smoke`
- [ ] Not applicable / explained below

## Notes

Add migration notes, rollout notes, screenshots, logs, or compatibility details.
