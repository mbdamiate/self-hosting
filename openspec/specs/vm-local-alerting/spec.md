# vm-local-alerting Specification

## Purpose
TBD - created by archiving change add-vm-observability. Update Purpose after archive.
## Requirements
### Requirement: A triggered alert is written to the host journal
When an uptime state-transition alert (per `vm-uptime-monitoring`) fires, it SHALL be recorded via `logger` under a dedicated, filterable tag.

#### Scenario: Alert fires
- **WHEN** a VM's status transitions from up to down or from down to up
- **THEN** a message is logged via `logger -t self-hosting-alert`, identifying the VM and the transition

### Requirement: A triggered alert is broadcast to logged-in host users
When an alert fires, it SHALL also be broadcast via `wall` to users currently logged into the host.

#### Scenario: Alert fires while an operator is logged into the host
- **WHEN** a VM's status transitions and an operator has an active session on the host
- **THEN** the operator sees a `wall` broadcast describing the transition

### Requirement: Recent alerts are shown at host login
Setup (when installing the host-wide monitoring infrastructure) SHALL install an `/etc/update-motd.d/` script that displays recent entries logged under the `self-hosting-alert` tag at login.

#### Scenario: Operator logs into the host after being away
- **WHEN** an operator logs into the host (locally or via SSH) after one or more alerts fired while no one was logged in
- **THEN** the login banner includes a summary of the most recent `self-hosting-alert` journal entries

#### Scenario: No recent alerts
- **WHEN** an operator logs into the host and no `self-hosting-alert` entries exist within the summary's lookback window
- **THEN** the login banner shows no alert summary (or an explicit "no recent alerts" line)

### Requirement: No remote alert channel is included
This capability SHALL NOT send alerts over the network (email, webhook, push notification, or otherwise) — alerts are host-local only.

#### Scenario: Alert fires
- **WHEN** any alert fires
- **THEN** no network request is made and no external service is contacted as part of delivering it

