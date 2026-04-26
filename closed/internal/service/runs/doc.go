// Package runs implements the execution state machine for project-scoped runs.
//
// States:
//   - created -> planned -> dryrun_running -> dryrun_succeeded | dryrun_failed
//
// Transitions are derived from the stored ExecutionPlan and step_executions via
// DeriveAndPersistWithAudit. Read-only callers should use Derive, which does not
// mutate persisted status. Explicit transitions (e.g. dryrun_running) must be
// applied through the service to enforce invariants.
//
// Auditing:
//   - Successful transitions emit exactly one run-level audit event.
//   - Rejected transitions do not emit audit events (callers should handle errors).
//
// Concurrency & idempotency:
//   - Transitions are applied under a run-row lock when executed inside a DB transaction.
//   - Re-applying the same transition is a no-op and does not emit duplicate audits.
//   - The service derives a stable idempotency key from (project_id, run_id, from, to),
//     and uses the request ID when present for correlation.
package runs
