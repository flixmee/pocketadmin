# PocketBase Multi-language System (Strapi-like i18n)

## Goal

Build a multilingual content system for PocketBase similar to Strapi i18n:

- One content entry can have multiple localized versions
- Locale-based querying
- Fallback locale support
- Translation relationships
- Admin UI locale switching
- Automation-ready architecture
- AI translation support
- SEO-friendly URLs

---

# 1. Recommended Architecture

## Use Separate Translation Records

Each locale is stored as an independent record.

Example:

| id | i18n_group_id | locale | title |
|---|---|---|---|
| blog_1 | grp_001 | en | Hello |
| blog_2 | grp_001 | vi | Xin chào |
| blog_3 | grp_001 | ja | こんにちは |

---

## Why This Is Better

Compared to JSON-based translations:

✅ Better indexing  
✅ Better querying  
✅ Better filtering  
✅ Better permissions  
✅ Better automation  
✅ Better relations  
✅ Better scalability  
✅ Easier SDK support

---

# 2. Database Structure

## Table: pb_locales

```sql
CREATE TABLE pb_locales (
    id TEXT PRIMARY KEY,

    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,

    enabled BOOLEAN NOT NULL DEFAULT true,
    is_default BOOLEAN NOT NULL DEFAULT false,

    created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## Table: pb_i18n_groups

```sql
CREATE TABLE pb_i18n_groups (
    id TEXT PRIMARY KEY,

    collection_name TEXT NOT NULL,

    default_locale TEXT NOT NULL,

    created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## Example Translatable Collection

```sql
CREATE TABLE blogs (
    id TEXT PRIMARY KEY,

    i18n_group_id TEXT NOT NULL,
    locale TEXT NOT NULL,

    is_source BOOLEAN DEFAULT false,

    title TEXT,
    slug TEXT,
    content TEXT,

    status TEXT DEFAULT 'draft',

    created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated DATETIME DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (i18n_group_id)
        REFERENCES pb_i18n_groups(id)
        ON DELETE CASCADE,

    FOREIGN KEY (locale)
        REFERENCES pb_locales(code)
);
```

---

# 3. Required Indexes

```sql
CREATE UNIQUE INDEX idx_blogs_i18n_unique
ON blogs(i18n_group_id, locale);

CREATE INDEX idx_blogs_locale
ON blogs(locale);

CREATE INDEX idx_blogs_group
ON blogs(i18n_group_id);

CREATE INDEX idx_blogs_slug_locale
ON blogs(slug, locale);

CREATE INDEX idx_blogs_status_locale
ON blogs(status, locale);
```

---

# 4. API Design

## Get Records By Locale

```http
GET /api/blogs?locale=vi
```

## Get All Translations

```http
GET /api/blogs/:id/translations
```

## Fallback Locale

```http
GET /api/blogs?locale=fr&fallback=true
```

Logic:

1. Try requested locale
2. Fallback to default locale

---

# 5. Admin UI Design

## Collection Setting

```text
[ ] Enable Localization
```

## Record Editor

```text
[ EN ] [ VI ] [ JA ]
```

Features:

- Switch locale
- Duplicate translation
- Missing translation indicator
- Auto-translate button

---

# 6. Record Creation Flow

## Create Source Locale

```text
Title: Hello
Locale: en
```

System:

```text
i18n_group_id = uuid()
is_source = true
```

## Create Translation

```text
Create Vietnamese Translation
```

System:

- duplicates source record
- changes locale
- keeps same i18n_group_id

---

# 7. PocketBase Collection Metadata

```json
{
  "i18n": {
    "enabled": true,
    "defaultLocale": "en",
    "localizedFields": [
      "title",
      "content",
      "slug"
    ]
  }
}
```

---

# 8. SDK Design

```ts
pb.collection("blogs").getList(1, 20, {
  locale: "vi",
  fallback: true
})
```

---

# 9. Translation Expansion Response

```json
{
  "id": "blog_2",
  "locale": "vi",

  "translations": [
    {
      "locale": "en",
      "id": "blog_1"
    },
    {
      "locale": "ja",
      "id": "blog_3"
    }
  ]
}
```

---

# 10. URL Structure

```text
/en/blog/my-post
/vi/blog/bai-viet
/ja/blog/article
```

---

# 11. Automation Integration

Example triggers:

```text
On Translation Missing
On Locale Published
On Translation Updated
On AI Translation Finished
```

Workflow example:

```text
English Published
    ↓
Automation Trigger
    ↓
Send to AI Translation
    ↓
Create VI Draft
    ↓
Notify Editor
```

---

# 12. AI Translation Tables

```sql
CREATE TABLE pb_translation_jobs (
    id TEXT PRIMARY KEY,

    collection_name TEXT NOT NULL,

    source_record_id TEXT NOT NULL,

    source_locale TEXT NOT NULL,
    target_locale TEXT NOT NULL,

    provider TEXT,
    model TEXT,

    status TEXT DEFAULT 'pending',

    error TEXT,

    created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

# 13. Translation Metadata Table

```sql
CREATE TABLE pb_i18n_meta (
    id TEXT PRIMARY KEY,

    collection_name TEXT NOT NULL,
    record_id TEXT NOT NULL,

    source_locale TEXT NOT NULL,

    translation_progress INTEGER DEFAULT 0,

    last_translated_at DATETIME,

    created DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

# 14. Recommended Hook System

PocketBase hooks:

```go
OnRecordCreate
OnRecordUpdate
OnRecordDelete
```

Used for:

- validating locales
- preventing duplicates
- syncing translation metadata
- cleanup translation relations
- updating translation status

---

# 15. Media Strategy

## Shared Media

Best for:

- blogs
- products
- news

## Localized Media

Best for:

- banners
- marketing pages
- localized campaigns

---

# 16. Important Constraints

```text
i18n_group_id + locale unique
```

---

# 17. Important Recommendation

## DO NOT use JSON translations

Avoid:

```json
{
  "title": {
    "en": "...",
    "vi": "..."
  }
}
```

Because it causes:

- poor indexing
- poor querying
- poor filtering
- poor scalability

---

# 18. Recommended Folder Structure

```text
/core/i18n
/core/locales
/core/translations
/apis/i18n
/ui/i18n
```

---

# 19. Migration Strategy

```bash
pb migrate:i18n blogs
```

Migration steps:

- add locale fields
- create i18n groups
- assign default locale
- generate indexes

---

# 20. Implementation Plan and Phases

## MVP Success Criteria

The first shippable version should support:

- superusers can configure enabled locales and one default locale
- collections can opt into localization
- localized records are stored as separate records linked by a translation group
- each group can have at most one record per locale
- record list/view APIs can filter by locale
- clients can request default-locale fallback when a translation is missing
- the admin UI can create, switch, and identify translations for a localized record
- targeted backend/API/UI tests cover the core lifecycle

Keep AI translation, bulk translation, diff review, domain routing, and complex editorial workflow out of the MVP.

---

## Phase 0: Repository Discovery and Final Design

Goal: confirm exact integration points before writing migrations.

Tasks:

- inspect collection model/options handling in `core/collection_model*.go`
- inspect record list/query flow in `apis/record*.go` and `core/record_query*.go`
- inspect collection table/index sync in `core/collection_record_table_sync.go`
- inspect admin collection editor and record editor patterns under `ui/src/collections` and `ui/src/records`
- decide whether locale/group storage should be hidden system collections or lower-level internal tables
- decide where collection i18n metadata lives in the existing collection `options` payload

Deliverables:

- final field names and constants
- final API parameters and response shape
- migration/backfill rules for existing collections
- tests list before implementation starts

Verification:

- no code behavior required yet
- document the final choices in this plan before moving to Phase 1

---

## Phase 1: Locale Registry Foundation

Goal: add global locale management primitives without changing record behavior yet.

Backend tasks:

- add locale model/storage for `code`, `name`, `enabled`, `is_default`
- enforce one default locale
- enforce unique locale codes and validate code format
- seed a default locale when none exists, preferably from existing app settings or a conservative `en`
- expose core helpers for enabled locales, default locale, and locale lookup

API tasks:

- add superuser-only locale management endpoints, or expose the system collection through an internal API if that matches the final design
- add list/create/update/delete tests, including default-locale constraints

UI tasks:

- add a Settings page for locales
- support enable/disable, default selection, create, edit, and delete
- use `app.components.select` for dropdowns; no native `<select>`

Verification:

- `go test ./core -run 'Locale|I18n|BaseApp'`
- `go test ./apis -run 'Locale|I18n'`
- `cd ui && npm run build`

Exit criteria:

- locales can be managed by superusers
- one default enabled locale always exists

---

## Phase 2: Collection Localization Metadata

Goal: allow a collection to opt into localized-record behavior.

Backend tasks:

- extend base collection options with an `i18n` block:

```json
{
  "i18n": {
    "enabled": true,
    "defaultLocale": "en",
    "localizedFields": ["title", "content", "slug"]
  }
}
```

- validate enabled locales and localized field names
- prevent invalid i18n config on unsupported collection types if needed
- add or reserve system fields when localization is enabled:
  - `i18n_group_id`
  - `locale`
  - `is_source`
- automatically manage required indexes:
  - unique `(i18n_group_id, locale)`
  - `locale`
  - `i18n_group_id`
  - optional common compound indexes such as `(slug, locale)` when `slug` exists

API/UI tasks:

- add collection settings controls:
  - enable localization
  - default locale
  - localized fields picker
- warn before disabling localization if localized records already exist

Verification:

- collection validation tests
- table/index sync tests
- UI build

Exit criteria:

- a collection can be saved with valid i18n options
- required system fields/indexes are created and preserved

---

## Phase 3: Translation Group Lifecycle

Goal: create and maintain separate localized records linked by group.

Backend tasks:

- add i18n group model/storage with collection name and default locale
- on source record creation, create a group and assign:
  - `i18n_group_id`
  - `locale`
  - `is_source = true`
- on translation creation, duplicate/copy from source and assign:
  - same `i18n_group_id`
  - selected target locale
  - `is_source = false`
- enforce unique group+locale on create/update
- validate that `locale` is enabled
- define delete behavior:
  - deleting one translation leaves the group if other translations remain
  - deleting the last translation removes the group
  - deleting the source either promotes another translation or blocks until explicit behavior is chosen

API tasks:

- add endpoints or record actions for:
  - create translation from record
  - list translations for record
  - get source/default translation for record
- ensure authorization follows the underlying collection record rules.

Verification:

- lifecycle tests for source create, translation create, duplicate prevention, delete cleanup
- API tests for permissions and translation listing

Exit criteria:

- localized records maintain consistent group relationships
- duplicate locale records cannot be created for the same group

---

## Phase 4: Locale-aware Querying and Fallback

Goal: make record APIs usable for locale-specific reads.

API behavior:

```http
GET /api/collections/blogs/records?locale=vi
GET /api/collections/blogs/records?locale=fr&fallback=true
```

Backend/API tasks:

- parse `locale` and `fallback` query params for localized collections
- filter localized collection list queries by requested locale
- if `fallback=true`, return requested locale when present and default-locale records otherwise
- define stable sorting/pagination semantics for fallback queries
- reject unknown/disabled locales with a clear validation error
- ensure non-localized collections ignore or reject i18n params consistently

Important design point:

- fallback list queries must avoid returning both requested and default records for the same group
- prefer deterministic SQL/query-builder behavior over post-processing large record sets in Go

Verification:

- record list tests for locale filter
- fallback tests with missing translations
- pagination/sort tests for fallback behavior
- permission tests to ensure fallback does not bypass record rules

Exit criteria:

- clients can list localized records by locale
- fallback behavior is deterministic and permission-safe

---

## Phase 5: Translation Expansion and Response Shape

Goal: expose translation relationships without requiring clients to hand-query groups.

API tasks:

- add `translations` expansion support or a dedicated translations endpoint:

```http
GET /api/collections/blogs/records/:id/translations
```

- response should include at least:
  - locale
  - record id
  - is source/default
  - missing locale indicators if requested
- consider an `expand=translations` style only if it fits existing expand infrastructure cleanly

UI tasks:

- show translation availability in record lists/details where practical

Verification:

- API tests for translation listing and missing-locale metadata
- response shape tests for non-localized collections

Exit criteria:

- clients can discover all translations for a record and identify missing locales

---

## Phase 6: Admin Record Editing Experience

Goal: make localization practical for content editors.

UI tasks:

- add locale tabs/switcher in the record editor for localized collections
- show missing translation states
- add "Create translation" action
- duplicate source/default record values into the new translation draft
- keep locale switch state local to the record editor unless it must be persisted
- avoid cloning/reassigning whole reactive arrays on every field edit
- use existing modal/form/action patterns from `ui/src/records`

Backend/API support:

- add any missing endpoint needed by the UI from Phase 3/5
- keep all actual validation in the backend

Verification:

- `cd ui && npm run build`
- targeted manual smoke test:
  - create source
  - create translation
  - switch locales
  - save each locale independently
  - verify list filtering

Exit criteria:

- superusers/editors can manage translations from the admin UI without touching raw system fields

---

## Phase 7: Slugs, URLs, and SDK Helpers

Goal: make localized content practical for websites.

Backend/API tasks:

- document and test localized slug uniqueness patterns:
  - global per-locale uniqueness: `(slug, locale)`
  - scoped uniqueness if project-specific fields exist
- add helpers/examples for locale-aware list/view calls
- expose SDK-friendly options:

```ts
pb.collection("blogs").getList(1, 20, {
  locale: "vi",
  fallback: true
})
```

Docs/examples:

- add examples for URL structures:
  - `/en/blog/my-post`
  - `/vi/blog/bai-viet`
- explain how fallback affects slugs and canonical URLs

Verification:

- slug+locale index tests
- docs/API preview update if applicable

Exit criteria:

- frontend consumers have a clear and tested pattern for localized routes and slugs

---

## Phase 8: Migration Tooling

Goal: help existing collections adopt localization safely.

CLI/migration tasks:

- add a migration command or documented migration helper similar to:

```bash
pb migrate:i18n blogs
```

- migration should:
  - enable collection i18n metadata
  - add system fields/indexes
  - create one i18n group per existing record
  - assign the default locale
  - mark existing records as source records
- support dry-run/report mode before mutation

Verification:

- migration tests with existing records
- rollback/backup guidance in docs

Exit criteria:

- existing single-language collections can be migrated to localized collections predictably

---

## Phase 9: Automation, AI Translation, and Editorial Workflow

Goal: build on the stable i18n foundation with higher-level workflows.

Automation tasks:

- add triggers:
  - translation missing
  - locale published
  - translation updated
  - AI translation finished
- expose template roots for locale/group/source/target metadata

AI translation tasks:

- add translation jobs table/model only after basic lifecycle is stable
- support provider/model config through existing settings patterns
- generate target-locale drafts rather than publishing automatically
- store job status/error/result metadata

Editor workflow tasks:

- translation completeness/progress
- bulk translation actions
- diff viewer
- review/approval states
- per-locale scheduling
- RTL support

Verification:

- automation runner tests
- AI job lifecycle tests with deterministic provider seam
- UI build and focused smoke tests

Exit criteria:

- translations can be generated and tracked as drafts without weakening the base record/i18n guarantees

---

# 21. Future Enterprise Features

- translation workflow
- human review pipeline
- translation memory
- locale scheduling
- RTL support
- per-locale SEO
- locale versioning
- domain-based locale routing

Example:

```text
en.example.com
vi.example.com
ja.example.com
```

---

# 22. Final Recommended Architecture

```text
pb_locales
pb_i18n_groups

blogs
products
pages
categories

pb_translation_jobs
pb_i18n_meta
```

Core philosophy:

```text
Separate localized records
+
translation group system
+
locale-aware APIs
+
automation-ready workflows
+
AI translation support
```
