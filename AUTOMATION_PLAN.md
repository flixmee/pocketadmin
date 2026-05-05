# PocketBase Automation Plan

## Review Summary

The original draft points in a good product direction, but it is not executable yet for this repo.

- It is too generic and does not map to PocketAdmin's actual seams.
- `go automation.Run(...)` directly from record hooks is the wrong execution model here because hooks may run inside transactions.
- A drag-and-drop graph builder is too large for MVP, especially since the current UI has no workflow/canvas infrastructure or graph dependency.
- The draft is missing persistence rules, validation, permissions, run logging, retry/error policy, and a realistic rollout order.

This revised plan narrows MVP to a backend-first automation system with a form/list editor in the admin UI. A node graph editor can come later after the runtime is stable.

## Current Status

The original phased plan is now mostly implemented.

- Phase 1 is complete: system collections, internal models, and validation exist.
- Phase 2 is complete: active registry, cron sync, and after-commit trigger dispatch exist.
- Phase 3 is complete: `condition`, `http`, `record.create`, `record.update`, and `record.delete` execute for real.
- Phase 4 is complete: dedicated superuser-only automation APIs exist.
- Phase 5 is complete: run observability, step timing, and failed-step indexing exist.
- Phase 6 is complete: the Automations settings page and list UI exist.
- Phase 7 is complete: the structured step editor replaced the raw `steps` JSON editor.
- Phase 8 is complete: recent runs list and run preview UI exist.
- Follow-up completed after the original plan: `mail.send` now exists in the runner and structured editor, including attachments from trigger-record file fields.

What remains is follow-up product work, not the original MVP foundation.

## MVP Scope

### In scope

- Superuser-managed automations.
- Triggers:
  - `record.create`
  - `record.update`
  - `record.delete`
  - `schedule.cron`
  - manual run from admin UI
- Linear execution only.
- Step types:
  - condition
  - outbound HTTP request
  - send mail
  - record create/update/delete
- Run logs with status, timestamps, and last error.
- Admin UI for list/create/edit/enable-disable/run-now/view recent runs.

### Out of scope

- Branching graphs.
- Loops or parallel execution.
- Arbitrary JS/user code execution.
- Retries, dead-letter queues, distributed workers.
- End-user self-service permissions.
- Visual drag-and-drop builder in the first release.

## Product Decisions

### 1. Storage

Use system collections created by migration:

- `_automations`
- `_automationRuns`

Why:

- durable and queryable
- protected from normal deletion/renaming
- fits existing PocketBase collection/model patterns
- easier to expose through dedicated APIs and admin UI

Recommended `_automations` fields:

- `name` text, required
- `active` bool, required
- `triggerType` text, required
- `collectionRef` text, optional for record triggers
- `cronExpr` text, optional for schedule triggers
- `steps` json, required
- `notes` editor/text, optional
- `lastRunAt` date, optional
- `lastRunStatus` text, optional
- `created` autodate
- `updated` autodate

Recommended `_automationRuns` fields:

- `automationRef` relation/text, required
- `triggerType` text, required
- `status` text, required
- `input` json, optional
- `stepResults` json, optional
- `error` text, optional
- `started` date, required
- `finished` date, optional

### 2. Execution timing

Do not launch runs directly from inside record hooks.

For record triggers:

- bind to `OnRecordAfterCreateSuccess`, `OnRecordAfterUpdateSuccess`, `OnRecordAfterDeleteSuccess`
- use `app.TxInfo().OnComplete(...)` to enqueue only after successful commit
- if the transaction fails or rolls back, do nothing

This is the main correction to the original draft.

### 3. Runtime model

Use an in-process async runner first.

- Hook/cron/manual API builds a trigger payload.
- Payload is handed to a lightweight runner.
- Runner executes steps sequentially.
- Every run writes a `_automationRuns` record.

No external queue is needed for MVP. If reliability requirements grow later, the runner can be moved behind a durable job table.

### 4. UI model

Do not start with a drag-and-drop builder.

Start with:

- automation list page
- upsert modal/page
- ordered step editor
- simple condition/action forms

This matches the current admin UI architecture and keeps the feature shippable.

### 5. Security model

- Automations are superuser-only to create/edit/run.
- Record actions execute with app/system privileges, not request-auth privileges.
- Outbound HTTP must not use raw `http.DefaultClient` for untrusted URLs.
- Reuse or extract the guarded outbound HTTP client pattern currently used by OAuth2 file fetch code.

## Backend Plan

### Phase 1. Migrations and internal models

Status: done

Files likely involved:

- `migrations/<timestamp>_automations.go`
- `core/automation_model.go`
- `core/automation_run_model.go`
- `core/automation_validate.go`

Tasks:

- Add system collections for automation definitions and run logs.
- Add typed record proxies similar to other internal collections.
- Add validation helpers for:
  - allowed trigger types
  - required `collectionRef` for record triggers
  - required `cronExpr` for scheduled triggers
  - valid cron syntax
  - valid `steps` shape
- Add indexes for active lookups and run history.

### Phase 2. Trigger registry and runner

Status: done

Files likely involved:

- `core/automation_runner.go`
- `core/automation_registry.go`
- `core/automation_context.go`
- `core/base.go`

Tasks:

- Register automation hooks during app setup.
- Load active automations into an in-memory registry/cache.
- Refresh that registry when `_automations` records change.
- Register/unregister cron jobs for active scheduled automations.
- Build a runner that:
  - creates a run log
  - executes steps in order
  - stops on first terminal error
  - persists final status and error details

Context available to steps:

- `trigger.type`
- `record`
- `recordOriginal` for update/delete when available
- `automation`
- `run`

### Phase 3. Step execution

Status: done

Files likely involved:

- `core/automation_steps.go`
- `core/automation_http.go`
- `core/automation_mail.go`
- `core/automation_records.go`
- `core/automation_templates.go`

Step types for MVP:

- `condition`
  - simple comparisons only: `eq`, `neq`, `in`, `exists`
  - field path lookup from trigger payload
- `http`
  - method, URL, headers, body template, timeout
  - safe outbound client only
- `mail.send`
  - recipients, subject, text/html body
  - uses app mail settings sender metadata
  - optional attachments from file fields on the trigger record
- `record.create`
  - target collection
  - field map template
- `record.update`
  - target collection
  - filter or explicit id template
  - patch field map
- `record.delete`
  - target collection
  - filter or explicit id template

Template strategy for MVP:

- support dot-path placeholders such as `{{record.id}}` and `{{record.status}}`
- no custom scripting
- fail validation if required placeholders reference unsupported roots

### Phase 4. APIs

Status: done

Files likely involved:

- `apis/automation.go`
- `apis/base.go`

Endpoints:

- `GET /api/automations`
- `POST /api/automations`
- `GET /api/automations/{id}`
- `PATCH /api/automations/{id}`
- `DELETE /api/automations/{id}`
- `POST /api/automations/{id}/run`
- `GET /api/automations/{id}/runs`

Notes:

- Keep these superuser-only.
- Validate and normalize the `steps` payload server-side.
- Manual runs should go through the same runner as hooks/cron.

### Phase 5. Observability and failure behavior

Status: done

Files likely involved:

- `core/automation_runner.go`
- `ui/src/settings/automations/*`

Tasks:

- Store per-run status: `queued` or `running`, `success`, `failed`.
- Persist last error string and optional step index.
- Show recent runs in the UI.
- Log unexpected runner failures via `app.Logger()`.
- Persist per-step `started`, `finished`, and `durationMs`.

MVP failure policy:

- stop workflow on first failed step
- no automatic retry
- manual rerun from UI

## Admin UI Plan

### Phase 6. Settings page and list

Status: done

Files likely involved:

- `ui/src/router.js`
- `ui/src/store.js`
- `ui/src/settings/settingsSidebar.js`
- `ui/src/settings/automations/pageAutomationsSettings.js`
- `ui/src/settings/automations/automationsList.js`

Tasks:

- Add an "Automations" settings page.
- Show list with trigger type, active state, last run status, last run time.
- Add buttons for create, edit, enable/disable, run now, delete.

### Phase 7. Automation editor

Status: done

Files likely involved:

- `ui/src/settings/automations/automationUpsertModal.js`
- `ui/src/settings/automations/stepEditor.js`
- `ui/src/settings/automations/conditionStepForm.js`
- `ui/src/settings/automations/httpStepForm.js`
- `ui/src/settings/automations/mailStepForm.js`
- `ui/src/settings/automations/recordStepForm.js`
- optional CSS additions under `ui/src/css/*`

Tasks:

- Build a form-based editor.
- Let users choose trigger type first.
- Show trigger-specific fields:
  - collection picker for record triggers
  - cron expression for scheduled triggers
- Let users add/reorder/remove steps.
- Use the shared select component, not native `<select>`.
- Keep transient row/editor state local rather than mutating reactive source objects on each keystroke.
- Support `mail.send` with:
  - `to`, `cc`, `bcc`
  - subject
  - text and/or HTML body
  - attachment checkboxes sourced from file fields on the selected trigger collection

### Phase 8. Runs view

Status: done

Files likely involved:

- `ui/src/settings/automations/automationRunsList.js`
- `ui/src/settings/automations/automationRunPreviewModal.js`

Tasks:

- Show recent runs for one automation.
- Show trigger payload summary, step result summary, and error.

## Testing Plan

### Go tests

- migration test for system collections
- validation tests for trigger and step payloads
- hook tests proving runs are dispatched only after successful commit
- rollback test proving no run occurs on failed transaction
- cron registration tests
- step execution tests for condition, HTTP, mail, and record actions
- API tests for CRUD and manual run

Likely files:

- `core/automation*_test.go`
- `apis/automation_test.go`

### UI verification

- `cd ui && npm run build`
- manual smoke tests for create/edit/enable/run-now/runs list

## Delivered Order

1. Locked MVP scope and data model.
2. Added system collections and backend validation.
3. Added runner, trigger dispatch, and cron registration.
4. Added HTTP and record action executors.
5. Added CRUD/manual-run APIs.
6. Added admin list/editor UI.
7. Added runs UI and observability polish.
8. Added `mail.send` as a follow-up step type after the base runtime and editor were stable.

## Recommended Next Work

1. Decide whether automation recursion protection belongs in v1.1 or remains an operational constraint.
2. Decide whether mail needs sender overrides, custom headers, or inline attachments.
3. Add richer filtering/polling in the runs UI only if operators need it.
4. Consider drag-and-drop only after the form builder and runtime continue to hold up in real use.

## Risks

- Running automations inside transactions would cause incorrect side effects; this must be avoided.
- Outbound HTTP introduces SSRF risk if the client is not guarded.
- Dynamic record updates can recurse into other automations; MVP should document that recursion protection is out of scope unless explicitly added.
- A graph UI first would consume most of the effort before the backend semantics are proven.

## Open Decisions

- Whether `_automationRuns` is required in v1 or can start as a lighter last-run summary plus logs.
- Whether to add recursion/depth protection in v1 or document it as an operational constraint.
- Whether `mail.send` should stay limited to trigger-record file field attachments or support broader attachment sourcing.

## Post-MVP

- drag-and-drop builder that compiles to the same `steps` payload
- retries and retry policy
- recursion/depth guards
- branching
- reusable templates/snippets
- secret references for headers/body values
- sender overrides, custom headers, and inline mail attachments
- webhook trigger
- user-facing automation permissions
