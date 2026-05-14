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
- Follow-up completed for localization: i18n automation triggers and translation-job completion triggers exist.

What remains is follow-up product work, not the original MVP foundation.

## Automation Documentation

This section describes the automation system as it exists in the app today. It is intended for implementers and operators who need to configure, debug, or extend automation behavior.

### Core concepts

Automations are stored in the protected `_automations` system collection and each execution is stored in `_automationRuns`.

An automation has:

- `name`: human-readable label.
- `active`: whether automatic triggers should run it.
- `triggerType`: event that starts the automation.
- `collectionRef`: optional or required collection scope, depending on trigger type.
- `cronExpr`: cron expression for scheduled triggers.
- `steps`: JSON array of ordered step definitions.
- `notes`: optional operator notes.
- `lastRunAt` and `lastRunStatus`: latest run summary.

A run has:

- `automationRef`: automation id.
- `triggerType`: trigger that started this run.
- `status`: `queued`, `running`, `success`, or `failed`.
- `input`: trigger payload.
- `stepResults`: per-step output, error, timing, and status.
- `error`: terminal run error, if any.
- `errorStepIndex`: zero-based step index that failed.
- `started` and `finished`: execution timestamps.

### Trigger types

Supported triggers:

- `record.create`: runs after a record is created.
- `record.update`: runs after a record is updated.
- `record.delete`: runs after a record is deleted.
- `schedule.cron`: runs on a cron schedule.
- `webhook`: runs when the public webhook endpoint is called.
- `manual`: runs from the admin API/UI.
- `i18n.translation_missing`: runs when an i18n source record is missing enabled locale translations.
- `i18n.locale_published`: runs when a locale becomes enabled.
- `i18n.translation_updated`: runs after an i18n record is updated.
- `i18n.ai_translation_finished`: runs when a translation job transitions to `finished`.

`collectionRef` is required for record triggers and for all i18n triggers except `i18n.locale_published`. `i18n.locale_published` may be global or scoped to a collection.

### Execution model

Record and i18n triggers run from successful record hooks, after the database write succeeds. The runner executes asynchronously in-process and writes a `_automationRuns` record for every run.

Execution is linear:

- steps run in array order
- the first failed step fails the run
- a non-matching `condition` step stops the run without treating it as an error
- no automatic retry is performed
- operators can rerun from an existing run payload

System collections are skipped as record-trigger sources to avoid self-triggering automation loops. Skipped collections include `_automations`, `_automationRuns`, `_locales`, `_i18nGroups`, and `_translationJobs`.

### API surface

Superuser-only endpoints:

- `GET /api/automations`
- `POST /api/automations`
- `GET /api/automations/{id}`
- `PATCH /api/automations/{id}`
- `DELETE /api/automations/{id}`
- `POST /api/automations/{id}/run`
- `GET /api/automations/{id}/runs?limit=20&offset=0`
- `POST /api/automations/{id}/runs/{runId}/rerun`
- `DELETE /api/automations/{id}/runs`

Public webhook endpoint:

- `POST /api/automation-webhooks/{id}`

The webhook endpoint only runs active automations whose `triggerType` is `webhook`. The webhook request body is parsed as JSON, form data, multipart values, or raw text based on `Content-Type`.

### Template data

Steps can use template placeholders such as `{{record.id}}` or `{{trigger.type}}`.

Available roots:

- `trigger`: trigger metadata.
- `request`: webhook request data.
- `i18n`: i18n trigger metadata.
- `record`: current trigger record data.
- `recordOriginal`: previous record data for update/delete triggers.
- `automation`: automation record data.
- `run`: automation run record data.
- `steps`: previous step results.
- `prevStep`: most recent step result.

`trigger` includes:

- `type`
- `collectionId`
- `collectionName`
- `request`
- `i18n`

Record trigger payloads include `record`, and update/delete payloads also include `recordOriginal`.

i18n trigger payloads may include:

- `collectionId`
- `collectionName`
- `groupId`
- `locale`
- `sourceLocale`
- `sourceRecordId`
- `targetLocale`
- `targetRecordId`
- `missingLocales`
- `translationTotal`
- `translationJobId`
- `provider`
- `model`

### i18n automation triggers

These triggers are intended for localization operations around i18n-enabled collections. Use them to notify translators, create translation work items, call an external translation service, or keep downstream systems synchronized when localized content changes.

#### Translation missing

Trigger type: `i18n.translation_missing`

Purpose:

- Detect that a source record does not have translations for all enabled locales.
- Start human or machine translation workflows from the source record.
- Notify editors that a record is not fully localized yet.

How to use:

- Create an automation with trigger type `i18n.translation_missing`.
- Set `collectionRef` to the i18n-enabled collection you want to monitor.
- Add steps such as `mail.send`, `http`, or `record.create` to notify a team, enqueue a translation job, or create an internal task.
- Use template values such as `{{record.id}}`, `{{record.title}}`, `{{i18n.sourceLocale}}`, and `{{i18n.missingLocales}}`.

When to use:

- Use it after content creation when every enabled locale must eventually have a translation.
- Use it to feed AI translation queues or external translation-management systems.
- Use it when missing translations are actionable and should create follow-up work.
- Avoid it for collections where partial localization is expected and missing translations should not create noise.

Typical payload fields:

- `i18n.collectionId`
- `i18n.collectionName`
- `i18n.groupId`
- `i18n.sourceLocale`
- `i18n.sourceRecordId`
- `i18n.missingLocales`
- `i18n.translationTotal`
- `record`

#### Locale published

Trigger type: `i18n.locale_published`

Purpose:

- React when a locale becomes enabled.
- Backfill or queue translations for the newly published locale.
- Notify teams that a new language is now active.

How to use:

- Create an automation with trigger type `i18n.locale_published`.
- Leave `collectionRef` empty for a global automation, or set it to a specific i18n-enabled collection.
- Use `{{i18n.locale}}` to reference the newly enabled locale code.
- Use `http` or `record.create` steps to create translation work for the new locale.

When to use:

- Use it when enabling a new locale should trigger translation preparation across localized collections.
- Use it to notify product, editorial, or support teams that a language is live.
- Use it to kick off bulk translation jobs for content that existed before the locale was enabled.
- Avoid it for disabled draft locales; the trigger only runs when the locale is enabled for the first time or transitions from disabled to enabled.

Typical payload fields:

- `i18n.collectionId`
- `i18n.collectionName`
- `i18n.locale`
- `i18n.localeRecordId`

#### Translation updated

Trigger type: `i18n.translation_updated`

Purpose:

- React when an i18n record changes.
- Synchronize translated content to caches, search indexes, publishing pipelines, or external systems.
- Notify reviewers that a localized record has been edited.

How to use:

- Create an automation with trigger type `i18n.translation_updated`.
- Set `collectionRef` to the i18n-enabled collection.
- Add a `condition` step when only specific locales, statuses, or fields should trigger follow-up actions.
- Use `{{i18n.locale}}`, `{{i18n.recordId}}`, `{{i18n.groupId}}`, and normal `{{record.*}}` placeholders in steps.

When to use:

- Use it for cache invalidation or downstream publishing after localized content changes.
- Use it to notify reviewers when translators update copy.
- Use it to synchronize locale-specific records into search or analytics indexes.
- Avoid it for expensive external calls unless you add conditions, because any update to a localized record can trigger it.

Typical payload fields:

- `i18n.collectionId`
- `i18n.collectionName`
- `i18n.groupId`
- `i18n.locale`
- `i18n.recordId`
- `i18n.isSource`
- `record`

#### AI translation finished

Trigger type: `i18n.ai_translation_finished`

Purpose:

- React when a translation job is marked `finished`.
- Continue the workflow after machine translation output is available.
- Notify reviewers, publish translated drafts, or synchronize generated translations to external services.

How to use:

- Create an automation with trigger type `i18n.ai_translation_finished`.
- Set `collectionRef` to the collection referenced by the translation job, or leave it empty for a global completion handler.
- Update a translation job in `_translationJobs` from any non-`finished` status to `finished`.
- Use `{{i18n.translationJobId}}`, `{{i18n.sourceRecordId}}`, `{{i18n.targetRecordId}}`, `{{i18n.sourceLocale}}`, `{{i18n.targetLocale}}`, `{{i18n.provider}}`, and `{{i18n.model}}` in steps.

When to use:

- Use it when AI output needs human review before publication.
- Use it to send completion notifications to editors or translators.
- Use it to update the target record status after a translation job completes.
- Use it to synchronize completed translations to a search index or external publishing system.
- Avoid using it as the place to call the AI provider itself; this trigger is for post-completion actions after a job is already finished.

Typical payload fields:

- `i18n.collectionId`
- `i18n.sourceRecordId`
- `i18n.targetRecordId`
- `i18n.sourceLocale`
- `i18n.targetLocale`
- `i18n.translationJobId`
- `i18n.provider`
- `i18n.model`

Webhook trigger request payloads include:

- `method`
- `path`
- `query`
- `headers`
- `body`
- `remoteIP`

### Step schemas

#### `condition`

Stops execution when the condition does not match.

```json
{
  "type": "condition",
  "path": "record.status",
  "op": "eq",
  "value": "published"
}
```

Supported operators:

- `eq`
- `neq`
- `in`
- `exists`
- `startsWith`
- `endsWith`
- `notStartsWith`
- `notEndsWith`
- `contains`

#### `http`

Sends an outbound HTTP request.

```json
{
  "type": "http",
  "method": "POST",
  "url": "https://example.com/hooks/posts",
  "headers": {
    "Content-Type": "application/json"
  },
  "body": {
    "id": "{{record.id}}",
    "title": "{{record.title}}"
  },
  "timeout": 10
}
```

`url` is required. `headers` must be an object when provided. `timeout` must be greater than zero when provided.

#### `mail.send`

Sends an email through the app mailer.

```json
{
  "type": "mail.send",
  "to": ["editor@example.com"],
  "cc": [],
  "bcc": [],
  "subject": "Post {{record.title}} was published",
  "text": "Record id: {{record.id}}",
  "html": "<p>Record id: {{record.id}}</p>",
  "attachments": ["cover"]
}
```

Rules:

- `to` must contain at least one recipient.
- `subject` is required.
- either `text` or `html` is required.
- `attachments` can only reference file fields on the trigger collection.
- attachments are only supported for `record.create` and `record.update` triggers.

#### `record.create`

Creates a record in a target collection.

```json
{
  "type": "record.create",
  "collection": "notifications",
  "data": {
    "title": "New post: {{record.title}}",
    "post": "{{record.id}}"
  }
}
```

`collection` and `data` are required.

#### `record.update`

Updates one or more records by explicit id or filter.

```json
{
  "type": "record.update",
  "collection": "posts",
  "id": "{{record.id}}",
  "data": {
    "synced": true
  }
}
```

```json
{
  "type": "record.update",
  "collection": "notifications",
  "filter": "post = '{{record.id}}'",
  "data": {
    "read": false
  }
}
```

`collection`, `data`, and either `id` or `filter` are required.

#### `record.delete`

Deletes one or more records by explicit id or filter.

```json
{
  "type": "record.delete",
  "collection": "notifications",
  "filter": "post = '{{record.id}}'"
}
```

`collection` and either `id` or `filter` are required.

#### `response`

Sets the response for a `webhook` automation.

```json
{
  "type": "response",
  "statusCode": 200,
  "headers": {
    "Content-Type": "application/json"
  },
  "body": {
    "ok": true,
    "recordId": "{{record.id}}"
  }
}
```

Rules:

- only valid for `webhook` triggers
- `statusCode` must be a valid HTTP status code when provided
- `headers` must be an object when provided

### Example automations

#### Send mail when a post is published

```json
{
  "name": "Notify editors when post is published",
  "active": true,
  "triggerType": "record.update",
  "collectionRef": "posts",
  "steps": [
    {
      "type": "condition",
      "path": "record.status",
      "op": "eq",
      "value": "published"
    },
    {
      "type": "mail.send",
      "to": ["editors@example.com"],
      "subject": "Published: {{record.title}}",
      "text": "Post {{record.id}} was published."
    }
  ]
}
```

#### Queue work when translations are missing

```json
{
  "name": "Notify missing translations",
  "active": true,
  "triggerType": "i18n.translation_missing",
  "collectionRef": "posts",
  "steps": [
    {
      "type": "http",
      "method": "POST",
      "url": "https://example.com/i18n/jobs",
      "body": {
        "collection": "{{trigger.collectionName}}",
        "sourceRecordId": "{{i18n.sourceRecordId}}",
        "sourceLocale": "{{i18n.sourceLocale}}",
        "missingLocales": "{{i18n.missingLocales}}"
      }
    }
  ]
}
```

#### Webhook with custom response

```json
{
  "name": "Inbound content webhook",
  "active": true,
  "triggerType": "webhook",
  "steps": [
    {
      "type": "record.create",
      "collection": "inbox",
      "data": {
        "source": "{{request.remoteIP}}",
        "payload": "{{request.body}}"
      }
    },
    {
      "type": "response",
      "statusCode": 202,
      "body": {
        "accepted": true
      }
    }
  ]
}
```

### Operational notes

- Automations execute with app/system privileges, not the request user's record rules.
- Keep HTTP targets trusted; outbound HTTP remains the main SSRF-sensitive surface.
- Prefer id-based record updates/deletes when possible. Filters are powerful but easier to over-broaden.
- Keep steps idempotent when they call external services because manual reruns reuse the original run payload.
- Use `_automationRuns` for debugging payloads, rendered outputs, timing, and failed step indexes.

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
