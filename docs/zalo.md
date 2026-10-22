# Zalo (built-in bridge integration)

PocketAdmin has built-in support for the **zalo-bridge** (Bun + [zca-js](https://github.com/RFS-ADRENO/zca-js)) that links a personal Zalo account. The PocketBase hooks and JS migration of `demo/zalo-pocketbase/pocketbase` are no longer needed.

The bridge itself still runs as a separate process (zca-js is a JavaScript-only library) - build and deploy it as described in `demo/zalo-pocketbase/README.md`, section 2-3.

> zca-js emulates Zalo Web and is not officially supported by Zalo. The account can be locked - use a secondary account and don't spam.

## Setup

1. Open **Settings > Zalo**, turn on **Enable Zalo**, set the **Bridge URL** (default `http://127.0.0.1:3939`) and a **Bridge secret** (at least 16 characters, **Generate a new secret** creates one) and save. Enabling creates the `zalo_session`, `zalo_messages` and `zalo_outbox` collections if they don't exist (existing ones are kept).
2. Create a dedicated superuser for the bridge.
3. Configure the bridge `.env` - the **Bridge setup** section on the page shows a ready template:
   ```bash
   PB_URL=http://127.0.0.1:8090
   PB_SUPERUSER_EMAIL=bridge@example.com
   PB_SUPERUSER_PASSWORD="<password>"
   BRIDGE_HOST=127.0.0.1
   BRIDGE_PORT=3939
   BRIDGE_SECRET=<the same secret>
   ```
4. Start the bridge. The **Bridge** card shows *Online* (or why it's offline: not running, wrong secret, timeout).
5. Click **Login with QR** and scan the QR code with the Zalo app. The account card shows the account name once logged in; **Logout** disconnects it from the bridge.

The page refreshes the status every 3 seconds.

### Migrating from the demo hooks

Delete `pb_hooks/zalo.pb.js` and `pb_hooks/zalo/bridge.js`. If they are still installed the app keeps working (their `/api/zalo/status|login|logout|kick` routes take precedence and a warning is logged), but they still use the old hardcoded secret. The `pb_migrations/1761100000_zalo_init.js` migration and the existing collections/messages can stay. The secret no longer comes from the `ZALO_BRIDGE_SECRET`/`ZALO_BRIDGE_URL` env vars (or the hardcoded value) - set it in **Settings > Zalo**.

## How it works

- **Incoming messages** - the bridge saves them in `zalo_messages` (subscribe with realtime).
- **Sending** - create a `zalo_outbox` record (`thread_id` + `thread_type`, or `phone`; `text` and/or `attachments`; optional `reply_to`). PocketAdmin forces `status = pending`, validates the record and notifies the bridge (`/kick`). If the bridge is offline the message stays `pending` and is sent when it's back.
- **Session** - the bridge keeps the status, QR code and login cookies in the `zalo_session` record `zalomainsession` (`credentials` is hidden).

Default rules (same as the demo migration): authenticated users can list/view the three collections and create outbox records. Adjust them in the collection settings if needed.

## Automations

### Trigger: On Zalo message

Runs for each message the bridge saves in `zalo_messages`. Messages sent by the connected account itself (`is_self`) are ignored, so an automation can reply without triggering itself. Available template values:

| Template                                     | Value                                          |
| -------------------------------------------- | ---------------------------------------------- |
| `{{zalo.text}}`                              | Message text                                   |
| `{{zalo.threadId}}`, `{{zalo.threadType}}`   | Conversation id and `user`/`group`             |
| `{{zalo.senderId}}`, `{{zalo.senderName}}`   | Sender                                         |
| `{{zalo.messageId}}`                         | The `zalo_messages` record id (for replies)    |
| `{{zalo.msgId}}`, `{{zalo.msgType}}`, `{{zalo.ts}}` | Zalo message id, type and timestamp     |
| `{{zalo.content.*}}`, `{{zalo.message.*}}`   | Raw content and the full record                |

(also available as `{{trigger.zalo.*}}`). Each message runs an automation once (deduplicated by the Zalo message id).

### Step: Send Zalo

Queues a message in `zalo_outbox` for the bridge:

| Option            | Description                                                                 |
| ----------------- | --------------------------------------------------------------------------- |
| `threadId`        | User or group id, e.g. `{{zalo.threadId}}`                                  |
| `threadType`      | `user` (default), `group` or `{{zalo.threadType}}`                          |
| `phone`           | Used when `threadId` is empty (the person must allow search by phone)       |
| `text`            | Message (templates supported)                                               |
| `replyTo`         | `zalo_messages` record id to quote, e.g. `{{zalo.messageId}}`               |
| `waitForDelivery` | Wait (max 60s) for the bridge result; the step fails on `failed`/timeout    |

Output: `{ queued, outboxId, status, threadId, threadType, phone, zaloMsgId }` (`status` is `sent` when waiting, otherwise `pending`). Dry runs only preview the rendered values.

Example auto-reply: trigger **On Zalo message** + **Send Zalo** with thread ID `{{zalo.threadId}}`, thread type *Same as the trigger message*, message `Thanks {{zalo.senderName}}, we'll get back to you soon!`. Add a condition step before it to reply only to some messages (e.g. `zalo.threadType = user`).

## API

| Method | Route                     | Auth          | Description                                                            |
| ------ | ------------------------- | ------------- | ---------------------------------------------------------------------- |
| `GET`  | `/api/zalo/status`        | any auth      | Bridge status (`503` with `status: "bridge_offline"` / `"disabled"`)  |
| `POST` | `/api/zalo/login`         | superuser     | Starts the QR login (the QR appears in `zalo_session.qr_image`)       |
| `POST` | `/api/zalo/logout`        | superuser     | Disconnects the account and clears the saved cookies                  |
| `POST` | `/api/zalo/kick`          | superuser     | Asks the bridge to send the pending outbox messages                   |
| `GET`  | `/api/zalo/overview`      | superuser     | Settings, bridge availability, collections and session (admin UI)     |
| `POST` | `/api/zalo/settings`      | superuser     | `{ enabled, bridgeURL, secret }` (empty secret keeps the current one) |

The first four routes are compatible with the former hooks, so existing app code keeps working.

## Code Locations

- `core/zalo.go` - collections, outbox/messages hooks, bridge client; settings in `core/settings_model.go` (`Credentials.Zalo`)
- `core/automation_zalo.go` - "On Zalo message" trigger and "Send Zalo" step
- `apis/zalo.go` - routes
- `ui/src/settings/zalo/pageZaloSettings.js`, `ui/src/css/zalo.css` - admin page
- `ui/src/settings/automations/zaloStepForm.js` - step form
