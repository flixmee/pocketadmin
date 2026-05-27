# PocketBase Notification Plan

## Goal

Add a first-class notification feature backed by a protected `_notifications` system collection and surfaced in the PocketAdmin header as a bell icon with an unread badge.

The bell should:

- load the current recipient's unread count on admin boot;
- subscribe to `_notifications` realtime changes;
- update the badge without requiring a page refresh;
- open a compact dropdown with recent notifications;
- let the user mark one or all visible notifications as read.

## Current Baseline

- The admin header is rendered from `ui/src/base/appHeader.js`.
- Global admin state lives in `ui/src/store.js`.
- The shared PocketBase client and auth lifecycle hooks live in `ui/src/pb.js`.
- The existing realtime system in `apis/realtime.go` already broadcasts collection record create/update/delete events to subscribers through normal collection list/view rules.
- System collections are created through `core.SystemMigrations` in `migrations/`.
- System collection name constants and model helpers commonly live in `core/`.
- The admin UI uses Remix Icon classes, `t.*` components, popover dropdowns, and CSS under `ui/src/css/`.

## Scope

### MVP

- Create `_notifications` as a system collection.
- Store per-recipient notifications.
- Support unread/read state.
- Add an admin header bell with a badge.
- Subscribe to `_notifications` realtime events for the current superuser.
- Display a recent-notifications dropdown.
- Provide mark-as-read and mark-all-read actions.
- Add backend and UI tests for core behavior.

### Later

- Notification preferences.
- Digest emails.
- Push/browser notifications.
- Notification templates.
- Retention UI.
- A full notifications page with filtering/search.
- App-user notification UI outside PocketAdmin.

## Data Model

Create `core.CollectionNameNotifications = "_notifications"`.

Recommended fields:

| Field | Type | Required | Notes |
|---|---|---:|---|
| `recipientCollection` | text | yes | Collection id/name for the recipient, for example `_superusers` or a custom auth collection. Prefer collection id in persisted records. |
| `recipientRef` | text | yes | Recipient record id. |
| `title` | text | yes | Short display title. |
| `message` | text | no | Body/preview text. |
| `type` | text | no | Product category such as `system`, `automation`, `security`, `backup`, `custom`. |
| `severity` | text | no | `info`, `success`, `warning`, `danger`; default `info`. |
| `read` | bool | yes | Default `false`. |
| `readAt` | date | no | Set when `read` becomes true. |
| `archived` | bool | yes | Default `false`; hidden from the bell. |
| `actionUrl` | url/text | no | Admin route hash or absolute URL. |
| `sourceCollection` | text | no | Optional source collection id/name. |
| `sourceRecord` | text | no | Optional source record id. |
| `data` | json | no | Machine payload for extensions and future UI. |
| `expiresAt` | date | no | Optional expiration/cleanup boundary. |
| `created` | autodate | yes | System create timestamp. |
| `updated` | autodate | yes | System update timestamp. |

Indexes:

- `idx_notifications_recipient_read_created` on `recipientCollection, recipientRef, read, created`.
- `idx_notifications_recipient_archived_created` on `recipientCollection, recipientRef, archived, created`.
- `idx_notifications_source` on `sourceCollection, sourceRecord` where useful.
- Optional cleanup index on `expiresAt`.

## Permissions

The collection should be system-owned, but records should still support recipient-scoped access if the feature is later exposed to normal auth records.

Recommended rules:

- `ListRule`: `@request.auth.id != "" && recipientRef = @request.auth.id && recipientCollection = @request.auth.collectionId && archived = false`
- `ViewRule`: same as list.
- `CreateRule`: `nil` by default; notifications should be created by trusted server code, automations, or superusers.
- `UpdateRule`: allow recipient updates only if paired with validation/hooks or dedicated endpoints that restrict changes to presentation state (`read`, `readAt`, `archived`). Do not allow recipients to change `recipientCollection`, `recipientRef`, `title`, `message`, source fields, or `data`.
- `DeleteRule`: `nil` by default; use archive or retention cleanup instead.

Important admin note: superusers can access all records, so the admin bell must always apply an explicit filter for the current superuser. Do not rely only on API rules for the PocketAdmin header state.

## How To Add A Notification

Notifications should be created from trusted server-side code, not directly from the public record API. Use `app.CreateNotification(...)` so defaults, validation, JSON serialization, and realtime dispatch all go through the normal save pipeline.

### Minimal Server-Side Example

```go
_, err := app.CreateNotification(core.NotificationCreateOptions{
    RecipientCollection: superuser.Collection().Id,
    RecipientRef:        superuser.Id,
    Title:               "Backup completed",
    Message:             "The scheduled backup finished successfully.",
    Type:                "backup",
    Severity:            core.NotificationSeveritySuccess,
    ActionURL:           "#/settings/backups",
})
if err != nil {
    return err
}
```

After the record is saved:

- `_notifications` realtime emits a create event;
- the header bell receives it if the authenticated user matches `recipientCollection` and `recipientRef`;
- the unread badge increments because `read` defaults to `false`;
- the notification appears at the top of the dropdown.

### Targeting A Superuser

Use the superuser auth collection and the superuser record id.

```go
superuser, err := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, "admin@example.com")
if err != nil {
    return err
}

_, err = app.CreateNotification(core.NotificationCreateOptions{
    RecipientCollection: superuser.Collection().Id,
    RecipientRef:        superuser.Id,
    Title:               "Security alert",
    Message:             "A new admin session was created.",
    Type:                "security",
    Severity:            core.NotificationSeverityWarning,
    ActionURL:           "#/logs/auth",
})
```

Prefer `superuser.Collection().Id` for persisted notifications because access rules compare against `@request.auth.collectionId`. Collection names are useful for lookups, but persisted notification records should use the auth collection id so the API, realtime filter, and record rules all agree.

### Targeting A Normal Auth Record

The backend schema supports any auth collection, even though the current first-party UI is the PocketAdmin header.

```go
user, err := app.FindAuthRecordByEmail("users", "user@example.com")
if err != nil {
    return err
}

_, err = app.CreateNotification(core.NotificationCreateOptions{
    RecipientCollection: user.Collection().Id,
    RecipientRef:        user.Id,
    Title:               "Invite accepted",
    Message:             "A teammate accepted your invitation.",
    Type:                "team",
    Severity:            core.NotificationSeverityInfo,
    ActionURL:           "#/collections",
    Data: map[string]any{
        "inviteId": invite.Id,
    },
})
```

Normal auth recipients can access their own notifications through the dedicated API endpoints if their client authenticates as that record. A separate app-user notification UI can be added later using the same endpoints and realtime filter.

### Recommended Producer Pattern

For product events, keep notification creation close to the event owner and call it after the main action succeeds.

```go
app.OnRecordAfterCreateSuccess("backups").BindFunc(func(e *core.RecordEvent) error {
    if err := e.Next(); err != nil {
        return err
    }

    superusers, err := e.App.FindAllRecords(core.CollectionNameSuperusers)
    if err != nil {
        return err
    }

    for _, superuser := range superusers {
        _, err = e.App.CreateNotification(core.NotificationCreateOptions{
            RecipientCollection: superuser.Collection().Id,
            RecipientRef:        superuser.Id,
            Title:               "Backup created",
            Message:             "A new backup is ready.",
            Type:                "backup",
            Severity:            core.NotificationSeveritySuccess,
            ActionURL:           "#/settings/backups",
            SourceCollection:    e.Record.Collection().Id,
            SourceRecord:        e.Record.Id,
        })
        if err != nil {
            return err
        }
    }

    return nil
})
```

Use a helper such as `notifySuperusers(...)` if the same producer pattern appears in multiple packages. Keep that helper server-side and pass explicit recipients; avoid broadcasting to every superuser unless the event is genuinely operational.

### Field Guidance

- `RecipientCollection`: use the recipient auth collection id when available.
- `RecipientRef`: use the recipient record id.
- `Title`: required and short enough for the dropdown.
- `Message`: optional preview text.
- `Type`: product category, for example `system`, `automation`, `security`, `backup`, or `custom`.
- `Severity`: use `core.NotificationSeverityInfo`, `Success`, `Warning`, or `Danger`; empty values default to `info`.
- `ActionURL`: use internal admin hash routes such as `#/settings/backups`; external URLs are allowed only if they parse as valid URLs. For record-triggered automation notifications, prefer the related record route: `/_/#/collections?collection={collection_name}&record={record_id}`.
- `SourceCollection` and `SourceRecord`: set these when the notification was caused by a specific record.
- `Data`: optional machine payload for future UI behavior; keep it small and non-sensitive.
- `ExpiresAt`: optional cleanup boundary; current query helpers do not hide expired records automatically yet.

### Automation Approval Notifications

When an automation enters a `wait.approval` step, the automation runtime should create a warning notification for every superuser so the approval can be resolved from the admin header.

Approval notifications use:

- `Title`: `Automation approval required`
- `Type`: `automation`
- `Severity`: `warning`
- `ActionURL`: `#/automations`
- `SourceCollection`: `_approvals`
- `SourceRecord`: the pending approval id
- `Data`: machine fields including `automationId`, `automationName`, `runId`, `approvalId`, `approvalStatus`, `approvalRole`, `assignee`, `stepIndex`, and `actions: ["approved", "rejected"]`

The admin notification dropdown should render `Approve` and `Reject` buttons when a notification references a pending approval. Those buttons call the existing approval decision endpoint:

- `POST /api/automations/approvals/{approvalId}/decision` with `{"decision":"approved"}`
- `POST /api/automations/approvals/{approvalId}/decision` with `{"decision":"rejected"}`

After a successful decision, the notification should be marked read. If the approval was resolved elsewhere first, the endpoint should return the existing "already resolved" error and the UI should surface the API error.

### Reading And Marking Notifications

Authenticated clients should use the dedicated endpoints:

- `GET /api/notifications?limit=20`
- `GET /api/notifications/unread-count`
- `POST /api/notifications/{id}/read`
- `POST /api/notifications/read`

The current PocketAdmin bell uses these endpoints and subscribes to `_notifications` realtime with a recipient filter. Producers only need to create the notification record; the bell handles loading, badge updates, and mark-read state.

## Backend Implementation

### 1. System Collection Migration

Add a system migration, for example `migrations/<timestamp>_notifications.go`.

Migration responsibilities:

- create `_notifications` if it does not exist;
- mark the collection and fields as system;
- add the fields and indexes above;
- set recipient-scoped list/view rules;
- make the down migration drop the table and remove the `_collections` row.

### 2. Core Constants And Helpers

Add core files similar to other system collection models:

- `core/notification_model.go`
- `core/notification_query.go`
- optional `core/notification_validate.go`

Useful helpers:

- `NewNotification(app App) *Notification`
- `CreateNotification(options NotificationCreateOptions)`
- `FindNotificationById(id string)`
- `FindNotificationsByRecipient(recipientCollection, recipientRef string, limit int)`
- `CountUnreadNotifications(recipientCollection, recipientRef string)`
- `MarkNotificationRead(recipientCollection, recipientRef, id string)`
- `MarkAllNotificationsRead(recipientCollection, recipientRef string)`

Validation should enforce:

- recipient fields are present;
- title is present;
- severity/type values are known or normalized;
- `readAt` is set when transitioning to read;
- `actionUrl` is either an internal `#/...` route or a valid URL if external links are allowed.

### 3. Creation API For Internal Producers

Prefer a small internal service/helper instead of requiring every producer to hand-build records.

Example shape:

```go
type NotificationCreateOptions struct {
    RecipientCollection string
    RecipientRef        string
    Title               string
    Message             string
    Type                string
    Severity            string
    ActionURL           string
    SourceCollection    string
    SourceRecord        string
    Data                map[string]any
    ExpiresAt           types.DateTime
}
```

Initial producers can be manual/server-side only. Later producers could include automations, failed backup restores, security events, long-running jobs, and plugin hooks.

### 4. Dedicated API Endpoints

The bell uses dedicated endpoints because they are cleaner for bulk actions and safer field updates.

Implemented endpoints:

- `GET /api/notifications?limit=20` returns recent notifications for the current auth record.
- `GET /api/notifications/unread-count` returns `{ "count": 3 }`.
- `POST /api/notifications/{id}/read` marks one notification as read.
- `POST /api/notifications/read` marks all current-recipient notifications as read.

Keep record API compatibility for realtime. Add an archive endpoint later if the UI exposes archive/hide behavior.

## Realtime Design

Use the existing collection realtime system:

```js
await app.pb.collection("_notifications").subscribe("*", onNotificationEvent, {
    filter: `recipientCollection="${collectionId}" && recipientRef="${recordId}"`,
});
```

Implementation details:

- Subscribe only after `app.pb.authStore.isValid` and `app.store.superuser?.id` are available.
- Unsubscribe on logout or auth change.
- Use explicit filters because superusers can see all notifications.
- On `create`, increment unread count when `read` is false and prepend to the dropdown list.
- On `update`, reconcile the existing item and unread count.
- On `delete`, remove it locally if present.
- On reconnect/auth refresh, reload unread count and recent notifications to correct any missed events.

## Admin UI Implementation

### Files

Suggested new files:

- `ui/src/notifications/notificationBell.js`
- `ui/src/notifications/notificationStore.js` or store methods in `ui/src/store.js`
- `ui/src/css/notifications.css`

Touchpoints:

- Import the notification component from `ui/src/main.js`.
- Render it in `ui/src/base/appHeader.js` near `colorSchemeButton()`.
- Add CSS import through `ui/src/css/_main.css` if CSS is split into a new file.

### Store State

Recommended state:

- `notifications.items`: recent records shown in the dropdown.
- `notifications.unreadCount`.
- `notifications.isLoading`.
- `notifications.isMarkingRead`.
- `notifications.subscriptionReady`.

Recommended methods:

- `loadNotifications()`
- `loadUnreadNotificationCount()`
- `initNotificationsRealtime()`
- `disposeNotificationsRealtime()`
- `handleNotificationRealtimeEvent(e)`
- `markNotificationRead(id)`
- `markAllNotificationsRead()`

Keep notification UI state in a dedicated store area, not on collection or field objects.

### Bell Component

Render:

- icon-only header button with `ri-notification-3-line`;
- a badge shown only when unread count is greater than zero;
- compact count display such as `9+` or `99+`;
- popover dropdown with recent notification rows;
- empty state when there are no notifications;
- loading state during initial fetch;
- "Mark all read" action when unread count is greater than zero.

Dropdown row content:

- severity/type icon or accent marker;
- title;
- optional message preview;
- relative or formatted creation date using existing date utilities where practical;
- unread visual state;
- optional action link.

Accessibility:

- Button title/aria label should include unread count.
- Badge should not be the only signal; unread rows need a visible style.
- Keyboard focus should work with the existing popover/dropdown behavior.

## Query Strategy

Initial count:

- use `getList(1, 1, { filter: recipientFilter + " && read=false && archived=false", requestKey })`;
- use `totalItems` for the badge count.

Initial list:

- use `getList(1, 20, { filter: recipientFilter + " && archived=false", sort: "-created" })`.

Recipient filter:

- for the admin header, use the authenticated superuser's collection id/name and id;
- prefer `collectionId` when available, but keep the schema and helper tolerant of collection name for migrations/imports.

## Styling

Add styles that match the existing header/dropdown language:

- Bell button should use `.app-header .header-link` conventions.
- Badge should be small, stable, and positioned inside the button bounds.
- Dropdown should reuse `.dropdown` layout and avoid nested cards.
- Rows should be dense and scannable.
- Use existing color variables such as `--dangerColor`, `--warningColor`, `--successColor`, `--accentColor`, and `--surfaceAlt*Color`.

## Tests

Backend:

- migration creates `_notifications` with expected system fields and indexes;
- recipient rules allow only owner access for normal auth records;
- superuser can create notifications;
- validation rejects missing recipient/title and invalid severity;
- marking read sets `read=true` and `readAt`;
- unread count helper returns the expected value;
- realtime sends create/update/delete events for permitted subscribers.

Frontend:

- bell hidden when not authenticated;
- initial unread count appears after load;
- badge updates on realtime create/update/delete;
- dropdown shows recent notifications in descending creation order;
- mark-read updates local state and persists;
- logout/auth change unsubscribes and clears state.

Verification commands:

```bash
go test ./...
cd ui && npm run build
```

## Rollout Phases

1. Add `_notifications` migration, constants, model/query helpers, and backend tests.
2. Add minimal internal create/count/mark-read helpers.
3. Add admin UI notification store and bell component using regular record APIs.
4. Add realtime subscription and reconciliation logic.
5. Add bulk mark-read endpoint if regular record updates are too slow or too permissive.
6. Add source producers, starting with one low-risk backend event.
7. Add retention cleanup and a future full notifications page if operators need history.

## Acceptance Criteria

- `_notifications` exists as a system collection after migration.
- A server-side helper can create a notification for a superuser.
- The admin header shows a bell for authenticated superusers.
- The bell badge shows the correct unread count.
- Creating, updating, or deleting matching `_notifications` records updates the bell via realtime.
- Users can mark notifications as read from the dropdown.
- The UI unsubscribes cleanly on logout/auth changes.
- `go test ./...` and `cd ui && npm run build` pass.

## Open Questions

- Should `_notifications` be admin-only for v1, or should normal auth collections receive notifications too? => yes
- Should users be allowed to archive/delete their own notifications, or only mark them read? => yes
- Should notifications expire automatically, and if yes what default retention period should be used? => yes
- Should `actionUrl` allow external URLs, or only internal admin routes? => yes
- Which first backend event should produce a real notification? => yes
