Goal (incl. success criteria):

- Fix current `monaco-editor` incompatibility with newer Vite/Rolldown worker resolution.
- Success: UI dependency/config/source changes allow the admin UI build to resolve Monaco workers under the current Vite/Rolldown toolchain.

Constraints/Assumptions:

- Follow `AGENTS.md` and `UI_DOCS.md`; keep UI changes focused.
- Existing unrelated backend/UI/dist changes may be present; do not revert them.
- Keep Go and frontend changes isolated unless required.
- Network access is restricted; dependency/audit commands may require approval if package registry access is needed.

Key decisions:

- Prefer the smallest dependency/config/source change that matches existing UI patterns and verifies with the UI build.
- Pin `monaco-editor` exactly to `0.53.0`: it resolves the Vite 8/Rolldown worker import failure, while `0.55.1` currently introduces moderate `dompurify` audit findings.

State:

- Done:
  - Read `CONTINUITY.md`.
  - Reproduced `npm run build` failure: Rolldown could not resolve `monaco-editor/esm/vs/editor/editor.worker?worker` from `ui/src/base/monacoEditor.js`.
  - Installed and tested `monaco-editor@0.55.1`; build passed, but `npm audit --omit=dev` reported 2 moderate vulnerabilities via `dompurify`.
  - Switched to exact `monaco-editor@0.53.0`; build passed and `npm audit --omit=dev` reported 0 vulnerabilities.
  - Rebuilt `ui/dist`, producing Monaco worker/editor chunks and updating `ui/dist/index.html`.
- Add automation triggers for before-create and before-update record lifecycle events.
- Success: automations can run code steps before a record is created/updated and mutate the in-flight record before persistence, with validation/UI options updated and tests run where practical.

Constraints/Assumptions:

- Follow `AGENTS.md`; keep backend/UI changes focused.
- Existing unrelated changes may be present; do not revert them.
- Preserve existing async behavior for post-save record create/update/delete triggers.
- Before-create/before-update triggers should run synchronously in the record lifecycle so code steps can customize the live record before persistence.

Key decisions:

- Added trigger values `record.beforeCreate` and `record.beforeUpdate`.
- Code steps now expose live record models as `$record` and `$recordOriginal`; use `$record.set(...)` in before triggers to mutate the saved record.
- Shared record-trigger lookup between async post-save queueing and sync pre-save execution.

State:

- Done:
  - Added backend trigger constants, validation allowlist, schema catalog entries, and registry scoping via existing record trigger logic.
  - Registered synchronous `OnRecordCreate` and `OnRecordUpdate` automation handlers for the new before triggers.
  - Added `$record`/`$recordOriginal` code step globals and refreshed record template data after each step.
  - Updated admin UI trigger options, labels, run formatting, data mapping suggestions, mail attachment trigger support, and code editor autocomplete.
  - Added focused tests for before-create and before-update code-step record customization and persistence.
  - Ran targeted Go tests successfully with `go test -mod=readonly ./core -run 'TestAutomationBeforeRecord(Create|Update)CodeStepCanCustomizeRecord|TestAutomationMailStepSendsMessageWithRecordAttachments|TestAutomationSchemas'`.
  - Attempted `npm run build`; Vite failed because local `node_modules` is missing `monaco-editor`, and dprint reported the known cache permission warning. Cleaned generated `ui/dist` churn from the failed build.
- Now:
  - Ready for user review.
- Next:
  - Install/restore the UI `monaco-editor` dependency and rerun `npm run build` if full frontend verification is needed.

Open questions (UNCONFIRMED if needed):

- Whether before-trigger values should be renamed is UNCONFIRMED; current implementation uses `record.beforeCreate` and `record.beforeUpdate`.

Working set (files/ids/commands):

- `ui/package.json`
- `ui/package-lock.json`
- `ui/dist/index.html`
- `ui/dist/assets/*` generated Monaco/editor chunks
- `npm run build`
- # `npm audit --omit=dev`
- `core/automation_model.go`
- `core/automation_validate.go`
- `core/automation_runner.go`
- `core/automation_steps.go`
- `core/automation_schema.go`
- `core/automation_runner_test.go`
- `ui/src/base/automationInput.js`
- `ui/src/settings/automations/automationUpsertModal.js`
- `ui/src/settings/automations/pageAutomationUpsert.js`
- `ui/src/settings/automations/automationsList.js`
- `ui/src/settings/automations/automationRunsList.js`
- `ui/src/settings/automations/automationRunPreviewModal.js`
- `ui/src/settings/automations/mailStepForm.js`
- `ui/src/settings/automations/stepEditor.js`
- `go test -mod=readonly ./core -run 'TestAutomationBeforeRecord(Create|Update)CodeStepCanCustomizeRecord|TestAutomationMailStepSendsMessageWithRecordAttachments|TestAutomationSchemas'`
- `npm run build` (failed: missing `monaco-editor`; dprint cache permission warning)
