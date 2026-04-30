# AGENTS.md

## Project Overview

PocketAdmin is a Go module for PocketBase. The repository contains the core library, HTTP/API handlers, CLI entry points, plugins, tools, tests, and a small frontend under `ui/`.

## Top-Level Structure

- `pocketbase.go` - root package entry point for constructing and starting the app.
- `cmd/` - Cobra commands used by the main executable, such as `serve` and admin commands.
- `core/` - main application framework, database layer, models, queries, events, fields, validation, and bootstrap logic.
- `apis/` - HTTP handlers, middleware, realtime, records, auth flows, backups, logs, settings, and server wiring.
- `plugins/` - optional extensions for JS VM hooks, migration commands, and GitHub update support.
- `tools/` - reusable utility packages shared across the codebase.
- `examples/base/` - minimal standalone app example and reference entry point for building a PocketBase binary.
- `migrations/` - built-in schema/data migrations embedded into the app.
- `tests/` - higher-level test assets and integration coverage.
- `ui/` - frontend assets and build files for the admin UI.

## Directory Notes

- `core/` is the best place to look for data model, query, validation, and lifecycle behavior.
- `apis/` is where request/response behavior lives, including route registration and middleware.
- `tools/` contains small shared packages rather than application logic.
- `ui/` is a separate frontend project with its own Node/Vite setup; keep Go and frontend changes isolated unless the feature spans both.

## Entry Points

- The main library entry point is `pocketbase.New()` in `pocketbase.go`.
- The default CLI server command is registered from `cmd/serve.go`.
- The minimal runnable example is `examples/base/main.go`.

## Working Notes

- Prefer focused changes in the package that already owns the behavior.
- Keep tests close to the code they cover; many packages already have matching `_test.go` files.
- The module targets Go 1.25.0 as declared in `go.mod`.
- Standard verification is `go test ./...`.
- When touching the frontend, review `ui/package.json` and `ui/vite.config.js` for build/runtime details.
- **When updating the UI**, refer to `UI_DOCS.md` for field type architecture, component patterns, utilities, CSS conventions, and slug field examples.

## Continuity Ledger (compaction-safe)

Maintain a single Continuity Ledger for this workspace in `CONTINUITY.md`. The ledger is the canonical session briefing designed to survive context compaction; do not rely on earlier chat text unless it's reflected in the ledger.

### How it works

- At the start of every assistant turn: read `CONTINUITY.md`, update it to reflect the latest goal/constraints/decisions/state, then proceed with the work.
- Update `CONTINUITY.md` again whenever any of these change: goal, constraints/assumptions, key decisions, progress state (Done/Now/Next), or important tool outcomes.
- Keep it short and stable: facts only, no transcripts. Prefer bullets. Mark uncertainty as `UNCONFIRMED` (never guess).
- If you notice missing recall or a compaction/summary event: refresh/rebuild the ledger from visible context, mark gaps `UNCONFIRMED`, ask up to 1-3 targeted questions, then continue.

### `functions.update_plan` vs the Ledger

- `functions.update_plan` is for short-term execution scaffolding while you work (a small 3-7 step plan with pending/in_progress/completed).
- `CONTINUITY.md` is for long-running continuity across compaction (the "what/why/current state"), not a step-by-step task list.
- Keep them consistent: when the plan or state changes, update the ledger at the intent/progress level (not every micro-step).

### In replies

- Begin with a brief "Ledger Snapshot" (Goal + Now/Next + Open Questions). Print the full ledger only when it materially changes or when the user asks.

### `CONTINUITY.md` format (keep headings)

- Goal (incl. success criteria):
- Constraints/Assumptions:
- Key decisions:
- State:
  - Done:
  - Now:
  - Next:
- Open questions (UNCONFIRMED if needed):
- Working set (files/ids/commands):