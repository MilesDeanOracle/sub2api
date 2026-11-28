# Fix activity production accounting

The production review found two refund regressions, an incompatible migration lineage, and consumption eligibility based on asynchronously scanned logs and the current gift balance. Fix these before deployment. Preserve independent budgets, eight lottery tiers, legacy ticket odds, and manual transfers to site balance.

## Scope

- Require an explicit force override when refunding a balance order without reclaiming principal; query providers in the original payment currency.
- Preserve immutable historical migrations and explicitly bridge the legacy and isolated activity schemas without replaying old core-table triggers on new installations.
- Persist consumption eligibility in the billing transaction, serialized with wallet transfers on the user row. Process durable events independently of sequence/commit order. Keep a deduplicated legacy-log reconciliation path.
- Put atomic accounting persistence in the repository layer and keep business decisions in services.
- Move lottery administration copy into synchronized zh/en locales.
