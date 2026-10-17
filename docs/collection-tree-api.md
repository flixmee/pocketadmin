# Collection Tree API

This document describes the collection tree option currently available in PocketAdmin.

It covers:

- the base collection `use_tree` option
- the `order` field requirement and its validation error
- admin UI behavior when enabling tree mode
- the Blog and E-commerce preset category fields that are tree-ready

## Overview

Base collections have a `use_tree` boolean option. When it is enabled, the collection records are meant to be presented as a tree, with siblings sorted by a number field named `order`.

`use_tree` is currently a stored flag only. The API validates and returns it, but the admin UI does not render records as a tree yet.

Only `base` collections support `use_tree`. The option is ignored for `auth` and `view` collections.

## Data Model

`use_tree` is a top-level property of the collection payload, next to the other base options such as `i18n`:

```json
{
  "id": "REDACTED",
  "name": "categories",
  "type": "base",
  "use_tree": true,
  "i18n": {
    "enabled": false,
    "defaultLocale": "",
    "localizedFields": []
  },
  "fields": [
    {
      "id": "REDACTED",
      "name": "order",
      "type": "number",
      "onlyInt": true,
      "min": 0
    }
  ]
}
```

Rules:

- The default value is `false`.
- New base collection scaffolds (`GET /api/collections/meta/scaffolds`) include `"use_tree": false`.
- When `use_tree` is `true`, the collection must have a field named exactly `order` with type `number`.
- The `order` field may have any number options (`onlyInt`, `min`, `max`, `required`, etc.).
- Turning `use_tree` off has no requirements.

The required field name is exposed in Go as `core.CollectionTreeOrderFieldName`.

## Enable Tree Mode

Tree mode is enabled through the standard collection endpoints. All of them require superuser authentication.

- `POST /api/collections`
- `PATCH /api/collections/{collection}`
- `PUT /api/collections/import`

### Request

```http
POST /api/collections
Content-Type: application/json
Authorization: YOUR_SUPERUSER_TOKEN
```

```json
{
  "name": "categories",
  "type": "base",
  "use_tree": true,
  "fields": [
    {
      "name": "name",
      "type": "text"
    },
    {
      "name": "order",
      "type": "number",
      "onlyInt": true,
      "min": 0
    },
    {
      "name": "parent",
      "type": "relation",
      "collectionId": "SAME_COLLECTION_ID",
      "maxSelect": 1
    }
  ]
}
```

The `parent` relation is not required by the validation, but a tree normally needs one to link a record to its parent record in the same collection.

### Response

Status: `200 OK`

Returns the saved collection, including `"use_tree": true`.

### curl

```bash
curl -X PATCH \
  -H 'Authorization: YOUR_SUPERUSER_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"use_tree":true}' \
  'http://127.0.0.1:8090/api/collections/categories'
```

## Error Responses

If `use_tree` is `true` and the collection has no `number` field named `order`, the save fails with `400`. This also happens when an `order` field exists with a different type, and when the `order` field is removed from a collection that already has tree mode on.

```json
{
  "status": 400,
  "message": "Failed to create collection.",
  "data": {
    "use_tree": {
      "code": "validation_tree_missing_order_field",
      "message": "Tree mode requires a number field named \"order\".",
      "params": {
        "fieldName": "order"
      }
    }
  }
}
```

The `message` is `Failed to update collection.` for `PATCH` requests.

## Admin UI Behavior

The switch is in the **Tree** section at the top of a base collection's **Options** tab.

| Situation | What happens |
|---|---|
| Turn on, `order` number field exists | Tree mode turns on |
| Turn on, no `order` field | A dialog asks to add it. **Add field** inserts an `order` number field (whole numbers, min `0`) before the `created`/`updated` fields and turns tree mode on. **Cancel** leaves it off. |
| Turn on, `order` exists but is not a number | The switch stays off and an error toast asks to change or rename the field |
| Tree mode on, `order` later removed | A warning with an **Add field** button is shown. Saving returns the `use_tree` error, displayed under the switch. |

Fields marked for deletion in the editor are treated as missing.

## Tree-Ready Preset Categories

The Blog and E-commerce collection presets (version `1.1.0`) include category collections with the fields a tree uses. Tree mode is not turned on by the presets; enable it after import if needed.

### Blog: `categories`

| Field | Type | Settings |
|---|---|---|
| `image` | `media` | 1 file; `image/jpeg`, `image/png`, `image/webp`, `image/gif` |
| `order` | `number` | `onlyInt: true`, `min: 0` |
| `parent` | `relation` | points to `categories`, `maxSelect: 1` |

### E-commerce: `product_categories`

| Field | Type | Settings |
|---|---|---|
| `image` | `media` | 1 file; `image/jpeg`, `image/png`, `image/webp`, `image/gif` |
| `order` | `number` | `onlyInt: true`, `min: 0` |
| `parent` | `relation` | points to `product_categories`, `maxSelect: 1` |

When a preset is imported with a prefix, the self-relation is resolved to the prefixed collection (for example `site_categories.parent` points to `site_categories`).

## Implementation Notes

- Option and validation: `core/collection_model_base_options.go` (`UseTree`, `validateTree`).
- Validation entry point: `validateOptions` in `core/collection_validate.go`.
- Admin UI: `ui/src/collections/collectionI18nOptionsTab.js`.
- Presets: `core/presets/blog.json`, `core/presets/ecommerce.json`.
- Tests: `core/collection_validate_test.go`, `core/presets/preset_test.go`.
