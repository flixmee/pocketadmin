# Show and Hide Record Table Columns

This document describes how PocketAdmin controls which collection fields appear as columns in the admin records table.

## Overview

Base collections can store an explicit list of displayed table fields in the `tableFields` collection property.

The setting:

- affects the records table in the admin UI
- is saved with the collection in the database
- is shared by users and browsers that load the collection
- uses stable field IDs, so field renames do not break the selection
- does not change record data, API rules, field visibility, or fields returned by the records API

The collection primary key is always displayed and is not included in `tableFields`.

## Configure Columns in the Admin UI

1. Open **Collections** and select a base collection.
2. Select **Collection settings**.
3. Open the **Options** tab.
4. Expand **Table fields**.
5. Check the fields that should appear in the records table.
6. Select **Save changes**.

The panel also provides:

- **Select all** to display every configurable field
- **Clear** to hide every optional field
- a selection badge such as `4/12 selected`

Clearing all fields still leaves the primary key column visible.

Only fields with a supported admin table view can be selected. Unsaved fields without a stable field ID are not available until the collection has been saved.

## Data Model

`tableFields` is an array of collection field IDs:

```json
{
  "id": "posts_collection_id",
  "name": "posts",
  "type": "base",
  "tableFields": [
    "title_field_id",
    "status_field_id",
    "created_field_id"
  ]
}
```

Use field IDs, not field names:

```json
{
  "fields": [
    {
      "id": "title_field_id",
      "name": "title",
      "type": "text"
    }
  ],
  "tableFields": ["title_field_id"]
}
```

Internally, the value is stored in the base collection options JSON in the `_collections` table:

```json
{
  "tableFields": ["title_field_id", "status_field_id"]
}
```

### Configured and Unconfigured States

The value has three meaningful states:

| Value | Behavior |
|---|---|
| missing or `null` | No database selection has been configured. The admin UI uses the legacy default/local preference behavior. |
| `[]` | The database selection is configured with no optional fields. Only the primary key is displayed. |
| `["field_id"]` | Only the primary key and the listed supported fields are displayed. |

Once `tableFields` is an array, including an empty array, the database selection is authoritative. Browser-local column history does not override it.

Unknown or deleted field IDs are ignored by the admin UI.

## Collection API

Collection endpoints require superuser authentication.

Send a valid superuser token:

```http
Authorization: YOUR_SUPERUSER_TOKEN
```

### Read the Current Selection

```http
GET /api/collections/{collection}
Authorization: YOUR_SUPERUSER_TOKEN
```

`collection` may be a collection ID or name.

Example response fragment:

```json
{
  "id": "posts_collection_id",
  "name": "posts",
  "type": "base",
  "fields": [
    {
      "id": "title_field_id",
      "name": "title",
      "type": "text"
    },
    {
      "id": "status_field_id",
      "name": "status",
      "type": "select"
    }
  ],
  "tableFields": ["title_field_id", "status_field_id"]
}
```

### Update the Selection

```http
PATCH /api/collections/{collection}
Content-Type: application/json
Authorization: YOUR_SUPERUSER_TOKEN
```

```json
{
  "tableFields": ["title_field_id", "status_field_id"]
}
```

To hide every optional column:

```json
{
  "tableFields": []
}
```

### curl

```bash
curl -X PATCH \
  -H 'Authorization: YOUR_SUPERUSER_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"tableFields":["title_field_id","status_field_id"]}' \
  'http://127.0.0.1:8090/api/collections/posts'
```

## Import and Export

`tableFields` is part of the normal base collection JSON representation. Collection export/import and preset payloads can include it alongside `fields`, `indexes`, `i18n`, and other collection properties.

Every referenced field ID must match the ID used by the corresponding field definition if the columns should appear after import.

## Legacy Browser Preferences

Collections without a configured `tableFields` array retain the previous table-column menu and browser-local column preferences.

After a database-backed selection is saved:

- the records table follows `tableFields`
- browser-local preferences no longer override the collection setting
- the legacy table-column menu is hidden
- future changes should be made from **Collection settings → Options → Table fields** or through the collection API

## Scope and Limitations

- The current collection Options panel exposes `tableFields` for base collections.
- The primary key column cannot be hidden.
- `tableFields` changes admin table presentation only.
- It does not change a field's `hidden` setting.
- It does not limit API response fields or provide a security boundary.
- It does not change record forms, record values, indexes, validation, or API rules.
