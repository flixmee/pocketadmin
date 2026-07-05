Goal (incl. success criteria):

- Create documentation under `docs/` explaining the project/admin UI i18n behavior.
- Success: new doc matches existing docs style, reflects the current implementation, and is checked for obvious formatting issues.
- Fix `Uncaught ReferenceError: data is not defined` in `ui/src/fields/json/settings.js`.
- Success: JSON field settings use the component `props` contract, no stale `data.field` references remain, and targeted UI build/checks pass.

Constraints/Assumptions:

- Follow `AGENTS.md`.
- Existing unrelated changes are present; do not revert them.
- When updating the UI, refer to `UI_DOCS.md`.
- Never introduce native HTML `<select>` elements in the admin UI.
- Avoid storing ephemeral presentation state on reactive `field`/`collection` objects when local/out-of-band state is enough.

Key decisions:

- Add `docs/i18n-api.md`.
- Scope the doc to API/data-model behavior: locales, collection `i18n` options, record locale queries, translation records, translation jobs, automation hooks, and admin UI notes.
- Use `props.field` for JSON schema state because field settings components receive `props` and other JSON settings already use `props.field`.

State:
  - Done:
    - Read `CONTINUITY.md`.
    - Reset active ledger state for the i18n docs request.
    - Inspected existing docs style and i18n implementation in `core/`, `apis/`, `migrations/`, and `ui/src/`.
    - Created `docs/i18n-api.md` covering locales, collection i18n options, system fields, record lifecycle, locale-aware lists, translation endpoints, translation jobs, automations, admin UI behavior, migration helper, and common errors.
    - Ran `git diff --check -- docs/i18n-api.md CONTINUITY.md`; passed.
    - Ran `rg -n "[^[:ascii:]]" docs/i18n-api.md`; no non-ASCII matches.
  - Now:
    - Ready to report the completed docs addition.
    - Inspected `ui/src/fields/json/settings.js`; found `data.field` used in `settings(props)`.
    - Inspected `ui/src/fields/json/schemaState.js`.
    - Reviewed `UI_DOCS.md` settings component contract: settings receive `props.field`.
    - Replaced all `data.field` references in JSON settings with `props.field`.
    - Confirmed no `data.field` references remain in `ui/src/fields/json`.
    - Ran `npm run build` from `ui/`; passed. It still prints the existing dprint cache permission warning under `~/Library/Caches`.
    - Reverted the generated `ui/dist/index.html` asset-reference churn caused by the build; source fix remains.
    - Ran `git diff --check -- ui/src/fields/json/settings.js CONTINUITY.md ui/dist/index.html`; passed.
  - Now:
    - Ready to report the JSON settings fix.
  - Next:
    - None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `docs/`
- `docs/i18n-api.md`
- `core/i18n_model.go`
- `core/i18n_translation_job.go`
- `core/collection_model_base_options.go`
- `core/i18n_migrate.go`
- `apis/i18n.go`
- `apis/record_crud.go`
- `migrations/1776000000_i18n.go`
- `migrations/1776000001_translation_jobs.go`
- `ui/src/collections/collectionI18nOptionsTab.js`
- `ui/src/settings/locales/localesList.js`
- `ui/src/records/recordUpsertModal.js`
- `git diff --check -- docs/i18n-api.md CONTINUITY.md`
- `rg -n "[^[:ascii:]]" docs/i18n-api.md`
- `ui/src/fields/json/settings.js`
- `ui/src/fields/json/schemaState.js`
- `UI_DOCS.md`
- `npm run build` from `ui/`
- `git diff --check -- ui/src/fields/json/settings.js CONTINUITY.md ui/dist/index.html`
