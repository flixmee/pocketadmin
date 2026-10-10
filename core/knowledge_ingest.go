package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/dbx"
	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/tools/hook"
)

const (
	CollectionNameKBDocuments    = "kb_documents"
	CollectionNameKBChunks       = "kb_chunks"
	CollectionNameKBChatSessions = "kb_chat_sessions"
	CollectionNameKBChatMessages = "kb_chat_messages"

	KBDocumentStatusPending    = "pending"
	KBDocumentStatusProcessing = "processing"
	KBDocumentStatusIndexed    = "indexed"
	KBDocumentStatusFailed     = "failed"

	KBChatRoleUser      = "user"
	KBChatRoleAssistant = "assistant"

	StoreKeyKnowledgeWorker = "pbAppKnowledgeWorker"
)

var kbDocumentStatuses = []string{
	KBDocumentStatusPending,
	KBDocumentStatusProcessing,
	KBDocumentStatusIndexed,
	KBDocumentStatusFailed,
}

var errKnowledgeDocumentChanged = errors.New("knowledge document changed during ingestion")

type knowledgeWorker struct {
	app    App
	ctx    context.Context
	cancel context.CancelFunc
	jobs   chan string

	mu     sync.Mutex
	queued map[string]struct{}
	wg     sync.WaitGroup
}

func (app *BaseApp) registerKnowledgeHooks() {
	app.OnBootstrap().Bind(&hook.Handler[*BootstrapEvent]{
		Id: "pbKnowledgeWorkerBootstrap",
		Func: func(e *BootstrapEvent) error {
			stopKnowledgeWorker(e.App)
			if err := e.Next(); err != nil {
				return err
			}

			recoverPendingKnowledgeDocuments(e.App)
			return nil
		},
		Priority: -100,
	})

	app.OnTerminate().Bind(&hook.Handler[*TerminateEvent]{
		Id: "pbKnowledgeWorkerTerminate",
		Func: func(e *TerminateEvent) error {
			stopKnowledgeWorker(e.App)
			return e.Next()
		},
		Priority: 100,
	})

	app.OnRecordValidate(CollectionNameKBDocuments).Bind(&hook.Handler[*RecordEvent]{
		Id: "pbKnowledgeDocumentValidate",
		Func: func(e *RecordEvent) error {
			if err := normalizeAndValidateKBDocument(e.Record); err != nil {
				return err
			}
			return e.Next()
		},
		Priority: 90,
	})

	app.OnRecordUpdate(CollectionNameKBDocuments).Bind(&hook.Handler[*RecordEvent]{
		Id: "pbKnowledgeDocumentReindex",
		Func: func(e *RecordEvent) error {
			if len(e.Record.GetUnsavedFiles("file")) > 0 {
				e.Record.Set("status", KBDocumentStatusPending)
				e.Record.Set("error", "")
			}
			return e.Next()
		},
		Priority: 90,
	})

	app.OnRecordAfterCreateSuccess(CollectionNameKBDocuments).Bind(&hook.Handler[*RecordEvent]{
		Id: "pbKnowledgeDocumentCreateQueue",
		Func: func(e *RecordEvent) error {
			if err := e.Next(); err != nil {
				return err
			}
			if e.Record.GetString("status") == KBDocumentStatusPending {
				QueueKnowledgeDocument(e.App, e.Record.Id)
			}
			return nil
		},
	})

	app.OnRecordAfterUpdateSuccess(CollectionNameKBDocuments).Bind(&hook.Handler[*RecordEvent]{
		Id: "pbKnowledgeDocumentUpdateQueue",
		Func: func(e *RecordEvent) error {
			if err := e.Next(); err != nil {
				return err
			}
			InvalidateKnowledgeQueryCacheForDocument(e.App, e.Record.Id)
			if e.Record.GetString("status") == KBDocumentStatusPending &&
				e.Record.Original().GetString("status") != KBDocumentStatusPending {
				QueueKnowledgeDocument(e.App, e.Record.Id)
			}
			return nil
		},
	})

	app.OnRecordAfterDeleteSuccess(CollectionNameKBDocuments).Bind(&hook.Handler[*RecordEvent]{
		Id: "pbKnowledgeDocumentDeleteCache",
		Func: func(e *RecordEvent) error {
			if err := e.Next(); err != nil {
				return err
			}
			InvalidateKnowledgeQueryCacheForDocument(e.App, e.Record.Id)
			return nil
		},
	})
}

func normalizeAndValidateKBDocument(record *Record) error {
	record.Set("title", strings.TrimSpace(record.GetString("title")))
	record.Set("mime_type", strings.TrimSpace(strings.ToLower(record.GetString("mime_type"))))
	record.Set("owner", strings.TrimSpace(record.GetString("owner")))
	record.Set("owner_collection", strings.TrimSpace(record.GetString("owner_collection")))
	status := strings.TrimSpace(strings.ToLower(record.GetString("status")))
	if status == "" {
		status = KBDocumentStatusPending
		record.Set("status", status)
	}

	errs := validation.Errors{}
	if err := validation.Validate(record.GetString("title"), validation.Required); err != nil {
		errs["title"] = err
	}
	if err := validation.Validate(status, validation.In(toAnySlice(kbDocumentStatuses)...)); err != nil {
		errs["status"] = err
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func startKnowledgeWorker(app App) *knowledgeWorker {
	value := app.Store().GetOrSet(StoreKeyKnowledgeWorker, func() any {
		ctx, cancel := context.WithCancel(context.Background())
		worker := &knowledgeWorker{
			app:    app,
			ctx:    ctx,
			cancel: cancel,
			jobs:   make(chan string, 128),
			queued: map[string]struct{}{},
		}
		for range 2 {
			worker.wg.Add(1)
			go worker.run()
		}
		return worker
	})
	return value.(*knowledgeWorker)
}

func stopKnowledgeWorker(app App) {
	worker, _ := app.Store().Get(StoreKeyKnowledgeWorker).(*knowledgeWorker)
	if worker == nil {
		return
	}
	worker.cancel()
	worker.wg.Wait()
	app.Store().Remove(StoreKeyKnowledgeWorker)
}

// QueueKnowledgeDocument schedules a document for asynchronous extraction and
// indexing. Repeated calls for the same record are coalesced.
func QueueKnowledgeDocument(app App, documentID string) {
	documentID = strings.TrimSpace(documentID)
	if app == nil || documentID == "" {
		return
	}
	worker, _ := app.Store().Get(StoreKeyKnowledgeWorker).(*knowledgeWorker)
	if worker == nil {
		worker = startKnowledgeWorker(app)
	}
	worker.enqueue(documentID)
}

func (worker *knowledgeWorker) enqueue(documentID string) {
	worker.mu.Lock()
	if _, exists := worker.queued[documentID]; exists {
		worker.mu.Unlock()
		return
	}
	worker.queued[documentID] = struct{}{}
	worker.mu.Unlock()

	select {
	case worker.jobs <- documentID:
	case <-worker.ctx.Done():
		worker.finish(documentID)
	default:
		worker.finish(documentID)
		worker.app.Logger().Warn("Knowledge ingestion queue is full", slog.String("documentId", documentID))
	}
}

func (worker *knowledgeWorker) finish(documentID string) {
	worker.mu.Lock()
	delete(worker.queued, documentID)
	worker.mu.Unlock()
}

func (worker *knowledgeWorker) run() {
	defer worker.wg.Done()
	for {
		select {
		case <-worker.ctx.Done():
			return
		case documentID := <-worker.jobs:
			requeue := worker.process(documentID)
			worker.finish(documentID)
			if requeue {
				worker.enqueue(documentID)
			}
		}
	}
}

func (worker *knowledgeWorker) process(documentID string) bool {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if worker.ctx.Err() != nil {
			return false
		}
		err = IngestKnowledgeDocument(worker.ctx, worker.app, documentID)
		if err == nil || errors.Is(err, context.Canceled) {
			return false
		}
		if errors.Is(err, errKnowledgeDocumentChanged) {
			return true
		}
		if attempt < 2 {
			delay := time.Duration(250*(1<<attempt)) * time.Millisecond
			select {
			case <-time.After(delay):
			case <-worker.ctx.Done():
				return false
			}
		}
	}

	worker.app.Logger().Error(
		"Knowledge document ingestion failed",
		slog.String("documentId", documentID),
		slog.String("error", err.Error()),
	)
	markKBDocumentFailed(worker.app, documentID, err)
	return false
}

func recoverPendingKnowledgeDocuments(app App) {
	if _, err := app.FindCachedCollectionByNameOrId(CollectionNameKBDocuments); err != nil {
		return
	}

	records := []*Record{}
	err := app.RecordQuery(CollectionNameKBDocuments).
		AndWhere(dbx.In("status", KBDocumentStatusPending, KBDocumentStatusProcessing)).
		All(&records)
	if err != nil {
		app.Logger().Warn("Failed to recover knowledge ingestion queue", slog.String("error", err.Error()))
		return
	}
	if len(records) == 0 {
		return
	}
	worker := startKnowledgeWorker(app)
	for _, record := range records {
		worker.enqueue(record.Id)
	}
}

// IngestKnowledgeDocument extracts, chunks, embeds, and atomically replaces a
// document's search index.
func IngestKnowledgeDocument(ctx context.Context, app App, documentID string) error {
	document, err := app.FindRecordById(CollectionNameKBDocuments, documentID)
	if err != nil {
		return err
	}
	document.Set("status", KBDocumentStatusProcessing)
	document.Set("error", "")
	if err := app.Save(document); err != nil {
		return fmt.Errorf("failed to mark knowledge document as processing: %w", err)
	}

	filename := document.GetString("file")
	if filename == "" {
		return errors.New("knowledge document has no file")
	}

	fsys, err := app.NewFilesystem()
	if err != nil {
		return err
	}
	defer fsys.Close()

	fileKey := document.BaseFilesPath() + "/" + filename
	reader, err := fsys.GetReader(fileKey)
	if err != nil {
		return fmt.Errorf("failed to open knowledge document: %w", err)
	}
	defer reader.Close()

	mimeType := document.GetString("mime_type")
	if mimeType == "" {
		if attrs, attrErr := fsys.Attributes(fileKey); attrErr == nil {
			mimeType = attrs.ContentType
		}
	}
	text, err := ExtractKnowledgeText(filename, mimeType, reader)
	if err != nil {
		return fmt.Errorf("failed to extract knowledge document: %w", err)
	}
	chunks, err := ChunkKnowledgeText(text, DefaultKnowledgeChunkTokens, DefaultKnowledgeChunkOverlap)
	if err != nil {
		return err
	}
	if len(chunks) == 0 {
		return errors.New("knowledge document produced no chunks")
	}

	provider := ResolveKnowledgeProvider(app)
	embeddings := make([][]float32, 0, len(chunks))
	for start := 0; start < len(chunks); start += 64 {
		end := start + 64
		if end > len(chunks) {
			end = len(chunks)
		}
		input := make([]string, end-start)
		for i := start; i < end; i++ {
			input[i-start] = chunks[i].Content
		}
		batch, embedErr := provider.Embed(ctx, input)
		if embedErr != nil {
			return fmt.Errorf("failed to embed knowledge chunks: %w", embedErr)
		}
		embeddings = append(embeddings, batch...)
	}
	if len(embeddings) != len(chunks) {
		return fmt.Errorf("embedding provider returned %d vectors for %d chunks", len(embeddings), len(chunks))
	}
	if _, err := validateKnowledgeEmbeddings(embeddings); err != nil {
		return err
	}

	err = app.RunInTransaction(func(txApp App) error {
		current, err := txApp.FindRecordById(CollectionNameKBDocuments, documentID)
		if err != nil {
			return err
		}
		if current.GetString("file") != filename {
			return errKnowledgeDocumentChanged
		}

		existing := []*Record{}
		if err := txApp.RecordQuery(CollectionNameKBChunks).
			AndWhere(dbx.HashExp{"document_id": documentID}).
			All(&existing); err != nil {
			return err
		}
		for _, chunk := range existing {
			if err := txApp.Delete(chunk); err != nil {
				return err
			}
		}

		chunkCollection, err := txApp.FindCachedCollectionByNameOrId(CollectionNameKBChunks)
		if err != nil {
			return err
		}
		for i, chunk := range chunks {
			record := NewRecord(chunkCollection)
			record.Set("document_id", documentID)
			record.Set("content", chunk.Content)
			record.Set("chunk_index", chunk.Index)
			record.Set("embedding", embeddings[i])
			record.Set("token_count", chunk.TokenCount)
			record.Set("metadata", map[string]any{
				"document_title": current.GetString("title"),
				"filename":       filename,
			})
			if err := txApp.Save(record); err != nil {
				return fmt.Errorf("failed to save knowledge chunk %d: %w", i, err)
			}
		}

		current.Set("status", KBDocumentStatusIndexed)
		current.Set("mime_type", mimeType)
		current.Set("error", "")
		if err := txApp.Save(current); err != nil {
			return fmt.Errorf("failed to mark knowledge document as indexed: %w", err)
		}
		return nil
	})
	if err == nil {
		InvalidateKnowledgeQueryCacheForDocument(app, documentID)
	}
	return err
}

func markKBDocumentFailed(app App, documentID string, ingestErr error) {
	document, err := app.FindRecordById(CollectionNameKBDocuments, documentID)
	if err != nil {
		return
	}
	message := "knowledge ingestion failed"
	if ingestErr != nil {
		message = ingestErr.Error()
	}
	if len(message) > 5000 {
		message = message[:5000]
	}
	document.Set("status", KBDocumentStatusFailed)
	document.Set("error", message)
	_ = app.Save(document)
}
