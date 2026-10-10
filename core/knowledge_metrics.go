package core

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"
)

const (
	KnowledgeStageEmbedQuery   = "embed_query"
	KnowledgeStageFTS5Search   = "fts5_search"
	KnowledgeStageVectorSearch = "vector_search"
	KnowledgeStageRerank       = "rerank"
	KnowledgeStageLLMTTFT      = "llm_ttft"
	KnowledgeStageLLMTotal     = "llm_total"
	KnowledgeStageDBWrite      = "db_write"

	StoreKeyKnowledgeLatencyCollector = "pbAppKnowledgeLatencyCollector"

	knowledgeLatencyWindowSize = 100
	knowledgeLatencyLogEvery   = 50
)

type knowledgeChatTraceContextKey struct{}

type KnowledgeLatencyPercentiles struct {
	Samples int     `json:"samples"`
	P50MS   float64 `json:"p50Ms"`
	P95MS   float64 `json:"p95Ms"`
	P99MS   float64 `json:"p99Ms"`
}

type KnowledgeLatencySnapshot struct {
	Requests int64                                  `json:"requests"`
	Window   int                                    `json:"window"`
	Stages   map[string]KnowledgeLatencyPercentiles `json:"stages"`
}

type KnowledgeChatTrace struct {
	app       App
	startedAt time.Time

	mu        sync.Mutex
	stages    map[string]time.Duration
	cacheHits map[string]bool
	completed bool
}

type knowledgeLatencyCollector struct {
	mu       sync.Mutex
	requests int64
	samples  map[string][]time.Duration
}

func NewKnowledgeChatTrace(app App) *KnowledgeChatTrace {
	return &KnowledgeChatTrace{
		app:       app,
		startedAt: time.Now(),
		stages:    map[string]time.Duration{},
		cacheHits: map[string]bool{},
	}
}

func WithKnowledgeChatTrace(ctx context.Context, trace *KnowledgeChatTrace) context.Context {
	if trace == nil {
		return ctx
	}
	return context.WithValue(ctx, knowledgeChatTraceContextKey{}, trace)
}

func RecordKnowledgeStage(ctx context.Context, stage string, startedAt time.Time) {
	trace, _ := ctx.Value(knowledgeChatTraceContextKey{}).(*KnowledgeChatTrace)
	trace.RecordStage(stage, startedAt)
}

func MarkKnowledgeCacheHit(ctx context.Context, cache string) {
	trace, _ := ctx.Value(knowledgeChatTraceContextKey{}).(*KnowledgeChatTrace)
	trace.MarkCacheHit(cache)
}

func (trace *KnowledgeChatTrace) RecordStage(stage string, startedAt time.Time) {
	if trace == nil || stage == "" || startedAt.IsZero() {
		return
	}

	completedAt := time.Now()
	duration := completedAt.Sub(startedAt)
	trace.mu.Lock()
	trace.stages[stage] += duration
	trace.mu.Unlock()

	if trace.app != nil {
		trace.app.Logger().Debug(
			"Knowledge chat stage completed",
			slog.String("stage", stage),
			slog.Time("startedAt", startedAt.UTC()),
			slog.Time("completedAt", completedAt.UTC()),
			slog.Float64("durationMs", knowledgeDurationMS(duration)),
		)
	}
}

func (trace *KnowledgeChatTrace) RecordDuration(stage string, duration time.Duration) {
	if trace == nil || stage == "" || duration < 0 {
		return
	}
	trace.mu.Lock()
	trace.stages[stage] += duration
	trace.mu.Unlock()
}

func (trace *KnowledgeChatTrace) MarkCacheHit(cache string) {
	if trace == nil || cache == "" {
		return
	}
	trace.mu.Lock()
	trace.cacheHits[cache] = true
	trace.mu.Unlock()
}

func (trace *KnowledgeChatTrace) Complete(requestErr error) {
	if trace == nil {
		return
	}

	trace.mu.Lock()
	if trace.completed {
		trace.mu.Unlock()
		return
	}
	trace.completed = true
	stages := make(map[string]time.Duration, len(trace.stages))
	for stage, duration := range trace.stages {
		stages[stage] = duration
	}
	cacheHits := make([]string, 0, len(trace.cacheHits))
	for cache, hit := range trace.cacheHits {
		if hit {
			cacheHits = append(cacheHits, cache)
		}
	}
	trace.mu.Unlock()
	sort.Strings(cacheHits)

	if trace.app == nil {
		return
	}

	stageMS := make(map[string]float64, len(stages))
	for stage, duration := range stages {
		stageMS[stage] = knowledgeDurationMS(duration)
	}
	attrs := []any{
		slog.Time("startedAt", trace.startedAt.UTC()),
		slog.Float64("totalMs", knowledgeDurationMS(time.Since(trace.startedAt))),
		slog.Any("stagesMs", stageMS),
		slog.Any("cacheHits", cacheHits),
	}
	if requestErr != nil {
		attrs = append(attrs, slog.String("error", requestErr.Error()))
	}
	trace.app.Logger().Info("Knowledge chat request completed", attrs...)

	collector := resolveKnowledgeLatencyCollector(trace.app)
	if snapshot, shouldLog := collector.record(stages); shouldLog {
		trace.app.Logger().Info(
			"Knowledge chat latency percentiles",
			slog.Int64("requests", snapshot.Requests),
			slog.Int("window", snapshot.Window),
			slog.Any("stages", snapshot.Stages),
		)
	}
}

func KnowledgeLatencyStats(app App) KnowledgeLatencySnapshot {
	if app == nil {
		return KnowledgeLatencySnapshot{Stages: map[string]KnowledgeLatencyPercentiles{}}
	}
	return resolveKnowledgeLatencyCollector(app).snapshot()
}

func resolveKnowledgeLatencyCollector(app App) *knowledgeLatencyCollector {
	value := app.Store().GetOrSet(StoreKeyKnowledgeLatencyCollector, func() any {
		return &knowledgeLatencyCollector{samples: map[string][]time.Duration{}}
	})
	return value.(*knowledgeLatencyCollector)
}

func (collector *knowledgeLatencyCollector) record(
	stages map[string]time.Duration,
) (KnowledgeLatencySnapshot, bool) {
	collector.mu.Lock()
	defer collector.mu.Unlock()

	collector.requests++
	for stage, duration := range stages {
		collector.samples[stage] = append(collector.samples[stage], duration)
		if extra := len(collector.samples[stage]) - knowledgeLatencyWindowSize; extra > 0 {
			collector.samples[stage] = append([]time.Duration(nil), collector.samples[stage][extra:]...)
		}
	}

	shouldLog := collector.requests%knowledgeLatencyLogEvery == 0
	if !shouldLog {
		return KnowledgeLatencySnapshot{}, false
	}
	return collector.snapshotLocked(), true
}

func (collector *knowledgeLatencyCollector) snapshot() KnowledgeLatencySnapshot {
	collector.mu.Lock()
	defer collector.mu.Unlock()
	return collector.snapshotLocked()
}

func (collector *knowledgeLatencyCollector) snapshotLocked() KnowledgeLatencySnapshot {
	snapshot := KnowledgeLatencySnapshot{
		Requests: collector.requests,
		Window:   knowledgeLatencyWindowSize,
		Stages:   make(map[string]KnowledgeLatencyPercentiles, len(collector.samples)),
	}
	for stage, values := range collector.samples {
		sorted := append([]time.Duration(nil), values...)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		snapshot.Stages[stage] = KnowledgeLatencyPercentiles{
			Samples: len(sorted),
			P50MS:   knowledgeDurationMS(knowledgePercentile(sorted, 0.50)),
			P95MS:   knowledgeDurationMS(knowledgePercentile(sorted, 0.95)),
			P99MS:   knowledgeDurationMS(knowledgePercentile(sorted, 0.99)),
		}
	}
	return snapshot
}

func knowledgePercentile(values []time.Duration, percentile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1)*percentile + 0.5)
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

func knowledgeDurationMS(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}
