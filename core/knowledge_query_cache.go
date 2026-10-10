package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	CollectionNameKBQueryCache = "kb_query_cache"

	KnowledgeQueryCacheExact    = "exact"
	KnowledgeQueryCacheSemantic = "semantic"
	KnowledgeQueryCacheMiss     = "miss"

	StoreKeyKnowledgeQueryCacheMetrics = "pbAppKnowledgeQueryCacheMetrics"

	KnowledgeQueryCacheTTLEnv        = "PB_KB_QUERY_CACHE_TTL"
	KnowledgeQueryCacheSimilarityEnv = "PB_KB_QUERY_CACHE_SIMILARITY"
	KnowledgeQueryCacheMaxEntriesEnv = "PB_KB_QUERY_CACHE_MAX_ENTRIES"
	KnowledgeQueryCacheAnswersEnv    = "PB_KB_QUERY_CACHE_ANSWERS"

	defaultKnowledgeQueryCacheTTL        = 30 * time.Minute
	defaultKnowledgeQueryCacheSimilarity = 0.92
	defaultKnowledgeQueryCacheMaxEntries = 500
)

type KnowledgeQueryCacheConfig struct {
	TTL                 time.Duration
	SimilarityThreshold float64
	MaxEntries          int
	CacheAnswers        bool
}

type KnowledgeQueryCacheSnapshot struct {
	ExactHits    uint64 `json:"cacheHitExact"`
	SemanticHits uint64 `json:"cacheHitSemantic"`
	Misses       uint64 `json:"cacheMiss"`
}

type KnowledgeQueryResult struct {
	Sources      []KnowledgeSearchResult
	CacheEntryID string
	CacheMatch   string
	CachedAnswer string
}

type knowledgeQueryCacheMetrics struct {
	mu       sync.Mutex
	exact    uint64
	semantic uint64
	misses   uint64
}

type knowledgeQueryCacheScope struct {
	OwnerID         string
	OwnerCollection string
	ProviderKey     string
	ResultLimit     int
}

type knowledgeQueryCacheRow struct {
	ID                string  `db:"id"`
	QueryHash         string  `db:"query_hash"`
	NormalizedQuery   string  `db:"normalized_query"`
	OriginalQuery     string  `db:"original_query"`
	Embedding         string  `db:"embedding"`
	RetrievedChunkIDs string  `db:"retrieved_chunk_ids"`
	DocumentIDs       string  `db:"document_ids"`
	CachedAnswer      string  `db:"cached_answer"`
	HitCount          int     `db:"hit_count"`
	Similarity        float64 `db:"similarity"`
}

func ResolveKnowledgeQueryCacheConfig(app App) KnowledgeQueryCacheConfig {
	config := KnowledgeQueryCacheConfig{
		TTL:                 defaultKnowledgeQueryCacheTTL,
		SimilarityThreshold: defaultKnowledgeQueryCacheSimilarity,
		MaxEntries:          defaultKnowledgeQueryCacheMaxEntries,
	}
	if app != nil && app.Settings() != nil {
		settings := app.Settings().AI
		if settings.KnowledgeCacheTTL > 0 {
			config.TTL = time.Duration(settings.KnowledgeCacheTTL) * time.Minute
		}
		if settings.KnowledgeCacheSimilarity > 0 {
			config.SimilarityThreshold = settings.KnowledgeCacheSimilarity
		}
		if settings.KnowledgeCacheMaxEntries > 0 {
			config.MaxEntries = settings.KnowledgeCacheMaxEntries
		}
		config.CacheAnswers = settings.KnowledgeCacheAnswers
	}

	if value := strings.TrimSpace(os.Getenv(KnowledgeQueryCacheTTLEnv)); value != "" {
		if duration, err := time.ParseDuration(value); err == nil && duration > 0 {
			config.TTL = duration
		}
	}
	if value := strings.TrimSpace(os.Getenv(KnowledgeQueryCacheSimilarityEnv)); value != "" {
		if threshold, err := strconv.ParseFloat(value, 64); err == nil && threshold >= 0.5 && threshold <= 1 {
			config.SimilarityThreshold = threshold
		}
	}
	if value := strings.TrimSpace(os.Getenv(KnowledgeQueryCacheMaxEntriesEnv)); value != "" {
		if maxEntries, err := strconv.Atoi(value); err == nil && maxEntries > 0 && maxEntries <= 10000 {
			config.MaxEntries = maxEntries
		}
	}
	if value := strings.TrimSpace(os.Getenv(KnowledgeQueryCacheAnswersEnv)); value != "" {
		if cacheAnswers, err := strconv.ParseBool(value); err == nil {
			config.CacheAnswers = cacheAnswers
		}
	}

	return config
}

func NormalizeKnowledgeCacheQuery(question string) string {
	normalized := strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(question))), " ")
	withoutTrailingPunctuation := strings.TrimSpace(strings.TrimRightFunc(normalized, unicode.IsPunct))
	if withoutTrailingPunctuation != "" {
		return withoutTrailingPunctuation
	}
	return normalized
}

func KnowledgeQueryCacheStats(app App) KnowledgeQueryCacheSnapshot {
	if app == nil {
		return KnowledgeQueryCacheSnapshot{}
	}
	metrics := resolveKnowledgeQueryCacheMetrics(app)
	metrics.mu.Lock()
	defer metrics.mu.Unlock()
	return KnowledgeQueryCacheSnapshot{
		ExactHits:    metrics.exact,
		SemanticHits: metrics.semantic,
		Misses:       metrics.misses,
	}
}

func UpdateKnowledgeQueryCacheAnswer(app App, entryID, answer string) error {
	entryID = strings.TrimSpace(entryID)
	answer = strings.TrimSpace(answer)
	if app == nil || entryID == "" || answer == "" || !ResolveKnowledgeQueryCacheConfig(app).CacheAnswers {
		return nil
	}
	encoded, err := json.Marshal(answer)
	if err != nil {
		return err
	}
	_, err = app.DB().NewQuery(`
		UPDATE {{kb_query_cache}}
		SET [[cached_answer]] = {:answer}
		WHERE [[id]] = {:id}
	`).Bind(dbx.Params{
		"answer": string(encoded),
		"id":     entryID,
	}).Execute()
	return err
}

func InvalidateKnowledgeQueryCacheForDocument(app App, documentID string) {
	documentID = strings.TrimSpace(documentID)
	if app == nil || documentID == "" || !knowledgeQueryCacheAvailable(app) {
		return
	}
	result, err := app.DB().NewQuery(`
		DELETE FROM {{kb_query_cache}}
		WHERE EXISTS (
			SELECT 1
			FROM json_each([[document_ids]])
			WHERE json_each.[[value]] = {:document}
		)
	`).Bind(dbx.Params{"document": documentID}).Execute()
	if err != nil {
		app.Logger().Warn(
			"Failed to invalidate knowledge query cache for document",
			slog.String("documentId", documentID),
			slog.String("error", err.Error()),
		)
		return
	}
	removed, _ := result.RowsAffected()
	app.Logger().Info(
		"Knowledge query cache invalidated",
		slog.String("documentId", documentID),
		slog.Int64("entries", removed),
	)
}

func invalidateAllKnowledgeQueryCache(app App) {
	if app == nil || !knowledgeQueryCacheAvailable(app) {
		return
	}
	if _, err := app.DB().NewQuery(`DELETE FROM {{kb_query_cache}}`).Execute(); err != nil {
		app.Logger().Warn("Failed to invalidate knowledge query cache", slog.String("error", err.Error()))
	}
}

func knowledgeQueryCacheScopeFor(provider KnowledgeProvider, options KnowledgeSearchOptions) (knowledgeQueryCacheScope, bool) {
	providerKey := knowledgeProviderCacheKey(provider)
	if providerKey == "" {
		return knowledgeQueryCacheScope{}, false
	}
	return knowledgeQueryCacheScope{
		OwnerID:         options.OwnerID,
		OwnerCollection: options.OwnerCollection,
		ProviderKey:     knowledgeCacheHash(providerKey),
		ResultLimit:     options.Limit,
	}, true
}

func findExactKnowledgeQueryCache(
	ctx context.Context,
	app App,
	scope knowledgeQueryCacheScope,
	queryHash string,
) (knowledgeQueryCacheRow, bool, error) {
	row := knowledgeQueryCacheRow{}
	err := app.ConcurrentDB().NewQuery(`
		SELECT
			[[id]], [[query_hash]], [[normalized_query]], [[original_query]],
			[[embedding]], [[retrieved_chunk_ids]], [[document_ids]],
			COALESCE([[cached_answer]], 'null') AS [[cached_answer]],
			[[hit_count]], 1.0 AS [[similarity]]
		FROM {{kb_query_cache}}
		WHERE [[owner]] = {:owner}
			AND [[owner_collection]] = {:ownerCollection}
			AND [[provider_key]] = {:providerKey}
			AND [[result_limit]] = {:resultLimit}
			AND [[query_hash]] = {:queryHash}
			AND [[ttl_expires_at]] > {:now}
		ORDER BY [[created]] DESC
		LIMIT 1
	`).Bind(knowledgeQueryCacheParams(scope, dbx.Params{
		"queryHash": queryHash,
		"now":       types.NowDateTime().String(),
	})).WithContext(ctx).One(&row)
	if errors.Is(err, sql.ErrNoRows) {
		return knowledgeQueryCacheRow{}, false, nil
	}
	return row, err == nil, err
}

func findSemanticKnowledgeQueryCache(
	ctx context.Context,
	app App,
	scope knowledgeQueryCacheScope,
	queryVector []float32,
	config KnowledgeQueryCacheConfig,
) (knowledgeQueryCacheRow, bool, error) {
	encoded, err := json.Marshal(queryVector)
	if err != nil {
		return knowledgeQueryCacheRow{}, false, err
	}
	params := knowledgeQueryCacheParams(scope, dbx.Params{
		"embedding": string(encoded),
		"now":       types.NowDateTime().String(),
		"threshold": config.SimilarityThreshold,
	})
	row := knowledgeQueryCacheRow{}
	err = app.ConcurrentDB().NewQuery(`
		SELECT
			[[id]], [[query_hash]], [[normalized_query]], [[original_query]],
			[[embedding]], [[retrieved_chunk_ids]], [[document_ids]],
			COALESCE([[cached_answer]], 'null') AS [[cached_answer]], [[hit_count]],
			1.0 - pb_vec_cosine_distance([[embedding]], {:embedding}) AS [[similarity]]
		FROM {{kb_query_cache}}
		WHERE [[owner]] = {:owner}
			AND [[owner_collection]] = {:ownerCollection}
			AND [[provider_key]] = {:providerKey}
			AND [[result_limit]] = {:resultLimit}
			AND [[ttl_expires_at]] > {:now}
			AND pb_vec_cosine_distance([[embedding]], {:embedding}) IS NOT NULL
			AND 1.0 - pb_vec_cosine_distance([[embedding]], {:embedding}) >= {:threshold}
		ORDER BY [[similarity]] DESC, [[hit_count]] DESC, [[created]] DESC
		LIMIT 1
	`).Bind(params).WithContext(ctx).One(&row)
	if err == nil {
		return row, true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return knowledgeQueryCacheRow{}, false, nil
	}

	return findSemanticKnowledgeQueryCacheFallback(ctx, app, scope, queryVector, config)
}

func findSemanticKnowledgeQueryCacheFallback(
	ctx context.Context,
	app App,
	scope knowledgeQueryCacheScope,
	queryVector []float32,
	config KnowledgeQueryCacheConfig,
) (knowledgeQueryCacheRow, bool, error) {
	rows := []knowledgeQueryCacheRow{}
	err := app.ConcurrentDB().NewQuery(`
		SELECT
			[[id]], [[query_hash]], [[normalized_query]], [[original_query]],
			[[embedding]], [[retrieved_chunk_ids]], [[document_ids]],
			COALESCE([[cached_answer]], 'null') AS [[cached_answer]], [[hit_count]],
			0.0 AS [[similarity]]
		FROM {{kb_query_cache}}
		WHERE [[owner]] = {:owner}
			AND [[owner_collection]] = {:ownerCollection}
			AND [[provider_key]] = {:providerKey}
			AND [[result_limit]] = {:resultLimit}
			AND [[ttl_expires_at]] > {:now}
		ORDER BY [[created]] DESC
		LIMIT {:maxEntries}
	`).Bind(knowledgeQueryCacheParams(scope, dbx.Params{
		"now":        types.NowDateTime().String(),
		"maxEntries": config.MaxEntries,
	})).WithContext(ctx).All(&rows)
	if err != nil {
		return knowledgeQueryCacheRow{}, false, err
	}

	bestIndex := -1
	bestSimilarity := config.SimilarityThreshold
	for index := range rows {
		var cachedVector []float32
		if json.Unmarshal([]byte(rows[index].Embedding), &cachedVector) != nil {
			continue
		}
		distance, ok := knowledgeCosineDistance(queryVector, cachedVector)
		if !ok {
			continue
		}
		similarity := 1 - distance
		if similarity >= bestSimilarity {
			bestSimilarity = similarity
			bestIndex = index
		}
	}
	if bestIndex < 0 {
		return knowledgeQueryCacheRow{}, false, nil
	}
	rows[bestIndex].Similarity = bestSimilarity
	return rows[bestIndex], true, nil
}

func writeKnowledgeQueryCache(
	ctx context.Context,
	app App,
	scope knowledgeQueryCacheScope,
	question string,
	queryVector []float32,
	sources []KnowledgeSearchResult,
	config KnowledgeQueryCacheConfig,
) (string, error) {
	if len(queryVector) == 0 {
		return "", nil
	}
	normalized := NormalizeKnowledgeCacheQuery(question)
	queryHash := knowledgeCacheHash(normalized)
	embedding, err := json.Marshal(queryVector)
	if err != nil {
		return "", err
	}
	chunkIDs := make([]string, 0, len(sources))
	documentIDs := make([]string, 0, len(sources))
	seenDocuments := map[string]struct{}{}
	for _, source := range sources {
		if source.ChunkID != "" {
			chunkIDs = append(chunkIDs, source.ChunkID)
		}
		if source.DocumentID != "" {
			if _, exists := seenDocuments[source.DocumentID]; !exists {
				seenDocuments[source.DocumentID] = struct{}{}
				documentIDs = append(documentIDs, source.DocumentID)
			}
		}
	}
	encodedChunkIDs, err := json.Marshal(chunkIDs)
	if err != nil {
		return "", err
	}
	encodedDocumentIDs, err := json.Marshal(documentIDs)
	if err != nil {
		return "", err
	}
	now := types.NowDateTime()
	entryID := ""
	err = app.RunInTransaction(func(txApp App) error {
		if err := cleanupKnowledgeQueryCache(txApp, config.MaxEntries, now.String()); err != nil {
			return err
		}
		_, err := txApp.DB().NewQuery(`
			INSERT INTO {{kb_query_cache}} (
				[[id]], [[query_hash]], [[normalized_query]], [[original_query]],
				[[owner]], [[owner_collection]], [[provider_key]], [[result_limit]],
				[[embedding]], [[retrieved_chunk_ids]], [[document_ids]], [[cached_answer]],
				[[hit_count]], [[ttl_expires_at]], [[created]]
			) VALUES (
				{:id}, {:queryHash}, {:normalizedQuery}, {:originalQuery},
				{:owner}, {:ownerCollection}, {:providerKey}, {:resultLimit},
				{:embedding}, {:chunkIDs}, {:documentIDs}, NULL,
				0, {:expiresAt}, {:created}
			)
			ON CONFLICT ([[owner_collection]], [[owner]], [[provider_key]], [[result_limit]], [[query_hash]])
			DO UPDATE SET
				[[normalized_query]] = excluded.[[normalized_query]],
				[[original_query]] = excluded.[[original_query]],
				[[embedding]] = excluded.[[embedding]],
				[[retrieved_chunk_ids]] = excluded.[[retrieved_chunk_ids]],
				[[document_ids]] = excluded.[[document_ids]],
				[[cached_answer]] = NULL,
				[[hit_count]] = 0,
				[[ttl_expires_at]] = excluded.[[ttl_expires_at]],
				[[created]] = excluded.[[created]]
		`).Bind(knowledgeQueryCacheParams(scope, dbx.Params{
			"id":              GenerateDefaultRandomId(),
			"queryHash":       queryHash,
			"normalizedQuery": normalized,
			"originalQuery":   strings.TrimSpace(question),
			"embedding":       string(embedding),
			"chunkIDs":        string(encodedChunkIDs),
			"documentIDs":     string(encodedDocumentIDs),
			"expiresAt":       now.Add(config.TTL).String(),
			"created":         now.String(),
		})).WithContext(ctx).Execute()
		if err != nil {
			return err
		}
		return txApp.DB().NewQuery(`
			SELECT [[id]]
			FROM {{kb_query_cache}}
			WHERE [[owner]] = {:owner}
				AND [[owner_collection]] = {:ownerCollection}
				AND [[provider_key]] = {:providerKey}
				AND [[result_limit]] = {:resultLimit}
				AND [[query_hash]] = {:queryHash}
			LIMIT 1
		`).Bind(knowledgeQueryCacheParams(scope, dbx.Params{"queryHash": queryHash})).Row(&entryID)
	})
	return entryID, err
}

func touchKnowledgeQueryCache(app App, entryID string) error {
	_, err := app.DB().NewQuery(`
		UPDATE {{kb_query_cache}}
		SET [[hit_count]] = [[hit_count]] + 1
		WHERE [[id]] = {:id}
	`).Bind(dbx.Params{"id": entryID}).Execute()
	return err
}

func deleteKnowledgeQueryCacheEntry(app App, entryID string) {
	if entryID == "" {
		return
	}
	_, _ = app.DB().NewQuery(`DELETE FROM {{kb_query_cache}} WHERE [[id]] = {:id}`).
		Bind(dbx.Params{"id": entryID}).
		Execute()
}

func cleanupKnowledgeQueryCache(app App, maxEntries int, now string) error {
	if _, err := app.DB().NewQuery(`
		DELETE FROM {{kb_query_cache}}
		WHERE [[ttl_expires_at]] <= {:now}
	`).Bind(dbx.Params{"now": now}).Execute(); err != nil {
		return err
	}
	_, err := app.DB().NewQuery(`
		DELETE FROM {{kb_query_cache}}
		WHERE [[id]] IN (
			SELECT [[id]]
			FROM {{kb_query_cache}}
			ORDER BY [[hit_count]] ASC, [[created]] ASC
			LIMIT CASE
				WHEN (SELECT COUNT(*) FROM {{kb_query_cache}}) >= {:maxEntries}
				THEN (SELECT COUNT(*) FROM {{kb_query_cache}}) - {:maxEntries} + 1
				ELSE 0
			END
		)
	`).Bind(dbx.Params{"maxEntries": maxEntries}).Execute()
	return err
}

func decodeKnowledgeQueryCacheRow(row knowledgeQueryCacheRow) ([]string, string, error) {
	chunkIDs := []string{}
	if err := json.Unmarshal([]byte(row.RetrievedChunkIDs), &chunkIDs); err != nil {
		return nil, "", fmt.Errorf("invalid cached chunk ids: %w", err)
	}
	answer := ""
	if raw := strings.TrimSpace(row.CachedAnswer); raw != "" && raw != "null" {
		if err := json.Unmarshal([]byte(raw), &answer); err != nil {
			return nil, "", fmt.Errorf("invalid cached answer: %w", err)
		}
	}
	return chunkIDs, answer, nil
}

func knowledgeQueryCacheAvailable(app App) bool {
	if app == nil {
		return false
	}
	_, err := app.FindCachedCollectionByNameOrId(CollectionNameKBQueryCache)
	return err == nil
}

func knowledgeQueryCacheParams(scope knowledgeQueryCacheScope, params dbx.Params) dbx.Params {
	if params == nil {
		params = dbx.Params{}
	}
	params["owner"] = scope.OwnerID
	params["ownerCollection"] = scope.OwnerCollection
	params["providerKey"] = scope.ProviderKey
	params["resultLimit"] = scope.ResultLimit
	return params
}

func recordKnowledgeQueryCacheLookup(
	ctx context.Context,
	app App,
	result string,
	queryHash string,
	similarity float64,
) {
	if app == nil {
		return
	}
	metrics := resolveKnowledgeQueryCacheMetrics(app)
	metrics.mu.Lock()
	switch result {
	case KnowledgeQueryCacheExact:
		metrics.exact++
	case KnowledgeQueryCacheSemantic:
		metrics.semantic++
	default:
		metrics.misses++
	}
	metrics.mu.Unlock()

	if result == KnowledgeQueryCacheExact || result == KnowledgeQueryCacheSemantic {
		MarkKnowledgeCacheHit(ctx, "query_"+result)
	}
	app.Logger().Info(
		"Knowledge query cache lookup",
		slog.String("result", result),
		slog.String("queryHash", queryHash),
		slog.Float64("similarity", similarity),
	)
}

func resolveKnowledgeQueryCacheMetrics(app App) *knowledgeQueryCacheMetrics {
	value := app.Store().GetOrSet(StoreKeyKnowledgeQueryCacheMetrics, func() any {
		return &knowledgeQueryCacheMetrics{}
	})
	return value.(*knowledgeQueryCacheMetrics)
}
