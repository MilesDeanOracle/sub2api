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

## 7. History cleanup controls

- [x] 7.1 Rename the past tab to 历史测试 / Test history, retain independently loaded group choices and expose catalog failures with retry.
- [x] 7.2 Add confirmed account cleanup, optionally scoped to the selected group, preserving configured memberships and shared artifacts through the existing cleanup pipeline.
- [x] 7.3 Verify group/account cleanup, authorization, failure handling and translations with targeted checks (completed before the subsequent request to skip testing).

## 8. Three-card lazy history stack

- [x] 8.1 Limit grouped result responses and initial row cards to the latest three matching results.
- [x] 8.2 Replace per-row More with three card backs layered behind the third result, progressively offset to the right, scaled down and faded as in the supplied reference; expand rightward on hover/focus/tap and fetch up to three older results after the first expansion, retaining loaded cards and previews when folded.
- [x] 8.3 Preserve horizontal scrolling, filters, cleanup and lazy-history request cancellation; synchronize existing fixtures and documents without executing tests, as requested.

## 9. Cached folded cards and parallel plans

- [x] 9.1 Retain activated card components, loaded HTML and preview iframe instances when folded or scrolled out of view; prevent unopened cards from initiating content requests while folded.
- [x] 9.2 Prepare due plans concurrently with a bounded pool, balance free database execution slots across plans and allow independent per-plan UI actions while preserving the global four-worker limit.
- [x] 9.3 Synchronize design and source documentation; honor the explicit instruction not to run tests, builds or browser verification.

## 10. Quick routing priority

- [x] 10.1 Include current account routing priority in the lightweight wall catalog; omit editing for deleted accounts.
- [x] 10.2 Reuse the account priority editor after each row title, persist through the existing admin account API and update the catalog without remounting previews.
- [x] 10.3 Document that smaller values take precedence across all of the account's groups, with bilingual labels; read back source without running tests or builds.
