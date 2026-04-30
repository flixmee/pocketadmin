# PocketBase Admin UI Documentation

## Overview

The PocketAdmin UI is a Vue-like JavaScript module-based frontend built with Vite. It manages collections, records, fields, and real-time interactions for the PocketBase backend. The UI is located under `ui/` and builds to `ui/dist/`.

## Quick Start

```bash
cd ui
npm ci                # Install dependencies from lockfile
npm run dev           # Start development server (Vite)
npm run build         # Format code + build for production
```

**Build Tool:** Vite + dprint (code formatter)  
**Dependencies:** leaflet, pocketbase  
**Dev Dependencies:** dprint, vite

## Directory Structure

```
ui/
├── src/
│   ├── main.js                 # App bootstrap; imports all field types
│   ├── index.html              # Entry point template
│   ├── fields/                 # Field type modules (one per type)
│   │   ├── text/
│   │   ├── number/
│   │   ├── email/
│   │   ├── select/
│   │   ├── relation/
│   │   ├── file/
│   │   ├── date/
│   │   ├── checkbox/
│   │   ├── json/
│   │   ├── user/
│   │   ├── password/
│   │   ├── slug/                # NEW: slug field module
│   │   │   ├── init.js         # Register field type
│   │   │   ├── input.js        # Record input component
│   │   │   ├── view.js         # Record view component
│   │   │   └── settings.js     # Collection editor (field options)
│   │   └── ...
│   ├── base/                   # Shared components and utilities
│   │   ├── fieldSettings.js    # Field settings editor wrapper
│   │   ├── select.js           # Select/dropdown component
│   │   ├── modal.js
│   │   └── ...
│   ├── collections/            # Collection management views
│   ├── records/                # Record management views
│   ├── css/                    # Global and field-specific styles
│   │   ├── recordSummary.css  # Record summary/list display
│   │   ├── table.css          # Table layout and columns
│   │   └── ...
│   └── utils.js                # Helper functions (slugify, truncate, randomString, etc.)
├── dist/                       # Built output (generated)
├── package.json               # Dependencies and scripts
├── vite.config.js             # Vite configuration
└── dprint.json                # Code formatter config
```

## Field Type Architecture

### Registering a New Field Type

All field types are registered in the global `app.fieldTypes` object. Each field type is registered from its module in `ui/src/main.js`:

```javascript
// In ui/src/main.js
import "./fields/slug/init";
``` 

### Field Module Structure

A field type module exports a registration object with:

```javascript
// ui/src/fields/slug/init.js
import { input } from "./input";
import { settings } from "./settings";
import { view } from "./view";

window.app.fieldTypes.slug = {
    icon: "ri-hashtag",           // Icon class (Remix Icon)
    label: "Slug",                 // Display name
    settings,                      // Settings panel component (function)
    input,                         // Record input component (function)
    view,                          // Record view component (function)
    filterModifiers: () => ["lower"], // Available filter modifiers
    dummyData: () => "example-slug",  // Dummy/preview data
};
```

### Component Pattern: `input(props)`

The input component receives:
- `props.field` – the field definition object (has `name`, `type`, `required`, `help`, custom options)
- `props.record` – the record being edited (mutable)
- `props.collection` – the parent collection definition

Returns a tree of `t.*` functions (a reactive DOM builder):

```javascript
export function input(props) {
    return t.div(
        { className: "record-field-input field-type-text field-type-slug" },
        t.input({
            type: "text",
            name: () => props.field.name,
            value: () => props.record[props.field.name] || "",
            oninput: (e) => {
                props.record[props.field.name] = e.target.value;
            },
        }),
    );
}
```

### Component Pattern: `view(props)`

The view component displays a read-only record value. Receives same props as input.

```javascript
export function view(props) {
    return t.div({ className: "record-field-view field-type-text field-type-slug" }, () => {
        const value = props.record[props.field.name] || "";
        if (value == "") {
            return t.span({ className: "missing-value" });
        }
        return t.span({ className: "txt txt-ellipsis" }, app.utils.truncate(value));
    });
}
```

### Component Pattern: `settings(props)`

The settings component edits collection-level field options (min length, max length, required flag, etc.). Called from the collection editor when adding/editing a field.

```javascript
export function settings(props) {
    const uniqueId = "f_" + app.utils.randomString();

    return app.components.fieldSettings(props, {
        // Optional header (often used for attached-field selectors)
        header: t.div(...),
        
        // Main content area (options like min/max/help)
        content: () => t.div(...),
        
        // Footer area (often required flag and other toggles)
        footer: () => [ ... ],
    });
}
```

**Helper:** `app.components.fieldSettings(props, config)` wraps field options in the standard UI layout.

### Selectors and Field References

To refer to other fields (e.g., for an attachment or relation), use **field id** for stability across renames:

```javascript
// Get attachment options (all fields except self)
const options = (props.collection?.fields || [])
    .filter((field) => !field[toDeleteProp] && field.name != props.field.name)
    .map((field) => ({
        value: field.id,  // Use field.id, NOT field.name
        label: () => t.span({}, field.name, " (", field.type, ")"),
    }));

// In the field config:
props.field.attachedField = fieldId; // Store id, not name
```

## Key UI Utilities

### `app.utils`

- `slugify(text, separator)` – Convert text to URL-safe slug
- `truncate(text, length)` – Truncate text with ellipsis
- `randomString(length)` – Generate random string for unique IDs
- `fallbackFieldIcon` – Default icon when field type icon is missing

### `app.components`

- `fieldSettings(props, config)` – Wrap field options in standard layout
- `select(options, value, onchange)` – Dropdown/select component
- `copyButton(text)` – Copy-to-clipboard button
- `modal(title, content, buttons)` – Modal dialog

### `app.attrs`

- `tooltip(text, position)` – Accessible tooltip attribute

### `app.fieldTypes`

Registry of all field types; accessed as `app.fieldTypes[typeName]`. Has properties: `icon`, `label`, `settings`, `input`, `view`, `filterModifiers`, `dummyData`.

## CSS Conventions

### Class Naming

- `.record-field-input` – wrapper for input components
- `.record-field-view` – wrapper for view components
- `.field-type-{type}` – type-specific styling (e.g., `.field-type-slug`, `.field-type-text`)
- `.col-field-type-{type}` – table column styling

### Record Summary & Tables

When adding a new field type, update:

- **[ui/src/css/recordSummary.css](ui/src/css/recordSummary.css)**: Add `.field-type-slug` rules alongside `.field-type-text` to preserve consistent widths/truncation.
- **[ui/src/css/table.css](ui/src/css/table.css)**: Add `.col-field-type-slug` rules alongside `.col-field-type-text` to consistent table column widths.

Example:
```css
.field-type-text.field-type-slug,
.col-field-type-slug {
    max-width: 180px;
}
```

## Slug Field Implementation (Example)

### Files

1. **[ui/src/fields/slug/init.js](ui/src/fields/slug/init.js)** – Register the field type with icon, label, and re-exports.
2. **[ui/src/fields/slug/input.js](ui/src/fields/slug/input.js)** – Record input component; normalizes text to slug format on input.
3. **[ui/src/fields/slug/view.js](ui/src/fields/slug/view.js)** – Record view component; displays slug with copy button.
4. **[ui/src/fields/slug/settings.js](ui/src/fields/slug/settings.js)** – Settings panel; includes attached-field selector, min/max length, required flag.

### Key Features

- **Normalization**: lowercases, trims, replaces non-alphanumeric chars with `-`, collapses multiple `-`.
- **Attached Field**: Can be synced from another field (selected by field id, not name).
- **Placeholder**: When attached, input shows `"Synced from {fieldName}"` placeholder.
- **Validation**: min/max length constraints, required flag.

### Usage in Records

1. **Create Record**: User types in the slug input; text is normalized in real-time.
2. **Attached Sync**: If an attached-field is set, the backend's record interceptor syncs the source value to the slug field automatically.
3. **View**: The slug displays with a copy-to-clipboard button and truncation if needed.

## Common Patterns

### Making Components Reactive

Use `() => expression` to wrap values/classes that change:

```javascript
t.input({
    value: () => props.record[props.field.name] || "",  // Reactive
    className: () => (props.field.required ? "required" : ""),
})
```

### Unique IDs for Labels

Generate unique IDs to link labels to inputs:

```javascript
const uniqueId = "slug_" + app.utils.randomString();
t.label({ htmlFor: uniqueId }, "Slug"),
t.input({ id: uniqueId, ... })
```

### Field Existence Checks

Check the `toDeleteProp` flag to filter out fields marked for deletion in the collection editor:

```javascript
import { toDeleteProp } from "@/base/fieldSettings";

.filter((field) => !field[toDeleteProp])
```

### Icons

All icons use **Remix Icon** classes (e.g., `ri-hashtag`, `ri-text-wrap`, `ri-link`). Browse the Remix Icon library for available icons: https://remixicon.com/

## Build & Dev Workflow

### Development

```bash
npm run dev
```
Starts Vite dev server on `http://localhost:3000` (or similar). Hot reload enabled.

### Production Build

```bash
npm run build
```
1. Runs `dprint fmt` to format code.
2. Runs `vite build` to optimize and minify for production.
3. Outputs to `ui/dist/`.

## Integration with Backend

The UI communicates with the Go backend via:

- **HTTP API**: Record CRUD, collection schema management, real-time subscriptions.
- **PocketBase JS SDK**: `pocketbase` npm package provides the client library.
- **Field Interceptors**: Backend field types can intercept record saves to sync/compute values (e.g., slug attached-field syncing).

Field options (like `attachedField`, `min`, `max`) are stored in the backend's `options` JSON column for the field definition.

## Troubleshooting

### Build Fails with "dprint not found"

Run `npm ci` to install dependencies from the lockfile before running `npm run build`.

### Field Type Not Showing in Admin

1. Ensure the field module is imported in `ui/src/main.js`.
2. Check browser console for errors in the field registration.
3. Reload the page or restart `npm run dev`.

### UI Layout Issues

1. Add field-type-specific CSS if the new field needs custom width/styling.
2. Inspect CSS classes in browser DevTools to see what's being applied.
3. Check `recordSummary.css` and `table.css` for conflicting rules.

### Attached Field Selector Not Working

1. Ensure field references use field `.id`, not `.name`.
2. Check that the selector options are being generated correctly.
3. Verify the `onchange` callback is updating `props.field.attachedField`.

## File Structure Reference

| File/Folder | Purpose |
|-------------|---------|
| `ui/src/main.js` | App bootstrap; import all field types here |
| `ui/src/fields/*/init.js` | Field type registration |
| `ui/src/fields/*/input.js` | Record input component |
| `ui/src/fields/*/view.js` | Record view component |
| `ui/src/fields/*/settings.js` | Collection editor (field options) |
| `ui/src/base/fieldSettings.js` | Field settings wrapper + utilities |
| `ui/src/base/select.js` | Dropdown/select component |
| `ui/src/css/recordSummary.css` | Record list/summary styles |
| `ui/src/css/table.css` | Table column styles |
| `ui/src/utils.js` | Global utility functions |
| `ui/dist/` | Built output (generated) |
| `package.json` | Dependencies & build scripts |
| `vite.config.js` | Vite configuration |

---

**Last Updated:** April 30, 2026  
**Covers:** Slug field implementation, general field type architecture, UI utilities, CSS conventions, build workflow.
