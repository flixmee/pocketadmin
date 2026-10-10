# Knowledge Base and RAG Support API

PocketAdmin includes a built-in retrieval-augmented support service. It stores
documents, chunks, embeddings, chat sessions, and messages in the primary
SQLite database and stores uploaded source files through the configured local
or S3 filesystem.

## Configure the models

Open **Settings → Application → AI provider**, enable the provider, and set:

- **Default model**: the chat/generation model used to draft answers.
- **Embedding model**: the model used for both document chunks and questions.

OpenAI-compatible providers use `POST {baseURL}/embeddings` and
`POST {baseURL}/chat/completions`. Gemini uses `batchEmbedContents` and
`generateContent`. Anthropic can generate answers but does not expose an
embedding API; applications using Anthropic must install a custom Go
`core.KnowledgeProvider`.

The embedding model follows the selected provider. OpenAI defaults to
`text-embedding-3-small`, Gemini defaults to `gemini-embedding-2`, and a
custom provider requires an explicit model name exposed by its
OpenAI-compatible endpoint. Anthropic has no settings-backed embedding model.

## Storage and retrieval

The system migration creates these protected collections:

- `kb_documents`
- `kb_chunks`
- `kb_chat_sessions`
- `kb_chat_messages`

`kb_chunks_fts` is an FTS5 virtual table maintained by SQLite triggers. Vector
embeddings are JSON arrays in `kb_chunks.embedding`; the default SQLite driver
registers `pb_vec_cosine_distance` for vector ranking. FTS5 retrieval runs in
parallel with query embedding plus vector retrieval, and the two rankings are
combined with reciprocal rank fusion. The default candidate pool is 10 and at
most five chunks are added to a chat prompt.

Exact duplicate query embeddings are held in a bounded in-memory LRU cache for
30 minutes. Final owner-scoped search results use a separate three-minute LRU
cache that is invalidated when an indexed document changes or is deleted. No
external cache service is required.

This design stays compatible with PocketBase's pure-Go SQLite driver. Apps
that need an external vector database or a platform-specific `sqlite-vec`
build can store a `core.KnowledgeSearchBackend` under
`core.StoreKeyKnowledgeSearchBackend` without changing the HTTP contract.

## Authentication and ownership

All `/api/kb` routes require an authenticated record. Regular users can only
access documents and sessions created under their auth record and auth
collection. Superusers can access all documents and sessions.

The underlying system collections have no public CRUD rules. Use the custom
routes instead of exposing them through generic record CRUD APIs.

## Endpoints

### Upload a document

```http
POST /api/kb/documents
Authorization: Bearer <auth-token>
Content-Type: multipart/form-data
```

Multipart fields:

- `file` (required, exactly one): `.txt`, `.md`, `.html`, `.docx`, or `.pdf`
- `title` (optional): defaults to the original filename without its extension
- `metadata` (optional): a JSON object

The response is `202 Accepted`. The ingestion worker extracts, chunks, embeds,
and indexes the document asynchronously. Pending and interrupted jobs are
recovered at application bootstrap and failed attempts are retried before the
document is marked `failed`.

PDFs must contain a text layer. Scanned or custom-font-encoded PDFs should be
OCRed before upload.

### List documents

```http
GET /api/kb/documents
Authorization: Bearer <auth-token>
```

Returns up to 100 documents ordered by creation time.

### Check indexing status

```http
GET /api/kb/documents/{id}/status
Authorization: Bearer <auth-token>
```

Example response:

```json
{
  "id": "document_id",
  "status": "indexed",
  "error": "",
  "chunkCount": 7,
  "updated": "2026-09-04 10:00:00.000Z"
}
```

Status is one of `pending`, `processing`, `indexed`, or `failed`.

### Delete a document

```http
DELETE /api/kb/documents/{id}
Authorization: Bearer <auth-token>
```

Returns `204 No Content`. Related chunks, FTS rows, and the stored file are
removed by collection relations, FTS triggers, and PocketBase's file manager.

### Ask a question

```http
POST /api/kb/chat
Authorization: Bearer <auth-token>
Content-Type: application/json
```

```json
{
  "question": "How do I reset my password?",
  "sessionId": "optional_existing_session_id",
  "topK": 5,
  "stream": false
}
```

`topK` defaults to 5 and is capped at 5 for chat prompts. The JSON response includes the
answer, session/message IDs, model usage, and the source chunks used to build
the prompt.

Set `stream` to `true`, or send `Accept: text/event-stream`, to receive SSE
events. A `ready` event establishes the stream, and upstream model deltas are
forwarded immediately as `token` events containing `{ "delta": "..." }`.
The final `done` event contains the complete JSON response. Streamed exchanges
are persisted asynchronously after `done`, so message IDs are intentionally
omitted from that event and a history read immediately afterward may briefly
precede the write.

Only the latest 10 messages are included as model context. The settings-backed
provider reuses one pooled keep-alive HTTP client; embedding calls have a
30-second deadline, regular generation calls 90 seconds, and streaming
generation calls two minutes.

## Latency measurement

Every chat request emits structured stage timings for `embed_query`,
`fts5_search`, `vector_search`, `rerank`, `llm_ttft`, `llm_total`, and
`db_write`. Cache hits are included in the request summary. PocketAdmin keeps a
rolling 100-request in-memory window and logs p50/p95/p99 stage percentiles
every 50 completed requests. Use real provider-backed traffic before making
model or infrastructure decisions; tests and synthetic requests should not be
treated as production latency evidence.

### Fetch session history

```http
GET /api/kb/sessions/{id}/messages
Authorization: Bearer <auth-token>
```

Returns up to 200 messages ordered oldest first.

## Custom provider

Set a provider before serving the app:

```go
type provider struct{}

func (provider) Embed(ctx context.Context, input []string) ([][]float32, error) {
    // Call a local model or another embedding service.
}

func (provider) Generate(ctx context.Context, prompt string) (core.KnowledgeGenerationResult, error) {
    // Generate an answer from the supplied source-grounded prompt.
}

app.Store().Set(core.StoreKeyKnowledgeProvider, provider{})
```

Custom providers may additionally implement `core.KnowledgeStreamingProvider`
to emit token deltas without buffering the complete answer. Implement
`core.KnowledgeProviderCacheKeyer` when query/result caching is safe for the
provider; changing the returned key invalidates its cache namespace.

The same embedding implementation must be used for ingestion and questions,
and every returned vector must have the same non-zero dimension.

To replace retrieval as well, implement `core.KnowledgeSearchBackend` and
store it under `core.StoreKeyKnowledgeSearchBackend`. The backend receives the
authenticated owner filters through `core.KnowledgeSearchOptions` and must
enforce them.
