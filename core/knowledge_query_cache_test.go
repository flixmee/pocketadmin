package core_test

import (
	"context"
	"testing"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/pocketbase/pocketbase/tools/types"
)

func TestNormalizeKnowledgeCacheQuery(t *testing.T) {
	t.Parallel()

	scenarios := map[string]string{
		"  How   Do I RESET?  ":      "how do i reset",
		"Keep internal, punctuation": "keep internal, punctuation",
		"???":                        "???",
	}
	for input, expected := range scenarios {
		if result := core.NormalizeKnowledgeCacheQuery(input); result != expected {
			t.Fatalf("Expected %q to normalize to %q, got %q", input, expected, result)
		}
	}
}

func TestKnowledgeQueryCacheConfigSettingsAndEnvironment(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	app.Settings().AI.KnowledgeCacheTTL = 12
	app.Settings().AI.KnowledgeCacheSimilarity = 0.87
	app.Settings().AI.KnowledgeCacheMaxEntries = 42
	app.Settings().AI.KnowledgeCacheAnswers = false
	config := core.ResolveKnowledgeQueryCacheConfig(app)
	if config.TTL != 12*time.Minute || config.SimilarityThreshold != 0.87 || config.MaxEntries != 42 || config.CacheAnswers {
		t.Fatalf("Unexpected settings-backed config: %#v", config)
	}

	t.Setenv(core.KnowledgeQueryCacheTTLEnv, "45s")
	t.Setenv(core.KnowledgeQueryCacheSimilarityEnv, "0.95")
	t.Setenv(core.KnowledgeQueryCacheMaxEntriesEnv, "17")
	t.Setenv(core.KnowledgeQueryCacheAnswersEnv, "true")
	config = core.ResolveKnowledgeQueryCacheConfig(app)
	if config.TTL != 45*time.Second || config.SimilarityThreshold != 0.95 || config.MaxEntries != 17 || !config.CacheAnswers {
		t.Fatalf("Unexpected environment-backed config: %#v", config)
	}
}

func TestKnowledgeQueryCacheExactAndSemanticHitsSkipRetrieval(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	provider := new(cachedTestKnowledgeProvider)
	app.Store().Set(core.StoreKeyKnowledgeProvider, provider)
	document := createKnowledgeCacheDocument(t, app, "cache-user", "Password reset instructions.")
	provider.embedCalls.Store(0)

	options := core.KnowledgeSearchOptions{OwnerID: "cache-user", OwnerCollection: "users", Limit: 5}
	first, err := core.SearchKnowledgeBaseWithCache(context.Background(), app, "Password reset?", options)
	if err != nil {
		t.Fatal(err)
	}
	if first.CacheMatch != core.KnowledgeQueryCacheMiss || first.CacheEntryID == "" || len(first.Sources) != 1 {
		t.Fatalf("Unexpected initial cache result: %#v", first)
	}
	if _, err := app.DB().NewQuery(`DROP TABLE {{kb_chunks_fts}}`).Execute(); err != nil {
		t.Fatal(err)
	}

	exact, err := core.SearchKnowledgeBaseWithCache(context.Background(), app, "  PASSWORD   reset!!! ", options)
	if err != nil {
		t.Fatalf("Expected exact hit to skip dropped retrieval table: %v", err)
	}
	if exact.CacheMatch != core.KnowledgeQueryCacheExact || exact.Sources[0].DocumentID != document.Id {
		t.Fatalf("Unexpected exact cache result: %#v", exact)
	}
	if provider.embedCalls.Load() != 1 {
		t.Fatalf("Expected exact hit to skip embedding, got %d calls", provider.embedCalls.Load())
	}

	semantic, err := core.SearchKnowledgeBaseWithCache(context.Background(), app, "How can I change my password?", options)
	if err != nil {
		t.Fatalf("Expected semantic hit to skip dropped retrieval table: %v", err)
	}
	if semantic.CacheMatch != core.KnowledgeQueryCacheSemantic || len(semantic.Sources) != 1 {
		t.Fatalf("Unexpected semantic cache result: %#v", semantic)
	}
	if provider.embedCalls.Load() != 2 {
		t.Fatalf("Expected semantic path to embed once, got %d total calls", provider.embedCalls.Load())
	}

	stats := core.KnowledgeQueryCacheStats(app)
	if stats.Misses != 1 || stats.ExactHits != 1 || stats.SemanticHits != 1 {
		t.Fatalf("Unexpected cache metrics: %#v", stats)
	}
}

func TestKnowledgeQueryCacheOwnerIsolationAndInvalidation(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	provider := new(cachedTestKnowledgeProvider)
	app.Store().Set(core.StoreKeyKnowledgeProvider, provider)
	documentOne := createKnowledgeCacheDocument(t, app, "owner-one", "Owner one password instructions.")
	createKnowledgeCacheDocument(t, app, "owner-two", "Owner two account instructions.")

	ownerOne := core.KnowledgeSearchOptions{OwnerID: "owner-one", OwnerCollection: "users", Limit: 5}
	ownerTwo := core.KnowledgeSearchOptions{OwnerID: "owner-two", OwnerCollection: "users", Limit: 5}
	if result, err := core.SearchKnowledgeBaseWithCache(context.Background(), app, "account help", ownerOne); err != nil || len(result.Sources) != 1 {
		t.Fatalf("Failed to prime owner-one cache: %#v %v", result, err)
	}
	if result, err := core.SearchKnowledgeBaseWithCache(context.Background(), app, "account help", ownerTwo); err != nil || len(result.Sources) != 1 {
		t.Fatalf("Failed to prime owner-two cache: %#v %v", result, err)
	}

	core.InvalidateKnowledgeQueryCacheForDocument(app, documentOne.Id)
	var ownerOneEntries int
	var ownerTwoEntries int
	if err := app.DB().NewQuery(`SELECT COUNT(*) FROM {{kb_query_cache}} WHERE [[owner]] = 'owner-one'`).Row(&ownerOneEntries); err != nil {
		t.Fatal(err)
	}
	if err := app.DB().NewQuery(`SELECT COUNT(*) FROM {{kb_query_cache}} WHERE [[owner]] = 'owner-two'`).Row(&ownerTwoEntries); err != nil {
		t.Fatal(err)
	}
	if ownerOneEntries != 0 || ownerTwoEntries != 1 {
		t.Fatalf("Expected targeted invalidation to preserve the other owner, got %d/%d", ownerOneEntries, ownerTwoEntries)
	}

	result, err := core.SearchKnowledgeBaseWithCache(context.Background(), app, "account help", ownerTwo)
	if err != nil || result.CacheMatch != core.KnowledgeQueryCacheExact {
		t.Fatalf("Expected preserved owner-two exact hit, got %#v %v", result, err)
	}
}

func TestKnowledgeQueryCacheTTLSizeAndOptionalAnswer(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	provider := new(cachedTestKnowledgeProvider)
	app.Store().Set(core.StoreKeyKnowledgeProvider, provider)
	app.Settings().AI.KnowledgeCacheMaxEntries = 2
	app.Settings().AI.KnowledgeCacheAnswers = true
	createKnowledgeCacheDocument(t, app, "cache-user", "Reusable account documentation.")

	options := core.KnowledgeSearchOptions{OwnerID: "cache-user", OwnerCollection: "users", Limit: 5}
	first, err := core.SearchKnowledgeBaseWithCache(context.Background(), app, "first question", options)
	if err != nil {
		t.Fatal(err)
	}
	if err := core.UpdateKnowledgeQueryCacheAnswer(app, first.CacheEntryID, "Cached final answer"); err != nil {
		t.Fatal(err)
	}
	exact, err := core.SearchKnowledgeBaseWithCache(context.Background(), app, "FIRST QUESTION!", options)
	if err != nil {
		t.Fatal(err)
	}
	if exact.CachedAnswer != "Cached final answer" {
		t.Fatalf("Expected optional exact answer reuse, got %#v", exact)
	}

	if _, err := app.DB().NewQuery(`
		UPDATE {{kb_query_cache}}
		SET [[ttl_expires_at]] = {:expired}
		WHERE [[id]] = {:id}
	`).Bind(dbx.Params{
		"expired": types.NowDateTime().Add(-time.Minute).String(),
		"id":      first.CacheEntryID,
	}).Execute(); err != nil {
		t.Fatal(err)
	}
	expired, err := core.SearchKnowledgeBaseWithCache(context.Background(), app, "first question", options)
	if err != nil {
		t.Fatal(err)
	}
	if expired.CacheMatch == core.KnowledgeQueryCacheExact || expired.CachedAnswer != "" {
		t.Fatalf("Expected expired entry to miss, got %#v", expired)
	}

	for _, question := range []string{"second question", "third question"} {
		if _, err := core.SearchKnowledgeBaseWithCache(context.Background(), app, question, options); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := app.DB().NewQuery(`SELECT COUNT(*) FROM {{kb_query_cache}}`).Row(&count); err != nil {
		t.Fatal(err)
	}
	if count > 2 {
		t.Fatalf("Expected max cache size 2, got %d", count)
	}
}

func createKnowledgeCacheDocument(t *testing.T, app core.App, owner, content string) *core.Record {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId(core.CollectionNameKBDocuments)
	if err != nil {
		t.Fatal(err)
	}
	upload, err := filesystem.NewFileFromBytes([]byte(content), owner+".md")
	if err != nil {
		t.Fatal(err)
	}
	document := core.NewRecord(collection)
	document.Set("title", owner+" cache document")
	document.Set("file", upload)
	document.Set("mime_type", "text/markdown")
	document.Set("status", core.KBDocumentStatusIndexed)
	document.Set("owner", owner)
	document.Set("owner_collection", "users")
	if err := app.Save(document); err != nil {
		t.Fatal(err)
	}
	if err := core.IngestKnowledgeDocument(context.Background(), app, document.Id); err != nil {
		t.Fatal(err)
	}
	return document
}
