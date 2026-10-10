Goal (incl. success criteria):

- Add a persistent exact/semantic query-result cache to the RAG chat pipeline.
- Success: exact duplicates skip embedding/retrieval/fusion, semantic near-matches reuse cached chunks after one query embedding, misses write through; cache is owner/provider scoped, TTL/max-size configurable, invalidated by referenced document changes, observable, and covered by focused tests.

Constraints/Assumptions:

- Follow `AGENTS.md` and `UI_DOCS.md`; preserve unrelated dirty worktree changes.
- The actual implementation stores embedding JSON and computes cosine in SQLite rather than using sqlite-vec; implement the requested small persistent cache against the real architecture without introducing an unavailable extension.
- Preserve incremental token streaming, existing source citations, owner isolation, and public provider/search overrides.
- Keep exact hits independent of the embedding provider call; semantic fallback may embed once.
- Real p50/p95/p99 values across 50–100 provider-backed requests require runtime traffic/credentials; implement instrumentation and benchmark seams, but do not fabricate measurements.
- Keep the public provider/search override interfaces intact.

Key decisions:

- Replace the earlier in-memory final-result cache with a persistent SQLite-backed module; retain only the in-memory embedding cache if it does not interfere with exact-hit semantics.
- Scope cache entries by owner, owner collection, provider cache key, and requested result limit so results cannot cross tenants/configurations.
- Parse the supported Markdown subset into an AST and render it with the UI's trusted `t.*` DOM builder; raw HTML/MDX expressions remain text and links are protocol-allowlisted.
- Keep parsing reactive so incomplete streaming markers stay readable and become formatted as the matching token arrives.
- Run lexical retrieval in parallel with the query-embedding/vector branch on `ConcurrentDB`; WAL and the single-writer pool were already configured correctly.
- Reuse one tuned keep-alive HTTP client and apply 30s embedding, 90s generation, and 2m streaming deadlines.
- Add an optional `KnowledgeStreamingProvider`; settings-backed OpenAI-compatible, Gemini, and Anthropic providers stream natively, while custom non-streaming providers retain a full-answer fallback.
- Cap chat retrieval/prompt context at five chunks and the latest 10 messages; reduce the default fusion candidate pool from 40 to 10.
- Use bounded in-memory LRU caches (30m query embeddings, 3m owner-scoped search results) and invalidate result caches on document changes/deletion.
- Record structured per-stage timings and rolling 100-request p50/p95/p99 aggregates every 50 completed chats; real provider-backed traffic is still required for meaningful benchmark values.

State:
  - Done:
    - Read the provided optimization task list and existing continuity ledger.
    - Previously completed and verified the Support document-upload popup/selection fix.
    - Audited the current pipeline: sequential retrieval, per-call clients, simulated post-generation SSE, non-streaming UI, 12-message history, and no caches/percentile telemetry were confirmed gaps.
    - Implemented parallel retrieval, request-scoped stage tracing, rolling latency percentiles, pooled provider HTTP, call-type deadlines, query/result LRU caches, cache invalidation, smaller candidate/context windows, native provider streaming, and asynchronous post-stream persistence.
    - Updated the Support UI to parse SSE incrementally, render token deltas immediately, preserve cancellation, and show a streaming state.
    - Added provider streaming, cache/invalidation, prompt bounding, latency, and asynchronous SSE persistence coverage; focused core and API knowledge tests pass.
    - Added optimization and observability details to `docs/knowledge-base-api.md`.
    - Verified `go build ./...` and `npm run build`.
    - Verified focused core and API knowledge suites under the Go race detector.
    - Refreshed the live Support page, verified the question/Ask interaction, and found no browser console warnings or errors.
    - Verified the incremental SSE parser separately with chunk-boundary and CRLF input.
    - Added reusable safe Markdown block/inline parsing and Support answer DOM rendering for headings, emphasis, lists, links, quotes, code, and citations.
    - Added focused Markdown parser coverage, including incomplete stream markers and unsafe link/raw-HTML handling.
    - Verified four focused Markdown/HTML/security tests, dprint, the UI production build, and the embedded Go production build.
    - Verified the updated live Support page reloads with no browser warnings or errors; the previous in-memory answer was cleared by the development reload and was not resent to avoid an unnecessary provider call/write.
  - Now:
    - Audit knowledge collections/settings and the current in-memory cache integration.
  - Next:
    - Add schema/config/module, integrate early lookup/write-through/invalidation/metrics, and run focused Go tests/builds.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: sqlite-vec is not present in the current implementation; semantic cache lookup will use bounded recent rows with Go cosine unless repository evidence shows the extension is available.

Working set (files/ids/commands):

- `CONTINUITY.md`
- `/Volumes/MacOS_WD/Users/hungtrancongvinh/hungtrancongvinh/Downloads/rag-chat-speed-optimization-tasks.md`
- `core/knowledge_cache.go`
- `core/knowledge_metrics.go`
- `core/knowledge_provider.go`
- `core/knowledge_search.go`
- `core/knowledge_ingest.go`
- `apis/knowledge_base.go`
- `ui/src/support/knowledgeChatStream.js`
- `ui/src/support/knowledgeAnswerMarkdown.js`
- `ui/src/support/pageKnowledgeBase.js`
- `ui/tests/knowledgeAnswerMarkdown.test.mjs`
- `ui/src/css/knowledgeBase.css`
- `node --test ui/tests/knowledgeAnswerMarkdown.test.mjs`
- `docs/knowledge-base-api.md`
- `go build ./...`
- `go test -race ./core -run 'Knowledge|SettingsKnowledgeProvider' -count=1`
- `go test -race ./apis -run '^TestKnowledge' -count=1`
- `cd ui && npm run build`
