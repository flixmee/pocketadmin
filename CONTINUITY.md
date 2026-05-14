Goal (incl. success criteria):

- Document the `Translation missing`, `Locale published`, `Translation updated`, and `AI translation finished` automations.
- Success: `AUTOMATION_PLAN.md` explains each trigger's purpose, how to use it, when to use it, and practical payload/template fields.

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
  - Now:
    - Focused i18n automation trigger documentation update is complete.
  - Next:
    - Await user review or follow-up edits.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: exact AI provider integration. For this pass, AI translation is represented as jobs plus automation trigger surfaces, not a provider call.

Working set (files/ids/commands):

- `/Volumes/MacOS_WD/Developer/pocketadmin/CONTINUITY.md`
- `/Volumes/MacOS_WD/Developer/pocketadmin/AUTOMATION_PLAN.md`
- `/Volumes/MacOS_WD/Developer/pocketadmin/cmd/i18n.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/i18n.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/i18n_test.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/collection_test.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/i18n_migrate.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/i18n_translation_job.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/i18n_model.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_runner.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/automation_runner_test.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/collection_query_test.go`
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
