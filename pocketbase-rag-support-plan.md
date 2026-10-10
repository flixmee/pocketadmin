# RAG-Powered Support Feature for PocketBase — Implementation Plan

## 1. High-Level Architecture

```
┌─────────────┐    ┌──────────────┐    ┌─────────────────┐    ┌──────────────┐
│  Upload API  │───▶│  Ingestion   │───▶│  Storage Layer   │───▶│ Hybrid Search│
│ (PocketBase) │    │  Pipeline    │    │ (SQLite + Vec)   │    │   + Rerank   │
└─────────────┘    └──────────────┘    └─────────────────┘    └──────┬───────┘
                                                                      │
┌─────────────┐    ┌──────────────┐    ┌─────────────────┐         │
│  Chat API    │◀───│  LLM Answer  │◀───│  Context Builder │◀────────┘
│  (Question)  │    │  Generation  │    │  (Prompt + Docs) │
└─────────────┘    └──────────────┘    └─────────────────┘
```

Since this is a direct fork/extension of PocketBase's Go core (not just using it as a hosted backend), the plan hooks deeply into PocketBase's existing SQLite database, file storage, and event system rather than bolting on external services.

---

## 2. Data Model (New Collections)

| Collection | Purpose | Key Fields |
|---|---|---|
| `kb_documents` | Uploaded source files | `title`, `file`, `mime_type`, `status` (pending/processing/indexed/failed), `owner`, `metadata` (json) |
| `kb_chunks` | Chunked + embedded text | `document_id` (relation), `content`, `chunk_index`, `embedding` (vector), `token_count`, `metadata` |
| `kb_chat_sessions` | Conversation threads | `user`, `title`, `created` |
| `kb_chat_messages` | Q&A history | `session_id`, `role` (user/assistant), `content`, `retrieved_chunk_ids`, `created` |

`kb_chunks.embedding` is the key addition — this needs vector storage support in SQLite (see §4).

---

## 3. Ingestion Pipeline

**Trigger:** `OnRecordAfterCreateSuccess` hook on `kb_documents`, or a background job queue if you want async processing (recommended for large files).

**Steps:**
1. **Extract text** — based on `mime_type`:
   - PDF → `pdfcpu` or call out to `pdftotext`/`unidoc`
   - DOCX → `unioffice` or similar Go lib
   - Plain text/Markdown → direct read
   - HTML → strip tags
2. **Chunk** — split into overlapping segments (e.g. 500–800 tokens, 10–15% overlap). Prefer semantic/structure-aware splitting (by heading/paragraph) over naive fixed-size splitting.
3. **Embed** — call an embedding model (OpenAI `text-embedding-3-small`, or self-hosted via Ollama/`bge-small`) for each chunk.
4. **Store** — insert chunk rows with embeddings; update `kb_documents.status`.
5. **Index for keyword search** — leverage SQLite FTS5 virtual table over `kb_chunks.content`.

Run this as a **queued background worker** inside PocketBase's Go runtime (goroutine pool + a simple job table) so uploads don't block the HTTP response.

---

## 4. Vector Storage Option

Since PocketBase uses SQLite, the most natural fit is the **`sqlite-vec`** extension (successor to `sqlite-vss`), loaded directly into PocketBase's Go SQLite driver:

- Store embeddings in a `vec0` virtual table alongside `kb_chunks`.
- Supports `MATCH` queries for KNN vector search directly in SQL.
- Avoids standing up a separate vector DB (Qdrant/Weaviate/Milvus) — simpler ops, single binary, consistent with PocketBase's philosophy.

**Alternative:** if the corpus grows very large or QPS gets high, plan a pluggable interface so you can swap in Qdrant/pgvector later without rewriting the search layer.

---

## 5. Hybrid Search Design

Combine two retrieval signals and merge results:

1. **Keyword/lexical search** — SQLite FTS5 (BM25 ranking) over `kb_chunks.content`.
2. **Semantic/vector search** — `sqlite-vec` KNN search over `kb_chunks.embedding` using the embedded query.
3. **Fusion** — merge ranked lists with **Reciprocal Rank Fusion (RRF)**:
   ```
   score(doc) = Σ 1 / (k + rank_i(doc))   for each retrieval method i
   ```
   (k ≈ 60 is a common default). This avoids needing to normalize BM25 vs cosine scores directly.
4. **(Optional) Rerank** — pass top ~20 fused candidates through a cross-encoder reranker (e.g. `bge-reranker`, Cohere Rerank API) to get the final top-k (e.g. 5).

---

## 6. Query / Answer Pipeline

1. User sends a question via `POST /api/kb/chat`.
2. Embed the question (same embedding model as ingestion).
3. Run hybrid search → top-k chunks.
4. Build a prompt: system instructions + retrieved chunks (with source citations) + chat history + user question.
5. Call LLM (streaming preferred) → return answer + cited sources.
6. Persist the exchange in `kb_chat_messages`, including which `chunk_id`s were used (for traceability/debugging).

Support **streaming responses** (SSE or chunked HTTP) so the UI feels responsive.

---

## 7. New API Endpoints (custom PocketBase routes)

| Method | Route | Purpose |
|---|---|---|
| POST | `/api/kb/documents` | Upload + trigger ingestion |
| GET | `/api/kb/documents/:id/status` | Check indexing progress |
| DELETE | `/api/kb/documents/:id` | Delete doc + cascade chunks |
| POST | `/api/kb/chat` | Ask a question (streaming) |
| GET | `/api/kb/sessions/:id/messages` | Fetch chat history |

Register these via `app.OnServe().BindFunc(...)` or PocketBase's router extension, alongside the existing REST API.

---

## 8. Suggested Phased Roadmap

| Phase | Deliverable |
|---|---|
| **Phase 1** | Schema (`kb_documents`, `kb_chunks`), file upload, plain-text/PDF extraction, chunking |
| **Phase 2** | Embedding generation + `sqlite-vec` integration, vector search working end-to-end |
| **Phase 3** | FTS5 keyword search + RRF hybrid fusion |
| **Phase 4** | Chat endpoint with LLM integration, prompt construction, streaming |
| **Phase 5** | Reranking, citation UI, chat history, background job robustness (retries, failure states) |
| **Phase 6** | Admin UI panel (extend PocketBase's dashboard) for managing docs/collections, monitoring index status |

---

## 9. Key Technical Decisions to Make Early

- **Embedding model**: hosted API (OpenAI/Voyage/Cohere) vs self-hosted (Ollama + `bge`/`nomic-embed`) — affects cost, latency, data privacy.
- **LLM provider**: same tradeoff (Claude/OpenAI API vs self-hosted).
- **Async processing**: in-process goroutines vs external queue (e.g. if heavy upload volume is expected, consider a lightweight job table + worker pool pattern rather than blocking hooks).
- **Multi-tenancy**: if this is multi-org, make sure chunk/document rules respect PocketBase's existing `rules` (API access rules) for row-level security.
- **Chunking strategy**: fixed-size vs structure-aware — impacts answer quality significantly, worth iterating on.
