## ADDED Requirements

### Requirement: Date filtered operating statistics
The system SHALL provide an administrator operations menu with total actual consumption, account cost, profit, request count and token count for the selected date range. Profit SHALL equal actual consumption minus account cost.

#### Scenario: Negative profit
- **WHEN** account cost exceeds actual consumption for the selected dates
- **THEN** the page displays a negative profit without clamping it to zero

### Requirement: Live upstream balances
The system SHALL query upstream `/v1/usage` using configured account base_url and api_key, deduplicate equivalent Base URLs and display upstream address, associated account count, amount, query time and result status without returning credentials. The balance table SHALL appear below statistics cards and above distribution charts.

#### Scenario: Reveal upstream table data
- **WHEN** an administrator opens the operations page
- **THEN** table data is blurred and its links cannot be focused or activated until the eye button is clicked
- **AND** clicking the eye button again hides the data, while revealed upstream addresses open in a new tab

#### Scenario: Duplicate upstream site
- **WHEN** multiple accounts configure equivalent Base URLs
- **THEN** only one upstream query is performed and the associated account count is displayed

#### Scenario: Upstream failure
- **WHEN** an upstream fails or does not supply a numeric balance
- **THEN** its balance is shown as unavailable with an explicit status

### Requirement: ECharts usage distributions
The system SHALL copy model and group distribution functionality from usage records into operations management and render the copies with ECharts while preserving date filters, token/actual-cost metrics, model-source selection and user breakdowns. The original usage records page SHALL retain its existing charts and functionality.

#### Scenario: Original usage charts remain available
- **WHEN** an administrator opens the usage records page
- **THEN** the original model and group distribution charts remain visible with their existing data loading and metric controls

#### Scenario: Source changes while a request is pending
- **WHEN** an administrator selects a new model source before the previous request finishes
- **THEN** only the latest source response is displayed
