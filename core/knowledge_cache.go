package core

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

const (
	StoreKeyKnowledgeSearchCache = "pbAppKnowledgeSearchCache"

	knowledgeEmbeddingCacheTTL = 30 * time.Minute
	knowledgeEmbeddingCacheMax = 512
)

type KnowledgeProviderCacheKeyer interface {
	KnowledgeCacheKey() string
}

type knowledgeEmbeddingCacheEntry struct {
	value      []float32
	expiresAt  time.Time
	lastAccess uint64
}

type knowledgeSearchCache struct {
	mu         sync.Mutex
	access     uint64
	embeddings map[string]knowledgeEmbeddingCacheEntry
}

func knowledgeProviderCacheKey(provider KnowledgeProvider) string {
	keyer, ok := provider.(KnowledgeProviderCacheKeyer)
	if !ok {
		return ""
	}
	return strings.TrimSpace(keyer.KnowledgeCacheKey())
}

func knowledgeCacheHash(parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(hash[:])
}

func resolveKnowledgeSearchCache(app App) *knowledgeSearchCache {
	value := app.Store().GetOrSet(StoreKeyKnowledgeSearchCache, func() any {
		return &knowledgeSearchCache{
			embeddings: map[string]knowledgeEmbeddingCacheEntry{},
		}
	})
	return value.(*knowledgeSearchCache)
}

func (cache *knowledgeSearchCache) getEmbedding(key string) ([]float32, bool) {
	if cache == nil || key == "" {
		return nil, false
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()

	entry, ok := cache.embeddings[key]
	if !ok || time.Now().After(entry.expiresAt) {
		delete(cache.embeddings, key)
		return nil, false
	}
	cache.access++
	entry.lastAccess = cache.access
	cache.embeddings[key] = entry
	return append([]float32(nil), entry.value...), true
}

func (cache *knowledgeSearchCache) putEmbedding(key string, value []float32) {
	if cache == nil || key == "" || len(value) == 0 {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()

	cache.access++
	cache.embeddings[key] = knowledgeEmbeddingCacheEntry{
		value:      append([]float32(nil), value...),
		expiresAt:  time.Now().Add(knowledgeEmbeddingCacheTTL),
		lastAccess: cache.access,
	}
	trimKnowledgeEmbeddingCache(cache)
}

func InvalidateKnowledgeSearchCache(app App) {
	invalidateAllKnowledgeQueryCache(app)
}

func trimKnowledgeEmbeddingCache(cache *knowledgeSearchCache) {
	for len(cache.embeddings) > knowledgeEmbeddingCacheMax {
		var oldestKey string
		var oldestAccess uint64
		for key, entry := range cache.embeddings {
			if oldestKey == "" || entry.lastAccess < oldestAccess {
				oldestKey = key
				oldestAccess = entry.lastAccess
			}
		}
		delete(cache.embeddings, oldestKey)
	}
}
