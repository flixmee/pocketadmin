# MCP Connections: Tools for the "AI generate" Automation Step

PocketAdmin can also act as an **MCP client**: superusers connect external MCP servers, and the **AI generate** automation step can call their tools while it generates a response (e.g. search the web, look up a CRM, read another PocketAdmin instance).

This is the opposite direction of the [MCP API](mcp-api.md), where external AI assistants connect *to* PocketAdmin.

## 1. Add a connection

1. Open **Settings > MCP connections** and click **New connection**.
2. Fill in:
   - **Name** - shown in the automation editor and in the tool descriptions sent to the model.
   - **Server URL** - the server's Streamable HTTP endpoint, e.g. `https://example.com/mcp`.
   - **Bearer token** - optional; sent as `Authorization: Bearer <token>`. It is stored on the server and never returned by the API (the UI shows `(unchanged)`; use **Remove token** to clear it).
   - **Custom headers** - optional extra headers such as `X-Api-Key`. Header values are visible to superusers, so prefer the bearer token for secrets.
   - **Enabled** - disabled connections make the steps using them fail.
3. Use the **Test connection** action (pulse icon) to connect and list the server tools.

> Tip: you can connect PocketAdmin to itself (or another instance) by creating an [MCP API key](mcp-guide.md) and using `https://<your-app>/api/mcp` as the server URL.

## Gmail (built-in)

Choose **Type: Gmail (built-in)** to give the AI access to a Gmail mailbox without hosting anything:

| Tool                  | What it does                                                                                     |
| --------------------- | ------------------------------------------------------------------------------------------------ |
| `gmail_search_emails` | Searches with the Gmail syntax (`from:`, `is:unread`, `after:2026/01/01`, ...) - max 50 results    |
| `gmail_read_email`    | Reads a message: headers, plain text body (HTML converted to text), attachment names              |
| `gmail_read_thread`   | Reads a whole conversation                                                                       |
| `gmail_send_email`    | Sends an email (`to`, `cc`, `bcc`, `subject`, `body`, optional `html`); `replyToMessageId` replies in the thread |

Setup:

1. In [Google Cloud Console](https://console.cloud.google.com/) enable the **Gmail API** and create an **OAuth client ID** of type *Web application*.
2. Add the authorized redirect URI shown in the connection form: `<App URL>/api/mcp/oauth/google/callback` (the **Application URL** in Settings > Application must be the public URL of your app).
3. Create the connection with the client ID/secret (or leave them empty to reuse the Google client from **Settings > Credentials**).
4. Sign in with the Gmail account in the popup (it opens right after creating the connection, or use **Connect** / **Reconnect** on the list). The list shows the connected account; **Disconnect** removes the stored tokens.

Notes:

- Requested scopes: `gmail.readonly`, `gmail.send` (+ `openid email` to show the account). The AI can't delete or modify emails.
- Tokens are refreshed automatically and never returned by the API.
- In Google's *Testing* publishing status refresh tokens expire after 7 days - publish the OAuth app (or use an *Internal* app on Google Workspace) for long-lived connections. If the authorization expires the steps fail with "reconnect the Gmail account".
- Sending is a real side effect: deselect `gmail_send_email` in the AI generate node when the step should only read.

## 2. Use the tools in an automation

1. Add or open an **AI generate** step.
2. In **MCP tools** click **Add MCP server**, pick the connection and optionally the allowed tools (leave empty to allow all tools of the connection). Add more rows to combine several servers.
3. Optionally set **Max tool calls** (default 10, max 50). When the limit is reached the model is asked to answer with what it has.

The model decides when to call the tools. Each call result is sent back to the model until it produces the final answer, which becomes the step output as usual (`output.text`, or a JSON object when an output schema is set).

Step output:

```json
{
  "model": "gpt-4.1",
  "output": { "text": "..." },
  "tokenUsage": { "input": 1200, "output": 150, "total": 1350 },
  "toolCalls": [
    { "server": "Web search", "tool": "search", "arguments": { "query": "..." }, "isError": false, "output": "..." }
  ]
}
```

`toolCalls[].output` is truncated to 2000 characters (the model receives up to 50000 characters per call).

Step JSON:

```json
{
  "type": "ai.generate",
  "input": "Summarize today's news about {{record.topic}}",
  "mcpServers": [
    { "server": "<connection id>", "tools": ["search", "fetch"] }
  ],
  "maxToolCalls": 5
}
```

## Notes

- Supported AI providers: OpenAI and OpenAI-compatible (custom), Anthropic and Gemini - configured in **Settings > Application > AI**.
- Only the **AI generate** step supports MCP tools.
- **Dry runs don't call the AI or the tools** (AI steps are only previewed).
- Tool errors are passed back to the model so it can recover; connection problems while loading the tools (disabled/missing connection, unknown tool, unreachable server) fail the step.
- Tools can have side effects (sending messages, writing data). Allow only the tools the step needs.
- Each call uses a 60s timeout; listing tools uses 30s.

## API

All routes require superuser auth.

| Method   | Route                          | Description                                                               |
| -------- | ------------------------------ | ------------------------------------------------------------------------- |
| `GET`    | `/api/mcp/servers`             | `{ items: [...connections] }` (`hasAuthToken` instead of the token)       |
| `POST`   | `/api/mcp/servers`             | Creates a connection: `name`, `kind` (`http`/`gmail`), `url`, `authToken`, `headers`, `clientId`, `clientSecret`, `active` |
| `PATCH`  | `/api/mcp/servers/{id}`        | Updates a connection. Omit `authToken` to keep it, send `""` to remove it |
| `DELETE` | `/api/mcp/servers/{id}`        | Deletes the connection (`204`)                                            |
| `GET`    | `/api/mcp/servers/{id}/tools`  | Connects and returns `{ server, tools: [{ name, title, description }] }`  |
| `POST`   | `/api/mcp/servers/{id}/oauth/start`      | Gmail: returns `{ authURL, redirectURL }` for the Google sign-in   |
| `POST`   | `/api/mcp/servers/{id}/oauth/disconnect` | Gmail: removes the stored tokens                                  |
| `GET`    | `/api/mcp/oauth/google/callback`         | Public Google redirect URI (validated with a one-time state)      |

## Code Locations

- `tools/mcpclient/` - Streamable HTTP MCP client (JSON and SSE responses, sessions, pagination)
- `core/mcp_server.go`, `migrations/1786000001_mcp_servers.go` - `_mcpServers` table and helpers
- `core/mcp_gmail.go` - Gmail tools and Google OAuth; `migrations/1786000003_mcp_servers_oauth.go`
- `apis/mcp_servers.go` - connections API
- `core/automation_ai_tools.go` - tool loading and the provider tool-calling loops
- `ui/src/settings/mcpConnections/` - settings page; `ui/src/settings/automations/stepEditor.js` (`mcpToolsEditor`) - step UI
