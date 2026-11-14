## Context

See proposal.md and docs/operations-console-design.md. Existing user Pelican is browser-local; existing operations management is financial reporting. The workspace contains unrelated uncommitted changes that must be preserved.

## Goals / Non-Goals

Goals: implement the approved design using existing Vue/Gin/Ent/PostgreSQL/S3 infrastructure and exactly two new tables. Non-goals: OAuth accounts, public artifacts, automatic quality scores and new dependencies.

## Decisions

- Use task and result tables; JSON snapshots avoid extra join/round/queue tables. Durable result rows support crash recovery and storage cleanup.
- Use four unique database pool slots and ownership tokens rather than a semaphore per request or instance. Generation and upload hold the same slot.
- Run a dedicated service with lifecycle-managed workers, schedule scans, recovery and expiry scans; do not reuse existing scheduled tests.
- Reuse account protocol/proxy/model mapping and S3 connection parsing. Stream adapters collect HTML and first-thinking/text/token measurements.
- Persist object bindings before upload, encrypt stored bindings, retrieve through authenticated JSON endpoints, and display only sandboxed srcdoc.
- Use the approved default model-per-account configuration with shared effort, ten-minute execution budget, no automatic regeneration and seven-day retention.

## Risks / Trade-offs

- Cross-system transactions are unavailable → persist deterministic object keys before PUT and retry recovery/deletion idempotently.
- Remote generation may survive disconnection → cancel on ownership loss and do not replay an abandoned generation before its deadline.
- Existing bucket permissions are external → check connection capabilities, preserve private preview routing, and report deletion failures.

## Migration Plan

Add two tables and their indexes, regenerate Ent and Wire, deploy normally. Roll back application code with data retained; do not drop artifacts or tables during rollback.

## Implementation Details

- Group lifecycle: classify each account/group membership separately. Current memberships belong to accounts selected in any nondeleted task, even if its schedule is disabled. Past memberships have retained result snapshots but are no longer configured for that account. A group can appear in both tabs with different accounts; another configured account in the same group must not keep a removed account's results current. Never-configured accounts without results are absent. Both group/account options and result queries honor this membership scope before pagination and ranking. Historical cleanup is group-specific and ignores UI status/task filters. A transaction locks task configuration against changes, excludes still-configured memberships, rejects running/uploading work, detaches the chosen group from shared snapshots, cancels exclusive queued work and expires exclusive results. Objects are removed before database rows; cleanup failures retain encrypted bindings for retry. No new tables are needed.

- The wall groups by account across plans, initially loading four newest matching tests per row from history, with no history/latest mode switch. Each row ends with a More button fetching three older matching results and appending them in the same row. Desktop rows fit three responsive cards per viewport; tablets fit two and phones one. Columns continue horizontally without wrapping. Wheel scrolling over cards, including inline previews, moves the account row; at either boundary it releases scrolling to the page. Fullscreen artifacts retain their mouse interactions. The grouped API reports whether each account has older results; individual account history uses a chronological cursor. Account pagination remains independent from result pagination. Legacy latest/history API queries remain supported.

- The result table keeps indexed queue, ownership, expiry and storage-location columns separately; `payload` stores the logical name/configuration snapshots and flat metrics described in the approved document. This preserves the API contract with exactly two tables. Internal coalesced trigger aliases also live in `payload` and are omitted from public DTOs.
- Compact cards: account names are shown once in the row heading. Card headers retain group/time, parameter tooltip and status in one compact line. Card widths are 80% of the prior responsive three/two/one-column widths; older results remain on the horizontal track. Operations previews opt into a stable 1280×800 virtual viewport, measure generated content including negative offsets from centered overflow inside the sandbox after load/fonts settle, and scale it to fit. The outer iframe scales proportionally with the available stage via ResizeObserver, including fullscreen, without internal preview scrollbars. Standalone user previews retain their existing sizing behavior.
- Both `apikey` and Base URL/key `upstream` accounts are eligible. Upstream protocol, proxy, authentication overrides and model mapping reuse account helpers. Live model catalogs have a five-minute cache whose fingerprint includes credentials and protocol configuration; explicit refresh bypasses it.
- Runtime loss of account/storage settings produces persisted failed units. Interrupted workers are fenced after the hard deadline plus a one-minute upload grace; known object positions remain available for cleanup, and interrupted generation is never automatically replayed.
- Preview reuse adds an opt-out for opening HTML directly in a new tab. Operations opts out, including the fullscreen failure fallback, so artifacts remain in the sandbox.
- Storage binding encryption requires the existing persistent `TOTP_ENCRYPTION_KEY`. The connection picker reuses backup S3 or effective image storage settings without changing their prefixes.
