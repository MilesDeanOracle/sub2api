## 1. Persistence

- [x] 1.1 Add task/result schemas, migration, indexes and generated Ent code.
- [x] 1.2 Implement transactional configuration, durable rounds, global slots, latest/history filtering and cleanup persistence.

## 2. Execution

- [x] 2.1 Implement configured-account discovery, upstream model discovery and protocol-specific streamed generation with metrics.
- [x] 2.2 Implement reused encrypted S3 bindings, deterministic artifacts and authenticated content reading.
- [x] 2.3 Implement independent scheduling, four shared slots, lifecycle, recovery and seven-day dual cleanup.

## 3. API and UI

- [x] 3.1 Wire administrator APIs, auditing, restricted-key denial and lifecycle into the application.
- [x] 3.2 Implement new-tab entry, independent layout and task configuration with bilingual text.
- [x] 3.3 Implement latest/history wall, dependent filters, progress, sandbox preview and fullscreen.

## 4. Verification

- [x] 4.1 Add meaningful pool, schedule, streaming, storage/cleanup, authorization and UI regression coverage.
- [x] 4.2 Run code generation, relevant backend/frontend checks and OpenSpec validation, resolving regressions.

## 5. Group lifecycle

- [x] 5.1 Derive current groups from accounts selected in nondeleted tasks (including disabled schedules), and past groups from retained snapshots, applying scope to filters and paginated results.
- [x] 5.2 Add current/past tabs and confirmed historical group cleanup; preserve other group associations of shared results and delete exclusive artifacts before database rows, retaining failed cleanup for retry.
- [x] 5.3 Verify classification, scoped pagination, cleanup protection, shared results, storage failures and UI regressions.
- [x] 5.4 Classify individual account/group memberships, verify removed accounts sharing a configured group move to past, and preserve configured results during mixed-group cleanup.

## 6. Compact fitted previews

- [x] 6.1 Remove repeated account names from card headers, reduce responsive card width to 80% and compact the header/footer while preserving metadata and metrics.
- [x] 6.2 Fit generated documents proportionally inside card and fullscreen previews, preserving sandbox isolation; verify oversized and centered content, resize, replay and mobile layout.
