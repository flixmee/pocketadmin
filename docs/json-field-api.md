# JSON Field API

This document describes the current API behavior for the `json` field type in PocketAdmin.

It covers:

- collection schema payloads for JSON fields
- record create/update payload behavior
- string normalization rules for JSON field inputs
- optional JSON Schema validation
- typical validation errors

## Overview

PocketAdmin stores JSON field values as JSON in the database and exposes them through the standard collection and record APIs.

The JSON field supports:

- arbitrary JSON values
- optional `required` validation
- optional `maxSize` validation
- optional `jsonSchema` validation using JSON Schema draft-07

JSON Schema validation is enforced on record create/update in the backend. It is not implemented as a hook that must be registered manually.

## Field Definition

A collection field of type `json` uses the standard field payload shape with these JSON-specific properties:

- `maxSize`: maximum allowed serialized JSON size in bytes
- `required`: rejects empty JSON values
- `jsonSchema`: optional JSON Schema definition string

Example collection field definition:

```json
{
  "type": "json",
  "id": "json1234567",
  "name": "metadata",
  "system": false,
  "required": false,
  "maxSize": 1048576,
  "jsonSchema": "{\"type\":\"object\",\"properties\":{\"name\":{\"type\":\"string\"}},\"required\":[\"name\"]}"
}
```

Notes:

- If `maxSize` is `0`, the default limit is about `1MB`.
- `jsonSchema` is stored as a string in the field definition payload.
- Invalid schema definitions are rejected when the collection is created or updated.

## Collection APIs

The JSON field is part of the standard collection endpoints, for example:

- `GET /api/collections`
- `GET /api/collections/{collection}`
- `POST /api/collections`
- `PATCH /api/collections/{collection}`
- `PUT /api/collections/import`

Example create/update payload fragment:

```json
{
  "name": "products",
  "type": "base",
  "fields": [
    {
      "type": "json",
      "id": "meta123456",
      "name": "metadata",
      "required": false,
      "maxSize": 4096,
      "jsonSchema": "{\"type\":\"object\",\"properties\":{\"sku\":{\"type\":\"string\"}},\"required\":[\"sku\"]}"
    }
  ]
}
```

## Record APIs

JSON field values are sent through the standard record endpoints, for example:

- `POST /api/collections/{collection}/records`
- `PATCH /api/collections/{collection}/records/{id}`
- `PUT /api/collections/{collection}/records/{id}`

### `application/json` requests

When the request body is JSON, send the field value as a normal JSON value.

Example:

```http
POST /api/collections/products/records
Content-Type: application/json
Authorization: YOUR_AUTH_TOKEN
```

```json
{
  "title": "Chair",
  "metadata": {
    "sku": "CHAIR-001",
    "dimensions": {
      "width": 40,
      "height": 80
    },
    "tags": ["wood", "indoor"]
  }
}
```

### `multipart/form-data` requests

When the request body is multipart form data, the JSON field may arrive as a plain string. To support that, PocketAdmin normalizes string inputs before parsing them as JSON.

## String Normalization Rules

When a JSON field value is submitted as a plain string, these normalization rules apply:

- `"true"` becomes JSON `true`
- `"false"` becomes JSON `false`
- `"null"` becomes JSON `null`
- numeric strings become JSON numbers
- `"[1,2,3]"` becomes JSON array `[1,2,3]`
- `"{\"a\":1}"` becomes JSON object `{"a":1}`
- already double-quoted strings are kept as-is
- any other string, including the empty string, is converted into a JSON string

Examples:

| Submitted value | Stored JSON value |
|---|---|
| `true` | `true` |
| `123` | `123` |
| `{"a":1}` | `{"a":1}` |
| `hello` | `"hello"` |
| `` | `""` |

Example multipart request:

```bash
curl -X POST \
  -H 'Authorization: YOUR_AUTH_TOKEN' \
  -F 'title=Chair' \
  -F 'metadata={"sku":"CHAIR-001","active":true}' \
  'http://127.0.0.1:8090/api/collections/products/records'
```

If you want to avoid plain-string normalization behavior, send an object or array as JSON in an `application/json` request.

## `required` Behavior

The JSON field `required` flag rejects empty JSON values.

The following values are treated as empty:

- `null`
- `""`
- `[]`
- `{}`
- missing/zero raw value

This means:

- a non-required JSON field may be `null`, `""`, `[]`, or `{}`
- a required JSON field rejects those values

## JSON Schema Validation

If `jsonSchema` is set on the field, non-empty JSON values are validated against it.

Behavior:

- schema validation runs during record create/update
- schema validation is skipped for `null`
- schema validation is skipped for `""`
- schema validation is skipped for empty raw values
- existing records are not retroactively validated when a schema is added

Example field schema:

```json
{
  "type": "json",
  "id": "meta123456",
  "name": "metadata",
  "jsonSchema": "{\"type\":\"object\",\"properties\":{\"sku\":{\"type\":\"string\"},\"price\":{\"type\":\"number\",\"minimum\":0}},\"required\":[\"sku\"]}"
}
```

Example valid record payload:

```json
{
  "metadata": {
    "sku": "CHAIR-001",
    "price": 199.99
  }
}
```

Example invalid record payload:

```json
{
  "metadata": {
    "price": -10
  }
}
```

The payload above may fail because:

- `sku` is missing
- `price` violates the schema minimum

## Supported Schema Standard

The backend compiles and validates schemas using JSON Schema draft-07.

Important constraints currently enforced in PocketAdmin:

- maximum schema size: `50KB`
- maximum schema nesting depth: `10`

If the schema string cannot be compiled, the collection update fails.

## Error Responses

### Invalid JSON value

If the field value is not valid JSON after normalization/parsing:

```json
{
  "status": 400,
  "message": "Failed to create record.",
  "data": {
    "metadata": {
      "code": "validation_invalid_json",
      "message": "Must be a valid json value"
    }
  }
}
```

### JSON size limit exceeded

If the serialized JSON value is larger than the field `maxSize`:

```json
{
  "status": 400,
  "message": "Failed to create record.",
  "data": {
    "metadata": {
      "code": "validation_json_size_limit",
      "message": "The maximum allowed JSON size is 4096 bytes"
    }
  }
}
```

### JSON Schema validation failure

If the value does not satisfy `jsonSchema`:

```json
{
  "status": 400,
  "message": "Failed to create record.",
  "data": {
    "metadata": {
      "code": "validation_json_schema",
      "message": "JSON schema validation failed: sku: missing property 'sku'"
    }
  }
}
```

For nested values, the backend includes the first failing instance path when available.

### Invalid field schema configuration

If the collection field `jsonSchema` itself is invalid, collection create/update fails with a field-settings validation error.

Typical error codes include:

- `validation_json_schema_compile`
- `validation_json_schema_too_large`
- settings validation errors for malformed schema content

## Response Shape

Record responses return the JSON field as a normal JSON value.

Example:

```json
{
  "id": "REDACTED",
  "collectionId": "REDACTED",
  "collectionName": "products",
  "metadata": {
    "sku": "CHAIR-001",
    "dimensions": {
      "width": 40,
      "height": 80
    },
    "tags": ["wood", "indoor"]
  }
}
```

## Notes

- Hidden JSON fields follow the same hidden-field API rules as other field types.
- JSON Schema validation applies only to record writes, not to historical data already stored before the schema was added.
- The admin UI may render a schema-based form for certain supported schema shapes, but API clients can still submit any value that passes backend validation.
