## 1. Existing Capability Audit

- [x] 1.1 Inventory the existing group list, account list, account detail/update, bulk update, mixed-channel validation, and account-group priority APIs used by the new workspace.
- [x] 1.2 Add or adjust typed frontend API helpers only where the existing endpoints cannot safely express add/remove membership without overwriting unrelated group relationships.
- [x] 1.3 Add focused backend tests and the smallest required endpoint only if group-relative `AccountGroup.priority` cannot be updated through an existing API.

## 2. Resource Pool Workspace Foundation

- [x] 2.1 Add `/admin/resource-pools` and a hidden legacy advanced-group route, with query parsing for all, ungrouped, and selected-group scopes.
- [x] 2.2 Implement the Element Plus resource pool page shell with responsive two-column desktop layout and narrow-screen navigation drawer/select behavior.
- [x] 2.3 Implement the Element Plus resource pool navigator with all accounts, ungrouped accounts, platform-grouped real pools, search, counts, and URL-synchronized selection.
- [x] 2.4 Implement the selected resource pool summary card using existing group availability/count data and contextual group actions.

## 3. Contextual Account Management

- [x] 3.1 Implement the Element Plus account table with server-side pagination, search, platform/status/type filters, selection, health state, usage summary, groups, and common account actions.
- [x] 3.2 Reuse the existing create/edit/test/statistics account flows and make account creation default to the currently selected real group.
- [x] 3.3 Implement clickable account group tags that navigate to the corresponding resource pool without losing supported filters.
- [x] 3.4 Implement an Element Plus “add existing accounts” dialog with search, pagination, exclusion of already-bound accounts, and mixed-channel validation.
- [x] 3.5 Implement batch add-to-group and remove-from-current-group operations that preserve every unrelated group membership and report partial failures.
- [x] 3.6 Implement group-relative priority display and editing against the verified relationship API without changing account-global priority.

## 4. Contextual Group Management

- [x] 4.1 Implement an Element Plus lightweight group creation flow and select the new group after successful creation.
- [x] 4.2 Provide selected-group edit, duplicate, refresh, and delete actions with explicit account-preservation messaging.
- [x] 4.3 Connect advanced pricing, model routing, RPM, Composite, and platform-specific settings to the existing complete group configuration capability with a return path to the selected pool.
- [x] 4.4 Refresh only affected navigation counts, summary data, and account lists after group or relationship mutations.

## 5. Navigation and Compatibility

- [x] 5.1 Replace the separate account and group sidebar entries with one “资源池管理” entry and add complete Chinese and English locale strings.
- [x] 5.2 Redirect `/admin/accounts` to the all-accounts resource pool scope and `/admin/groups` to the group management scope while preserving meaningful query context.
- [x] 5.3 Update dashboard shortcuts, onboarding text, subscription links, channel guidance, and other internal links to target the resource pool workspace or hidden advanced-group route as appropriate.

## 6. Verification

- [x] 6.1 Add component tests for resource pool navigation, URL restoration, all/ungrouped/group account queries, and responsive navigation behavior.
- [x] 6.2 Add interaction tests for default group creation context, adding existing accounts, preserving multi-group memberships, removing only the current membership, and editing relationship priority.
- [x] 6.3 Add route and sidebar tests covering the new menu and both legacy redirects.
- [x] 6.4 Run frontend lint, type checking, affected Vitest suites, and any backend tests introduced for relationship operations.
- [x] 6.5 Perform a production build and manually verify the primary desktop and narrow-screen resource pool workflows.

## 7. Compatible Upstream Account Inspection

- [x] 7.1 Add an authenticated backend probe endpoint that validates a temporary upstream Base URL, calls the compatible `/api/v1/keys` endpoint with a Bearer credential, aggregates bound groups, applies timeout/response-size/redirect safety, and returns a stable group projection without persisting or logging credentials.
- [x] 7.2 Add typed frontend API support and an Element Plus upstream account management drawer opened from the resource pool page, using 50% desktop width and clearing credentials/results on close.
- [x] 7.3 Render all probed upstream groups in an Element Plus table with loading, empty, success, and actionable failure states.
- [x] 7.4 Add focused backend and frontend tests for successful probing, authentication or incompatible-response failures, drawer interaction, table rendering, and ephemeral credential cleanup.
- [x] 7.5 Run affected backend tests, frontend Vitest/type checking/lint, and the production build.
