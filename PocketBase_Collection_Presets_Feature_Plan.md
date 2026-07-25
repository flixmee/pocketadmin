# PocketBase Collection Presets - Feature Plan

## Overview

This feature allows PocketBase administrators to quickly create multiple
related collections from predefined presets such as:

-   Blog (Posts, Categories, Tags, Comments)
-   E-commerce (Products, Product Categories, Orders, Customers)
-   CMS (Pages, Menus, Media)
-   Project Management (Projects, Tasks, Labels)

The implementation should build on PocketBase's existing collection
import mechanism rather than introducing a new schema system.

------------------------------------------------------------------------

# Goals

-   Provide ready-to-use collection templates.
-   Import multiple related collections in a single transaction.
-   Preserve compatibility with existing collection export/import.
-   Allow future community and custom presets.

------------------------------------------------------------------------

# User Flow

1.  Open **Collections**.
2.  Click **Create Collection**.
3.  Select **Import from Preset**.
4.  Choose a preset.
5.  Preview collections and relationships.
6.  Configure options:
    -   Collection prefix
    -   Collections to import
    -   Sample data
7.  Validate.
8.  Confirm import.
9.  Create all collections in one transaction.

------------------------------------------------------------------------

# Preset Structure

Each preset is stored as JSON.

``` json
{
  "id": "blog",
  "name": "Blog",
  "version": "1.0.0",
  "description": "Basic blog preset",
  "collections": [],
  "sampleData": []
}
```

Recommended structure:

``` text
core/
    presets/
        blog.json
        ecommerce.json
```

The JSON files are embedded in the Go binary. Built-in presets are immutable at
runtime and must not contain database-specific collection IDs.

------------------------------------------------------------------------

# Relation Resolution

Instead of storing collection IDs directly, use symbolic references.

``` json
{
  "collectionRef": "$collection.categories"
}
```

Import process:

1.  Generate collection IDs.
2.  Build mapping.
3.  Replace symbolic references.
4.  Import final schema.

------------------------------------------------------------------------

# Conflict Strategy

### MVP

-   Stop import if a collection already exists.

Future improvements:

-   Skip
-   Rename
-   Merge

Support optional collection prefix:

    blog_posts
    blog_categories
    blog_tags

Prefix semantics are explicit: the API accepts a token such as `blog`, validates
it as an identifier, and inserts one underscore before each collection name.
An empty prefix leaves preset names unchanged. MVP imports are create-only and
never update a matching collection.

------------------------------------------------------------------------

# Backend APIs

## List Presets

    GET /api/collection-presets

## Preview

    POST /api/collection-presets/{id}/preview

Returns:

-   Collections to create
-   Relationships
-   Conflicts
-   Warnings

## Import

    POST /api/collection-presets/{id}/import

All three endpoints require superuser authentication. Preview and import accept:

``` json
{
  "prefix": "blog"
}
```

Preview returns the resolved schemas, relationships, conflicts, warnings, and a
`canImport` flag. Import reruns resolution and conflict checks so that it does
not trust stale preview state.

The dedicated `/api/collection-presets` route family avoids ambiguous matches
with existing collection-scoped routes such as
`/api/collections/{collection}/impersonate/{id}`.

------------------------------------------------------------------------

# Import Pipeline

    Load preset
        ↓
    Validate preset
        ↓
    Filter collections
        ↓
    Resolve dependencies
        ↓
    Apply prefix
        ↓
    Generate IDs
        ↓
    Resolve relations
        ↓
    Detect conflicts
        ↓
    Import transaction
        ↓
    Insert sample data

Phase 1 stops after the schema transaction. Sample records are added in Phase 2
with an explicit transaction design that prevents schema-only partial success.

------------------------------------------------------------------------

# Dependencies

Collections may depend on each other.

Example:

-   Posts → Categories
-   Posts → Tags

Importer should automatically include required collections or return
validation errors.

------------------------------------------------------------------------

# Sample Data

Keep sample records separate from schema.

Example references:

    $record.categories.general

Workflow:

1.  Insert parent records.
2.  Store generated IDs.
3.  Resolve references.
4.  Insert dependent records.

------------------------------------------------------------------------

# Suggested Components

    PresetImportModal
    ├── PresetGallery
    ├── PresetDetails
    ├── CollectionSelector
    ├── Preview
    ├── ConflictList
    └── Result

------------------------------------------------------------------------

# Validation

Validate:

-   Preset version
-   Duplicate names
-   Reserved names
-   Relation targets
-   Circular dependencies
-   API rules
-   Indexes
-   Existing collections
-   Prefix
-   Sample data references

------------------------------------------------------------------------

# Blog MVP

Collections:

-   Product categories
-   Tags
-   Posts

Posts includes:

-   title
-   slug
-   excerpt
-   content
-   cover
-   status
-   publishedAt
-   category relation
-   tags relation
-   author relation

The author relation targets the existing `_superusers` collection in Phase 1.
It is optional and treated as an external dependency, so prefixing affects the
three preset collections but not `_superusers`.

------------------------------------------------------------------------

# E-commerce Preset

Collections:

-   Product categories
-   Products
-   Product variants
-   Product images
-   Customers (auth collection)
-   Addresses
-   Carts
-   Cart items
-   Orders
-   Order items
-   Coupons
-   Reviews
-   Wishlists

The preset uses normalized child collections for variants, images, addresses,
cart items, and order items. Order items retain name, SKU, quantity, and price
snapshots while keeping optional product/variant relations. Product categories support
a self-referencing parent relation, and wishlists use a multi-product relation.
Sample data remains empty until the transactional sample-data work in Phase 2.

------------------------------------------------------------------------

# Jobs & Career Preset

Collections:

-   Companies
-   Jobs
-   Job categories
-   Applicants (auth collection)
-   Resumes
-   Applications
-   Skills
-   Locations

Jobs relate to a company, category, location, and required skills. Job categories
support a self-referencing parent relation. Applicants own resumes and can relate
to a location and skills; applications join an applicant, job, and optional resume.
Company logos and resume documents use shared media references. Sample data remains
empty until the transactional sample-data work in Phase 2.

------------------------------------------------------------------------

# Task Management Preset

Collections:

-   Workspaces
-   Members (auth collection)
-   Boards
-   Custom fields
-   Task groups
-   Task statuses
-   Task tags
-   Tasks
-   Task custom field values
-   Task updates
-   Task attachments

The preset follows a Monday.com-style board model. Tasks belong to a board and
group, support parent/subtask relationships, multiple assignees, board-scoped
custom statuses, tags, priority, budget and currency, start/due/completion dates,
progress, estimated hours, ordering, and board-scoped custom fields. Typed custom
field values are stored separately with one value per task and field. Updates
provide the item activity thread, and attachments use shared media references.
Every collection API rule requires an authenticated request. Sample data remains
empty until the transactional sample-data work in Phase 2.

------------------------------------------------------------------------

# Testing

## Unit

-   Load preset
-   Resolve relations
-   Prefix handling
-   Conflict detection
-   Dependency validation

## Integration

-   Successful import
-   Rollback on failure
-   Existing collections
-   Prefix import

## UI

-   Browse presets
-   Preview
-   Import
-   Error handling

------------------------------------------------------------------------

# Roadmap

## Phase 1

-   Blog preset
-   Embedded and versioned built-in preset catalog
-   Symbolic relation resolution after prefix and ID generation
-   Superuser-only list, preview, and import endpoints
-   Prefix validation and deterministic naming
-   Create-only conflict detection
-   Admin UI preset picker, relationship preview, and import result
-   Backend unit/API tests and UI build verification

## Phase 2

-   Collection selection
-   Automatic inclusion of required selected dependencies
-   Optional sample data with `$record.<collection>.<key>` references
-   One transaction covering schema and sample records
-   E-commerce, CMS, and Project Management presets
-   Dependency and sample-reference tests

## Phase 3

-   Export as preset
-   Import custom presets
-   Custom preset registration API for application code/plugins
-   Community preset providers with explicit trust and version policies
-   Compatibility checks and preset upgrade metadata

## Phase 4

-   Explicit `skip` and `rename` conflict strategies
-   Schema merge after field-level merge semantics are specified
-   Migration generation

------------------------------------------------------------------------

# MVP Acceptance Criteria

-   Import from preset is available.
-   Blog preset works.
-   Preview before import.
-   Detect conflicts.
-   Transaction rollback on failure.
-   Existing collections remain unchanged.
-   Display import summary.

------------------------------------------------------------------------

# Review Decisions

-   Presets extend the existing collection import format; they do not define a
    second schema format.
-   Collection IDs are generated from final prefixed names before symbolic
    relations are resolved.
-   Preview is side-effect free. Import repeats resolution and conflict checks,
    then calls the existing transactional importer with `deleteMissing=false`.
-   Phase 1 includes the UI because backend endpoints alone do not satisfy the
    documented administrator flow.
-   Circular collection relations are valid PocketBase schemas and are not
    rejected merely for being circular. Dependency cycles matter in Phase 2 for
    sample-record insertion ordering.
-   API rules and indexes remain ordinary exported collection properties and
    are validated by the existing collection validator during import.
