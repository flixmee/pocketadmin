Goal (incl. success criteria):

- Show realtime notifications as toast messages.
- Success: when a new unread `_notifications` realtime record arrives for the current recipient, the admin UI calls `app.toasts` with the notification title/message/severity, and notification actions are passed through as toast action options.

Constraints/Assumptions:

- Follow `AGENTS.md` and UI guidance.
- Existing unrelated changes may be present; do not revert them.
- `toast.js` is the current app-integrated implementation, imported from `ui/src/main.js`.
- `toastv2.js` is currently a standalone Sonner-style global `toast` helper and is not imported by the app.
- Assumption: "treat it like toast.js" means expose v2 through `window.app.toasts` with compatible methods (`info`, `success`, `error`, `remove`, `removeAll`) and switch the app import to v2.
- Realtime notification actions should reuse existing dropdown behavior where possible, especially automation approval decisions.

Key decisions:

- Keep app call sites unchanged by adding a compatibility layer in `toastv2.js`.
- Preserve v2's richer API on `window.app.toasts.toast` and, where present, `window.toast`.
- Put realtime toast triggering in `app.store.handleNotificationRealtimeEvent`, the existing central handler for `_notifications` realtime records.

State:
  - Done:
    - Read existing ledger and refreshed it for the toast v2 integration task.
    - Compared `ui/src/base/toast.js` and `ui/src/base/toastv2.js`.
    - Searched UI usage; app code calls `app.toasts.*`, and `ui/src/main.js` imports `./base/toast`.
    - Updated `toastv2.js` with `app.toasts` compatibility methods, keyed replacement/removal, DOM node content support, app-themed CSS variables, bottom-center defaults, and richer v2 helper passthroughs.
    - Switched `ui/src/main.js` to import `./base/toastv2`.
    - Verified `npm run build` from `ui/`; Vite completed successfully. dprint reported the known sandbox cache write warning.
    - Read notification realtime/store and notification dropdown approval action handling.
    - Added realtime toast triggering for new unread notifications in `app.store.handleNotificationRealtimeEvent`.
    - Mapped notification severity `danger` to toast `error`, `data.actions` to toast actions, approval actions to the existing confirmation/decision flow, and `actionUrl` to an `Open` toast action.
    - Verified `npm run build` from `ui/`; Vite completed successfully. dprint reported the known sandbox cache write warning.
  - Now:
    - Reporting the realtime notification toast integration.
  - Next:
    - Optionally visual-check a seeded realtime notification in the running admin UI.

Open questions (UNCONFIRMED if needed):

- Whether the old `toast.js` file should remain in place for fallback/reference is UNCONFIRMED.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `ui/src/base/toast.js`
- `ui/src/base/toastv2.js`
- `ui/src/main.js`
- `ui/src/store.js`
- Check passed: `npm run build` from `ui/` (dprint cache write warning, Vite success).
