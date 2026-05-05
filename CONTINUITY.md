Goal (incl. success criteria):

- Extend automations with a `mail.send` step.
- Success: superusers can configure a send-mail step in the automation editor, the runner can send mail through the existing app mailer, and record-triggered automations can attach files from the source record.

Constraints/Assumptions:

- Use the existing PocketBase app/hooks/cron architecture rather than introducing a separate workflow runtime.
- Prefer a backend-first MVP with a form/list editor; no drag-and-drop graph editor in the first delivery phase.
- Keep the plan aligned with current admin UI conventions under `ui/src/settings/*`.
- Keep outbound HTTP execution guarded; do not use an unrestricted default HTTP client for automation requests.

Key decisions:

- The original draft is too generic for this repo and assumes unsafe trigger execution from inside DB hooks.
- Automation triggers should be dispatched only after successful transaction completion, using transaction completion callbacks rather than raw goroutines from inside hook bodies.
- Workflow definitions should be persisted in dedicated internal storage and edited through a superuser settings page.
- Phase 3 should keep the existing in-process runner and extend it with linear step executors rather than introducing a new workflow runtime abstraction.
- Phase 3 HTTP execution uses a guarded outbound client by default, with an app-store override seam for deterministic tests.
- Phase 4 should use a dedicated superuser-only `/api/automations` subgroup rather than overloading the generic records API for system collections.
- Manual runs should be allowed from the API even for inactive automations, so the API uses a direct core manual-run seam rather than the active-only registry lookup.
- Phase 5 should improve observability without introducing a separate job system or changing the linear execution model.
- Phase 5 stores per-step timing in `stepResults`, records a failed-step index on terminal failures, logs runner panics through `app.Logger()`, and keeps the MVP failure policy as stop-on-first-error with manual rerun only.
- Phase 6 should live under `ui/src/settings/*`, reuse the existing settings sidebar/page shell, and avoid jumping straight to a full workflow builder.
- Phase 6 create/edit can use a minimal JSON-based modal for `steps`; the richer form/step builder remains Phase 7.
- The Phase 6 list includes create/edit, enable/disable, run-now, and delete actions; recent runs remain a later dedicated UI slice.
- Phase 7 should replace the raw steps JSON editor with structured forms for `condition`, `http`, and `record.*` steps while keeping step-row UI state local to the modal/editor layer.
- Phase 7 uses editor-local step objects plus save-time payload conversion so structural edits replace the step array only on add/remove/reorder, while field edits mutate the local step rows in place.
- Phase 8 should reuse the existing `/api/automations/{id}/runs` endpoint, add navigation from both the automations list and editor, and use read-only UI primitives for payload/result inspection.
- Phase 8 uses modal-based recent-runs and run-preview views instead of a new route, keeping runs inspection within the existing settings workflow.
- The mail step should reuse `App.NewMailClient()` and the existing mail settings sender metadata instead of introducing separate SMTP configuration under automations.
- Record-backed mail attachments should resolve from file fields on the trigger record and stay within the existing record/filesystem model.
- Mail-step attachments are intentionally stored as trigger-record file field names, not arbitrary file templates, so validation can confirm the selected fields exist and are file fields.

State:
  - Done:
    - Read the previous ledger, `AUTOMATION_PLAN.md`, and relevant backend/UI files.
    - Confirmed available seams: record hooks, app cron scheduler, settings pages, collection persistence, transaction completion callbacks.
    - Confirmed there is no existing workflow-builder UI infrastructure or graph library in `ui/package.json`.
    - Rewrote `AUTOMATION_PLAN.md` into a phased implementation plan with explicit MVP scope, storage model, backend phases, UI phases, testing, and risks.
    - Added system migration `1775000000_automations.go` for `_automations` and `_automationRuns`.
    - Added `core/automation_model.go`, `core/automation_run_model.go`, and `core/automation_validate.go`.
    - Wired automation validation hooks into `core/base.go`.
    - Added focused tests in `core/automation_model_test.go`.
    - `go test ./core -run 'Automation|BaseApp'` passed.
    - Added `core/automation_query.go`, `core/automation_registry.go`, `core/automation_runner.go`, and `core/automation_runner_test.go`.
    - Bootstrapped and cached active automation registry in app store; synced scheduled automations into app cron jobs.
    - Bound record create/update/delete success hooks to enqueue matching automation runs asynchronously.
    - Implemented run logging into `_automationRuns` and automation `lastRunAt` / `lastRunStatus` updates.
    - Added Phase 3 backend execution files: `core/automation_steps.go`, `core/automation_templates.go`, `core/automation_http.go`, and `core/automation_records.go`.
    - Replaced placeholder step execution with real handlers for `condition`, `http`, `record.create`, `record.update`, and `record.delete`.
    - Added recursive template rendering for supported roots: `trigger`, `record`, `recordOriginal`, `automation`, and `run`.
    - Expanded automation validation with per-step schema checks and unsupported-template-root detection.
    - Extended targeted tests to cover stopped conditions, HTTP execution, templated record creation, record update/delete actions, and the stricter validation cases.
    - `go test ./core -run 'Automation|BaseApp'` passed after Phase 3 backend changes.
    - Added `apis/automation.go` and registered the superuser-only `/api/automations` routes in `apis/base.go`.
    - Added Phase 4 endpoints for list/create/view/update/delete/manual-run/run-history.
    - Added `RunAutomationManually` to the `core.App` interface and implemented it on `BaseApp` so manual API runs can execute inactive automations too.
    - Added `apis/automation_test.go` covering authorization, CRUD, manual run dispatch, and run history.
    - `go test ./apis -run 'Automation'` passed.
    - `go test ./core -run 'Automation|BaseApp'` passed after the Phase 4 API changes.
    - Added `_automationRuns.errorStepIndex` via `1775000001_automation_run_observability.go` and included it in the base automation migration for fresh installs.
    - Extended `AutomationRun` with failed-step helpers and settled on an internal unset sentinel for `errorStepIndex` because numeric fields cannot persist `nil`.
    - Extended the runner to capture per-step `started` / `finished` / `durationMs`, persist failed-step index on terminal failures, and recover/log panics with stack traces through `app.Logger()`.
    - Added recent-run query support in `core/automation_query.go` and `limit` / `offset` support for `GET /api/automations/{id}/runs`.
    - Expanded core/API automation tests for failed-step observability and recent-run API querying.
    - `go test ./core -run 'Automation|BaseApp'` passed after the Phase 5 observability changes.
    - `go test ./apis -run 'Automation'` passed after the Phase 5 observability changes.
    - Added the `#/settings/automations` route and settings sidebar navigation entry.
    - Added `ui/src/settings/automations/pageAutomationsSettings.js` and `automationsList.js` for the new automations settings page and management list.
    - Added `ui/src/settings/automations/automationUpsertModal.js` with a minimal trigger-aware create/edit modal and raw `steps` JSON editor.
    - `cd ui && npm run build` passed; `dprint` reported a sandbox cache write warning outside the workspace but formatting and the Vite build still completed successfully.
    - Replaced the raw `steps` JSON editor in `automationUpsertModal.js` with a structured step editor.
    - Added `ui/src/settings/automations/stepEditor.js`, `conditionStepForm.js`, `httpStepForm.js`, and `recordStepForm.js`.
    - Added save-time conversion from editor step objects back into API payloads with local validation for JSON objects, record targeting, and HTTP timeout/value parsing.
    - Updated the Automations settings page copy to reflect the structured editor.
    - `cd ui && npm run build` passed after the Phase 7 editor changes; `dprint` still reported the same sandbox cache write warning outside the workspace, but formatting and the Vite build completed successfully.
    - Fixed a Phase 7 regression where `addStep` in `stepEditor` appeared broken because `automationUpsertModal` passed `steps` as a static array instead of a reactive getter; the modal now passes `steps: () => data.form.steps`.
    - `cd ui && npm run build` passed after the `addStep` reactive-prop fix.
    - Updated `recordStepForm.js` so record create/update steps prefill `Data JSON` from the selected collection fields when the data block is still empty or auto-generated.
    - The record-step default JSON builder skips hidden, primary-key, and `autodate` fields and uses the existing field-type `dummyData` conventions for example values.
    - `cd ui && npm run build` passed after the collection-based record-step default data change.
    - Added `ui/src/settings/automations/automationRunsList.js` for recent runs loading, refresh, pagination, and run-summary rows backed by `/api/automations/{id}/runs`.
    - Added/imported `ui/src/settings/automations/automationRunPreviewModal.js` as an explicit module export and used it for read-only run inspection with payload JSON, step-result summaries, and raw step-result JSON.
    - Wired recent-runs entry points from both `automationsList.js` and `automationUpsertModal.js`.
    - `cd ui && npm run build` passed after the Phase 8 runs/history UI changes; `dprint` still reported the same sandbox cache write warning outside the workspace, but formatting and the Vite build completed successfully.
    - Added the backend `mail.send` automation step in `core/automation_mail.go`, wired it into step execution, and added validation for recipients, subject/body, and record-file attachments.
    - Extended automation step types with `mail.send` and added targeted core tests for validation plus successful mail delivery with trigger-record attachments.
    - Added `ui/src/settings/automations/mailStepForm.js` and wired it into `stepEditor.js` and `automationUpsertModal.js`.
    - The structured editor now supports send-mail configuration with default-sender display, recipient fields, text/HTML body fields, and attachment checkboxes sourced from file fields on the selected trigger collection.
    - `go test ./core -run 'Automation|BaseApp'` passed after the mail-step changes when rerun with access to the system Go build cache.
    - `cd ui && npm run build` passed after the mail-step UI changes; `dprint` still reported the same sandbox cache write warning outside the workspace, but the Vite build completed successfully.
    - Fixed a UI regression in `ui/src/settings/automations/stepEditor.js` where file-scoped `renderStepForm()` referenced `props` from `stepEditor()`, causing the new mail-step branch to crash because `props` was undefined.
    - `renderStepForm()` now receives explicit trigger context from `stepEditor()` instead of closing over out-of-scope state.
    - `cd ui && npm run build` passed after the `renderStepForm` scope fix; `dprint` still reported the same sandbox cache write warning outside the workspace, but formatting and the Vite build completed successfully.
    - Updated `AUTOMATION_PLAN.md` to reflect shipped automation work instead of only the original proposal state.
    - The plan doc now includes a current-status snapshot, marks Phases 1 through 8 as done, adds `mail.send` to scope/phase details/testing, and moves remaining work into follow-up decisions.
  - Now:
    - The automation plan document is aligned with the implementation state in the repo.
  - Next:
    - Decide whether follow-up mail features are needed, such as custom headers, sender overrides, inline attachments, or richer attachment sourcing rules.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: whether automation definitions should live in one system collection or split into definition/run-log collections from day one.
- UNCONFIRMED: whether the first action set should include email in MVP or defer it until HTTP and record actions are stable.
- UNCONFIRMED: whether v1 should block automation-triggered writes from recursively triggering other automations, or leave that as an operator responsibility for MVP.

Working set (files/ids/commands):

- `/Volumes/MacOS_WD/Developer/pocketadmin/AUTOMATION_PLAN.md`
- `/Volumes/MacOS_WD/Developer/pocketadmin/CONTINUITY.md`
- `/Volumes/MacOS_WD/Developer/pocketadmin/migrations/1775000000_automations.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_model.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_run_model.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_validate.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_model_test.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_query.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_registry.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_runner.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_runner_test.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_steps.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_http.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_records.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_templates.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/app.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/base.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/events.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/cron.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/collection.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/automation.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/automation_test.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/base.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/migrations/1775000001_automation_run_observability.go`
- `go test ./core -run 'Automation|BaseApp'`
- `go test ./apis -run 'Automation'`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/record_auth_with_oauth2.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/router.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/store.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/crons/pageCronsSettings.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/settingsSidebar.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/automations/pageAutomationsSettings.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/automations/automationsList.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/automations/automationUpsertModal.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/automations/stepEditor.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/automations/conditionStepForm.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/automations/httpStepForm.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/automations/recordStepForm.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/automations/automationRunsList.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/automations/automationRunPreviewModal.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_mail.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/automations/mailStepForm.js`
- `go test ./core -run 'Automation|BaseApp'`
- `cd ui && npm run build`
