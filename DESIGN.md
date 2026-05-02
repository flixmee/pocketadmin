# Media Management UI — Design Document

**Version:** 1.0  
**Style:** Black & White Minimal  
**Last Updated:** May 2026

---

## 1. Overview

This document defines the design system, screen specifications, component architecture, and interaction patterns for the Media Management UI. The aesthetic is strict black-and-white — no color accents, no gradients — relying entirely on typography, whitespace, and contrast to communicate hierarchy and state.

---

## 2. Design Principles

**Contrast over color.** Every state change (hover, selected, active, disabled) is expressed through contrast shifts within the grayscale ramp, never through hue.

**Typography does the heavy lifting.** Font weight and size differences carry hierarchy. Labels, captions, and headings are visually distinct without color.

**Whitespace is structural.** Generous spacing between elements replaces decorative dividers. Breathing room signals grouping.

**Density on demand.** The default view is spacious (grid). Users can switch to a compact list view when working with many files.

**Black is the single accent.** Pure black (`#000000`) is used exclusively for primary actions — the main CTA button, active nav items, selected checkboxes. Everything else lives in grays.

---

## 3. Design Tokens

### 3.1 Color Palette

| Token            | Hex       | Usage                                      |
|------------------|-----------|--------------------------------------------|
| `color-black`    | `#000000` | Primary actions, selected state, headings  |
| `color-900`      | `#1A1A1A` | Body text, strong labels                   |
| `color-700`      | `#3D3D3D` | Secondary text, icons                      |
| `color-500`      | `#737373` | Muted text, placeholders, captions         |
| `color-300`      | `#B3B3B3` | Disabled text, inactive borders            |
| `color-200`      | `#E0E0E0` | Borders, dividers, input outlines          |
| `color-100`      | `#F5F5F5` | Hover backgrounds, surface fills           |
| `color-50`       | `#FAFAFA` | Page background                            |
| `color-white`    | `#FFFFFF` | Card backgrounds, modal surfaces           |

> No other colors are permitted. Status states (error, warning, success) use icons + text labels, not color fills.

### 3.2 Typography

**Font Family:** `DM Mono` (monospace) for file names and metadata. `Syne` (geometric sans) for all UI labels and headings.

```
Font stack:
  Display  — Syne, sans-serif
  UI       — Syne, sans-serif
  Code/ID  — DM Mono, monospace
```

**Type Scale:**

| Role       | Size  | Weight | Line Height | Usage                        |
|------------|-------|--------|-------------|------------------------------|
| `h1`       | 28px  | 500    | 1.2         | Page title                   |
| `h2`       | 20px  | 500    | 1.3         | Section heading              |
| `h3`       | 16px  | 500    | 1.4         | Card title, panel heading    |
| `body`     | 14px  | 400    | 1.6         | General UI text              |
| `caption`  | 12px  | 400    | 1.5         | Metadata, timestamps         |
| `mono`     | 13px  | 400    | 1.5         | File names, sizes, IDs       |
| `label`    | 11px  | 500    | 1.0         | Tags, badges, chip labels    |

### 3.3 Spacing Scale

Base unit: `4px`

```
space-1  =  4px
space-2  =  8px
space-3  = 12px
space-4  = 16px
space-5  = 20px
space-6  = 24px
space-8  = 32px
space-10 = 40px
space-12 = 48px
```

### 3.4 Border Radius

```
radius-sm  = 4px   — inputs, tags, badges
radius-md  = 8px   — cards, buttons, panels
radius-lg  = 12px  — modals, drawers
radius-xl  = 16px  — full-page containers
```

### 3.5 Shadows

Shadows are used sparingly. No color tints.

```
shadow-sm  = 0 1px 3px rgba(0,0,0,0.08)    — cards at rest
shadow-md  = 0 4px 12px rgba(0,0,0,0.12)   — modals, dropdowns
shadow-lg  = 0 8px 24px rgba(0,0,0,0.16)   — drag-and-drop preview
```

---

## 4. Screen Specifications

### 4.1 Library / Dashboard

The main view. Displays all media assets in a responsive grid with a search bar, filter row, and status bar.

**Layout:**
```
┌─────────────────────────────────────────────┐
│  Top Bar: Logo · Search · Upload button     │
├──────────┬──────────────────────────────────┤
│          │  Filter chips · View toggle      │
│ Sidebar  ├──────────────────────────────────┤
│  Nav     │                                  │
│          │  Media Grid (4-col default)      │
│          │                                  │
│          ├──────────────────────────────────┤
│          │  Status bar: N items · N selected│
└──────────┴──────────────────────────────────┘
```

**Grid card anatomy:**
- Thumbnail area: 100% width, 16:9 or 1:1 ratio, `color-100` background
- File type badge: top-left corner, black pill, white label text
- File name: 13px mono, truncated with ellipsis
- File size + date: 12px caption, `color-500`
- Hover state: card border changes from `color-200` → `color-700`, subtle `shadow-sm` lifts
- Selected state: black 2px border, black filled checkbox top-right

**Toolbar / filter row:**
- Search input: full-width, 40px height, `color-200` border, 8px radius
- Filter chips: `All / Images / Videos / Audio / Docs` — inactive = `color-100` bg + `color-700` text; active = black bg + white text
- View toggle: two icon buttons (grid / list), active icon filled black

### 4.2 Detail Panel (Right Drawer)

Opens on single asset click. Slides in from the right, 320px wide on desktop.

**Sections:**
1. **Preview area** — full-width, fixed 200px height, checkered background for transparent assets
2. **File name** — editable inline on click, `h3` weight
3. **Metadata table** — two-column, key in `color-500`, value in `color-900`
   - Type, Size, Dimensions (if image), Duration (if video/audio), Uploaded, Modified
4. **Tags row** — horizontally scrollable pill list, `+` button to add
5. **Action buttons** — Download (black, primary), Rename, Move, Delete (spaced row)

**Close behavior:** Click outside drawer, press `Esc`, or click the × button.

### 4.3 Upload Modal

Triggered by the "Upload" button in the top bar.

**Structure:**
- Overlay: `rgba(0,0,0,0.45)` backdrop
- Modal card: white, 520px wide, `radius-lg`, `shadow-md`
- Drop zone: dashed `color-300` border, `radius-md`, 180px height
  - Default state: upload icon + "Drop files here or click to browse"
  - Drag-over state: border → black, background → `color-100`
- File queue: list of queued files below the drop zone, each showing filename + size + remove button
- Progress bar: `color-200` track, black fill, percentage label
- Action row: "Upload X files" (black button) + "Cancel" (ghost button)

**Accepted formats:** Images (PNG, JPG, GIF, WebP, SVG), Video (MP4, MOV, WebM), Audio (MP3, WAV, OGG), Documents (PDF, DOCX)

### 4.4 Bulk Actions Toolbar

Appears at the top of the content area when 1 or more items are selected. Replaces the filter row.

**Anatomy:**
- Left: checkbox count ("3 selected") + "Select all" link
- Center: action buttons — Move, Tag, Download, Duplicate
- Right: Delete (with confirmation step) + Deselect (×)

**Style:** Black background bar, white text and icons. Slides down with a 200ms ease animation.

---

## 5. Components

### 5.1 Button

| Variant   | Background  | Text         | Border          | Hover              |
|-----------|-------------|--------------|-----------------|---------------------|
| Primary   | `#000000`   | `#FFFFFF`    | none            | `#1A1A1A` bg        |
| Secondary | `#FFFFFF`   | `#000000`    | 1px `color-200` | `color-100` bg      |
| Ghost     | transparent | `#3D3D3D`    | none            | `color-100` bg      |
| Danger    | `#FFFFFF`   | `#000000`    | 1px `color-200` | `color-100`, icon red-labeled |
| Disabled  | `color-100` | `color-300`  | none            | no change           |

Height: 36px (default), 28px (compact). Border radius: 8px.

### 5.2 Input / Search

- Height: 40px
- Border: 1px solid `color-200`
- Focus border: 1px solid `#000000`
- Placeholder: `color-300`
- Border radius: 8px
- Padding: 0 12px
- Icon prefix: 16px, `color-500`

### 5.3 Checkbox

- Default: 18×18px square, 4px radius, `color-200` border
- Checked: black fill, white checkmark SVG
- Indeterminate: black fill, white dash
- Hover: `color-700` border

### 5.4 Tag / Chip

- Height: 24px
- Padding: 0 10px
- Background: `color-100`
- Text: 11px label, `color-700`
- Border radius: 100px (full pill)
- Active: black bg, white text

### 5.5 File Type Badge

- Height: 20px
- Padding: 0 6px
- Background: `#000000`
- Text: 10px label, white, uppercase
- Border radius: 4px
- Examples: `IMG`, `VID`, `PDF`, `AUD`, `SVG`

### 5.6 Progress Bar

- Track: `color-200`, 6px height, full radius
- Fill: `#000000`, animated width transition 300ms ease
- Label: caption below, right-aligned percentage

---

## 6. Layout & Grid

### 6.1 App Shell

```
┌─────────────────────────────────┐
│  Top Bar          48px          │
├─────────┬───────────────────────┤
│         │                       │
│ Sidebar │   Main Content Area   │
│  220px  │   flex-1              │
│         │                       │
└─────────┴───────────────────────┘
```

- Top bar: fixed, `color-white` bg, 1px bottom border `color-200`
- Sidebar: fixed left, `color-white` bg, 1px right border `color-200`
- Main area: scrollable, `color-50` background

### 6.2 Media Grid

| Breakpoint | Columns | Card min-width |
|------------|---------|----------------|
| < 640px    | 2       | 140px          |
| 640–1024px | 3       | 160px          |
| > 1024px   | 4       | 180px          |
| List mode  | 1       | full-width     |

Gap between cards: `space-4` (16px)

### 6.3 List View Row

- Height: 52px
- Columns: Checkbox · Thumbnail (40×40px) · Name · Type · Size · Modified · Actions
- Hover: `color-100` background
- Selected: `color-100` background + black left border 2px

---

## 7. Interaction Patterns

### 7.1 Selection

- Single click: open detail panel (no selection)
- Checkbox click: select item, bulk bar appears
- `Shift + click`: range select
- `Cmd/Ctrl + A`: select all visible
- `Esc`: deselect all

### 7.2 Drag and Drop

- Items can be dragged to folder items in the sidebar
- Drag preview: semi-transparent card with `shadow-lg`, stacked badge if multiple items
- Valid drop target: black dashed border highlight
- Invalid drop target: no highlight, cursor `not-allowed`

### 7.3 Keyboard Navigation

| Key              | Action                        |
|------------------|-------------------------------|
| `↑ ↓ ← →`       | Navigate grid cells           |
| `Enter / Space`  | Open detail panel             |
| `Delete`         | Delete selected (with confirm)|
| `Esc`            | Close modal / deselect all    |
| `Cmd/Ctrl + U`   | Open upload modal             |
| `Cmd/Ctrl + A`   | Select all                    |

### 7.4 Empty States

- **No media yet:** Centered illustration (simple line-art upload icon), heading "No files yet", body "Upload your first file to get started", primary button.
- **Search no results:** Icon + "No results for '[query]'", suggest clearing search.
- **Folder empty:** Icon + "This folder is empty", drag-and-drop hint.

---

## 8. Animation & Motion

All animations use `ease-out` timing. Respect `prefers-reduced-motion`.

| Element           | Property          | Duration | Easing     |
|-------------------|-------------------|----------|------------|
| Bulk bar in/out   | `translateY`      | 200ms    | ease-out   |
| Detail panel      | `translateX`      | 250ms    | ease-out   |
| Modal             | `opacity + scale` | 200ms    | ease-out   |
| Card hover lift   | `box-shadow`      | 150ms    | ease       |
| Progress fill     | `width`           | 300ms    | ease       |
| Checkbox check    | `opacity + scale` | 100ms    | ease-out   |

---

## 9. Accessibility

- All interactive elements have a visible focus ring: `2px solid #000000`, `outline-offset: 2px`
- Color is never the sole differentiator — states are also communicated through icons, borders, or text
- Images include `alt` text derived from file name
- Modal traps focus when open, returns focus on close
- ARIA roles: `role="grid"` on media grid, `role="gridcell"` on cards, `aria-label` on icon buttons
- Minimum contrast ratios: 4.5:1 for body text, 3:1 for large text and UI components

---

## 10. File Structure (Frontend)

```
src/
├── components/
│   ├── layout/
│   │   ├── AppShell.tsx
│   │   ├── Sidebar.tsx
│   │   └── TopBar.tsx
│   ├── media/
│   │   ├── MediaGrid.tsx
│   │   ├── MediaCard.tsx
│   │   ├── MediaListRow.tsx
│   │   └── DetailPanel.tsx
│   ├── upload/
│   │   ├── UploadModal.tsx
│   │   ├── DropZone.tsx
│   │   └── FileQueue.tsx
│   └── ui/
│       ├── Button.tsx
│       ├── Input.tsx
│       ├── Checkbox.tsx
│       ├── Tag.tsx
│       ├── Badge.tsx
│       ├── ProgressBar.tsx
│       └── BulkBar.tsx
├── tokens/
│   └── design-tokens.css
└── styles/
    └── globals.css
```

---

## 11. Design Checklist

Before handoff, verify:

- [ ] All text meets 4.5:1 contrast ratio against its background
- [ ] Every interactive element has a defined hover, focus, active, and disabled state
- [ ] All states are expressed without color alone
- [ ] Grid reflows correctly at 320px, 768px, and 1280px breakpoints
- [ ] Empty states are designed for all three main containers
- [ ] Keyboard navigation tested end-to-end
- [ ] Upload flow tested with slow (2G) network simulation
- [ ] `prefers-reduced-motion` disables all transitions
- [ ] Dark mode variant is scoped for future implementation (tokens only, no hardcoded hex)

---

*End of DESIGN.md*
