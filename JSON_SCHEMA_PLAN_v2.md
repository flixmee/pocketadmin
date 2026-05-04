# JSON Schema Plan
**PocketBase JSON Field — Backend Validation & Admin UI**
Version 2.0 | Status: Draft | Target: draft-07

---

## Goal

Provide a clear, maintainable plan for JSON Schema support in PocketBase JSON fields, covering backend Go validation via PocketBase hooks, admin UI configuration, error handling, and rollout criteria.

## Current State

- Backend JSON field supports optional schema persistence and validation.
- Admin UI supports enabling schema validation and editing schema rules.
- Schema editor has both visual and raw JSON modes.

---

## PocketBase Integration Points

This section is PocketBase-specific and describes exactly where schema validation hooks into the framework.

### Validation Hooks

Schema validation runs **synchronously** in PocketBase's before-save hooks, blocking the save on failure. No async validation path is used.

- Record creates: `OnRecordBeforeCreateRequest`
- Record updates: `OnRecordBeforeUpdateRequest`
- Both hooks call the same shared `validateJSONField(record, field, schema)` function.

### Go Plugin / Registration

Validation is registered as a custom record validator in `main.go` (or a dedicated plugin file). It is not a modification to PocketBase core, ensuring upgrade safety.

```go
// Example hook registration (main.go)
app.OnRecordBeforeCreateRequest().Add(func(e *core.RecordCreateEvent) error {
    return validateJSONSchemaFields(e.Record)
})
app.OnRecordBeforeUpdateRequest().Add(func(e *core.RecordUpdateEvent) error {
    return validateJSONSchemaFields(e.Record)
})
```

### Schema Storage

Schemas are stored as a property inside the JSON field's `options` object, which PocketBase serializes as part of the collection definition JSON.

- **Storage location:** collection schema JSON, inside each JSON field's `options` property.
- The schema string is stored as compact JSON. Complex schemas should be kept focused to avoid bloating the collection definition.
- **Recommended size limit:** schemas exceeding 50KB should trigger a settings-validation warning.

### JSON Schema Draft Version

This implementation targets **JSON Schema draft-07**. Rationale:

- Broadest library support in the Go ecosystem.
- Sufficient for all planned visual-mode constraints.
- Avoids lock-in risk from 2019-09 / 2020-12 breaking keyword changes.

The targeted draft version must be stored with the schema (e.g., as a `$schema` property) to enable future migrations.

### Validation Library

**Go library: `santhosh-tekuri/jsonschema/v6`**

- Full draft-07 support.
- Returns structured errors with instance paths — used for error message generation.
- Schema compilation is performed at settings-save time to catch structural errors early.
- Compiled schema instances should be cached in memory keyed by `collection + field name` to avoid recompilation on every record save.

---

## Scope

### In Scope

- JSON field schema definition and persistence in field options.
- Synchronous record-time schema validation in PocketBase before-save hooks.
- Admin UI configuration for schema setup (enable/disable, visual and raw editing).
- Structured validation error responses via the REST API.
- Focused test coverage and build verification.

### Out of Scope

- Full JSON Schema draft feature parity in visual mode.
- Automated schema migration between draft versions.
- Runtime schema registry shared across fields.
- Retroactive validation of pre-existing records when schema is first enabled.

---

## Functional Requirements

1. Users can enable or disable schema validation per JSON field.
2. Users can configure schema via a dialog-based editor.
3. Visual mode supports common constraints: root type, object properties and required flags, array item type, string length limits, number/integer min-max limits.
4. Raw JSON mode supports all draft-07 schema keywords.
5. Unsupported advanced schemas remain editable in raw mode without data loss.
6. Invalid schema definitions fail settings validation with a clear error.
7. Non-conforming record values fail field validation with a structured error response.
8. Schema validation is skipped when the JSON field value is `null` or empty string. Required-field enforcement remains the responsibility of the existing `required` flag.
9. Enabling schema validation on a field with existing records does not retroactively validate those records. Existing records are only validated on their next create or update operation.
10. Only admin users (collection owners) can configure schema settings for a field.

---

## Error Handling

### Validation Error Format

Validation errors follow a structured format to support both UI display and API consumers. For nested JSON fields, path-aware errors are required.

```json
{
  "code": 400,
  "message": "Failed to create record.",
  "data": {
    "fieldName": {
      "code": "validation_json_schema",
      "message": "data.address.zip must match pattern '^[0-9]{5}$'"
    }
  }
}
```

- Each failing field produces one error entry — only the first schema violation is returned for that field.
- The message includes the JSON path (e.g., `data.address.zip`) for nested violations.
- Schema compilation errors (invalid schema) appear on the field's settings object, not the record.

### HTTP Status Codes

| Scenario | HTTP Status |
|---|---|
| Record value fails schema validation | 400 Bad Request |
| Schema definition is structurally invalid (settings save) | 400 Bad Request |
| Field value is null / empty (validation skipped) | 200 OK (or 201) |

---

## Technical Plan

### 1. Data Model and Validation

- Field `options` object gains a `jsonSchema` property (string, nullable).
- Settings validation compiles the schema string using `santhosh-tekuri/jsonschema/v6` to verify structural validity.
- Compiled schema instances are cached in memory (`map[collectionId+fieldName]*jsonschema.Schema`) and invalidated on collection update.
- Record validation: null/empty field values bypass schema check. Non-null values are unmarshalled and validated against the compiled schema.
- Validation errors return a single structured message with the JSON instance path.

### 2. UI Configuration Flow

- Schema enable/disable toggle remains inline in field settings.
- Schema authoring moves to a dedicated modal dialog (not inline).
- **Dialog — visual mode:** supports root type, object properties + required flags, array item type, string length limits, number/integer min/max.
- **Dialog — raw mode:** full draft-07 JSON editing with syntax highlighting.
- Visual mode shows a clear "This schema cannot be represented visually" fallback when the current schema uses unsupported keywords, switching to raw mode automatically.
- When schema validation is enabled, the admin UI shows a dismissable info banner: *"Existing records are not retroactively validated. They will be validated on next save."*

### 3. Null and Empty Value Handling

> **Policy:** Schema validation is skipped entirely when the JSON field value is `null`, empty string (`""`), or absent from the request payload. The existing `required` field flag handles mandatory-field enforcement. Schema validation only applies to non-null, non-empty JSON values.

### 4. Existing Record Migration Policy

> **Policy:** Enabling schema validation on a field with existing records does **not** retroactively validate those records. Existing records become subject to schema validation only when they are next created or updated.
>
> **Future enhancement (v2):** an optional "audit existing records" admin action that runs validation against all current records and reports non-conforming ones, without blocking any saves.

### 5. Schema Size and Complexity

- Maximum schema size: **50KB** (enforced at settings-save time).
- Maximum nesting depth: **10 levels** (enforced at schema compilation).
- Schemas exceeding these limits are rejected with a clear settings error.

### 6. Testing Strategy

#### Backend unit tests

- Valid draft-07 schema is accepted at settings save.
- Structurally invalid schema is rejected at settings save.
- Record value matching schema passes validation.
- Record value not matching schema fails validation with correct error shape.
- Null field value bypasses schema validation (no error).
- Empty string field value bypasses schema validation (no error).
- Schema exceeding size limit is rejected.
- Compiled schema cache is invalidated when collection is updated.

#### API integration tests

- `POST /api/collections/:id/records` returns 400 with structured error when value fails schema.
- `PATCH /api/collections/:id/records/:recordId` returns 400 on schema failure.
- Null field value in POST/PATCH returns 200/201 without schema error.

#### UI verification

- Build passes after settings/dialog changes (`npm run build` — zero errors).
- Manual smoke test: enable schema, configure via visual mode, save, reopen — settings preserved correctly.
- Manual smoke test: switch to raw mode, enter advanced schema, save, reopen — schema preserved without corruption.
- Manual smoke test: submit a record that violates the schema — error message displays correctly in the admin UI.

---

## Delivery Checklist

- [ ] Add `jsonSchema` property to JSON field options model
- [ ] Implement schema compilation and validation using `santhosh-tekuri/jsonschema/v6`
- [ ] Register `OnRecordBeforeCreateRequest` and `OnRecordBeforeUpdateRequest` hooks
- [ ] Implement compiled-schema cache with collection-update invalidation
- [ ] Add null/empty-value bypass logic in validation path
- [ ] Add schema size and depth limit enforcement at settings save
- [ ] Add schema validation to record save paths with structured error output
- [ ] Add schema editing support in admin UI (visual + raw modes)
- [ ] Move schema editor into a dedicated dialog modal
- [ ] Add "existing records not retroactively validated" info banner to UI *(new)*
- [ ] Add backend unit tests for all validation scenarios
- [ ] Add API integration tests for 400/200 schema error responses *(new)*
- [ ] Run UI build verification (`npm run build`)
- [ ] Complete manual smoke test checklist (3 scenarios above) *(new)*

---

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Visual editor cannot represent all schema constructs. | Raw JSON mode is a first-class fallback. Visual mode gracefully degrades with a clear message when the current schema uses unsupported keywords. |
| Validation errors become too verbose. | Return only the first relevant error per field with a concise, path-aware message. |
| Performance impact for frequent record validation. | Cache compiled schema instances per collection+field. Invalidate on collection update. |
| Schema serialization bloats collection definition JSON. | Enforce 50KB max schema size at settings save. Log a warning in dev mode for schemas over 10KB. |
| Draft version lock-in (draft-07 schemas hard to migrate later). | Store `$schema` keyword in every saved schema. This enables automated detection and migration tooling in future. |
| Existing records silently become non-conforming when schema is tightened. | Show info banner in UI when enabling schema. Future: add audit-records action to surface non-conforming records without blocking saves. |

---

## Future Enhancements

- Visual mode: add support for `enum`, `pattern`, `minItems`/`maxItems` constraints.
- Visual mode: add nested object editing and reusable schema snippet library.
- Raw editor: add pre-save linting hints (e.g., underline unknown keywords).
- Performance: evaluate schema compile caching for high-throughput workloads using `sync.Map`.
- Migration: add a "dry-run / audit existing records" admin action that validates all current records against the new schema and reports non-conforming ones before enabling.
- Draft migration: build a one-time migration tool to upgrade stored draft-07 schemas to draft 2020-12 when the Go library ecosystem matures.

---

## Acceptance Criteria

All of the following must be true before this work is considered complete:

1. A field owner can fully configure basic schema rules without writing raw JSON.
2. Advanced schemas remain editable and round-trip correctly via raw mode.
3. Invalid schema definitions are rejected at settings save with a clear error.
4. Non-conforming record values are rejected via the API with a structured 400 error that includes the JSON path of the failing property.
5. Null and empty field values are accepted without triggering schema errors.
6. Enabling schema on a field with existing records does not break those records.
7. Backend unit tests and API integration tests pass in CI.
8. Admin UI build passes with zero errors.
9. All three manual smoke test scenarios pass.

---

*JSON Schema Plan v2.0 — PocketBase JSON Field*
