Goal (incl. success criteria):

- Implement Platform Phase 3 policy and recursion safety MVP.
- Success: add run audit fields, policy decision checks for depth/rate/concurrency/cooldown/dedupe, and focused tests without breaking normal automations.

Constraints/Assumptions:

- Preserve existing i18n model/API conventions already present in the repo.
- Avoid touching unrelated dirty user work; `ui/src/css/vars.css` is currently modified by someone else.
- Reuse the existing automation runner instead of adding a separate workflow runtime.
- Keep Phase 9 AI support as job/workflow infrastructure; do not wire a real external AI provider by default.

Key decisions:

- Phase 8 will be implemented as a core migration helper plus a built-in CLI command, so app code and tests can call the same path.
- Phase 9 will add translation job persistence and i18n automation trigger seams using the existing automation registry/runner.
- Translation creation should generate locale-suffixed slugs when copying a source record would violate an existing single-column unique slug index.

State:
  - Done:
    - Read `CONTINUITY.md` at the start of the turn.
    - User requested: "Please help me improve Collection Option i18n UI".
    - Inspected `collectionI18nOptionsTab.js`, nearby collection option tabs, `UI_DOCS.md`, and shared select/list/form CSS patterns.
    - Improved the collection i18n options tab with a section heading, switch-style enable control, clearer help text, enabled-locale-aware default locale select labels, selected-field count, and Select all/Clear field actions.
    - Ran `cd ui && npm run build`; build passed. dprint still emitted the existing cache write warning outside the workspace but formatted 1 file and Vite completed successfully.
    - User requested Locales Settings UI improvements based on a reference image and Twemoji flags.
    - Reworked Locales Settings with a table-style locale list, Twemoji flag images, default check buttons, enable/disable and delete row actions, and a collapsible add-language panel.
    - Added a searchable language picker backed by `app.components.select`, with common language presets and editable display name/code fields.
    - Added `ui/src/css/locales.css` and imported it from `_main.css`.
    - Ran `cd ui && npm run build`; build passed. dprint still emitted the existing cache write warning outside the workspace but formatted 1 file and Vite completed successfully.
    - User asked to fix flag URLs to `https://cdnjs.cloudflare.com/ajax/libs/twemoji/14.0.0/svg/`.
    - Updated `TWEMOJI_BASE_URL` and rebuilt UI. Build passed; dprint still emitted the existing cache write warning outside the workspace.
    - Added record-view enrichment for localized records to include `locale` and `localeLinks`.
    - Added focused API test coverage for localized record view metadata.
    - Ran `go test ./apis -run TestI18nRecordLocaleListFallbackAndTranslations`; passed.
    - Ran `go test ./core ./apis`; failed on existing fixture-count/watcher expectations unrelated to this change (`TestFindAllCollections`, `TestNotifyWatcher_SettingsUpdate`, `TestCollectionsList`, `TestCollectionsImport`).
    - User reported `POST /api/collections/posts/records/{id}/translations` failing with `slug: Value must be unique`.
    - User requested Phases 8 and 9.
    - Added locale-suffixed translation slug generation and regression coverage for unique slug fields.
    - Added Phase 8 core i18n migration helper and `migrate:i18n` CLI command with dry-run reporting.
    - Added Phase 9 `_translationJobs`, translation job API, i18n automation triggers, trigger payload templating, and admin UI trigger labels.
    - Updated collection-count fixtures for the added `_translationJobs` system collection.
    - Targeted Go tests passed for core/API/cmd i18n, automation, and collection list coverage.
    - `ui` build passed; dprint still emitted the existing cache write warning outside the workspace before Vite completed successfully.
    - Broad `go test ./...` was attempted; remaining failures are existing watcher/fixture breadth plus sandboxed `httptest` listener permission failures in packages that open local ports.
    - User requested automation documentation in `AUTOMATION_PLAN.md`.
    - Added automation documentation to `AUTOMATION_PLAN.md` covering current capabilities, API routes, trigger payloads, step schemas, examples, and operational notes.
    - User requested focused documentation for Translation missing, Locale published, Translation updated, and AI translation finished automation.
    - Added a dedicated `i18n automation triggers` section to `AUTOMATION_PLAN.md` covering purpose, how to use, when to use, and payload fields for all four triggers.
    - User requested review of `FUTURE_AUTOMATION_PLATFORM_PLAN.md` and a phase-by-phase execution plan.
    - Reviewed `FUTURE_AUTOMATION_PLATFORM_PLAN.md`, `AUTOMATION_PLAN.md`, and key automation runtime/model files.
    - Appended a `Phase-by-Phase Execution Roadmap` to `FUTURE_AUTOMATION_PLATFORM_PLAN.md` with Platform Phase 0-13, including goals, deliverables, verification, and exit criteria.
    - Renamed the earlier future sketch from `Recommended New Phases` / `Phase 9-14` to `Recommended Platform Capability Areas` / `Area A-F` to avoid conflicting with the already completed automation/i18n Phase 9.
    - User requested implementation of Platform Phase 0-11.
    - Implemented Platform Phase 0 foundation: max automation steps, template output, HTTP timeout, and HTTP body guardrails with tests.
    - Implemented Platform Phase 1 foundation: `core.AutomationSchemas()` plus superuser `GET /api/automations/schemas` with tests.
    - Implemented Platform Phase 11 backend foundation: `RunAutomationDryRun` plus superuser `POST /api/automations/{id}/dry-run`; dry-run previews side-effecting steps and does not persist `_automationRuns`.
    - Ran `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./core -run 'TestAutomation'`; passed.
    - Ran `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./apis -run 'TestAutomation'`; passed.
    - User requested Platform Phase 2.
    - Added `_capabilities` system collection migration.
    - Added `Capability` model/proxy, query helpers, validation hooks, and app interface methods.
    - Added built-in capability registry for `http.request`, `mail.send`, `record.create`, `record.update`, and `record.delete`.
    - Added `capability` automation step support with `capability`/`key` plus `input` object shape, mapped internally to existing legacy step handlers.
    - Extended schema discovery to include built-in capabilities and the `capability` step schema.
    - Updated system collection count fixtures for `_capabilities`.
    - Removed stray debug `fmt.Println` from translation job hook.
    - Ran `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./core -run 'TestAutomation|TestCapability|TestFindAllCollections|TestImportCollections'`; passed.
    - Ran `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./apis -run 'TestAutomation|TestCollectionsList|TestCollectionsImport'`; passed.
    - Broader `go test ./core ./apis` was attempted; after fixture updates, remaining failures were known environment/existing issues: `TestNotifyWatcher_SettingsUpdate` watcher event and `TestRecordAuthWithOAuth2` sandboxed `httptest` listener.
    - User requested Platform Phase 3.
    - Added automation run audit fields: `parentRunId`, `depth`, `dedupeKey`, and `policyDecision`.
    - Added migration `1778000000_automation_policy_audit.go` for policy audit fields and indexes.
    - Added app-level `AutomationPolicyConfig` via `StoreKeyAutomationPolicyConfig` with defaults for max depth, max runs per minute, max concurrent runs, plus optional cooldown and dedupe windows.
    - Added runtime policy evaluation before execution; denied runs are persisted as failed with policyDecision audit JSON.
    - Added parent/depth inheritance for automations triggered by automation-created records.
    - Added tests for rate limit, concurrency limit, duplicate dedupe key, and recursive depth rejection.
    - Ran `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./core -run 'TestAutomationPolicy|TestAutomationRunFields|TestAutomationCollectionsExist'`; passed.
    - Ran `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./core -run 'TestAutomation|TestCapability|TestFindAllCollections|TestImportCollections'`; passed.
    - Ran `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./apis -run 'TestAutomation|TestCollectionsList|TestCollectionsImport'`; passed.
    - Broader `go test ./core ./apis` was attempted; remaining failures are known environment/existing issues: watcher tests and sandboxed `httptest` listener in OAuth2 test.
  - Now:
    - Platform Phase 3 implementation slice is complete.
  - Next:
    - Continue with Platform Phase 4 persistent workflow state MVP.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: exact AI provider integration. For this pass, AI translation is represented as jobs plus automation trigger surfaces, not a provider call.

Working set (files/ids/commands):

- `/Volumes/MacOS_WD/Developer/pocketadmin/CONTINUITY.md`
- `/Volumes/MacOS_WD/Developer/pocketadmin/FUTURE_AUTOMATION_PLATFORM_PLAN.md`
- `/Volumes/MacOS_WD/Developer/pocketadmin/AUTOMATION_PLAN.md`
- `/Volumes/MacOS_WD/Developer/pocketadmin/cmd/i18n.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/automation.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/automation_test.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/collection_import_test.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/i18n.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/i18n_test.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/collection_test.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/i18n_migrate.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/i18n_translation_job.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/i18n_model.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/app.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/base.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_capability_model.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_capability_runtime.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_schema.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_model.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_http.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_policy.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_run_model.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_runner.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_runner_test.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_steps.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_templates.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_validate.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_model_test.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/collection_query_test.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/migrations/1777000000_capabilities.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/migrations/1778000000_automation_policy_audit.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/migrations/1776000001_translation_jobs.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/automations/automationUpsertModal.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/automations/automationsList.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/automations/automationRunsList.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/automations/automationRunPreviewModal.js`
- `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./core -run 'TestI18n|TestAutomationI18n|TestAutomation|TestFindAllCollections'`
- `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./apis -run 'TestI18n|TestLocales|TestTranslation|TestCollectionsList'`
- `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./cmd -run 'TestSuperuser|TestI18n'`
- `cd ui && npm run build`
- `sed -n '110,300p' AUTOMATION_PLAN.md`
- `sed -n '1,1040p' FUTURE_AUTOMATION_PLATFORM_PLAN.md`
- `rg -n "Platform Phase|Recommended Build Order|numbering note|Phase 9" FUTURE_AUTOMATION_PLATFORM_PLAN.md AUTOMATION_PLAN.md`
- `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./core -run 'TestAutomation'`
- `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./apis -run 'TestAutomation'`
- `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./core -run 'TestAutomation|TestCapability|TestFindAllCollections|TestImportCollections'`
- `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./apis -run 'TestAutomation|TestCollectionsList|TestCollectionsImport'`
- `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./core ./apis`
- `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./core -run 'TestAutomationPolicy|TestAutomationRunFields|TestAutomationCollectionsExist'`
