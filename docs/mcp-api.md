# MCP API

This document describes the Model Context Protocol (MCP) server built into PocketAdmin.

For a step-by-step setup guide (creating keys, connecting Claude, Cursor and VS Code, security tips) see [mcp-guide.md](./mcp-guide.md).

It covers:

- the MCP endpoint and how clients authenticate
- MCP API keys, their permissions and collection allowlist
- the available tools
- the superuser API used by the admin UI to manage keys

## Overview

PocketAdmin exposes an MCP server at `POST /api/mcp` so that AI assistants (Claude, Cursor, VS Code, etc.) can read and modify collections and records.

The server implements the MCP Streamable HTTP transport in stateless mode:

- every JSON-RPC message (or batch) is sent with `POST /api/mcp`
- responses are always plain `application/json` (no SSE streams, no `Mcp-Session-Id`)
- notifications and client responses are acknowledged with `202 Accepted`
- `GET` and `DELETE` return `405 Method Not Allowed`

Supported protocol versions: `2025-11-25`, `2025-06-18`, `2025-03-26`, `2024-11-05`. The server echoes the version requested in `initialize` when supported, otherwise it answers with the newest one.

Supported methods: `initialize`, `ping`, `tools/list`, `tools/call` and `notifications/*`.

## Authentication

Every request must include an MCP API key, either as:

```
Authorization: Bearer pbmcp_...
```

or:

```
X-API-Key: pbmcp_...
```

Superuser/auth record tokens are not accepted by the MCP endpoint. Missing, unknown, disabled or expired keys return `401`.

Example client configuration:

```json
{
  "mcpServers": {
    "pocketadmin": {
      "type": "http",
      "url": "https://example.com/api/mcp",
      "headers": {
        "Authorization": "Bearer pbmcp_..."
      }
    }
  }
}
```

## API Keys

Keys are managed from **Settings > MCP** in the admin UI.

Keys are stored in the internal `_mcpKeys` table (it is not a collection and it is not reachable through the records API). Only a SHA-256 hash of the key and a short display prefix are stored. The full key is returned only once, when the key is created or regenerated.

Each key has:

| Property      | Description                                                                                       |
| ------------- | ------------------------------------------------------------------------------------------------- |
| `name`        | Display name (required, max 100 chars).                                                           |
| `permissions` | Any of `collections.read`, `collections.write`, `records.read`, `records.write`, `media.read`, `media.write`. |
| `collections` | Allowed collection ids. Empty means all non-system collections.                                   |
| `active`      | Disabled keys are rejected.                                                                       |
| `expiresAt`   | Optional expiration date. Empty means the key never expires.                                      |
| `lastUsedAt`  | Updated on use (at most once per minute).                                                         |

Access rules:

- system collections (e.g. `_superusers`) are never accessible
- `tools/list` returns only the tools allowed by the key permissions
- keys with a collection allowlist can't create new collections
- relation `expand` skips collections that are not accessible with the key
- hidden fields (e.g. `password`, `tokenKey`) are never returned; auth record emails are always returned
- record and collection writes go through `app.Save` / `app.Delete`, so model hooks and validations apply, but API rules are not evaluated (the key permissions replace them)

## Tools

| Tool                | Permission          | Arguments                                                                  |
| ------------------- | ------------------- | -------------------------------------------------------------------------- |
| `list_collections`  | `collections.read`  | -                                                                          |
| `get_collection`    | `collections.read`  | `collection`                                                               |
| `create_collection` | `collections.write` | `data` (collection JSON)                                                   |
| `update_collection` | `collections.write` | `collection`, `data` (merged into the collection; `fields` replaces all)  |
| `delete_collection` | `collections.write` | `collection`                                                               |
| `list_records`      | `records.read`      | `collection`, `filter`, `sort`, `page`, `perPage` (max 200), `skipTotal`, `expand` |
| `get_record`        | `records.read`      | `collection`, `id`, `expand`                                               |
| `create_record`     | `records.write`     | `collection`, `data`                                                       |
| `update_record`     | `records.write`     | `collection`, `id`, `data` (supports modifiers like `tags+`)               |
| `delete_record`     | `records.write`     | `collection`, `id`                                                         |
| `list_media`        | `media.read`        | `folder` (path, default `/`; missing → empty list, `exists: false`), `search`, `page`, `perPage` (max 200) |
| `upload_media`      | `media.write`       | `content` + optional `encoding` (`base64` default / `text`) or `url` (http/https), `name`, `folder` (created if missing) |

`collection` accepts a collection name or id. `filter` and `sort` use the regular PocketBase syntax. Uploads to regular `file` fields are not supported; files go through the media library instead.

`upload_media` stores the file in the `_medias` system collection (folders in `folder` are created as needed, conflicting names get a `-N` suffix) and returns `{ id, name, kind, path, url, mime, size, created }`. Base64 `content` is decoded leniently (standard or URL-safe alphabet, missing/extra padding, line breaks, literal `\n` escapes, `data:...;base64,` prefix); raw markup starting with `<` (e.g. SVG) and non-base64 data URIs are stored as text. Decoding errors report the reason (truncated data or the invalid character position). The returned `url` (`/api/files/{mediasCollectionId}/{id}/{file}`) is the value to store in a `media` field. The max size is the `_medias.file` field `maxSize` (default 5 MB); remote downloads time out after 30s. Media access is controlled only by the `media.*` permissions, independently of the collections allowlist.

Tool results are returned as a single JSON text content item. Tool failures (validation errors, missing records, inaccessible collections) are returned as results with `isError: true` so the model can see and react to them.

Example call:

```sh
curl -X POST https://example.com/api/mcp \
  -H "Authorization: Bearer pbmcp_..." \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_records","arguments":{"collection":"posts","filter":"status = \"published\"","sort":"-created"}}}'
```

## Keys Management API

All routes require superuser auth.

| Method   | Route                              | Description                                                   |
| -------- | ---------------------------------- | ------------------------------------------------------------- |
| `GET`    | `/api/mcp/keys`                    | `{ items: [...keys], permissions: [...] }`                    |
| `POST`   | `/api/mcp/keys`                    | Creates a key. Returns `{ key, token }`.                      |
| `PATCH`  | `/api/mcp/keys/{id}`               | Updates `name`, `permissions`, `collections`, `active`, `expiresAt`. |
| `POST`   | `/api/mcp/keys/{id}/regenerate`    | Replaces the key secret. Returns `{ key, token }`.            |
| `DELETE` | `/api/mcp/keys/{id}`               | Deletes the key (`204`).                                      |

Invalid permissions, unknown or system collection ids and invalid dates return `400` with field errors.

## Code Locations

- `core/mcp_key.go` - key model, hashing, lookup helpers
- `migrations/1786000000_mcp_keys.go` - `_mcpKeys` table
- `apis/mcp.go` - routes, keys management handlers, JSON-RPC transport
- `apis/mcp_tools.go` - tool definitions and handlers
- `apis/mcp_tools_media.go` - media library tools (`list_media`, `upload_media`)
- `ui/src/settings/mcp/` - admin UI page, key modals
