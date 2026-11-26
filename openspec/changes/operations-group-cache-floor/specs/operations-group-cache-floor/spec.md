## Purpose

Allow administrators to review each public group's real cache hit rate and to configure a minimum hit rate that the public snapshot discloses for that group.

## ADDED Requirements

### Requirement: Operations console menu
The system SHALL add a Group Cache menu to the administrator operations console that lists every group exported by the public snapshot with its multiplier, model count, availability, real cumulative cache hit rate, the real cache read, cache creation and input token counts of the cumulative window shown as three lines in one column, the floor value and the enabled state.

#### Scenario: Real values in the console
- **WHEN** an administrator opens the menu
- **THEN** each row shows the real cumulative cache hit rate and token counts regardless of the configured floor, and an enabled row whose real rate is below 70% highlights that rate in red

### Requirement: Query form
The system SHALL place a query form above the table with a group name keyword, a platform and an enabled state. Only submitting the form SHALL request the list from the server with those conditions, and the server SHALL return the matching groups. Resetting SHALL clear the conditions and request the unfiltered list.

#### Scenario: Query by name, platform and state
- **WHEN** an administrator enters "claude", picks the Anthropic platform and the enabled state, then submits
- **THEN** the server receives all three conditions and the table lists only the enabled Anthropic groups whose name contains "claude"

#### Scenario: Nothing matches
- **WHEN** the submitted query matches no group
- **THEN** the response still carries an empty list and the table shows the empty state for queries

### Requirement: Per-group floor
The system SHALL let full administrators set, for each group, an enabled state and a floor value greater than 0 and less than 100 with at most one decimal place, and SHALL reject any other value.

#### Scenario: Quick edit
- **WHEN** an administrator edits the floor value inline and confirms
- **THEN** the value is persisted for that group alone without changing other groups

### Requirement: Snapshot adjustment
The system SHALL, when serving the public snapshot, compare each enabled group's real hit rate with its floor for the cumulative, last 7 day and last 24 hour windows. For each window whose real rate is below the floor it SHALL return the floor and scale the cache read tokens so that the rate and the token counts agree.

#### Scenario: Real rate below the floor
- **WHEN** a group is enabled with floor 85 and its real cumulative rate is 64.7%
- **THEN** the snapshot returns 85 for the cumulative rate, leaves input and cache creation tokens unchanged and increases cache read tokens accordingly

#### Scenario: Real rate at or above the floor
- **WHEN** a group is enabled with floor 85 and its real rate is 92.5%
- **THEN** the snapshot returns the unmodified window

#### Scenario: Paused or unset group
- **WHEN** a group is paused or has no floor value
- **THEN** the snapshot returns the unmodified windows
