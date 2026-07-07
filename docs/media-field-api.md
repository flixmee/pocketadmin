# Media Field API

This document describes the current media-library and `media` field behavior in PocketAdmin.

It covers:

- the `_medias` system collection
- media file and folder records
- collection schema payloads for `media` fields
- record create/update payload behavior
- value normalization rules
- validation behavior
- admin UI picker behavior

## Overview

PocketAdmin has two related media concepts:

- `_medias`: a system collection used as the shared media library.
- `media`: a collection field type that stores references to items from `_medias`.

The `media` field is different from the existing `file` field:

- `file` uploads files directly onto the owning record.
- `media` references files already stored in the shared `_medias` library.

The admin UI includes a Media page at `#/media` and a record-editor picker for `media` fields.

## `_medias` Collection

The `_medias` system collection stores both folders and files.

Important fields:

- `name`: display name and path segment
- `kind`: either `file` or `folder`
- `parent`: parent folder record id, or empty string for root-level items
- `file`: uploaded file value for `file` records
- `mime`: detected mime type for `file` records
- `size`: detected byte size for `file` records
- `created`: creation timestamp
- `updated`: update timestamp

Rules:

- `kind` is required and must be `file` or `folder`.
- `name` cannot contain `/` or `\`.
- `parent`, when set, must point to an existing folder in `_medias`.
- Folders cannot have an uploaded file.
- Files must have an uploaded file.
- When a file is uploaded without an explicit `name`, the original filename is used.
- File `mime` and `size` metadata are populated from the upload or stored file.

The collection has a unique index on `parent, name`, so two media items with the same parent cannot share the same name.

## Media Paths and File URLs

Media references can be resolved in two ways.

### Path References

A media path is built from folder and item names:

```text
/docs/hero.png
```

Normalization rules:

- leading and trailing whitespace is trimmed
- `\` path separators are converted to `/`
- missing leading `/` is added
- path segments are cleaned with normal path-cleaning rules
- empty paths and `/` are treated as empty values

Examples:

| Submitted value | Normalized value |
|---|---|
| `docs/hero.png` | `/docs/hero.png` |
| `docs\hero.png` | `/docs/hero.png` |
| ` /docs//hero.png ` | `/docs/hero.png` |

### File URL References

The admin media picker stores selected files as normalized file URLs:

```text
/api/files/_medias/abc123/hero.png
```

Absolute file URLs are also supported:

```text
https://example.com/api/files/_medias/abc123/hero.png
```

Normalization rules:

- only URLs shaped like `/api/files/{collection}/{recordId}/{filename}` are treated as file URLs
- query strings and fragments are removed
- relative file URLs remain relative
- absolute file URLs remain absolute

Example:

| Submitted value | Normalized value |
|---|---|
| `/api/files/_medias/abc123/hero.png?token=abc` | `/api/files/_medias/abc123/hero.png` |
| `https://example.com/api/files/_medias/abc123/hero.png?download=1` | `https://example.com/api/files/_medias/abc123/hero.png` |

## Field Definition

A collection field of type `media` uses the standard field payload shape with these media-specific properties:

- `maxSelect`: maximum number of selected media values
- `mimeTypes`: optional list of allowed file mime types
- `allowFolders`: whether folder paths are valid values
- `required`: whether the field must contain at least one value
- `help`: optional admin UI help text

Example single-value field:

```json
{
  "type": "media",
  "id": "media12345",
  "name": "hero",
  "system": false,
  "required": false,
  "maxSelect": 1,
  "mimeTypes": ["image/png", "image/jpeg"],
  "allowFolders": false,
  "help": "Select the hero image from the media library."
}
```

Example multiple-value field:

```json
{
  "type": "media",
  "id": "gallery123",
  "name": "gallery",
  "maxSelect": 10,
  "mimeTypes": ["image/png", "image/jpeg", "image/webp"],
  "allowFolders": false
}
```

Notes:

- If `maxSelect` is `0` or `1`, the field stores a single string value.
- If `maxSelect` is greater than `1`, the field stores a JSON array of strings.
- `mimeTypes` only applies to file records.
- `allowFolders` allows folder path values in backend validation. The current admin picker navigates folders but selects files.

## Collection APIs

The `media` field is part of the standard collection endpoints, for example:

- `GET /api/collections`
- `GET /api/collections/{collection}`
- `POST /api/collections`
- `PATCH /api/collections/{collection}`
- `PUT /api/collections/import`

Example create/update payload fragment:

```json
{
  "name": "posts",
  "type": "base",
  "fields": [
    {
      "type": "media",
      "id": "hero12345",
      "name": "hero",
      "required": false,
      "maxSelect": 1,
      "mimeTypes": ["image/png", "image/jpeg"],
      "allowFolders": false
    }
  ]
}
```

## Media Library APIs

Media library items use the standard record APIs for the `_medias` system collection.

All examples require a superuser token.

### Create a Folder

```http
POST /api/collections/_medias/records
Content-Type: application/json
Authorization: YOUR_SUPERUSER_TOKEN
```

```json
{
  "kind": "folder",
  "name": "docs",
  "parent": ""
}
```

### Upload a File

```http
POST /api/collections/_medias/records
Content-Type: multipart/form-data
Authorization: YOUR_SUPERUSER_TOKEN
```

```bash
curl -X POST \
  -H 'Authorization: YOUR_SUPERUSER_TOKEN' \
  -F 'kind=file' \
  -F 'parent=FOLDER_RECORD_ID' \
  -F 'file=@hero.png' \
  'http://127.0.0.1:8090/api/collections/_medias/records'
```

If `name` is omitted for a file upload, PocketAdmin uses the original uploaded filename.

### List Media Items

```http
GET /api/collections/_medias/records?filter=parent%3D%22%22&sort=-kind,name
Authorization: YOUR_SUPERUSER_TOKEN
```

Typical UI list queries filter by `parent` and sort folders before files:

```text
sort=-kind,name
```

### Download a Media File

Use the standard file endpoint:

```http
GET /api/files/_medias/{recordId}/{filename}
```

If the file requires a file token, create one with:

```http
POST /api/files/token
Authorization: YOUR_SUPERUSER_TOKEN
```

## Record APIs

Media field values are sent through the standard record endpoints, for example:

- `POST /api/collections/{collection}/records`
- `PATCH /api/collections/{collection}/records/{id}`
- `PUT /api/collections/{collection}/records/{id}`

### Single-Value Fields

For `maxSelect` `0` or `1`, send a string.

```json
{
  "title": "Home",
  "hero": "/api/files/_medias/abc123/hero.png"
}
```

Path values are also accepted:

```json
{
  "title": "Home",
  "hero": "/docs/hero.png"
}
```

If multiple values are submitted to a single-value field, PocketAdmin keeps the last normalized value.

### Multiple-Value Fields

For `maxSelect` greater than `1`, send an array of strings.

```json
{
  "title": "Home",
  "gallery": [
    "/api/files/_medias/abc123/hero.png",
    "/api/files/_medias/def456/details.png"
  ]
}
```

Duplicate and empty values are removed during normalization.

## Validation Behavior

On record create/update, a `media` field validates the normalized media references.

Behavior:

- Empty values are allowed unless `required` is true.
- `required` rejects empty strings and empty arrays.
- `maxSelect` limits the number of selected values.
- Each value must resolve to an existing `_medias` record.
- File URLs must point to the `_medias` collection and match the referenced record's stored filename.
- Path references are resolved through `_medias.parent` and `_medias.name`.
- Folder records are rejected unless `allowFolders` is true.
- File records are rejected when `mimeTypes` is set and the resolved media record's `mime` is not listed.

When an existing record already contains a stale media value, unchanged stale values are tolerated during validation. If the media field value itself is changed, new unresolved values are rejected.

Typical validation errors include:

```json
{
  "status": 400,
  "message": "Failed to create record.",
  "data": {
    "hero": {
      "code": "validation_missing_media_path",
      "message": "Failed to resolve the selected media value."
    }
  }
}
```

```json
{
  "status": 400,
  "message": "Failed to create record.",
  "data": {
    "hero": {
      "code": "validation_invalid_media_type",
      "message": "The selected media type is not allowed."
    }
  }
}
```

## Admin UI Behavior

The record editor renders `media` fields as a selected-media list with thumbnails and an "Open media picker" button.

The media picker supports:

- browsing folders
- searching by name
- list and grid view modes
- selecting one or more files
- upload from local files
- drag-and-drop upload
- upload from remote image URLs
- mime-type filtering based on the field settings

Selected values from the picker are stored as normalized file URLs such as:

```text
/api/files/_medias/abc123/hero.png
```

Record list/detail views render valid file URL values as thumbnails. Non-file-URL path values are still displayed, but the UI marks them as legacy-style values because they cannot be thumbnailed directly from the URL shape.

## Practical Notes

- Use `media` when several records should reference shared library files.
- Use `file` when the file belongs only to a single record.
- Renaming or moving media items changes their path references, but stored record values are not automatically rewritten.
- File URL references survive folder renames because they point to the `_medias` record id and stored filename.
- Deleting a media item does not automatically clear `media` field values from other records.
