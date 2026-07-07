# Collection Rearrange API

This document describes the `rearrange` collection property currently available in PocketAdmin.

## Overview

`rearrange` stores admin UI presentation metadata for a collection.

The current admin UI uses it to persist the layout created from the Form layout builder. It does not change the collection field definitions, record form field order, record table columns, record values, API rules, or validation behavior.

The value is stored on the internal `_collections` table as JSON:

```sql
rearrange JSON DEFAULT "{}" NOT NULL
```

All collection create and update endpoints require superuser authentication.

## Authentication

Send a valid superuser token in the `Authorization` header:

```http
Authorization: YOUR_SUPERUSER_TOKEN
```

If the request is unauthenticated, the API returns `401`.

If the request is authenticated as a regular auth record instead of a superuser, the API returns `403`.

## Data Model

`rearrange` is a JSON object. Empty or unset rearrange data is represented as `{}`.

The form layout builder currently stores this shape:

```json
{
  "type": "sections",
  "layout": [
    {
      "id": "title_field_id",
      "x": 0,
      "y": 0,
      "w": 12,
      "h": 1
    },
    {
      "id": "email_field_id",
      "x": 0,
      "y": 0,
      "w": 6,
      "h": 1
    }
  ],
  "sections": [
    {
      "id": "main",
      "name": "Content",
      "description": "Primary editorial fields.",
      "layout": [
        {
          "id": "title_field_id",
          "x": 0,
          "y": 0,
          "w": 12,
          "h": 1
        }
      ]
    },
    {
      "id": "section_contact",
      "name": "Contact",
      "description": "Public contact details.",
      "layout": [
        {
          "id": "email_field_id",
          "x": 0,
          "y": 0,
          "w": 6,
          "h": 1
        }
      ]
    }
  ],
  "hidden": ["description_field_id"]
}
```

Properties:

- `type`: layout mode used by the admin UI; supported values are `sections` and `normal`
- `layout`: flattened compatibility fallback for fields shown in the layout metadata
- `layout[].id`: collection field id, not field name
- `layout[].x`: horizontal grid position
- `layout[].y`: vertical grid position
- `layout[].w`: field width in grid columns
- `layout[].h`: field height in grid rows
- `sections`: ordered form sections shown by the admin UI
- `sections[].id`: stable client-generated section id
- `sections[].name`: section heading text
- `sections[].description`: optional section helper text
- `sections[].layout`: section-local grid item definitions using the same item shape as `layout`
- `hidden`: collection field ids intentionally omitted from the form layout metadata

If `type` is missing or unknown, the admin UI treats the layout as `sections` for backward compatibility. The UI label for `sections` is "Sections and tabs".

### Layout Types

`sections` is the default layout type and preserves the existing section builder behavior. Sections are ordered top to bottom and can be added, renamed, described, moved, removed, and used as drag targets.

`normal` uses the same `sections` storage shape, but reserves two structural section ids:

- `main`: the main/left column
- `right`: the right column

In `normal` mode, `main` and `right` are fixed structural areas. The admin UI shows their labels but does not allow renaming, removing, or reordering them. Users can still add extra sections in normal mode; those sections stack in the main/left column before the right column and remain editable, removable, reorderable, and available as field drop targets.

Example normal layout:

```json
{
  "type": "normal",
  "layout": [
    {
      "id": "title_field_id",
      "x": 0,
      "y": 0,
      "w": 12,
      "h": 1
    },
    {
      "id": "status_field_id",
      "x": 0,
      "y": 0,
      "w": 12,
      "h": 1
    },
    {
      "id": "slug_field_id",
      "x": 0,
      "y": 0,
      "w": 12,
      "h": 1
    }
  ],
  "sections": [
    {
      "id": "main",
      "name": "Main",
      "description": "",
      "layout": [
        {
          "id": "title_field_id",
          "x": 0,
          "y": 0,
          "w": 12,
          "h": 1
        }
      ]
    },
    {
      "id": "section_metadata",
      "name": "Metadata",
      "description": "Publishing state and classification.",
      "layout": [
        {
          "id": "status_field_id",
          "x": 0,
          "y": 0,
          "w": 12,
          "h": 1
        }
      ]
    },
    {
      "id": "right",
      "name": "Right column",
      "description": "",
      "layout": [
        {
          "id": "slug_field_id",
          "x": 0,
          "y": 0,
          "w": 12,
          "h": 1
        }
      ]
    }
  ],
  "hidden": []
}
```

### Sections

`sections` is the primary section-aware layout format used by the current admin UI.

Each section represents one form layout area. Fields are assigned to a section by placing their grid item in that section's `layout` array. The order of `sections` controls the order of areas in the layout metadata.

The top-level `layout` is kept as a flattened compatibility fallback. When writing section-aware data, keep `layout` equal to all `sections[].layout` items flattened in section order.

If `sections` is missing or empty, the admin UI treats the top-level `layout` as a legacy single-section layout with the default section id `main`. If both `sections` and `layout` are empty, fields are normalized in collection field order.

Section metadata rules:

- `sections[].id` should be unique within the collection layout.
- Empty `name` and `description` values are allowed.
- In `normal` mode, `main` and `right` are reserved structural ids.
- Section headings are meaningful only when there is more than one section or at least one section has a name or description.
- Empty sections can be saved and remain available as drop areas in the layout builder.

Notes:

- Field ids are used so layouts survive field renames.
- Unknown or deleted field ids are ignored by the admin UI when normalizing the layout.
- When `sections` exists, fields not found in `sections` or `hidden` are appended to the first normalized section.
- When `sections` exists, the admin UI uses section order and section-local layouts.
- When `type` is `normal`, the admin UI preserves extra non-structural sections, keeps the `right` section as the side column, and normalizes unknown section ids as main-column sections.
- The admin UI clamps grid coordinates and sizes before rendering.

## Collection APIs

`rearrange` is part of the standard collection payloads returned by:

- `GET /api/collections`
- `GET /api/collections/{collection}`
- `POST /api/collections`
- `PATCH /api/collections/{collection}`
- `PUT /api/collections/import`

## Read Rearrange Data

### Request

```http
GET /api/collections/{collection}
Authorization: YOUR_SUPERUSER_TOKEN
```

Path params:

- `collection`: collection id or name

### Response

Status: `200 OK`

Example collection fragment:

```json
{
  "id": "REDACTED",
  "name": "posts",
  "type": "base",
  "rearrange": {
    "type": "sections",
    "layout": [
      {
        "id": "title_field_id",
        "x": 0,
        "y": 0,
        "w": 12,
        "h": 1
      },
      {
        "id": "email_field_id",
        "x": 0,
        "y": 0,
        "w": 6,
        "h": 1
      }
    ],
    "sections": [
      {
        "id": "main",
        "name": "Content",
        "description": "Primary editorial fields.",
        "layout": [
          {
            "id": "title_field_id",
            "x": 0,
            "y": 0,
            "w": 12,
            "h": 1
          }
        ]
      },
      {
        "id": "section_contact",
        "name": "Contact",
        "description": "Public contact details.",
        "layout": [
          {
            "id": "email_field_id",
            "x": 0,
            "y": 0,
            "w": 6,
            "h": 1
          }
        ]
      }
    ],
    "hidden": ["description_field_id"]
  }
}
```

### curl

```bash
curl \
  -H 'Authorization: YOUR_SUPERUSER_TOKEN' \
  'http://127.0.0.1:8090/api/collections/posts'
```

## Update Rearrange Data

### Request

```http
PATCH /api/collections/{collection}
Content-Type: application/json
Authorization: YOUR_SUPERUSER_TOKEN
```

Path params:

- `collection`: collection id or name

Body fragment:

```json
{
  "rearrange": {
    "type": "sections",
    "layout": [
      {
        "id": "title_field_id",
        "x": 0,
        "y": 0,
        "w": 12,
        "h": 1
      },
      {
        "id": "status_field_id",
        "x": 0,
        "y": 0,
        "w": 6,
        "h": 1
      }
    ],
    "sections": [
      {
        "id": "main",
        "name": "Content",
        "description": "Primary editorial fields.",
        "layout": [
          {
            "id": "title_field_id",
            "x": 0,
            "y": 0,
            "w": 12,
            "h": 1
          }
        ]
      },
      {
        "id": "section_metadata",
        "name": "Metadata",
        "description": "Publishing state and classification.",
        "layout": [
          {
            "id": "status_field_id",
            "x": 0,
            "y": 0,
            "w": 6,
            "h": 1
          }
        ]
      }
    ],
    "hidden": ["description_field_id"]
  }
}
```

### Response

Status: `200 OK`

Returns the updated collection model, including the saved `rearrange` value.

### curl

```bash
curl -X PATCH \
  -H 'Authorization: YOUR_SUPERUSER_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"rearrange":{"type":"sections","layout":[{"id":"title_field_id","x":0,"y":0,"w":12,"h":1},{"id":"status_field_id","x":0,"y":0,"w":6,"h":1}],"sections":[{"id":"main","name":"Content","description":"Primary editorial fields.","layout":[{"id":"title_field_id","x":0,"y":0,"w":12,"h":1}]},{"id":"section_metadata","name":"Metadata","description":"Publishing state and classification.","layout":[{"id":"status_field_id","x":0,"y":0,"w":6,"h":1}]}],"hidden":["description_field_id"]}}' \
  'http://127.0.0.1:8090/api/collections/posts'
```

## Reset Rearrange Data

To clear all stored form layout metadata, update the collection with an empty object:

```json
{
  "rearrange": {}
}
```

After reset, the admin UI falls back to the default form layout based on the collection field order.

## Admin UI Behavior

The record form layout modal:

- reads `collection.rearrange` first
- falls back to the old local browser history value only when `collection.rearrange` is empty
- saves `{ "type": "...", "sections": [...], "layout": [...], "hidden": [...] }` to `collection.rearrange`
- lets users choose `normal` or `sections` mode from the layout type selector
- renders normal mode with a fixed `main` area, a fixed `right` column, and optional extra sections in the main/left column
- lets fields move between sections by dragging across section areas
- appends newly discovered fields to the first section when they are not already present in `sections` or `hidden`
- clears the old local browser history value after a successful collection save
- updates the in-memory collection store after a successful save

The record create/update modal currently renders fields in collection field order and does not apply `rearrange` layout metadata.

## Notes

- `rearrange` is collection-level metadata and is not stored on records.
- `rearrange` does not affect record API response shapes.
- `rearrange` does not reorder `collection.fields`.
- `hidden` in `rearrange` is separate from a field's own `hidden` flag.
