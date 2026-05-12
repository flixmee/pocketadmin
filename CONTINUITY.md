Goal (incl. success criteria):

- Improve the admin UI for Locales Settings using the provided reference image.
- Success: the locales settings page has a clearer language table, an add-language flow with searchable language choices and Twemoji flags, follows existing admin UI patterns, avoids native HTML `<select>`, and the UI build passes.

Constraints/Assumptions:

- Use the existing i18n backend/API shape already implemented in this workspace.
- Keep the change focused to the admin UI unless a UI issue exposes a backend mismatch.
- Follow `UI_DOCS.md` and AGENTS admin UI notes.
- Do not store ephemeral presentation-only state on reactive `field`/`collection` objects.
- In admin UI, use `app.components.select`; do not introduce native HTML `<select>`.

Key decisions:

- Use `app.components.select` for language picking and represent flags with Twemoji SVG image assets.
- Keep backend locale payload unchanged: `code`, `name`, `enabled`, and `is_default`.

State:
  - Done:
    - Read `CONTINUITY.md` at the start of the turn.
    - User requested: "Please help me improve Collection Option i18n UI".
    - Inspected `collectionI18nOptionsTab.js`, nearby collection option tabs, `UI_DOCS.md`, and shared select/list/form CSS patterns.
    - Improved the collection i18n options tab with a section heading, switch-style enable control, clearer help text, enabled-locale-aware default locale select labels, selected-field count, and Select all/Clear field actions.
    - Ran `cd ui && npm run build`; build passed. dprint still emitted the existing cache write warning outside the workspace but formatted 1 file and Vite completed successfully.
    - User requested Locales Settings UI improvements based on a reference image and Twemoji flags.
    - Reworked Locales Settings with a table-style locale list, Twemoji flag images, default check buttons, enable/disable and delete row actions, and a collapsible add-language panel.
    - Added a searchable language picker backed by `app.components.select`, with common language presets and editable display name/code fields.
    - Added `ui/src/css/locales.css` and imported it from `_main.css`.
    - Ran `cd ui && npm run build`; build passed. dprint still emitted the existing cache write warning outside the workspace but formatted 1 file and Vite completed successfully.
    - User asked to fix flag URLs to `https://cdnjs.cloudflare.com/ajax/libs/twemoji/14.0.0/svg/`.
    - Updated `TWEMOJI_BASE_URL` and rebuilt UI. Build passed; dprint still emitted the existing cache write warning outside the workspace.
  - Now:
    - Preparing final summary.
  - Next:
    - User can review the Locales Settings page in the admin UI.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: exact list of built-in language choices desired beyond common locales.

Working set (files/ids/commands):

- `/Users/suytbily/dev/gits/harry/pocketadmin/CONTINUITY.md`
- `/Users/suytbily/dev/gits/harry/pocketadmin/UI_DOCS.md`
- `/Users/suytbily/dev/gits/harry/pocketadmin/ui/src/collections/collectionI18nOptionsTab.js`
- `/Users/suytbily/dev/gits/harry/pocketadmin/ui/src/settings/locales/pageLocalesSettings.js`
- `/Users/suytbily/dev/gits/harry/pocketadmin/ui/src/settings/locales/localesList.js`
- `/Users/suytbily/dev/gits/harry/pocketadmin/ui/src/css/_main.css`
- `/Users/suytbily/dev/gits/harry/pocketadmin/ui/src/css/locales.css`
- `/Users/suytbily/dev/gits/harry/pocketadmin/ui/dist/index.html`
- `/Users/suytbily/dev/gits/harry/pocketadmin/ui/dist/assets/index-CZrJxHf1.js`
- `/Users/suytbily/dev/gits/harry/pocketadmin/ui/dist/assets/index-DIyOGyNl.css`
- `cd ui && npm run build`
