# MCP Guide: Connect AI Assistants to PocketAdmin

This guide explains how to let AI assistants such as Claude, Cursor or VS Code (GitHub Copilot) work with your PocketAdmin data through the Model Context Protocol (MCP).

For the technical reference (protocol details, tool arguments, keys management API) see [mcp-api.md](./mcp-api.md).

## What is MCP?

MCP is an open standard that lets AI assistants call external tools. PocketAdmin ships with a built-in MCP server, so once an assistant is connected you can ask it in plain language to:

- explore your collections and their fields
- search, filter and summarize records
- create, update or delete records
- design new collections or change existing ones

Every assistant connects with its own **MCP API key**. The key decides what the assistant is allowed to do and which collections it can see.

## 1. Create an API key

1. Sign in to the admin UI as a superuser.
2. Open **Settings > MCP**.
3. Click **New API key**.
4. Fill in the form:
   - **Name** - something that identifies the client, e.g. `Claude Desktop - Anna`.
   - **Permissions** - choose only what the assistant needs:

     | Permission         | Allows                                     |
     | ------------------ | ------------------------------------------ |
     | Read collections   | List collections and view their schema     |
     | Manage collections | Create, update and delete collections      |
     | Read records       | List, filter and view records              |
     | Write records      | Create, update and delete records          |
     | Read media         | Browse and search the media library        |
     | Upload media       | Upload files to the media library          |

   - **Allowed collections** - leave empty to allow every non-system collection, or pick specific ones.
   - **Expires at** - optional; the key stops working after this date.
   - **Enabled** - turn off to block the key without deleting it.
5. Click **Create key**.
6. **Copy the key now.** It starts with `pbmcp_` and is shown only once. If you lose it, use **Regenerate** to get a new one.

The dialog also shows a ready-to-use client configuration with your server URL and key filled in.

> Tip: start with **Read collections** + **Read records**. Add write permissions later, when you trust the workflow.

## 2. Connect your assistant

You need two values:

- **Endpoint URL** - shown on the **Settings > MCP** page, e.g. `https://your-domain.com/api/mcp`
- **API key** - the `pbmcp_...` value from step 1

The key is sent in the `Authorization: Bearer <key>` header (the `X-API-Key: <key>` header works too).

Replace `https://your-domain.com` and `pbmcp_...` in the examples below with your own values.

### Claude Code

```sh
claude mcp add --transport http pocketadmin https://your-domain.com/api/mcp \
  --header "Authorization: Bearer pbmcp_..."
```

Or add it to a project `.mcp.json` file:

```json
{
  "mcpServers": {
    "pocketadmin": {
      "type": "http",
      "url": "https://your-domain.com/api/mcp",
      "headers": {
        "Authorization": "Bearer ${POCKETADMIN_MCP_KEY}"
      }
    }
  }
}
```

and set `POCKETADMIN_MCP_KEY` in your environment, so the key is not committed to the repository.

Run `/mcp` inside Claude Code to check that the server is connected.

### Claude Desktop

Claude Desktop connects to servers that need a custom header through the [`mcp-remote`](https://www.npmjs.com/package/mcp-remote) bridge (requires Node.js).

Open **Settings > Developer > Edit Config** and add:

```json
{
  "mcpServers": {
    "pocketadmin": {
      "command": "npx",
      "args": [
        "-y",
        "mcp-remote",
        "https://your-domain.com/api/mcp",
        "--header",
        "Authorization:${POCKETADMIN_AUTH}"
      ],
      "env": {
        "POCKETADMIN_AUTH": "Bearer pbmcp_..."
      }
    }
  }
}
```

Restart Claude Desktop. The PocketAdmin tools appear in the tools menu of the chat input.

> The header is passed through an environment variable because some systems split arguments that contain spaces.

### Cursor

Add to `~/.cursor/mcp.json` (all projects) or `.cursor/mcp.json` (one project):

```json
{
  "mcpServers": {
    "pocketadmin": {
      "url": "https://your-domain.com/api/mcp",
      "headers": {
        "Authorization": "Bearer pbmcp_..."
      }
    }
  }
}
```

Then check **Cursor Settings > MCP** - the server should be listed with its tools.

### VS Code (GitHub Copilot)

Create `.vscode/mcp.json`. With an `inputs` entry VS Code asks for the key once and stores it securely instead of keeping it in the file:

```json
{
  "inputs": [
    {
      "type": "promptString",
      "id": "pocketadmin-key",
      "description": "PocketAdmin MCP API key",
      "password": true
    }
  ],
  "servers": {
    "pocketadmin": {
      "type": "http",
      "url": "https://your-domain.com/api/mcp",
      "headers": {
        "Authorization": "Bearer ${input:pocketadmin-key}"
      }
    }
  }
}
```

Open Copilot Chat in **Agent** mode and pick the PocketAdmin tools from the tools menu.

### Other clients

Any client that supports remote MCP servers over **Streamable HTTP** with custom headers works. Use the endpoint URL and the `Authorization: Bearer <key>` header.

To test the connection manually, run the official MCP Inspector:

```sh
npx @modelcontextprotocol/inspector
```

Choose **Streamable HTTP**, enter the endpoint URL, add the `Authorization` header and click **Connect**. You can then list and call the tools directly.

## 3. Example prompts

Once connected, just ask. Good first prompts:

- "What collections do I have in PocketAdmin and what are they used for?"
- "Show me the 10 most recent orders with status `pending`."
- "How many users signed up this month?"
- "Find posts without a cover image and list their titles."
- "Create a new product called 'Desk Lamp' priced at 39.90 in the `products` collection."
- "Mark all tasks assigned to record `abc123` as done." (needs **Write records**)
- "Generate a cover image for the post 'Hello world', upload it and set it as the post cover." (needs **Upload media** + **Write records**)
- "Create a `feedback` collection with fields: message (text, required), rating (number 1-5) and user (relation to users)." (needs **Manage collections**)

The assistant usually starts by listing the collections to learn their fields, then queries or changes the records. Most clients ask you to approve each tool call - read them, especially before deletes.

## Available tools

| Tool                | What it does                                   | Permission         |
| ------------------- | ---------------------------------------------- | ------------------ |
| `list_collections`  | Lists collections with a summary of fields     | Read collections   |
| `get_collection`    | Shows the full schema of one collection        | Read collections   |
| `create_collection` | Creates a collection                           | Manage collections |
| `update_collection` | Changes a collection (fields, rules, indexes)  | Manage collections |
| `delete_collection` | Deletes a collection and all its records       | Manage collections |
| `list_records`      | Searches records with filter, sort and paging  | Read records       |
| `get_record`        | Shows one record                               | Read records       |
| `create_record`     | Creates a record                               | Write records      |
| `update_record`     | Updates a record                               | Write records      |
| `delete_record`     | Deletes a record                               | Write records      |
| `list_media`        | Browses or searches the media library          | Read media         |
| `upload_media`      | Uploads a file (base64 or from a URL)          | Upload media       |

Filters and sorting use the normal PocketBase syntax, e.g. `status = 'active' && total > 100` and `-created`.

### Uploading media (e.g. a post cover)

Media fields point to files in the shared media library (**Media** in the sidebar). To set a cover with an assistant:

1. The assistant calls `upload_media` with either the file content (base64, or plain text with `encoding: "text"` for SVG and other text files) or a public `url` to download it from, plus an optional `folder` such as `/posts/covers` (folders that don't exist yet are created). If the name is already taken a suffix is added, e.g. `cover-1.png`.
2. The tool returns the new media item including its `url`, e.g. `/api/files/<mediaCollectionId>/<id>/cover_abc123.png`.
3. The assistant sets that `url` as the media field value with `update_record`/`create_record`, e.g. `{"cover": "/api/files/..."}`.

The file size limit is the one configured for the media library (5 MB by default). Assistants that can't produce binary data themselves (most chat clients) should pass a `url`.

## Security

Keep these in mind:

- **The key bypasses collection API rules.** An MCP key acts like a limited superuser: its permissions and allowed collections are the only restrictions. Hooks and field validation still run.
- **System collections are never accessible** (e.g. `_superusers`), and hidden fields such as passwords and token keys are never returned. Auth collection emails *are* returned.
- **Use one key per person and per client.** You can see when each key was last used and revoke one without affecting the others.
- **Grant the minimum.** Prefer read-only keys and restrict them to the collections the assistant needs. A key limited to specific collections also can't create new collections.
- **Be careful with write permissions.** AI assistants can make mistakes. Keep backups (**Settings > Backups**) and review tool calls before approving deletes or collection changes.
- **Never commit keys.** Use environment variables or your client's secret storage.
- **Upload media from a URL** makes the server download that URL. Only grant **Upload media** to clients you trust, since the server may reach addresses that are not public (e.g. your internal network).
- **Use HTTPS** in production so keys are not sent in plain text.
- **Set an expiration date** for temporary access, and disable or delete keys you no longer use.
- If a key leaks, **Regenerate** or delete it immediately - the old key stops working right away.

## Managing keys

On **Settings > MCP** each key shows its status, key prefix, permissions, allowed collections, last use and expiration. From the row actions you can:

- **Enable / disable** - toggle the switch; disabled keys are rejected immediately.
- **Edit** - change the name, permissions, allowed collections or expiration. The key value stays the same.
- **Regenerate** - create a new key value. Clients using the old one stop working until updated.
- **Delete** - remove the key permanently.

## Limitations

- Regular `file` fields can't be uploaded through MCP; use media fields with `upload_media` instead.
- Media items can't be renamed, moved or deleted through MCP (use the admin UI).
- Realtime subscriptions, backups, settings and logs are not exposed.
- `list_records` returns at most 200 records per page; the assistant pages through larger results.
- Editing a collection's `fields` replaces the whole field list - removed fields are deleted with their data. Assistants are told to load the collection first, but double check before approving.

## Troubleshooting

| Problem                                       | What to check                                                                                                     |
| --------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `401 Missing or invalid MCP API key`          | The key is mistyped, disabled, expired or was regenerated. Check the header is `Authorization: Bearer pbmcp_...`. |
| Client shows the server but no tools          | The key has no permissions. Edit the key and enable at least one.                                                 |
| A tool is missing                             | The key doesn't have the permission for that tool (see the table above).                                          |
| "collection doesn't exist or is not accessible" | The collection is a system collection, isn't in the key's allowed collections, or the name is wrong.            |
| "keys restricted to specific collections can't create new collections" | Clear **Allowed collections** on the key, or create the collection in the admin UI. |
| `405 Method Not Allowed`                      | The client tried the old SSE transport. Choose **Streamable HTTP** / `http` in the client config.                 |
| Connection refused or timeouts                | The endpoint URL must be reachable from the machine running the assistant (not `localhost` from another computer). |
| Validation errors when creating records       | Ask the assistant to check the collection schema first; the error lists the failing fields.                       |
