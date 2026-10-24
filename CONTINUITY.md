Goal (incl. success criteria) [2026-10-24, automations editor autocomplete from recent runs, branch `automation_run_autocomplete`]:
- When an automation has recent runs, the template `{{` autocomplete (and Data mapping palette) suggests the real data paths seen in those runs, with their last value.

Constraints/Assumptions:
- UI-only change; no API/Go changes (uses existing GET /api/automations/{id}/runs?limit=10).
- Out-of-band store in ui/src/base/automationInput.js (not on reactive form objects), same pattern as automationEditorManualInputNames.

Key decisions:
- Paths from run.input (trigger.type/collectionId/collectionName/caller, request, telegram, zalo, i18n, input, record, recordOriginal) and run.stepResults (steps.N.status/output/error; N = executed order, same as template `steps`).
- Array indexes use dot form (`a.0.b`, Go fast path; keeps typing-filter working since codeEditor word chars exclude `[`); non-identifier keys use `["key"]` and then indexes switch to `[0]` (goja-evaluated). All verified to resolve at runtime.
- Limits: 400 paths, depth 6, 50 keys/object, newest run's value wins; sample values truncated to 60 chars; string values under keys matching authorization/cookie/password/secret/token/api key masked.
- Run paths hidden for roots of other trigger types (request->webhook, telegram, zalo, i18n, record*); input/trigger/steps always shown.
- Autocomplete ranks sampled paths first (after startsWith); items show the last value under the token. Palette "Recent runs" group capped to 60 buttons, hover shows the last value.
- Samples reload after the Recent runs modal closes and after Run; cleared on mount/unmount.

State:
  - Done: automationInput.js (setAutomationRunSamples, automationRunSampleValue, Recent runs group, label/sort), pageAutomationUpsert.js (loadRunSamples), stepEditor.js palette, automations.css. vite build ok. Headless e2e on scratch :8091 (webhook automation, 2+ runs): suggestions with values, `{{order.lines.0.s` filter, Enter inserts, saved and resolved at runtime ("ok B-9", bracket keys "ok Ann z"), trigger filtering, no console errors.
  - Done [2026-10-24 08:15]: rebuilt ui/dist + Linux amd64 examples/base/server.
  - Done [2026-10-24]: committed on new branch `automation_run_autocomplete` (from develop 4fdabb51); NOT pushed. ui/dist/index.html left uncommitted (build output, as before).
  - Now: awaiting next task. User chose to skip the template bug below for now.
  - Found (pre-existing, not fixed): core/automation_templates.go automationWholeTemplatePattern `^\s*\{\{\s*(.*?)\s*\}\}\s*$` treats `{{a}} {{b}}` (whole value = two tokens) as one expression -> eval error.

Working set: ui/src/base/automationInput.js, ui/src/settings/automations/{pageAutomationUpsert,stepEditor}.js, ui/src/css/automations.css

---- Previous ledger ----

Goal (incl. success criteria) [2026-10-23, Automations page UI + bulk approvals, branch `improve_automations`]:
- Tabs on the Automations page (Automations | Approvals with pending badge); approvals get Pending/History views, search, multi-select and bulk approve/reject.

State:
  - Done: API POST /api/automations/approvals/decisions {ids, decision, comment} -> {resolved, failed} (max 500, dedupe, workflows continued sequentially in background); approvals list adds automationName (custom record data), status=resolved filter, limit. Tests apis/automation_approvals_bulk_test.go pass.
  - Done: UI pageAutomationsSettings.js tabs (remembered in localStorage, counts, both lists kept mounted), automationApprovalsList.js rewrite (segmented Pending/History, search, checkboxes, select-all, sticky bulk bar, confirm), automationsList oncount, CSS (+ fixed pre-existing mobile search-field height bug). Headless verified desktop + 390px.
  - Done [2026-10-23 11:41]: rebuilt Linux binary; committed and pushed branch improve_automations to origin (tracking).
  - Now: awaiting next task.

---- Previous ledger ----

Goal (incl. success criteria) [2026-10-22, built-in Zalo bridge integration, branch `zalo_bridge`]:
- Replace demo/zalo-pocketbase pb_hooks + JS migration with built-in Go support: settings (enable, bridge URL, secret), auto-created collections zalo_session/zalo_messages/zalo_outbox, outbox hooks (normalize + kick bridge), /api/zalo/* routes; admin UI page to enable, check bridge online/offline, show QR, Login/Logout.
- External Bun bridge (zca-js) stays a separate process (JS-only lib); protocol unchanged (X-Bridge-Secret, /status /login /logout /kick).
- Success: tests pass; UI verified against a fake bridge; build ok.

Key decisions:
- Settings: Credentials.Zalo {enabled, bridgeURL (default http://127.0.0.1:3939), secret (masked)}.
- Collections created on enable/startup only when Zalo enabled (no test churn; existing ones from the JS migration are reused untouched); non-system, same schema/rules as the demo migration.
- Keep /api/zalo/status|login|logout|kick compatible with demo hooks; add superuser /api/zalo/overview + /api/zalo/settings.
- Old pb_hooks/zalo.pb.js + zalo/bridge.js must be removed (route conflict).

State:
  - Done: core/zalo.go (EnsureZaloCollections, outbox request/after-create hooks w/ async kick, CallZaloBridge, bootstrap ensure), settings Credentials.Zalo (secret masked, min 16), apis/zalo.go (overview, settings; compat status/{action} registered in late OnServe (priority 9999) and skipped if old JS hooks registered them -> no duplicate-route panic, verified with demo hooks installed). Tests apis/zalo_test.go pass.
  - Done: UI Settings > Zalo (ui/src/settings/zalo/pageZaloSettings.js, css/zalo.css, route+nav): enable/URL/secret(generate), bridge online/offline w/ reason, account status, QR, Login/Logout/Send pending, setup .env template; 3s polling without re-creating form inputs. Headless e2e with fake bridge: offline -> online -> QR -> logged in -> logged out, no console errors.
  - Done: docs/zalo.md. Full suite: only the 8 pre-existing failures.
  - Done [2026-10-22 15:57]: rebuilt ui/dist + Linux amd64 examples/base/server (includes Zalo).
  - Done [2026-10-22]: Zalo automation nodes — trigger `zalo.message` (fired from zalo_messages after-create, ignores is_self, dedupe by msg_id, `{{zalo.*}}` + `{{trigger.zalo.*}}`, template root whitelisted) and step `zalo.send` (creates zalo_outbox record -> bridge kick; threadId/threadType/phone/text/replyTo, optional waitForDelivery 60s). UI: palette entry, zaloStepForm.js, trigger options/labels/hint, tokens, icon. Tests core/automation_zalo_test.go pass; e2e via API + headless UI verified. Docs in docs/zalo.md.
  - Done [2026-10-22 16:15]: rebuilt Linux binary (Zalo + nodes); committed and pushed branch zalo_bridge to origin (tracking). develop not updated with Zalo.
  - Done [2026-10-22]: AI generate `systemPrompt` (template; OpenAI system message / Anthropic `system` / Gemini `systemInstruction`; plain + tool loop; not stored in memory; validated string <= max template size; preview/schema). UI field in AI generate node. Tests pass; headless verified (saved). Docs in docs/ai-generate-memory.md. On branch zalo_bridge; Linux binary rebuilt 2026-10-22 16:56; committed + pushed to origin/zalo_bridge.
  - Done [2026-10-23]: fast-forward merged zalo_bridge (929864e2) into local develop (NOT pushed; develop 10 commits ahead of origin/develop). Created branch `improve_automations` from develop (current).
  - Now: on `improve_automations`, awaiting next task. demo/ folder is untracked (user's) - not modified, not committed.

---- Previous ledger ----

Goal (incl. success criteria) [2026-10-21, built-in Gmail MCP connection, branch `mcp_client`]:
- MCP connections get kind "gmail": Google OAuth "Connect Google account" (offline refresh token), built-in in-process tools usable by AI generate: gmail_search_emails, gmail_read_email, gmail_read_thread, gmail_send_email (incl. replies). User chose: built-in (not external), tools = search/read + send/reply (no drafts/labels).
- Success: tests (token refresh, search/read/send/reply, OAuth start/callback/disconnect) pass; UI type select, connect/disconnect; build ok.

Key decisions:
- `_mcpServers` gains columns kind, clientId, clientSecret, oauth (JSON: refresh/access token, expiry, email) via new ALTER migration (table already deployed).
- Client id/secret per connection, fallback to Settings Credentials > Google Sheets client.
- Redirect URI: {AppURL}/api/mcp/oauth/google/callback (public; validated by one-time state in app store, 15 min TTL).
- Scopes: openid email gmail.readonly gmail.send. Tool sessions abstracted (MCPToolSession interface) so HTTP + built-in share the AI tool loop.

State:
  - Done: backend — core/mcp_gmail.go (OAuth URL/state/code exchange/refresh, 4 tools, MIME send incl. reply headers, header-injection guard), core/mcp_server.go (kind/clientId/clientSecret/oauth, MCPToolSession, NewToolSession), migrations/1786000003_mcp_servers_oauth.go (ALTER; verified on existing DB), apis oauth start/disconnect/public callback page. Tests: core/mcp_gmail_test.go, apis/mcp_servers_gmail_test.go pass.
  - Done: UI — type select, Gmail fields + redirect URI copy, auto Google sign-in after create (popup, fallback same tab), Connect/Reconnect/Disconnect on list. Fixed: hidden required URL input blocked Gmail form submit. Headless verified (popup opened accounts.google.com with client_id; row "Not connected"; callback error page).
  - Done: docs/mcp-client.md Gmail section. Full suite: only the 8 pre-existing failures.
  - Done: committed a4c29148 + pushed to origin/mcp_client. Rebuilt ui/dist + Linux amd64 examples/base/server (2026-10-22 08:57, includes Gmail).
  - Done [2026-10-22]: fix Gmail redirect URI behind nginx (was built from Application URL = localhost). UI now derives it from the browser location (app.pb.buildURL + location) and sends it to /oauth/start; server validates (NormalizeGoogleOAuthRedirectURL: absolute http(s), path ends with callback, no query/userinfo), stores it in the one-time state and reuses it for the code exchange. AppURL remains fallback. Tests pass; verified behind a sub-path reverse proxy. Committed + pushed to origin/mcp_client. Linux binary rebuilt 2026-10-22 09:22 (includes fix).
  - Done [2026-10-22]: fast-forward merged mcp_client (594b8883) into local develop (NOT pushed; develop ahead of origin/develop). Created branch `zalo_bridge` from develop (current).
  - Now: on `zalo_bridge`, awaiting next task. Real Google account flow UNCONFIRMED (needs user's Google OAuth client).

---- Previous ledger ----

Goal (incl. success criteria) [2026-10-21, AI generate memory, branch `mcp_client`]:
- Optional conversation memory for the "AI generate" node: when enabled, previous turns (user input + final AI answer) for the same session key are sent as chat history.
- Success: tests pass; UI toggle/key/size + clear memory; works with and without MCP tools for OpenAI/Anthropic/Gemini.

Key decisions:
- Step option `memory: {enabled, key (template, default "default"), maxMessages (default 20, max 100)}`; only ai.generate.
- Storage: plain table `_aiMemory` (automationId, sessionKey, role, content, created); scope = automation + rendered key; pruned to maxMessages per session after each save; deleted with the automation.
- Only the final answer is stored (no tool calls). Dry runs don't read/write memory.
- API: GET/DELETE /api/automations/{id}/ai-memory (?key=) for admins.

State:
  - Done: backend — core/automation_ai_memory.go (+tests), migrations/1786000002_ai_memory.go, history threaded through all provider calls (with/without tools), validation/schema, delete-with-automation hook; apis/automation_ai_memory.go (+tests). Ordering uses rowid (insertion order).
  - Done: UI — Memory section in AI generate node (toggle, session key template, messages to remember, "Manage memory"), aiMemoryModal.js (sessions, messages, clear one/all), automationId plumbed into step forms. vite build ok.
  - Done: e2e on scratch :8091 — enabled via UI, saved, 2 runs with fake provider: 2nd request contained 1st turn; modal showed 4 messages. No console errors.
  - Done: docs/ai-generate-memory.md. Full suite: only the 8 pre-existing failures.
  - Done [2026-10-21 13:49]: rebuilt ui/dist + Linux amd64 examples/base/server (includes memory).
  - Done: committed 2bb58e95 on `mcp_client` (all work except pre-existing ui/dist/index.html mod) and pushed to origin/mcp_client (tracking). develop not updated.
  - Now: awaiting next task.

---- Previous ledger ----

Goal (incl. success criteria) [2026-10-21, MCP client + AI generate tools, branch `mcp_client`]:
- Go MCP client (Streamable HTTP) + admin-managed external MCP server connections; "AI generate" automation node can use tools from selected MCP servers (agentic tool loop for OpenAI-compatible/Anthropic/Gemini).
- Success: go tests pass for client/servers API/AI tool loop; vite build ok; UI to manage connections + pick servers/tools in AI generate step.

Constraints/Assumptions:
- No new Go deps. Keep deterministic AI provider (tests) working; dry runs preview AI steps (tools never executed in dry run).

Key decisions:
- tools/mcpclient: JSON + SSE responses, Mcp-Session-Id, pagination, notifications/initialized.
- Connections in plain table `_mcpServers` (name, url, authToken secret [never returned, hasAuthToken], headers JSON, active). Superuser API /api/mcp/servers (+ /{id}/tools).
- AI generate step config: `mcpServers: [{server, tools: []}]` (empty tools = all), `maxToolCalls` (default 10, max 50). Output adds `toolCalls`.
- UI: Settings > "MCP connections" page; AI generate form gets "MCP tools" section.

State:
  - Done: backend — tools/mcpclient (+tests), core/mcp_server.go + migrations/1786000001_mcp_servers.go, apis/mcp_servers.go (+tests), core/automation_ai_tools.go (OpenAI/Anthropic/Gemini tool loops, +tests), validation/schema for mcpServers/maxToolCalls. All pass.
  - Done: UI — Settings > "MCP connections" (ui/src/settings/mcpConnections/), nav+route, AI generate "MCP tools" section (mcpToolsEditor in stepEditor.js), CSS. vite build ok. Headless-Chrome verified (list/test/error toast/edit modal/header rows; step editor server+tool pickers, validation, generated config); no console errors.
  - Done: e2e on scratch :8091 — connection to its own /api/mcp + fake OpenAI provider → AI generate called list_collections via MCP, success.
  - Done: docs/mcp-client.md (+ links from mcp-guide.md, mcp-api.md). Full core/apis suite: only the 8 pre-existing failures (verified identical on clean develop worktree).
  - Done [2026-10-21]: rebuilt ui/dist + Linux amd64 binary examples/base/server (static ELF x86-64, ~69MB) from uncommitted mcp_client work.
  - Done [2026-10-21]: fix — AI node MCP connection dropdown stayed empty (module-level cache loaded once per session; connections added later never appeared without full reload). Now reloads connections on every step-form open, refreshes tools once per open, reload button always enabled, warning hint when no connections. Repro verified fixed in headless Chrome. Rebuilt ui/dist + Linux server binary.
  - Now: awaiting user review. Not committed.

Open questions: none.

Working set: tools/mcpclient/, core/mcp_server.go, core/automation_ai*.go, apis/mcp_servers.go, ui/src/settings/mcpConnections/, ui/src/settings/automations/stepEditor.js

---- Previous ledger ----

Goal (incl. success criteria) [2026-10-20, MCP API on branch `mcp`]:
- MCP server endpoint so AI clients can manage records + collections; admin UI to manage MCP API keys.
- Success: go tests for new code pass; vite build ok; UI page create/edit/delete/regenerate keys; MCP initialize/tools/list/tools/call work with key.

Constraints/Assumptions:
- Branch `mcp` (from develop after merging connect-together; develop NOT pushed yet, mcp local only).
- No new Go deps: hand-rolled MCP Streamable HTTP (stateless JSON responses, JSON-RPC 2.0).
- ui/dist/index.html local mod is pre-existing — leave alone.

Key decisions:
- Keys in plain internal table `_mcpKeys` (migration, like `_collection_groups`), NOT a collection (avoids collection-count test churn, never exposed via records API). Store sha256 hash + display prefix; plaintext shown once.
- Key fields: name, permissions [collections.read, collections.write, records.read, records.write], collections allowlist (collection ids; empty = all non-system), active, expiresAt, lastUsedAt.
- Endpoint POST /api/mcp (Authorization: Bearer <key> or X-API-Key). System collections always blocked. Key mgmt: /api/mcp/keys (superuser).
- Tools: list_collections, get_collection, create/update/delete_collection, list_records, get_record, create/update/delete_record.
- UI: Settings > System > "MCP" page (ui/src/settings/mcp/).

State:
  - Done: backend (core/mcp_key.go, migrations/1786000000_mcp_keys.go, apis/mcp.go, apis/mcp_tools.go, apis/mcp_test.go — 28 scenarios pass). Full apis/core failures = same 8 as clean HEAD (pre-existing).
  - Done: UI Settings > System > MCP (ui/src/settings/mcp/{pageMcpSettings,mcpKeysList,mcpKeyUpsertModal,mcpKeyTokenModal,mcpUtils}.js, ui/src/css/mcp.css, route + nav in router.js/store.js). Browser-verified via headless Chrome on scratch :8091 (create/edit/toggle/regenerate/delete, dark+light). curl e2e of MCP ok. docs/mcp-api.md added.
  - Done: committed c85e570e on `mcp`, pushed (origin/mcp, tracking). Not done: ui/dist not rebuilt; local develop (merge 591d035c) not pushed.
  - Done: user guide docs/mcp-guide.md (setup, Claude Code/Desktop, Cursor, VS Code configs, prompts, security, troubleshooting), linked from docs/mcp-api.md. Committed + pushed to origin/mcp.
  - Done [2026-10-21]: MCP media upload — new perms media.read/media.write; tools list_media (folder/search) + upload_media (base64/data URI or http(s) url, auto-create folders, -N suffix on conflict, size cap = _medias.file maxSize/5MB, 30s timeout); returns `url` to set as media field value. apis/mcp_tools_media.go + apis/mcp_media_test.go (pass), UI perm options + tools table, docs updated. Not committed.
  - Done [2026-10-21]: fix "content is not valid base64" — decodeMCPContent: lenient base64 (url-safe, padding, whitespace, literal \\n, quotes, data URI incl. charset), raw `<...>` markup/non-base64 data URI stored as text, new `encoding` arg (base64|text), detailed errors (truncated / invalid char pos). list_media missing folder → empty list `exists:false` (upload already auto-creates folders). Tests pass. Not committed.
  - Done [2026-10-21]: committed 837e278b on `mcp`, fast-forward merged into local `develop` (not pushed; develop ahead of origin), created branch `mcp_client` from develop (current).
  - Now: on `mcp_client`, awaiting next task. UNCONFIRMED: exact payload the user's AI sent.

Working set: apis/mcp_tools_media.go, apis/mcp_media_test.go, core/mcp_key.go, migrations/1786000000_mcp_keys.go, apis/mcp.go, apis/mcp_tools.go, apis/mcp_test.go, ui/src/settings/mcp/*, ui/src/css/mcp.css, docs/mcp-api.md

---- Previous ledger ----

Goal (incl. success criteria) [2026-10-20, Telegram "chat not found"]:
- Diagnose/fix Telegram step error "Bad Request: chat not found".
- Success: chat IDs from templates (e.g. {{telegram.chat.id}}) render as plain integers; tests pass.

Key decisions:
- Root cause: webhook JSON decoded to float64; fmt.Sprint renders large IDs in exponent form (-1.00123456789e+12). Fix: toString formats floats without exponent; template interpolation uses toString.

State:
  - Done: fix + core/automation_templates_test.go; core automation tests + vet pass. TestSettingsRegisterTelegramWebhook fails with or without the fix (pre-existing).
  - Done: rebuilt examples/base/server (go build, 2026-10-20).
  - Done: committed b3465f1e (connect-together), merged into develop; new branch `mcp` from develop.
  - Now: done.

Working set: core/automation_runner.go (toString), core/automation_templates.go, core/automation_telegram.go

---- Previous ledger ----

Goal (incl. success criteria) [2026-10-17, use_tree option]:
- Base collections get option `use_tree` (bool, JSON key literally `use_tree` per user). Enabling requires a number field named `order`; UI asks user to add it first.

Key decisions:
- Backend: collectionBaseOptions.UseTree + validateTree (error key `use_tree`), wired in validateOptions for base only. Existing i18n base validation is NOT wired (pre-existing) — left untouched.
- Only `order` is required (no parent check) — per user request.

State:
  - Done: core/collection_model_base_options.go, core/collection_validate.go, 4 scenarios in collection_validate_test.go (pass). API verified by curl on scratch :8091.
  - Done: UI Tree section in ui/src/collections/collectionI18nOptionsTab.js (switch name=use_tree; confirm "Add field" inserts order number field onlyInt min 0; warning alert if enabled but field missing). dprint + vite build pass.
  - Failing suites match pre-existing list (TestCollectionDBExport/3_auth fails on HEAD too; logger test flaky).
  - Now: UI not browser-verified (no browser tools); scratch pb :8091 + vite :5174 left running for user check (qa@example.com / QaPass12345!).
  - Done: test build per AGENTS.md (ui dist + examples/base/server linux/amd64) incl. use_tree + ecommerce preset.
  - Done: repo doc docs/collection-tree-api.md (use_tree API + UI + preset category fields); also a Claude Doc write-up (user wanted repo file instead).
  - Next: commit on user approval (ecommerce preset + use_tree still uncommitted).

Working set: core/collection_model_base_options.go, core/collection_validate.go, ui/src/collections/collectionI18nOptionsTab.js

---- Previous ledger ----

Goal (incl. success criteria):
- Add How to Build instructions to AGENTS.md for the UI and Linux amd64 binary.
Constraints/Assumptions:
- Documentation-only; preserve unrelated work.
Key decisions:
- Use the existing examples/base/ directory (user referred to example/base).
State:
  - Done: Confirmed AGENTS.md and examples/base/main.go exist.
  - Done: Added How to Build with npm run build followed by the Linux amd64 Go build; git diff --check passed.
  - Now: Complete.
  - Next: None.
Open questions (UNCONFIRMED if needed):
- None.
Working set (files/ids/commands):
- AGENTS.md, CONTINUITY.md

---- Previous ledger ----

Goal (incl. success criteria) [2026-10-17, blog preset]:
- Blog + E-commerce presets: add image, order, parent to category collections. Success: core/presets tests pass.

State:
  - Done: core/presets/blog.json categories += image (media, 1, images), order (int number, min 0), parent (self relation, max 1); version 1.0.0 -> 1.1.0. Tests updated (core/presets/preset_test.go: 4 relationships, field + import asserts; apis/collection_preset_test.go version). core/presets pass; apis TestCollectionPresetsList still fails on pre-existing "Jobs \u0026 Career" escaping (fails on HEAD too).
  - Done: committed 7b9d5c7f + pushed (develop).
  - Done: ecommerce.json product_categories += image + order (parent already existed); version 1.0.0 -> 1.1.0; preset_test.go asserts added; core/presets pass.
  - Done: test build per AGENTS.md: ui npm run build, then examples/base/server (linux/amd64, CGO_ENABLED=0) with both preset changes.
  - Now: user testing build; ecommerce change not committed.

Working set: core/presets/ecommerce.json, core/presets/blog.json, core/presets/preset_test.go, apis/collection_preset_test.go

---- Previous ledger ----

Goal (incl. success criteria) [2026-10-13, automations editor UI polish]:
- Improve Automations editor page UI: consistent, polished styles for inputs/selects/checkboxes/forms in trigger panel + step drawer.
- Success: visual QA in browser (dark+light), vite build passes, no native <select>.

Constraints/Assumptions:
- Follow AGENTS.md; CSS mostly in ui/src/css/automations.css; user's own server runs on :8090 — use scratch :8091 + vite :5174.
- Uncommitted pre-existing changes (dprint formatting in manualInputs.js/stepEditor.js, pageMedia.js, dist/index.html) — leave alone.

Key decisions:
- Fixed app-wide code-editor overlay misalignment caused by bad merge c10d935a in ui/src/css/form.css (base rules targeted stale `.editor-content`).

State:
  - Done: scratch env (pb binary + data in scratchpad, test superuser in scratchpad/creds.txt); form.css code-editor fix verified.
  - Done: info panel scrolls (flex column, fields overflow-y auto); Notify admins is a real `.field` switch; manual input rows restyled (compact inputs, focus borders, sm checkbox, card bg); field focus borders. Verified dark+light in browser; dprint check + vite build (scratch outDir) pass. Scratch servers stopped.
  - Done: "Add action" drawer grouped by category (stepPaletteGroups in stepEditor.js: Logic/Records/Messaging/Integrations/Wait & approval/AI/Output, unmapped -> Other), per-step descriptions, search (Enter adds first match, reset on open), tinted icons per group, ti-send icon (ti-brand-telegram was undefined). Verified dark+light; build ok.
  - Done: flow node "+" source placeholder now opens the Add action drawer (insert position kept) instead of inserting a Condition step; verified in browser.
  - Done (committed abb4fae6, pushed): UI polish + grouped drawer + "+" opens drawer.
  - Done: new step `execution.data` "Execution data" (manual trigger only): `data` object of templated fields; flow continues; outputs merged across steps (later wins); returned as run output (manual run API/modal) and as `output` of the calling `automation.call` step; precedence: execution data > response body > last step output. Backend core/automation_execution_data.go (+ _test.go, 4 tests pass), wired in steps/validate/schema/call. UI: step option (Output group), form reusing objectRowsEditor, new "JSON" row value type (also fixes object values loading as "[object Object]" in call input rows), ti-database-export icon. Verified end-to-end in browser (save round-trip, direct run, via caller).
  - Now: awaiting user review. Not committed; ui/dist not rebuilt.
  - Next: optional — rebuild ui/dist, further polish of step drawer forms if user wants.

Open questions: none.

Working set: ui/src/css/automations.css, ui/src/css/form.css, ui/src/settings/automations/{stepEditor,manualInputs,pageAutomationUpsert}.js

---- Previous ledger ----

Goal (incl. success criteria) [2026-10-13, automations session]:
- Review Automations; improve Manual trigger (typed input fields, run with input data, optional wait-for-result); add a step that calls another (manual-trigger) automation with input data, in two modes: `wait` (block, use child output) or `parallel` (fire-and-forget, returns child runId).
- Success: go tests for core/apis automation pass incl. new tests; UI build passes; editor supports input fields, call step form, run-with-input modal.

Constraints/Assumptions:
- Follow AGENTS.md; no native <select> in UI; another session may edit concurrently — don't touch unrelated files.
- Callable targets = automations with triggerType `manual`, must be active (DECISION, may revisit).

Key decisions:
- New `_automations.manualInputs` JSON field (migration 1785000000) = [{name,label,type(text|number|boolean|json),required,default,description}].
- Payload gets `Input` (template root `input`, also `trigger.input`) and `Caller` (`trigger.caller`).
- New step type `automation.call` {automation, mode: wait|parallel, input{}, continueOnError?}; depth/loop guard via existing policy MaxDepth (ParentRunId/Depth).
- Response step allowed for manual trigger too = return value of a called automation.
- POST /api/automations/{id}/run accepts {input, wait}; returns run summary (wait) or {runId,status:"queued"} (202).

State:
  - Done: backend (core/automation_call.go, runner/steps/validate/schema/versions/template pkg, migration 1785000000, apis /run {input,wait} -> 202/200), 8 new tests in core/automation_call_test.go pass; template root `input` whitelisted (core/automation_templates.go).
  - Done (review fixes): delay scheduler double-close panic + goroutine querying closed DB (core/automation_delay_scheduler.go: sync.Once shutdown, IsBootstrapped guard, recover).
  - Pre-existing failures on HEAD (not ours): TestSQLRun, TestSettings_DBExport, TestSettingsMarshalJSON, TestCollectionDBExport, TestNotifyWatcher_*; flaky TestCollectionPresetsList, TestSettingsRegisterTelegramWebhook.
  - Done (UI): manualInputs.js (inputs editor + Run modal w/ wait toggle), call step form in stepEditor.js, Run button on page + list (manual only), autocomplete `input` group, response step allowed for manual ("Return response").
  - Done (review fix): pageAutomationUpsert hydrated form by re-normalizing editor steps -> wait.delay reset to 1m / capability inputs lost on load+save. Now normalizes from API data; change detection compares API payloads.
  - Verified: browser QA on scratch server (:8091/:5174, stopped) — run modal validation + result, call step form, save round-trip byte-identical; automation tests + vet + vite build green. Scratch worktree removed.
  - Now: done; awaiting user review. Not committed.
  - Next: user decision on commit; optional follow-ups (pre-existing failing tests, dead automationUpsertModal.js, global policy context race).

Open questions: none yet.

Working set: core/automation_runner.go, automation_steps.go, automation_validate.go, automation_schema.go, automation_versions.go, automation_template_package.go, new core/automation_call.go, migrations/1785000000_automation_manual_inputs.go, apis/automation.go, ui/src/settings/automations/*.

---- Previous ledger ----

Goal (incl. success criteria):
- Fix startup panic from duplicate POST /api/automation-webhooks/{id} registration; verify automation routes build and tests pass.
Constraints/Assumptions:
- Preserve unrelated changes in apis/settings_credentials.go and ui/src/media/pageMedia.js.
- Follow AGENTS.md; backend-only fix.
Key decisions:
- Remove duplicate route registration at its source; keep all supported webhook methods.
State:
  - Done:
    - Read prior ledger and inspected route registrations; POST webhook registered twice in apis/automation.go.
    - Removed duplicate POST registration; helper retains GET/POST/PUT/PATCH/DELETE.
    - go test ./apis -run TestAutomation -count=1 passed (4.796s); includes router construction and webhook behavior.
    - git diff --check passed.
    - Previous ledger reports Telegram features verified; media UI work remains unfinished (UNCONFIRMED current status).
  - Now:
    - Fix complete and verified.
  - Next:
    - User can restart the server to load the fix.
Open questions (UNCONFIRMED if needed):
- None for this fix.
Working set (files/ids/commands):
- CONTINUITY.md
- apis/automation.go
- apis/automation_test.go
- go test ./apis -run TestAutomation

---- Media page improvements (2026-10-12, separate session) ----
- Goal: improve admin UI Media page. UI-only (ui/src/media/pageMedia.js, mediaFolderModal.js, ui/src/css/media.css).
- Done: safe pb.filter params; per-instance request keys (fixed stuck loader on route change); paginated "Load more" (100/page) + "X of Y" footer; search name+mime; no-flicker background refresh (debounced); instant click selection; Enter/Delete/Backspace shortcuts; file Download/Copy URL/Rename; upload progress panel; empty-state actions + no-results state with Clear search; localStorage try/catch.
- Verified: vite build ok; browser-tested search, no-results, selection, menu, rename modal, delete confirm (cancelled).
- Done (2): marquee drag-to-select (pointer events on .page-content, data-media-id on items, Cmd/Ctrl/Shift adds to selection, empty-area click clears); browser-tested grid + list.
- Next: user review; not committed.

---- Node editor connections (2026-10-13, connect-together branch) ----
- Done: remove connections (hover ×, or select + Delete/Backspace); "Add node" creates an unconnected node at viewport center (non-overlapping); manual connect by dragging output -> node/input.
- Model: unconnected nodes saved in new `_automations.detachedSteps` (migration 1785000001), list of step groups, validated loosely, never executed. Editor workspace helpers in stepEditor.js (createWorkspace/connectWorkspaceSteps/disconnectWorkspaceEdge).
- Library fixes (flowdesigner.vanilla.js): per-edge `deletable:false`; Delete key ignored when canvas container focused; connection drop target never found (pointer capture) -> now elementFromPoint + node-body drop.
- Verified: browser QA on scratch :8091/:5174 (stopped); go automation tests + vite build green. NOT committed. User's :8090 server needs restart for migrations.
