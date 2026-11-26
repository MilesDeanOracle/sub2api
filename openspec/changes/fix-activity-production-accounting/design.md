# Accounting and migration decisions

Billing owns the authoritative spend event. While holding the existing user debit
lock, the repository fixes the gift/principal split and writes an activity event
and durable processing outbox in the same transaction. Both individual and merged
billing use this path. Activity event IDs use a negative sequence to coexist with
historical positive usage-log IDs; workers claim pending rows independently of
sequence order. Image holds reserve their gift composition at reservation time,
return unused gifts on release/capture, and never consume later wallet transfers.
Unknown pre-upgrade image hold composition is excluded from reward progress.
Administrative correction accepts either signed nonzero event ID; its ledger is
ordered by occurrence time. User-facing processing status reads durable pending
events and unacknowledged pre-cutover logs, not the diagnostic scan cursor.

The worker reconciles legacy logs using a forward page plus a rotating historical
page and durable per-log acknowledgements. Each pass visits bounded pages; the
historical sweep has no age cutoff, so late commits below the high-water mark are
not discarded. Modern logs are matched to their already committed billing event. Logs dated after the cutover never create reward progress without a transactional bill; simple-mode, non-billed usage cannot earn rewards.
A delayed legacy log can only absorb gift available before its timestamp; later
wallet transfers are excluded. Full historical source guarantees require draining
old writers before the billing cutover. Modern billing does not rely on log arrival.

`repository/activitystore` provides transaction-scoped persistence for billing,
wallet transfers, refunds and debt recovery without importing services. Services
retain eligibility checks, refund split calculations and orchestration; SQL and
Ent callers pass their own transaction so boundaries do not weaken atomicity.

The migration runner retains and validates all applied immutable historical files.
It explicitly retires missing trigger-based files instead of installing them on
new deployments. A compatibility prelude adds the old schema's missing columns,
archives its outbox and removes old triggers/core foreign keys before existing
isolated migrations execute. Forward conversion preserves eligible legacy spend,
entitlements, audits and already credited rewards without issuing funds twice.
Original migration records/checksums are never fabricated, removed or rewritten.
