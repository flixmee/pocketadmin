Goal (incl. success criteria):

- Fix localized collection enablement/update failing with `UNIQUE constraint failed: posts.i18n_group_id, posts.locale` when existing records receive blank i18n system fields before the unique index is created.
- Success: enabling i18n on a collection with existing records backfills each existing record with its own i18n group/default locale/source marker before the unique `(i18n_group_id, locale)` index is created, and targeted regression tests pass.

Constraints/Assumptions:

- Use separate localized records linked by an i18n group, matching the architecture already drafted in `LOCALES_PLAN.md`.
- Keep Go/backend and admin UI work isolated by phase unless a phase explicitly spans both.
- Follow existing PocketBase collection metadata/index/migration patterns instead of introducing an external workflow/runtime.
- UI work must follow admin UI conventions from `AGENTS.md`; no native HTML `<select>` elements.
- `go.mod` declares Go 1.25.0.

Key decisions:

- Treat AI translation, bulk translation, advanced SEO/domain routing, and enterprise workflow features as post-MVP follow-ups.
- Plan an internal backend foundation before adding admin UI controls so record/query behavior is testable independently.
- Locale-aware record querying should start as explicit query/API behavior and only later be surfaced through SDK helpers.

State:
  - Done:
    - Read `CONTINUITY.md` at the start of the turn.
    - Read `LOCALES_PLAN.md`.
    - Scanned relevant repo areas for collection options, indexes, record query, API, and admin UI patterns.
    - Replaced the broad MVP scope in `LOCALES_PLAN.md` with an implementation roadmap covering MVP success criteria and Phases 0-9.
    - Checked the resulting diff and confirmed the new section starts at `LOCALES_PLAN.md:451`.
    - User requested implementation of Phases 1-7.
    - Added i18n system collections `_locales` and `_i18nGroups` with system migration `1776000000_i18n.go`.
    - Added core locale/i18n proxy models, locale helpers, collection i18n options, system fields/indexes, record lifecycle hooks, duplicate translation validation, and empty-group cleanup.
    - Added superuser locale APIs under `/api/locales`.
    - Added localized record list filtering/fallback via `locale` and `fallback` query params.
    - Added record translation metadata and create-translation endpoints under `/api/collections/{collection}/records/{id}/translations`.
    - Added admin UI locale settings page, collection i18n options tab, and record modal locale/translation controls.
    - Added targeted core/API tests for locale lifecycle, collection metadata, list filtering, fallback, and translations metadata.
    - Verification passed:
      - `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./core -run 'TestI18n|TestCollectionMarshalJSON|TestCollectionUnmarshalJSON|TestCollectionDBExport'`
      - `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./apis -run 'TestLocalesList|TestI18nRecordLocaleListFallbackAndTranslations'`
      - `cd ui && npm run build` (Vite passed; dprint still reported the existing cache write warning outside the workspace).
    - User reported collection update error while enabling i18n: unique index creation failed because existing records share empty `i18n_group_id` and `locale`.
    - Fixed i18n enablement by backfilling existing records with one `_i18nGroups` row per record, default locale, and `is_source=true` before creating i18n indexes.
    - Added `TestI18nEnableCollectionBackfillsExistingRecordsBeforeUniqueIndex` regression coverage.
    - Verification passed:
      - `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./core -run 'TestI18n|TestCollectionMarshalJSON|TestCollectionUnmarshalJSON|TestCollectionDBExport'`
      - `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./apis -run 'TestLocalesList|TestI18nRecordLocaleListFallbackAndTranslations'`
      - `cd ui && npm run build` (Vite passed; dprint still reported the existing cache write warning outside the workspace).
  - Now:
    - Preparing final summary for the fix.
  - Next:
    - User can retry `PATCH /api/collections/pbc_1125843985`.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: exact internal collection/table naming should be `pb_locales`/`pb_i18n_groups` or use PocketBase-style system collections.
- Resolved: MVP uses PocketBase-style system collections `_locales` and `_i18nGroups`.
- Resolved: per-collection default locale is supported through `collection.i18n.defaultLocale`, falling back to the global default.
- Resolved for MVP: localized fields are stored as collection metadata and used by the UI/configuration layer; backend localization applies to the whole separate localized record.
- UNCONFIRMED: whether translation create/list endpoints should fully mirror public collection rules or stay superuser/editor oriented.

Working set (files/ids/commands):

- `/Volumes/MacOS_WD/Developer/pocketadmin/CONTINUITY.md`
- `/Volumes/MacOS_WD/Developer/pocketadmin/LOCALES_PLAN.md`
- `sed -n '1,260p' LOCALES_PLAN.md`
- `sed -n '261,520p' LOCALES_PLAN.md`
- `sed -n '520,760p' LOCALES_PLAN.md`
- `sed -n '1,120p' go.mod`
- `rg "options|i18n|locale|collection.*options|Indexes" core apis ui/src -g '*.go' -g '*.js'`
- `git diff -- LOCALES_PLAN.md CONTINUITY.md`
- `rg -n "Implementation Plan|Phase 0|Phase 1|Phase 9|MVP Success" LOCALES_PLAN.md`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/i18n_model.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/i18n_model_test.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/i18n.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/apis/i18n_test.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/migrations/1776000000_i18n.go`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/locales/pageLocalesSettings.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/settings/locales/localesList.js`
- `/Volumes/MacOS_WD/Developer/pocketadmin/ui/src/collections/collectionI18nOptionsTab.js`
- `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./core -run 'TestI18n|TestCollectionMarshalJSON|TestCollectionUnmarshalJSON|TestCollectionDBExport'`
- `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./apis -run 'TestLocalesList|TestI18nRecordLocaleListFallbackAndTranslations'`
- `cd ui && npm run build`
- `/Volumes/MacOS_WD/Developer/pocketadmin/core/collection_record_table_sync.go`
- `env GOCACHE=/private/tmp/pocketadmin-go-cache go test ./core -run 'TestI18n'`
