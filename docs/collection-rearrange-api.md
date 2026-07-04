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

- `layout`: flattened compatibility fallback for fields shown in the record form
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
- `hidden`: collection field ids intentionally omitted from the record form

### Sections

`sections` is the primary section-aware layout format used by the current admin UI.

Each section represents one form area. Fields are assigned to a section by placing their grid item in that section's `layout` array. The order of `sections` controls the order of areas in the record form.

The top-level `layout` is kept as a flattened compatibility fallback. When writing section-aware data, keep `layout` equal to all `sections[].layout` items flattened in section order.

If `sections` is missing or empty, the admin UI treats the top-level `layout` as a legacy single-section layout with the default section id `main`. If both `sections` and `layout` are empty, fields are rendered in collection field order.

Section metadata rules:

- `sections[].id` should be unique within the collection layout.
- Empty `name` and `description` values are allowed.
- Section headings are shown in record forms only when there is more than one section or at least one section has a name or description.
- Empty sections can be saved and remain available as drop areas in the layout builder.

Notes:

- Field ids are used so layouts survive field renames.
- Unknown or deleted field ids are ignored by the admin UI when rendering the form.
- When `sections` exists, fields not found in `sections` or `hidden` are appended to the first normalized section.
- When `sections` exists, the admin UI uses section order and section-local layouts.
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
  -d '{"rearrange":{"layout":[{"id":"title_field_id","x":0,"y":0,"w":12,"h":1},{"id":"status_field_id","x":0,"y":0,"w":6,"h":1}],"sections":[{"id":"main","name":"Content","description":"Primary editorial fields.","layout":[{"id":"title_field_id","x":0,"y":0,"w":12,"h":1}]},{"id":"section_metadata","name":"Metadata","description":"Publishing state and classification.","layout":[{"id":"status_field_id","x":0,"y":0,"w":6,"h":1}]}],"hidden":["description_field_id"]}}' \
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
- saves `{ "sections": [...], "layout": [...], "hidden": [...] }` to `collection.rearrange`
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
