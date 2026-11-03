## 1. Backend

- [x] 1.1 Add paginated account loading, URL deduplication and protected balance endpoint.
- [x] 1.2 Query remote usage with stored credentials and existing validated transport.
- [x] 1.3 Cover deduplication, balance variants and upstream errors with tests.

## 2. Frontend

- [x] 2.1 Add operations navigation, route, zh/en strings and five statistics.
- [x] 2.2 Copy model/group distributions using ECharts and retain user breakdowns; preserve the original usage records charts.
- [x] 2.3 Add live upstream balance table with independent refresh and error states.
- [x] 2.4 Cover time filters, profits, stale responses and chart lifecycle with tests.
- [x] 2.5 Position upstream balances on the second row, simplify table columns and explanatory text, and add default blur with an eye toggle and clickable revealed addresses.

## 3. Verification

- [x] 3.1 Run backend tests, frontend lint, typecheck, tests and production build (one pre-existing Stripe chunk assertion fails; see verification.md).
- [x] 3.2 Verify desktop, mobile and dark theme in a real browser.
