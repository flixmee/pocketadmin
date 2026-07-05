# i18n API

This document describes the current internationalization behavior in PocketAdmin.

It covers:

- locale records and locale management endpoints
- collection-level `i18n` options
- record locale filtering and fallback behavior
- translation creation and translation metadata endpoints
- translation jobs
- automation triggers emitted by i18n events
- admin UI behavior that affects API payloads

## Overview

PocketAdmin uses a separate-record localization model.

When i18n is enabled for a base collection, each locale variant is stored as its own record. Records that represent the same content in different locales are linked by a hidden `i18n_group_id` value. Each record also stores its own hidden `locale` value and an `is_source` marker.

For example, a post translated into English and Vietnamese is stored as two records:

| id | i18n_group_id | locale | is_source | title |
|---|---|---|---|---|
| `post_en` | `group_123` | `en` | `true` | `Hello` |
| `post_vi` | `group_123` | `vi` | `false` | `Xin chao` |

This means localized records use the normal record create, update, delete, validation, expansion, and rule machinery. The i18n layer adds grouping, locale validation, locale-aware list filtering, translation helper endpoints, and automation events.

## System Collections

i18n adds these system collections:

- `_locales`: configured locales
- `_i18nGroups`: translation groups that connect per-locale records
- `_translationJobs`: optional translation job tracking records

The i18n system migration creates `_locales` and `_i18nGroups`, then ensures an English default locale:

```json
{
  "code": "en",
  "label": "English",
  "enabled": true,
  "is_default": true
}
```

## Locale Model

Locale records use this shape:

```json
{
  "id": "REDACTED",
  "code": "en",
  "label": "English",
  "enabled": true,
  "is_default": true,
  "created": "2026-01-01 00:00:00.000Z",
  "updated": "2026-01-01 00:00:00.000Z"
}
```

Properties:

- `code`: locale code, for example `en`, `vi`, or `pt-BR`
- `label`: human-readable locale name
- `enabled`: whether records can use this locale
- `is_default`: marks the global default locale

Locale code validation currently accepts lowercase 2-3 letter language codes with an optional uppercase/digit region suffix:

```text
^[a-z]{2,3}(-[A-Z0-9]{2,8})?$
```

Examples:

- valid: `en`, `vi`, `pt-BR`
- invalid: `EN`, `pt-br`, `en_US`

Notes:

- Locale codes are trimmed before saving.
- Locale labels are trimmed before saving.
- Setting `is_default` to `true` automatically enables the locale and clears `is_default` on other locales.
- The default locale cannot be disabled by unsetting `enabled`.
- The default locale cannot be deleted.

## Locale APIs

All `/api/locales` endpoints require superuser authentication.

Send a valid superuser token:

```http
Authorization: YOUR_SUPERUSER_TOKEN
```

### List Locales

```http
GET /api/locales
Authorization: YOUR_SUPERUSER_TOKEN
```

Response: `200 OK`

```json
[
  {
    "id": "REDACTED",
    "code": "en",
    "label": "English",
    "enabled": true,
    "is_default": true
  }
]
```

Locales are ordered with the default locale first, then by `code`.

### View Locale

```http
GET /api/locales/{idOrCode}
Authorization: YOUR_SUPERUSER_TOKEN
```

`idOrCode` may be either the locale record id or the locale code.

### Create Locale

```http
POST /api/locales
Authorization: YOUR_SUPERUSER_TOKEN
Content-Type: application/json
```

```json
{
  "code": "vi",
  "name": "Vietnamese",
  "enabled": true,
  "is_default": false
}
```

Response: `200 OK`

The request body uses `name`; the stored field is `label`.

### Update Locale

```http
PATCH /api/locales/{idOrCode}
Authorization: YOUR_SUPERUSER_TOKEN
Content-Type: application/json
```

```json
{
  "name": "Tieng Viet",
  "enabled": true
}
```

Response: `200 OK`

### Delete Locale

```http
DELETE /api/locales/{idOrCode}
Authorization: YOUR_SUPERUSER_TOKEN
```

Response: `204 No Content`

Deleting the default locale returns a validation error.

## Collection i18n Options

Only base collections support i18n.

A base collection can include an `i18n` options object:

```json
{
  "name": "posts",
  "type": "base",
  "i18n": {
    "enabled": true,
    "defaultLocale": "en",
    "localizedFields": ["title", "slug", "body"]
  }
}
```

Properties:

- `enabled`: turns i18n behavior on for the collection
- `defaultLocale`: locale used for new source records when no locale is supplied
- `localizedFields`: field names marked as translatable collection metadata

Validation:

- `defaultLocale` must be a valid existing locale code when i18n is enabled.
- `localizedFields` values must be unique existing collection field names.
- `localizedFields` cannot contain empty field names.

Notes:

- `localizedFields` is stored as collection metadata for translation workflows and admin configuration.
- Localized records are still full records. Backend record validation runs against the whole record.
- Enabling i18n adds hidden system fields and indexes to the collection.

## i18n System Fields

When i18n is enabled for a collection, PocketAdmin ensures these hidden system fields exist:

| Field | Type | Required | Purpose |
|---|---|---:|---|
| `i18n_group_id` | text | yes | Translation group id shared by all locale variants |
| `locale` | text | yes | Locale code for this record |
| `is_source` | bool | no | Marks the source record in the group |

The collection also receives these indexes:

- unique index on `i18n_group_id, locale`
- non-unique index on `locale`
- non-unique index on `i18n_group_id`
- non-unique index on `slug, locale` when the collection has a `slug` field

The unique `i18n_group_id, locale` index prevents duplicate translations for the same locale in a group.

## Record Lifecycle

### New source records

When a new record is created in an i18n-enabled collection:

- if `locale` is empty, it is set to the collection default locale
- if `i18n_group_id` is empty, a new `_i18nGroups` record is created
- `is_source` is set to `true` for the new source record

### Translation records

A translation record must:

- use an enabled locale
- reference an existing i18n group for the same collection
- be unique for its `i18n_group_id` and `locale`

Deleting the last record in a group removes the now-empty `_i18nGroups` record.

### Enabling i18n on existing collections

When i18n is enabled on a collection that already has records, existing records are backfilled:

- each existing record receives its own new i18n group
- each existing record receives the collection default locale
- each existing record is marked as `is_source`

## Locale-Aware Record Lists

The standard records list endpoint accepts `locale` and `fallback` query parameters for i18n-enabled collections:

```http
GET /api/collections/{collection}/records?locale=vi
```

Behavior:

- If `locale` is omitted, the collection default locale is used.
- `locale` must reference an enabled locale.
- `locale` and `fallback` are consumed by the i18n layer and removed before normal search parsing.
- Non-i18n collections ignore these parameters.

### Exact Locale

```http
GET /api/collections/posts/records?locale=vi
```

Returns only records whose hidden `locale` field is `vi`.

### Fallback

```http
GET /api/collections/posts/records?locale=ja&fallback=true
```

When `fallback=true` or `fallback=1`, the list returns:

- the requested locale record when it exists for a group
- otherwise the default locale record for that group

Fallback is skipped when the requested locale is already the collection default locale.

## Record View Metadata

The standard record view endpoint enriches i18n-enabled records with locale metadata:

```http
GET /api/collections/{collection}/records/{id}
```

Response fragment:

```json
{
  "id": "post_vi",
  "title": "Xin chao",
  "locale": "vi",
  "localeLinks": [
    { "id": "post_en", "locale": "en" },
    { "id": "post_vi", "locale": "vi" }
  ]
}
```

Notes:

- `locale` is unhidden on record view responses for i18n-enabled collections.
- `localeLinks` lists existing translations in the same group, ordered by locale.
- `i18n_group_id` and `is_source` remain hidden in normal API responses unless explicitly unhidden by backend code.

## Translation Metadata

Use the translations endpoint to inspect which enabled locales exist or are missing for a record group:

```http
GET /api/collections/{collection}/records/{id}/translations
```

Response: `200 OK`

```json
[
  {
    "locale": "en",
    "id": "post_en",
    "is_source": true,
    "is_missing": false
  },
  {
    "locale": "vi",
    "id": "post_vi",
    "is_source": false,
    "is_missing": false
  },
  {
    "locale": "ja",
    "is_source": false,
    "is_missing": true
  }
]
```

The response includes one item for every enabled locale.

## Create Translation

Create a missing translation from an existing source or translation record:

```http
POST /api/collections/{collection}/records/{id}/translations
Authorization: YOUR_SUPERUSER_TOKEN
Content-Type: application/json
```

```json
{
  "locale": "vi"
}
```

Response: `200 OK`

The created record:

- copies non-system field values from the source record
- uses the same `i18n_group_id` as the source record
- sets `locale` to the requested enabled locale
- sets `is_source` to `false`

If the collection has a unique `slug` field, PocketAdmin generates a locale-suffixed slug to avoid uniqueness conflicts.

Example:

```json
{
  "title": "Hello World",
  "slug": "hello-world-vi",
  "locale": "vi"
}
```

Slug generation behavior:

- the suffix is based on the requested locale, lowercased and normalized from `_` to `-`
- the base slug is truncated if needed to respect the slug field max length
- numeric suffixes are tried when the first candidate already exists
- a random suffix is used as a final fallback

## Translation Jobs

Translation jobs are tracked in the `_translationJobs` system collection.

They are useful for external or automated translation workflows. PocketAdmin stores the job state and emits an automation event when a job first reaches `finished`.

Job statuses:

- `pending`
- `running`
- `finished`
- `failed`
- `cancelled`

### Translation Job Model

```json
{
  "id": "REDACTED",
  "collectionRef": "posts_collection_id",
  "sourceRecordId": "post_en",
  "targetRecordId": "post_vi",
  "sourceLocale": "en",
  "targetLocale": "vi",
  "provider": "provider-name",
  "model": "model-name",
  "status": "pending",
  "error": "",
  "result": {},
  "created": "2026-01-01 00:00:00.000Z",
  "updated": "2026-01-01 00:00:00.000Z"
}
```

Validation:

- `collectionRef` is required and must reference a base or auth collection.
- `sourceRecordId` is required.
- `sourceLocale` and `targetLocale` are required valid locale codes.
- empty `status` defaults to `pending`.
- `status` must be one of the known job statuses.

### Translation Job APIs

All `/api/translation-jobs` endpoints require superuser authentication.

List jobs:

```http
GET /api/translation-jobs
Authorization: YOUR_SUPERUSER_TOKEN
```

Optional filters:

- `collection`: filters by `collectionRef`
- `status`: filters by job status

View job:

```http
GET /api/translation-jobs/{id}
Authorization: YOUR_SUPERUSER_TOKEN
```

Create job:

```http
POST /api/translation-jobs
Authorization: YOUR_SUPERUSER_TOKEN
Content-Type: application/json
```

```json
{
  "collectionRef": "posts_collection_id",
  "sourceRecordId": "post_en",
  "targetRecordId": "post_vi",
  "sourceLocale": "en",
  "targetLocale": "vi",
  "provider": "openai",
  "model": "example-model",
  "status": "pending"
}
```

Update job:

```http
PATCH /api/translation-jobs/{id}
Authorization: YOUR_SUPERUSER_TOKEN
Content-Type: application/json
```

```json
{
  "status": "finished",
  "result": {
    "fields": {
      "title": "Xin chao"
    }
  }
}
```

## Automation Triggers

i18n can queue automation runs for these trigger types:

| Trigger | When it runs |
|---|---|
| `i18n.translation_missing` | After an i18n record is created and enabled locales are missing from its group |
| `i18n.locale_published` | After a locale is created enabled or changed from disabled to enabled |
| `i18n.translation_updated` | After an i18n-enabled collection record is updated |
| `i18n.ai_translation_finished` | After a translation job changes to `finished` for the first time |

Common i18n automation payload values include:

- `i18n.collectionId`
- `i18n.collectionName`
- `i18n.groupId`
- `i18n.locale`
- `i18n.sourceLocale`
- `i18n.sourceRecordId`
- `i18n.targetLocale`
- `i18n.targetRecordId`
- `i18n.missingLocales`
- `i18n.translationTotal`
- `i18n.translationJobId`
- `i18n.provider`
- `i18n.model`

The exact keys depend on the trigger.

## Admin UI Behavior

The admin UI currently exposes i18n in three places:

- Settings > Languages and translations manages `/api/locales`.
- Collection options can enable `i18n`, choose `defaultLocale`, and select `localizedFields`.
- Record create/update modals show a locale switcher for existing records in i18n-enabled collections.

Record modal behavior:

- The locale switcher is hidden for new records.
- Existing records load `/api/locales` and `/api/collections/{collection}/records/{id}/translations`.
- Clicking an existing locale switches to that translation record.
- Clicking a missing locale creates a translation through `POST /api/collections/{collection}/records/{id}/translations`.
- Unsaved changes must be discarded before switching locale.

## Migration Helper

The Go API exposes `MigrateCollectionI18n` for enabling i18n on an existing collection and reporting the backfill impact.

Options:

```go
core.I18nMigrationOptions{
    CollectionNameOrId: "posts",
    DefaultLocale:      "en",
    LocalizedFields:    []string{"title", "body"},
    DryRun:             true,
}
```

Report:

```json
{
  "collectionId": "REDACTED",
  "collectionName": "posts",
  "defaultLocale": "en",
  "localizedFields": ["title", "body"],
  "recordsTotal": 10,
  "groupsToCreate": 10,
  "applied": false
}
```

Notes:

- `DryRun` returns the report without changing the collection.
- If `LocalizedFields` is empty, all non-system fields are selected.
- Applying the migration enables collection i18n and lets normal collection save hooks backfill existing records.

## Common Errors

Unknown or disabled locale:

```json
{
  "code": "validation_unknown_locale",
  "message": "Unknown or disabled locale."
}
```

Duplicate locale in a group:

```json
{
  "code": "validation_duplicated_i18n_locale",
  "message": "The translation locale already exists for this record group."
}
```

Missing or invalid group:

```json
{
  "code": "validation_invalid_i18n_group",
  "message": "Missing or invalid i18n group."
}
```

Default locale delete:

```json
{
  "code": "validation_default_locale_delete",
  "message": "The default locale cannot be deleted."
}
```
