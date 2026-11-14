## Purpose

Provide administrators with an isolated operations workspace for scheduled HTML generation using existing upstream accounts, persistent results, comparable performance metrics, and automatic removal of expired artifacts and database records.

## ADDED Requirements

### Requirement: Administrator workspace
The system SHALL open an administrator-only operations workspace in a new browser tab with Pelican Wall and Configuration menus, and SHALL authenticate all result content requests.

#### Scenario: Access control
- **WHEN** an ordinary user requests the workspace or its APIs
- **THEN** access is denied and credentials and artifacts are not disclosed.

### Requirement: Multiple generation plans
The system SHALL support multiple hourly plans with selected API-key accounts, upstream-discovered models, one shared reasoning effort per plan, editable prompts, and reused object-storage connections.

#### Scenario: Immediate enable and manual execution
- **WHEN** a plan is enabled
- **THEN** one round is queued immediately and subsequent rounds follow its hourly interval; manual rounds use the same pool without shifting the schedule.

### Requirement: Global execution pool
The system SHALL persist account work items using only two new business tables and SHALL permit at most four globally active generation units across plans and instances, refilling a free slot immediately.

#### Scenario: Ten accounts and another plan
- **WHEN** ten accounts are queued and another plan is triggered
- **THEN** no more than four units are active and remaining units stay queued until slots become available.

### Requirement: Protocols and measurements
The system SHALL use configured account protocols, preserve failed results, measure first visible reasoning or fallback text latency, and report TPS only from complete upstream usage and valid text timing.

#### Scenario: Multiple group membership
- **WHEN** an account belongs to multiple groups in a round
- **THEN** it is called once and its result records all group snapshots.

### Requirement: Wall and historical artifacts
The system SHALL show one row per account across plans, initially containing four newest matching test results in descending creation order, including failures and active tests. It SHALL expose no history/latest mode buttons. Each row SHALL offer View more to append three older matching results for that account until exhausted. It SHALL support dependent group/account filters, sandboxed preview and fullscreen, with group, account, model, effort, time, status and measurements. Account pagination SHALL operate on whole account rows, independently of the per-row result cursor.

#### Scenario: Latest failure
- **WHEN** a newer run fails after an older successful run
- **THEN** the default wall shows the failure and history retains the older successful artifact.

#### Scenario: Account rows across plans and pages
- **WHEN** one account has six results across multiple plans and another has one result
- **THEN** the first row initially shows the first account's four newest matching results, the second row shows the other account's result, and loading another page never splits or duplicates an account row.

#### Scenario: Load older tests in one row
- **WHEN** an account has ten matching tests and the administrator clicks View more twice
- **THEN** its row grows from four to seven and then ten results without affecting other rows; the row reports exhaustion and retains all active filters.

#### Scenario: Compact card with oversized artwork
- **WHEN** a generated document is larger than the card preview, including content centered outside the viewport
- **THEN** the document is scaled proportionally to remain fully visible without preview scrollbars, resize and fullscreen preserve the fit, account names appear only in row headings, and card widths are reduced by twenty percent with metadata and metrics retained.

### Requirement: Storage and retention
The system SHALL store artifacts below operations/pelican/, record recoverable storage locations before upload, and remove expired objects before removing database records after seven days.

#### Scenario: Delete failure
- **WHEN** deleting an expired object fails
- **THEN** its database record and location remain available for cleanup retry while the expired result is hidden from the wall.

### Requirement: Group lifecycle tabs and manual cleanup
The wall SHALL provide Current detection groups and Previously detected groups tabs. Current memberships SHALL derive from current memberships of accounts selected in nondeleted tasks, including tasks with disabled schedules. Past memberships SHALL derive from retained snapshots excluding memberships still configured for that specific account. The same group MAY appear in both tabs with different accounts. Never-configured accounts without results SHALL not appear. Scope SHALL apply to filters, result ranking and both forms of pagination.

#### Scenario: Remove an account from configuration
- **WHEN** an account has retained test results but is not selected in any current task
- **THEN** its results appear only in the past tab, even if another account in the same group remains configured, with a group-specific cleanup action that preserves configured accounts' results.

#### Scenario: Clean a historical group
- **WHEN** an administrator confirms cleanup for a historical group
- **THEN** exclusive results have their objects deleted before database removal, shared results retain their other group associations, and storage failures preserve retryable cleanup records.

#### Scenario: Concurrent configuration or execution
- **WHEN** cleanup targets a group that has only currently configured memberships, or its eligible historical memberships have running/uploading tests
- **THEN** cleanup is rejected without deleting its results; mixed groups only clean eligible historical memberships.
