# Collection Groups API

This document describes the collection-group related API behavior currently available in PocketAdmin.

## Overview

Collection groups are stored as the `collectionGroup` string property on a collection.

There is also an internal `_collection_groups` registry table used to populate the selectable list of existing groups in the admin UI.

All collection-group endpoints are mounted under:

```text
/api/collections/meta/groups
```

All endpoints in this document require superuser authentication.

## Authentication

Send a valid superuser token in the `Authorization` header:

```http
Authorization: YOUR_SUPERUSER_TOKEN
```

If the request is unauthenticated, the API returns `401`.

If the request is authenticated as a regular auth record instead of a superuser, the API returns `403`.

## Data Model

Collection groups are plain strings.

Normalization rules:

- Leading and trailing whitespace is trimmed.
- Empty values are treated as ungrouped.
- Group names are not stored as IDs or foreign keys.

## Create a Group

There is currently no dedicated `POST /api/collections/meta/groups` endpoint.

A group is created implicitly when you create or update a collection with a non-empty `collectionGroup` value.

Example:

```http
POST /api/collections
Content-Type: application/json
Authorization: YOUR_SUPERUSER_TOKEN
```

```json
{
  "name": "posts",
  "type": "base",
  "collectionGroup": "Content",
  "fields": [
    {
      "type": "text",
      "id": "title12345",
      "name": "title"
    }
  ]
}
```

Successful response:

```json
{
  "id": "REDACTED",
  "name": "posts",
  "type": "base",
  "collectionGroup": "Content"
}
```

The same behavior also applies to:

- `PATCH /api/collections/{collection}`
- `PUT /api/collections/import`

## List Groups

Returns all registered collection group names sorted alphabetically.

### Request

```http
GET /api/collections/meta/groups
Authorization: YOUR_SUPERUSER_TOKEN
```

### Response

Status: `200 OK`

```json
[
  "Content",
  "Primary"
]
```

### curl

```bash
curl \
  -H 'Authorization: YOUR_SUPERUSER_TOKEN' \
  'http://127.0.0.1:8090/api/collections/meta/groups'
```

## Rename a Group

Renames a registered group and updates all collections currently assigned to that group.

### Request

```http
PATCH /api/collections/meta/groups/{name}
Content-Type: application/json
Authorization: YOUR_SUPERUSER_TOKEN
```

Path params:

- `name`: the current group name

Body:

```json
{
  "name": "Primary"
}
```

### Response

Status: `200 OK`

Returns the full updated list of registered groups.

```json
[
  "Primary"
]
```

### Notes

- Both the path value and body `name` are trimmed before processing.
- Renaming a group updates every collection whose `collectionGroup` matches the old name.
- If the old and new names are the same after trimming, the API keeps the group registered and returns success.

### curl

```bash
curl -X PATCH \
  -H 'Authorization: YOUR_SUPERUSER_TOKEN' \
  -H 'Content-Type: application/json' \
  -d '{"name":"Primary"}' \
  'http://127.0.0.1:8090/api/collections/meta/groups/Content'
```

## Delete a Group

Deletes a registered group and removes that group assignment from all collections using it.

### Request

```http
DELETE /api/collections/meta/groups/{name}
Authorization: YOUR_SUPERUSER_TOKEN
```

Path params:

- `name`: the group name to remove

### Response

Status: `204 No Content`

### Notes

- Deleting a group does not delete any collections.
- Affected collections remain in place, but their `collectionGroup` becomes an empty string.

### curl

```bash
curl -X DELETE \
  -H 'Authorization: YOUR_SUPERUSER_TOKEN' \
  'http://127.0.0.1:8090/api/collections/meta/groups/Content'
```

## Error Responses

Typical error shapes:

```json
{
  "status": 401,
  "message": "Missing or invalid authentication token.",
  "data": {}
}
```

```json
{
  "status": 403,
  "message": "Only superusers can perform this action.",
  "data": {}
}
```

```json
{
  "status": 400,
  "message": "Failed to rename collection group.",
  "data": {}
}
```

## Related Collection APIs

The `collectionGroup` field is also part of the standard collection payloads returned by:

- `GET /api/collections`
- `GET /api/collections/{collection}`
- `POST /api/collections`
- `PATCH /api/collections/{collection}`
- `PUT /api/collections/import`

Example collection fragment:

```json
{
  "id": "REDACTED",
  "name": "posts",
  "type": "base",
  "system": false,
  "collectionGroup": "Content"
}
```
