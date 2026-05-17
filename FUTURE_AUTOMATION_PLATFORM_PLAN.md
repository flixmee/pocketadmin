# Future Automation Platform Plan

## Vision

Transform PocketAdmin automation from a simple workflow engine into a full capability orchestration platform capable of powering:

- no-code automation
- AI-native workflows
- enterprise orchestration
- reusable integrations
- long-running processes
- human approval systems
- event-driven architectures
- marketplace ecosystems

---

# Strategic Shift

Current model:

```text
trigger → steps → done
```

Future model:

```text
events → capabilities → orchestration → memory → AI → approvals → policies
```

The future system should focus on:

- capabilities
- state
- orchestration
- intent
- safety
- observability
- scalability

instead of only step execution.

---

# Core Architecture Goals

## Goals

- non-technical users can automate complex operations
- workflows become reusable and composable
- AI can safely participate in execution
- long-running workflows become possible
- integrations become pluggable
- workflows become observable and replayable
- runtime becomes scalable and resilient

---

# Major Architectural Upgrades

---

# 1. Capability Registry

## Problem

Current steps are implementation-centric.

Example:

```json
{
  "type": "http"
}
```

This is not scalable for no-code systems.

---

## Solution

Introduce reusable capabilities.

Example:

```json
{
  "capability": "slack.send_message"
}
```

Capabilities abstract implementation details.

---

## New System Collection

```text
_capabilities
```

---

## Recommended Fields

| Field | Purpose |
|---|---|
| key | unique capability key |
| version | semantic version |
| category | grouping |
| icon | UI icon |
| inputSchema | JSON schema |
| outputSchema | JSON schema |
| authStrategy | OAuth/API key/etc |
| runtimeHandler | execution binding |
| configUI | dynamic form config |
| active | enable/disable |

---

## Benefits

- reusable integrations
- plugin architecture
- AI-assisted generation
- safer upgrades
- marketplace compatibility
- schema validation
- autocomplete support

---

# 2. Persistent Workflow State

## Problem

Current runs are stateless.

This prevents:

- resumable workflows
- approvals
- delayed jobs
- long-running execution
- AI memory

---

## Solution

Add persistent workflow state.

---

## New Collection

```text
_workflowState
```

---

## Example

```json
{
  "workflowId": "abc",
  "context": {
    "approvalRequired": true,
    "customerTier": "enterprise"
  }
}
```

---

## Benefits

- resumable execution
- async coordination
- AI context memory
- recovery support
- durable orchestration

---

# 3. Async Wait/Resume Engine

## Problem

Current execution is synchronous.

Future workflows need:

- delays
- approvals
- waiting for webhooks
- waiting for external systems
- event subscriptions

---

## New Step Types

### wait.delay

```json
{
  "type": "wait.delay",
  "duration": "3d"
}
```

---

### wait.webhook

```json
{
  "type": "wait.webhook",
  "key": "payment_completed"
}
```

---

### wait.event

```json
{
  "type": "wait.event",
  "event": "invoice.paid"
}
```

---

### wait.approval

```json
{
  "type": "wait.approval",
  "role": "manager"
}
```

---

## Runtime Changes

Workflow runtime must support:

- pausing
- resuming
- checkpointing
- event correlation
- expiration handling

---

# 4. Human Approval System

## New Collection

```text
_approvals
```

---

## Recommended Fields

| Field | Purpose |
|---|---|
| workflowId | linked workflow |
| stepId | pending step |
| assignee | target user |
| role | approval role |
| status | pending/approved/rejected |
| decision | response |
| comment | reviewer note |
| created | timestamp |
| resolved | timestamp |

---

## Use Cases

- publishing approval
- financial approval
- moderation
- AI review
- legal review

---

# 5. AI-Native Workflow Steps

## Problem

Future automation platforms require AI orchestration.

---

## New AI Step Types

### ai.extract

Extract structured data.

---

### ai.classify

Classify sentiment/category/risk/etc.

---

### ai.generate

Generate emails/content/messages.

---

### ai.summarize

Summarize records/documents.

---

### ai.agent

Multi-tool autonomous execution.

---

## Important Design Rule

Avoid arbitrary prompts initially.

Prefer:

- structured inputs
- typed outputs
- schema validation
- constrained generation

---

## Example

```json
{
  "type": "ai.extract",
  "model": "gpt-5",
  "schema": {
    "invoiceTotal": "number",
    "vendor": "string"
  }
}
```

---

# 6. Typed Schemas Everywhere

## Problem

Template-only systems become fragile at scale.

Example:

```handlebars
{{record.title}}
```

becomes difficult to maintain.

---

## Solution

Every trigger, capability, and step should define:

- inputSchema
- outputSchema

---

## Benefits

- validation
- autocomplete
- visual mapping
- AI generation
- safer workflows
- reusable templates

---

# 7. Internal Event Bus

## Problem

Current triggers are tightly coupled.

---

## Solution

Publish internal events.

Examples:

```text
record.created
invoice.paid
translation.completed
ai.review.finished
```

Workflows subscribe to events.

---

## Benefits

- decoupling
- replay support
- analytics
- scalability
- observability
- distributed systems support

---

# 8. Connector Ecosystem

## Goal

Support integrations with:

- Gmail
- Slack
- Discord
- Stripe
- Shopify
- OpenAI
- GitHub
- Notion
- Telegram

without hardcoding logic.

---

## New Collection

```text
_connectors
```

---

## Recommended Fields

| Field | Purpose |
|---|---|
| provider | service name |
| authType | OAuth/API key |
| credentials | encrypted secrets |
| scopes | permissions |
| rateLimits | API safety |
| active | enable/disable |

---

## Connector Responsibilities

- auth management
- token refresh
- rate limiting
- capability exposure
- API abstraction

---

# 9. Policy + Safety Engine

## Problem

AI + external actions create risk.

---

## New Collection

```text
_policies
```

---

## Example Policies

- max emails per hour
- domain allowlists
- blocked IP ranges
- PII restrictions
- AI moderation
- approval thresholds
- execution quotas

---

## Benefits

- abuse prevention
- SSRF mitigation
- AI safety
- enterprise compliance
- operational control

---

# 10. Recursion Protection

## Must Not Be Delayed

Future AI systems can create infinite loops.

---

## Required Controls

- max recursion depth
- cycle detection
- dedupe keys
- execution quotas
- cooldown windows
- workflow throttling

---

## Example

```json
{
  "maxDepth": 5,
  "maxRunsPerMinute": 100
}
```

---

# 11. Workflow Versioning

## Problem

Editing live workflows is dangerous.

---

## Required Features

- draft workflows
- published workflows
- rollback support
- immutable historical runs
- change history

---

## Recommended Collections

```text
_workflowVersions
```

---

# 12. Simulation / Dry Run

## Goal

Allow users to safely test workflows.

---

## Features

- replay payloads
- fake outputs
- variable inspection
- execution preview
- time-travel debugging

---

## Benefits

- easier onboarding
- safer deployments
- easier debugging
- AI-assisted workflow generation

---

# 13. Workflow Marketplace

## Goal

Enable reusable workflow templates.

---

## New Collection

```text
_workflowTemplates
```

---

## Features

- import/export
- sharing
- versioning
- organization templates
- AI-generated workflows
- one-click install

---

# Runtime Evolution

---

# Current Runtime

```text
hook → runner → steps
```

---

# Future Runtime

```text
events
  ↓
orchestrator
  ↓
capabilities
  ↓
state engine
  ↓
policy engine
  ↓
AI runtime
  ↓
connectors
  ↓
observability
```

---

# Recommended Platform Capability Areas

---

# Area A — State + Async Runtime

## Goals

- resumable workflows
- delayed execution
- event waiting
- approval suspension

---

## Deliverables

### Collections

- _workflowState
- _workflowLocks

---

## Runtime Features

- pause/resume
- checkpointing
- event correlation
- expiration handling

---

## New Steps

- wait.delay
- wait.event
- wait.webhook
- wait.approval

---

# Area B — Capability Registry

## Goals

- reusable integrations
- plugin system
- schema-driven execution

---

## Deliverables

### Collections

- _capabilities
- _connectors

---

## Runtime Features

- capability registry
- connector abstraction
- OAuth support
- token refresh

---

# Area C — AI Runtime

## Goals

- AI-native workflows
- structured generation
- intelligent orchestration

---

## Deliverables

### AI Step Types

- ai.extract
- ai.classify
- ai.generate
- ai.agent
- ai.summarize

---

## Safety Features

- schema validation
- moderation hooks
- token quotas
- audit logging

---

# Area D — Event Bus

## Goals

- decoupled architecture
- scalable orchestration

---

## Deliverables

### Event Features

- publish/subscribe
- replay
- event retention
- subscriptions

---

## Event Examples

```text
record.created
payment.completed
translation.finished
```

---

# Area E — Versioning + Simulation

## Goals

- safe deployment
- debugging
- rollback

---

## Deliverables

### Features

- workflow drafts
- publish flow
- rollback
- dry-run
- replay engine

---

# Area F — Visual Builder

## Important

Do NOT prioritize this before runtime maturity.

---

## Goals

- no-code workflow editing
- visual mapping
- AI-assisted generation

---

## Features

- drag-and-drop graph editor
- visual data mapping
- inline validation
- template library

---

# Folder Structure Recommendation

```text
core/
  automation/
    runtime/
    orchestration/
    capabilities/
    connectors/
    ai/
    policies/
    events/
    state/
    templates/

ui/
  automations/
    builder/
    runtime/
    simulation/
    marketplace/
```

---

# Security Recommendations

## High Priority

- encrypted secrets
- outbound HTTP restrictions
- SSRF protection
- AI moderation
- execution quotas
- audit trails
- permission boundaries

---

# Enterprise Requirements

## Required Eventually

- multi-tenant isolation
- RBAC
- audit logs
- workflow approvals
- SSO integration
- compliance controls
- export/import
- usage analytics

---

# Recommended Strategic Priority

## Most Important Next Step

Build:

1. capability registry
2. typed schemas
3. async runtime

before visual builders.

These three decisions unlock nearly every future capability.

---

# Final Recommendation

The current architecture is already strong enough to evolve into:

- AI workflow platform
- no-code orchestration engine
- enterprise automation platform
- capability marketplace
- agentic execution system

The critical shift is moving from:

```text
step execution
```

to:

```text
capability orchestration
```

---

# Phase-by-Phase Execution Roadmap

This roadmap turns the future platform direction into implementation phases that fit the current PocketAdmin automation system.

Important numbering note: the existing `AUTOMATION_PLAN.md` already used Phase 9 for i18n translation jobs and i18n automation triggers. To avoid confusion, the future platform work should be tracked as **Platform Phase 0+** instead of continuing the old phase numbers.

---

## Platform Phase 0 — Baseline Hardening

### Goal

Make the current automation runtime stable enough to extend without mixing old step execution behavior with future orchestration behavior.

### Deliverables

- Document current automation contracts in one internal place:
  - trigger payload shape
  - step payload shape
  - step result shape
  - run status lifecycle
- Add missing runtime guardrails:
  - max steps per automation
  - max template output size
  - max HTTP timeout/body size
  - clearer failure reasons for invalid step definitions
- Add a shared internal automation package boundary only when needed by the next phase; avoid a large folder move before behavior is covered.

### Verification

- Existing automation tests still pass.
- Add focused tests for limits and invalid payload errors.
- Admin UI build still passes.

### Exit Criteria

- Current automations behave exactly as before.
- New work can depend on documented trigger/step/run contracts.

---

## Platform Phase 1 — Typed Schemas Foundation

### Goal

Introduce schemas before adding more orchestration features, because schemas unlock validation, autocomplete, capability execution, AI-safe outputs, simulation, and visual mapping.

### Deliverables

- Add internal Go types for:
  - trigger schema
  - step input schema
  - step output schema
  - capability input/output schema
- Start with JSON Schema-compatible objects, stored as JSON.
- Register built-in schemas for current triggers:
  - `record.create`
  - `record.update`
  - `record.delete`
  - `schedule.cron`
  - `webhook`
  - `manual`
  - i18n triggers
- Register built-in schemas for current steps:
  - `condition`
  - `http`
  - `mail.send`
  - `record.create`
  - `record.update`
  - `record.delete`
  - `response`
- Expose a superuser API endpoint for schema discovery.
- Update the step editor to use discovered schema metadata where practical, without replacing the whole editor.

### Verification

- Unit tests for schema registration and lookup.
- API tests for schema discovery permissions.
- UI build.

### Exit Criteria

- The system can answer: “What inputs and outputs does this trigger or step produce?”
- No workflow behavior depends on hardcoded UI-only knowledge.

---

## Platform Phase 2 — Capability Registry MVP

### Goal

Move from implementation-centric step types toward reusable capabilities while keeping the current step system working.

### Deliverables

- Add `_capabilities` system collection.
- Add `Capability` model/query/validation helpers.
- Fields:
  - `key`
  - `version`
  - `category`
  - `icon`
  - `inputSchema`
  - `outputSchema`
  - `runtimeHandler`
  - `configUI`
  - `active`
- Add a runtime registry for built-in capabilities.
- Map existing step types to built-in capabilities internally:
  - `http` -> `http.request`
  - `mail.send` -> `mail.send`
  - `record.create` -> `record.create`
  - `record.update` -> `record.update`
  - `record.delete` -> `record.delete`
- Add a new `capability` step shape, but keep legacy step shapes valid.

### Verification

- Migration tests for `_capabilities`.
- Validation tests for duplicate keys, invalid schemas, and inactive capabilities.
- Runner tests proving capability steps can execute through the same run logging path.

### Exit Criteria

- Current steps can gradually become capability-backed.
- New integrations no longer require adding a new hardcoded step type first.

---

## Platform Phase 3 — Policy and Recursion Safety

### Goal

Add safety controls before durable async, connectors, and AI expand the blast radius.

### Deliverables

- Add `_policies` system collection or app-level automation policy settings if collection-level policies are too broad for MVP.
- Implement runtime limits:
  - max run depth
  - max runs per automation per minute
  - max concurrent runs per automation
  - dedupe keys
  - cooldown windows
- Add outbound HTTP restrictions:
  - blocked private/link-local IP ranges
  - optional domain allowlist
  - max redirects
  - max response size
- Add audit fields to runs:
  - parentRunId
  - depth
  - dedupeKey
  - policyDecision

### Verification

- Tests for recursive record-trigger loops.
- Tests for throttling and cooldown.
- Tests for blocked HTTP targets.

### Exit Criteria

- The runtime can reject risky executions with clear run-level audit output.
- AI and connector phases can build on shared policy checks.

---

## Platform Phase 4 — Persistent Workflow State MVP

### Goal

Create durable execution state without implementing every wait/resume feature at once.

### Deliverables

- Add `_workflowState` system collection.
- Add `_workflowLocks` system collection if DB-level locking helpers are not enough.
- Add state model/query helpers.
- Store:
  - automationRef
  - runRef
  - status
  - currentStepIndex
  - context
  - checkpoints
  - resumeToken
  - waitingFor
  - expires
- Refactor the runner into resumable units:
  - create run
  - execute next step
  - persist checkpoint
  - finalize run
- Keep synchronous execution as the default path.

### Verification

- Tests for checkpoint persistence after each step.
- Tests for recovery from a partially completed run.
- Tests that existing synchronous automations still finish as before.

### Exit Criteria

- A run can be paused after a step and later resumed from stored state.

---

## Platform Phase 5 — Async Wait/Resume Steps

### Goal

Use persistent workflow state to support long-running workflows.

### Deliverables

- Add wait step validation and execution:
  - `wait.delay`
  - `wait.webhook`
  - `wait.event`
- Add resume APIs:
  - internal resume by state id/token
  - webhook resume endpoint
- Add scheduler integration for expired delays.
- Add expiration handling:
  - timeout status
  - timeout branch later, if needed
- Add UI visibility for paused/waiting runs.

### Verification

- Tests for delay scheduling and resume.
- Tests for webhook correlation token.
- Tests for event wait registration without event bus replay yet.

### Exit Criteria

- A workflow can wait, persist, survive process restart, and resume.

---

## Platform Phase 6 — Human Approval System

### Goal

Add human-in-the-loop execution using the wait/resume engine.

### Deliverables

- Add `_approvals` system collection.
- Add `wait.approval` step.
- Approval fields:
  - workflowStateRef
  - automationRef
  - runRef
  - stepIndex
  - assignee
  - role
  - status
  - decision
  - comment
  - resolved
- Add superuser/admin approval APIs first.
- Add optional auth collection assignee support later.
- Add admin UI approval list and decision modal.

### Verification

- Tests for approval creation, approval resume, rejection, and duplicate decisions.
- Permission tests for approval APIs.
- UI build.

### Exit Criteria

- Workflows can pause for approval and continue or fail based on the decision.

---

## Platform Phase 7 — Connector Foundation

### Goal

Prepare external integrations without hardcoding each provider into the runner.

### Deliverables

- Add `_connectors` system collection.
- Store encrypted credentials using existing PocketBase secret/encryption conventions where available.
- Connector fields:
  - provider
  - authType
  - credentials
  - scopes
  - rateLimits
  - active
- Add connector runtime interface:
  - resolve credentials
  - refresh token hook
  - rate-limit check
  - execute capability
- Start with non-OAuth connectors:
  - generic API key connector
  - generic bearer token connector
- Link capabilities to connector requirements.

### Verification

- Tests that credentials are not exposed through normal API responses.
- Tests for inactive connector and missing scope failures.
- Tests for connector-backed capability execution.

### Exit Criteria

- Capabilities can require a connector, and the runtime can execute them through a connector abstraction.

---

## Platform Phase 8 — Internal Event Bus MVP

### Goal

Decouple triggers from direct hook dispatch and enable event-based orchestration.

### Deliverables

- Add internal event envelope:
  - id
  - name
  - source
  - subject
  - payload
  - occurred
  - correlationId
  - causationId
- Add publish/subscribe API inside `core`.
- Bridge existing triggers to events:
  - record hooks publish `record.created`, `record.updated`, `record.deleted`
  - i18n jobs publish translation events
  - webhook trigger publishes webhook event
- Add optional `_automationEvents` retention collection only if replay/debugging is included in this phase.
- Make automations subscribe to event names while preserving current `triggerType` compatibility.

### Verification

- Tests for event publication from existing hooks.
- Tests that current automations still fire exactly once.
- Tests for event correlation metadata.

### Exit Criteria

- New trigger types can be added by publishing events instead of wiring direct runner calls.

---

## Platform Phase 9 — AI Runtime MVP

### Goal

Add AI-native steps with structured input/output and shared safety controls.

### Deliverables

- Add AI provider abstraction.
- Add provider settings or connector-backed provider credentials.
- Add constrained AI steps:
  - `ai.extract`
  - `ai.classify`
  - `ai.generate`
  - `ai.summarize`
- Defer `ai.agent` until policies, connector permissions, and simulation are mature.
- Require output schemas for extract/classify/generate where possible.
- Add token quotas and audit fields to run step results.
- Add moderation/policy hook before external actions that consume AI output.

### Verification

- Unit tests with fake AI provider.
- Schema validation tests for AI output.
- Policy rejection tests.
- No network-dependent tests by default.

### Exit Criteria

- AI steps can run deterministically in tests through a fake provider.
- AI outputs are typed and auditable.

---

## Platform Phase 10 — Versioning and Publish Flow

### Goal

Make workflow edits safe for production use.

### Deliverables

- Add `_workflowVersions` system collection.
- Add draft/published workflow lifecycle.
- Store immutable version snapshot on each run.
- Add rollback support.
- Add change history metadata:
  - createdBy
  - publishedBy
  - publishedAt
  - notes
- Update APIs/UI so editing an active automation creates or updates a draft instead of mutating the published runtime immediately.

### Verification

- Tests that running workflows use the published version.
- Tests that old runs preserve old step definitions.
- Tests for rollback.

### Exit Criteria

- Operators can edit safely without changing the currently published workflow until they publish.

---

## Platform Phase 11 — Simulation and Replay

### Goal

Let users test workflows before publishing and debug failures without side effects.

### Implementation status

- Backend dry-run execution exists for saved automations.
- Existing run payloads can now be replayed into dry-run mode without creating new `_automationRuns` rows or executing side effects.
- Admin run history exposes both real rerun and dry-run replay preview actions.

### Deliverables

- Add dry-run execution mode.
- Add fake outputs for capabilities.
- Add side-effect blocking:
  - no real HTTP calls
  - no real email sends
  - no record writes unless explicitly using a temporary transaction/sandbox mode
- Add variable inspection for each step.
- Add replay from existing run payloads into dry-run mode.
- Add admin UI simulation panel.

### Verification

- Tests that dry-run mode blocks side effects.
- Tests for replaying a run payload into simulation.
- UI build.

### Exit Criteria

- A user can preview step inputs/outputs and errors before publishing.

---

## Platform Phase 12 — Visual Builder and Mapping UI

### Goal

Build the no-code workflow experience after runtime contracts are mature.

### Implementation status

- Admin UI includes a visual builder/list hybrid with the structured editor kept as fallback.
- The builder uses schema discovery, capability browsing, trigger/template token hints, inline validation surfacing, and the existing dry-run modal.
- This phase remains extensible for deeper graph edges and richer mapping widgets, but the MVP no-code workflow surface is in place.

### Deliverables

- Replace or augment the structured step editor with:
  - visual step graph/list hybrid
  - schema-aware data mapping
  - inline validation
  - capability browser
  - trigger payload autocomplete
  - dry-run preview integration
- Keep the existing structured editor available as a fallback until the visual builder is stable.

### Verification

- UI build.
- Browser smoke checks for editing, mapping, validation, and saving.
- API validation still rejects malformed workflows.

### Exit Criteria

- Non-technical users can assemble typed workflows without editing raw JSON.

---

## Platform Phase 13 — Templates and Marketplace

### Goal

Enable reusable workflows after versioning, capabilities, connectors, and simulation exist.

### Implementation status

- Added organization-local `_workflowTemplates` foundation.
- Automations can be exported into a versioned workflow-template package with required capability, connector, and collection metadata.
- Template packages can be imported and installed as inactive automations after dependency checks.
- Public marketplace discovery remains intentionally deferred.

### Deliverables

- Add `_workflowTemplates` system collection.
- Add import/export format.
- Add template install flow:
  - required capabilities
  - required connectors
  - required collections
  - configuration prompts
- Add organization-local templates first.
- Defer public marketplace/discovery until the package format and trust model are stable.

### Verification

- Tests for export/import roundtrip.
- Tests for missing dependency detection.
- UI build.

### Exit Criteria

- A workflow can be packaged, imported, configured, and installed without manual JSON editing.

---

# Recommended Build Order

Do not start with the visual builder. The correct order is:

1. Baseline hardening.
2. Typed schemas.
3. Capability registry.
4. Policy and recursion safety.
5. Persistent state.
6. Wait/resume.
7. Approvals.
8. Connectors.
9. Event bus.
10. AI runtime.
11. Versioning.
12. Simulation.
13. Visual builder.
14. Templates and marketplace.

This order keeps risk controlled: first define contracts, then add reusable capabilities, then add safety, then add long-running orchestration, then add AI and no-code UX.
