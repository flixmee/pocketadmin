package migrations

import (
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	core.SystemMigrations.Register(func(txApp core.App) error {
		return createKBQueryCacheCollection(txApp)
	}, func(txApp core.App) error {
		collection, err := txApp.FindCollectionByNameOrId(core.CollectionNameKBQueryCache)
		if err != nil {
			return nil
		}
		return txApp.Delete(collection)
	})
}

func createKBQueryCacheCollection(app core.App) error {
	if _, err := app.FindCollectionByNameOrId(core.CollectionNameKBQueryCache); err == nil {
		return nil
	}

	collection := core.NewBaseCollection(core.CollectionNameKBQueryCache)
	collection.System = true
	collection.Fields.Add(&core.TextField{Name: "query_hash", System: true, Required: true, Max: 64})
	collection.Fields.Add(&core.TextField{Name: "normalized_query", System: true, Required: true, Max: 8000})
	collection.Fields.Add(&core.TextField{Name: "original_query", System: true, Required: true, Max: 8000})
	collection.Fields.Add(&core.TextField{Name: "owner", System: true, Max: 255})
	collection.Fields.Add(&core.TextField{Name: "owner_collection", System: true, Max: 255})
	collection.Fields.Add(&core.TextField{Name: "provider_key", System: true, Required: true, Max: 64, Hidden: true})
	collection.Fields.Add(&core.NumberField{Name: "result_limit", System: true, Required: true, OnlyInt: true})
	collection.Fields.Add(&core.JSONField{Name: "embedding", System: true, Hidden: true, Required: true, MaxSize: 4 << 20})
	collection.Fields.Add(&core.JSONField{Name: "retrieved_chunk_ids", System: true, Required: true})
	collection.Fields.Add(&core.JSONField{Name: "document_ids", System: true, Required: true, Hidden: true})
	collection.Fields.Add(&core.JSONField{Name: "cached_answer", System: true, Hidden: true, MaxSize: 1 << 20})
	collection.Fields.Add(&core.NumberField{Name: "hit_count", System: true, OnlyInt: true})
	collection.Fields.Add(&core.DateField{Name: "ttl_expires_at", System: true, Required: true})
	collection.Fields.Add(&core.AutodateField{Name: "created", System: true, OnCreate: true})
	collection.AddIndex(
		"idx_kb_query_cache_exact",
		true,
		"owner_collection, owner, provider_key, result_limit, query_hash",
		"",
	)
	collection.AddIndex(
		"idx_kb_query_cache_scope_expiry",
		false,
		"owner_collection, owner, provider_key, result_limit, ttl_expires_at",
		"",
	)
	collection.AddIndex("idx_kb_query_cache_eviction", false, "hit_count, created", "")
	return app.Save(collection)
}
