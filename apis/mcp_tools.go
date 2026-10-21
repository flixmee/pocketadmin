package apis

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/search"
	"github.com/spf13/cast"
)

const mcpMaxPerPage = 200

type mcpTool struct {
	Name        string
	Title       string
	Description string
	Permission  string
	ReadOnly    bool
	Destructive bool
	Properties  map[string]any
	Required    []string
	Handler     func(s *mcpServer, args mcpArgs) (any, error)
}

func (t *mcpTool) definition() map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": t.Properties,
	}
	if len(t.Required) > 0 {
		schema["required"] = t.Required
	}

	return map[string]any{
		"name":        t.Name,
		"title":       t.Title,
		"description": t.Description,
		"inputSchema": schema,
		"annotations": map[string]any{
			"title":           t.Title,
			"readOnlyHint":    t.ReadOnly,
			"destructiveHint": t.Destructive,
			"idempotentHint":  t.ReadOnly,
			"openWorldHint":   false,
		},
	}
}

func findMCPTool(name string) *mcpTool {
	for _, tool := range mcpTools {
		if tool.Name == name {
			return tool
		}
	}

	return nil
}

var mcpCollectionProp = map[string]any{
	"type":        "string",
	"description": "Collection name or id.",
}

var mcpExpandProp = map[string]any{
	"type":        "string",
	"description": "Optional comma separated relation fields to expand, e.g. `author,tags`.",
}

var mcpTools = []*mcpTool{
	{
		Name:        "list_collections",
		Title:       "List collections",
		Description: "Lists the collections accessible with this key together with a short summary of their fields.",
		Permission:  core.MCPPermissionCollectionsRead,
		ReadOnly:    true,
		Properties:  map[string]any{},
		Handler:     mcpListCollections,
	},
	{
		Name:        "get_collection",
		Title:       "Get collection",
		Description: "Returns the full schema of a single collection (fields with their options, API rules, indexes).",
		Permission:  core.MCPPermissionCollectionsRead,
		ReadOnly:    true,
		Properties:  map[string]any{"collection": mcpCollectionProp},
		Required:    []string{"collection"},
		Handler:     mcpGetCollection,
	},
	{
		Name:  "create_collection",
		Title: "Create collection",
		Description: "Creates a new collection. `data` uses the PocketBase collection JSON format, e.g. " +
			"`{\"name\":\"posts\",\"type\":\"base\",\"fields\":[{\"name\":\"title\",\"type\":\"text\",\"required\":true}]}`. " +
			"Not available for keys restricted to specific collections.",
		Permission: core.MCPPermissionCollectionsWrite,
		Properties: map[string]any{
			"data": map[string]any{"type": "object", "description": "The collection definition."},
		},
		Required: []string{"data"},
		Handler:  mcpCreateCollection,
	},
	{
		Name:  "update_collection",
		Title: "Update collection",
		Description: "Updates an existing collection. `data` is merged into the current collection definition. " +
			"Note that `fields`, when provided, replaces the whole fields list (fields missing from it are deleted together with their data), " +
			"so call get_collection first and send back the complete list.",
		Permission:  core.MCPPermissionCollectionsWrite,
		Destructive: true,
		Properties: map[string]any{
			"collection": mcpCollectionProp,
			"data":       map[string]any{"type": "object", "description": "The collection properties to change."},
		},
		Required: []string{"collection", "data"},
		Handler:  mcpUpdateCollection,
	},
	{
		Name:        "delete_collection",
		Title:       "Delete collection",
		Description: "Deletes a collection and all of its records.",
		Permission:  core.MCPPermissionCollectionsWrite,
		Destructive: true,
		Properties:  map[string]any{"collection": mcpCollectionProp},
		Required:    []string{"collection"},
		Handler:     mcpDeleteCollection,
	},
	{
		Name:        "list_records",
		Title:       "List records",
		Description: "Returns a paginated list of collection records, optionally filtered and sorted using the PocketBase filter syntax.",
		Permission:  core.MCPPermissionRecordsRead,
		ReadOnly:    true,
		Properties: map[string]any{
			"collection": mcpCollectionProp,
			"filter": map[string]any{
				"type":        "string",
				"description": "Optional filter expression, e.g. `status = 'active' && total > 10`.",
			},
			"sort": map[string]any{
				"type":        "string",
				"description": "Optional comma separated sort fields; prefix with `-` for DESC, e.g. `-created,title`.",
			},
			"page": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"description": "Page number (default 1).",
			},
			"perPage": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"maximum":     mcpMaxPerPage,
				"description": "Records per page (default 30, max 200).",
			},
			"skipTotal": map[string]any{
				"type":        "boolean",
				"description": "Skip the total count query for faster responses (totalItems and totalPages will be -1).",
			},
			"expand": mcpExpandProp,
		},
		Required: []string{"collection"},
		Handler:  mcpListRecords,
	},
	{
		Name:        "get_record",
		Title:       "Get record",
		Description: "Returns a single collection record by its id.",
		Permission:  core.MCPPermissionRecordsRead,
		ReadOnly:    true,
		Properties: map[string]any{
			"collection": mcpCollectionProp,
			"id":         map[string]any{"type": "string", "description": "The record id."},
			"expand":     mcpExpandProp,
		},
		Required: []string{"collection", "id"},
		Handler:  mcpGetRecord,
	},
	{
		Name:  "create_record",
		Title: "Create record",
		Description: "Creates a new record. `data` is an object with the field values, e.g. `{\"title\":\"Hello\",\"tags\":[\"id1\",\"id2\"]}`. " +
			"File fields can't be uploaded through this tool; for media fields upload the file with upload_media and use the returned url as the field value.",
		Permission: core.MCPPermissionRecordsWrite,
		Properties: map[string]any{
			"collection": mcpCollectionProp,
			"data":       map[string]any{"type": "object", "description": "The record field values."},
		},
		Required: []string{"collection", "data"},
		Handler:  mcpCreateRecord,
	},
	{
		Name:  "update_record",
		Title: "Update record",
		Description: "Updates an existing record. Only the fields present in `data` are changed. " +
			"Supports the PocketBase modifiers, e.g. `{\"tags+\":\"id3\"}` or `{\"views+\":1}`.",
		Permission: core.MCPPermissionRecordsWrite,
		Properties: map[string]any{
			"collection": mcpCollectionProp,
			"id":         map[string]any{"type": "string", "description": "The record id."},
			"data":       map[string]any{"type": "object", "description": "The record field values to change."},
		},
		Required: []string{"collection", "id", "data"},
		Handler:  mcpUpdateRecord,
	},
	{
		Name:        "delete_record",
		Title:       "Delete record",
		Description: "Deletes a single record by its id.",
		Permission:  core.MCPPermissionRecordsWrite,
		Destructive: true,
		Properties: map[string]any{
			"collection": mcpCollectionProp,
			"id":         map[string]any{"type": "string", "description": "The record id."},
		},
		Required: []string{"collection", "id"},
		Handler:  mcpDeleteRecord,
	},
	{
		Name:  "list_media",
		Title: "List media",
		Description: "Lists the files and folders of the media library inside `folder` (default the root), " +
			"or searches all media items by name when `search` is set. File items include the `url` to use as a media field value.",
		Permission: core.MCPPermissionMediaRead,
		ReadOnly:   true,
		Properties: map[string]any{
			"folder": map[string]any{
				"type":        "string",
				"description": "Optional folder path, e.g. `/posts/covers`. Defaults to the root folder. A missing folder returns an empty list with `exists: false`.",
			},
			"search": map[string]any{
				"type":        "string",
				"description": "Optional text to search in the media item names (searches all folders).",
			},
			"page": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"description": "Page number (default 1).",
			},
			"perPage": map[string]any{
				"type":        "integer",
				"minimum":     1,
				"maximum":     mcpMaxPerPage,
				"description": "Items per page (default 100, max 200).",
			},
		},
		Handler: mcpListMedia,
	},
	{
		Name:  "upload_media",
		Title: "Upload media",
		Description: "Uploads a file (e.g. a post cover image) to the media library. Provide either `content` (base64, a data URI, or plain text with `encoding` `text`, e.g. for SVG) " +
			"or a public http(s) `url` to download the file from (preferred for large binary files). " +
			"Folders in `folder` that don't exist yet are created automatically and a numeric suffix is added when the name is taken. " +
			"Returns the media item; set its `url` as the value of a media field (e.g. update_record with `{\"cover\":\"<url>\"}`).",
		Permission: core.MCPPermissionMediaWrite,
		Properties: map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "File name including the extension, e.g. `cover.jpg`. Required with `content`; defaults to the remote file name with `url`.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The file content: base64 (a data URI is also accepted), or plain text when `encoding` is `text`.",
			},
			"encoding": map[string]any{
				"type":        "string",
				"enum":        []string{"base64", "text"},
				"description": "Encoding of `content` (default `base64`). Use `text` for text files such as SVG, Markdown or CSV.",
			},
			"url": map[string]any{
				"type":        "string",
				"description": "Public http(s) URL to download the file from.",
			},
			"folder": map[string]any{
				"type":        "string",
				"description": "Optional destination folder path, e.g. `/posts/covers`; missing folders are created. Defaults to the root folder.",
			},
		},
		Handler: mcpUploadMedia,
	},
}

// -------------------------------------------------------------------

type mcpArgs map[string]any

func (a mcpArgs) string(key string) string {
	return strings.TrimSpace(cast.ToString(a[key]))
}

func (a mcpArgs) requiredString(key string) (string, error) {
	v := a.string(key)
	if v == "" {
		return "", fmt.Errorf("missing required argument %q", key)
	}
	return v, nil
}

func (a mcpArgs) object(key string) (map[string]any, error) {
	switch v := a[key].(type) {
	case map[string]any:
		return v, nil
	case string:
		// some clients send nested objects as serialized JSON
		result := map[string]any{}
		if err := json.Unmarshal([]byte(v), &result); err == nil {
			return result, nil
		}
	}

	return nil, fmt.Errorf("argument %q must be an object", key)
}

func (s *mcpServer) findCollection(nameOrId string) (*core.Collection, error) {
	collection, err := s.app.FindCachedCollectionByNameOrId(nameOrId)
	if err != nil || !s.key.CanAccessCollection(collection) {
		return nil, fmt.Errorf("collection %q doesn't exist or is not accessible with this key", nameOrId)
	}

	return collection, nil
}

func (s *mcpServer) findRecord(args mcpArgs) (*core.Record, error) {
	nameOrId, err := args.requiredString("collection")
	if err != nil {
		return nil, err
	}

	id, err := args.requiredString("id")
	if err != nil {
		return nil, err
	}

	collection, err := s.findCollection(nameOrId)
	if err != nil {
		return nil, err
	}

	record, err := s.app.FindRecordById(collection, id)
	if err != nil {
		return nil, fmt.Errorf("record %q doesn't exist in collection %q", id, collection.Name)
	}

	return record, nil
}

// expandRecords expands the requested relations, skipping the ones
// pointing to collections that are not accessible with the current key.
func (s *mcpServer) expandRecords(records []*core.Record, expand string) error {
	if expand == "" || len(records) == 0 {
		return nil
	}

	expands := strings.Split(expand, ",")
	for i := range expands {
		expands[i] = strings.TrimSpace(expands[i])
	}

	failed := s.app.ExpandRecords(records, expands, func(relCollection *core.Collection, relIds []string) ([]*core.Record, error) {
		if !s.key.CanAccessCollection(relCollection) {
			return nil, fmt.Errorf("collection %q is not accessible with this key", relCollection.Name)
		}

		return s.app.FindRecordsByIds(relCollection.Id, relIds)
	})

	if len(failed) > 0 {
		errs := make([]string, 0, len(failed))
		for k, err := range failed {
			errs = append(errs, k+": "+err.Error())
		}
		return errors.New("failed to expand " + strings.Join(errs, "; "))
	}

	return nil
}

func prepareMCPRecords(records ...*core.Record) {
	for _, r := range records {
		r.IgnoreEmailVisibility(true)
	}
}

// -------------------------------------------------------------------
// Collections
// -------------------------------------------------------------------

func mcpListCollections(s *mcpServer, args mcpArgs) (any, error) {
	collections, err := s.app.FindAllCollections()
	if err != nil {
		return nil, err
	}

	items := []map[string]any{}
	for _, collection := range collections {
		if !s.key.CanAccessCollection(collection) {
			continue
		}

		fields := make([]map[string]any, 0, len(collection.Fields))
		for _, field := range collection.Fields {
			if field.GetHidden() {
				continue
			}

			summary := map[string]any{
				"name": field.GetName(),
				"type": field.Type(),
			}

			raw, _ := json.Marshal(field)
			options := map[string]any{}
			_ = json.Unmarshal(raw, &options)
			if required, _ := options["required"].(bool); required {
				summary["required"] = true
			}
			for _, opt := range []string{"values", "maxSelect", "collectionId"} {
				if v, ok := options[opt]; ok && v != nil && v != "" {
					summary[opt] = v
				}
			}

			fields = append(fields, summary)
		}

		items = append(items, map[string]any{
			"id":     collection.Id,
			"name":   collection.Name,
			"type":   collection.Type,
			"fields": fields,
		})
	}

	return map[string]any{"items": items}, nil
}

func mcpGetCollection(s *mcpServer, args mcpArgs) (any, error) {
	nameOrId, err := args.requiredString("collection")
	if err != nil {
		return nil, err
	}

	return s.findCollection(nameOrId)
}

func mcpCreateCollection(s *mcpServer, args mcpArgs) (any, error) {
	if len(s.key.Collections) > 0 {
		return nil, errors.New("keys restricted to specific collections can't create new collections")
	}

	data, err := args.object("data")
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	collection := core.NewCollection(cast.ToString(data["type"]), cast.ToString(data["name"]))
	if err := json.Unmarshal(raw, collection); err != nil {
		return nil, fmt.Errorf("invalid collection data: %w", err)
	}
	collection.System = false

	if err := s.app.Save(collection); err != nil {
		return nil, err
	}

	return collection, nil
}

func mcpUpdateCollection(s *mcpServer, args mcpArgs) (any, error) {
	nameOrId, err := args.requiredString("collection")
	if err != nil {
		return nil, err
	}

	data, err := args.object("data")
	if err != nil {
		return nil, err
	}

	cached, err := s.findCollection(nameOrId)
	if err != nil {
		return nil, err
	}

	// load a fresh copy to avoid mutating the cached collection on failure
	collection, err := s.app.FindCollectionByNameOrId(cached.Id)
	if err != nil {
		return nil, err
	}

	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(raw, collection); err != nil {
		return nil, fmt.Errorf("invalid collection data: %w", err)
	}
	collection.Id = cached.Id
	collection.System = false

	if err := s.app.Save(collection); err != nil {
		return nil, err
	}

	return collection, nil
}

func mcpDeleteCollection(s *mcpServer, args mcpArgs) (any, error) {
	nameOrId, err := args.requiredString("collection")
	if err != nil {
		return nil, err
	}

	collection, err := s.findCollection(nameOrId)
	if err != nil {
		return nil, err
	}

	if err := s.app.Delete(collection); err != nil {
		return nil, err
	}

	return map[string]any{"deleted": true, "id": collection.Id, "name": collection.Name}, nil
}

// -------------------------------------------------------------------
// Records
// -------------------------------------------------------------------

func mcpListRecords(s *mcpServer, args mcpArgs) (any, error) {
	nameOrId, err := args.requiredString("collection")
	if err != nil {
		return nil, err
	}

	collection, err := s.findCollection(nameOrId)
	if err != nil {
		return nil, err
	}

	page := cast.ToInt(args["page"])
	if page <= 0 {
		page = 1
	}

	perPage := cast.ToInt(args["perPage"])
	if perPage <= 0 {
		perPage = search.DefaultPerPage
	}
	if perPage > mcpMaxPerPage {
		perPage = mcpMaxPerPage
	}

	resolver := core.NewRecordFieldResolver(s.app, collection, nil, false)

	provider := search.NewProvider(resolver).
		Query(s.app.RecordQuery(collection)).
		Page(page).
		PerPage(perPage).
		SkipTotal(cast.ToBool(args["skipTotal"]))

	if filter := args.string("filter"); filter != "" {
		provider.AddFilter(search.FilterData(filter))
	}

	if sort := args.string("sort"); sort != "" {
		for _, field := range search.ParseSortFromString(sort) {
			provider.AddSort(field)
		}
	}

	records := []*core.Record{}

	result, err := provider.Exec(&records)
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %w", err)
	}

	if err := s.expandRecords(records, args.string("expand")); err != nil {
		return nil, err
	}

	prepareMCPRecords(records...)

	return result, nil
}

func mcpGetRecord(s *mcpServer, args mcpArgs) (any, error) {
	record, err := s.findRecord(args)
	if err != nil {
		return nil, err
	}

	if err := s.expandRecords([]*core.Record{record}, args.string("expand")); err != nil {
		return nil, err
	}

	prepareMCPRecords(record)

	return record, nil
}

func mcpCreateRecord(s *mcpServer, args mcpArgs) (any, error) {
	nameOrId, err := args.requiredString("collection")
	if err != nil {
		return nil, err
	}

	data, err := args.object("data")
	if err != nil {
		return nil, err
	}

	collection, err := s.findCollection(nameOrId)
	if err != nil {
		return nil, err
	}

	if collection.IsView() {
		return nil, fmt.Errorf("can't create records in view collection %q", collection.Name)
	}

	record := core.NewRecord(collection)
	record.Load(data)

	if err := s.app.Save(record); err != nil {
		return nil, err
	}

	prepareMCPRecords(record)

	return record, nil
}

func mcpUpdateRecord(s *mcpServer, args mcpArgs) (any, error) {
	data, err := args.object("data")
	if err != nil {
		return nil, err
	}

	record, err := s.findRecord(args)
	if err != nil {
		return nil, err
	}

	if record.Collection().IsView() {
		return nil, fmt.Errorf("can't update records in view collection %q", record.Collection().Name)
	}

	delete(data, "id")
	record.Load(data)

	if err := s.app.Save(record); err != nil {
		return nil, err
	}

	prepareMCPRecords(record)

	return record, nil
}

func mcpDeleteRecord(s *mcpServer, args mcpArgs) (any, error) {
	record, err := s.findRecord(args)
	if err != nil {
		return nil, err
	}

	if record.Collection().IsView() {
		return nil, fmt.Errorf("can't delete records in view collection %q", record.Collection().Name)
	}

	if err := s.app.Delete(record); err != nil {
		return nil, err
	}

	return map[string]any{"deleted": true, "id": record.Id}, nil
}
