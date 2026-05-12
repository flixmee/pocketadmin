Goal (incl. success criteria):

- When viewing a record in a locale-enabled collection, include `locale` and `localeLinks` fields on the record payload.
- Success: `locale` identifies the record locale, and `localeLinks` is an array of related record IDs plus their locales.

Constraints/Assumptions:

- Keep the change focused on record read/view behavior unless tests reveal a required supporting change.
- Preserve existing API compatibility for non-locale collections.
- Use existing locale/i18n model conventions already present in the repo.
- `localeLinks` shape implemented as `[{ "id": "...", "locale": "..." }]`, including all records in the same i18n group, ordered by locale.

Key decisions:

- Expose `locale` by unhiding the existing hidden i18n field only during record view enrichment.
- Export `localeLinks` as a dedicated i18n metadata field without enabling broad custom-data export.

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
  - Now:
    - Preparing final summary.
  - Next:
    - User can review localized record view payloads.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `/Users/suytbily/dev/gits/harry/pocketadmin/CONTINUITY.md`
- `/Users/suytbily/dev/gits/harry/pocketadmin/apis/i18n.go`
- `/Users/suytbily/dev/gits/harry/pocketadmin/apis/i18n_test.go`
- `/Users/suytbily/dev/gits/harry/pocketadmin/apis/record_crud.go`
- `/Users/suytbily/dev/gits/harry/pocketadmin/core/i18n_model.go`
- `/Users/suytbily/dev/gits/harry/pocketadmin/core/record_model.go`
- `go test ./apis -run TestI18nRecordLocaleListFallbackAndTranslations`
- `go test ./core ./apis`
