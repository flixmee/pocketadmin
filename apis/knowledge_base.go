package apis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/pocketbase/pocketbase/tools/router"
)

func bindKnowledgeBaseApi(app core.App, rg *router.RouterGroup[*core.RequestEvent]) {
	subGroup := rg.Group("/kb").Bind(RequireAuth())
	subGroup.GET("/documents", kbDocumentsList)
	subGroup.POST("/documents", kbDocumentCreate).Bind(BodyLimit(core.MaxKnowledgeDocumentBytes + (2 << 20)))
	subGroup.GET("/documents/{id}/status", kbDocumentStatus)
	subGroup.DELETE("/documents/{id}", kbDocumentDelete)
	subGroup.POST("/chat", kbChat)
	subGroup.GET("/sessions/{id}/messages", kbSessionMessages)
}

func kbDocumentsList(e *core.RequestEvent) error {
	query := e.App.RecordQuery(core.CollectionNameKBDocuments).OrderBy("created DESC").Limit(100)
	if !e.HasSuperuserAuth() {
		query.AndWhere(dbx.HashExp{
			"owner":            e.Auth.Id,
			"owner_collection": e.Auth.Collection().Id,
		})
	}
	documents := []*core.Record{}
	if err := query.All(&documents); err != nil {
		return e.BadRequestError("Failed to load knowledge documents.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, map[string]any{"items": documents})
	})
}

func kbDocumentCreate(e *core.RequestEvent) error {
	files, err := e.FindUploadedFiles("file")
	if err != nil || len(files) != 1 {
		return e.BadRequestError("Exactly one knowledge document file is required.", err)
	}
	upload := files[0]

	title := strings.TrimSpace(e.Request.FormValue("title"))
	if title == "" {
		title = strings.TrimSuffix(upload.OriginalName, filepath.Ext(upload.OriginalName))
	}
	metadata := map[string]any{}
	if raw := strings.TrimSpace(e.Request.FormValue("metadata")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
			return e.BadRequestError("Knowledge document metadata must be a JSON object.", err)
		}
	}

	mimeType, err := detectKnowledgeUploadMIME(upload)
	if err != nil {
		return e.BadRequestError("Failed to detect the knowledge document type.", err)
	}

	collection, err := e.App.FindCachedCollectionByNameOrId(core.CollectionNameKBDocuments)
	if err != nil {
		return e.InternalServerError("Knowledge base is not initialized.", err)
	}
	document := core.NewRecord(collection)
	document.Set("title", title)
	document.Set("file", upload)
	document.Set("mime_type", mimeType)
	document.Set("status", core.KBDocumentStatusPending)
	document.Set("owner", e.Auth.Id)
	document.Set("owner_collection", e.Auth.Collection().Id)
	document.Set("metadata", metadata)
	if err := e.App.Save(document); err != nil {
		return firstApiError(err, e.BadRequestError("Failed to upload knowledge document.", err))
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusAccepted, document)
	})
}

func kbDocumentStatus(e *core.RequestEvent) error {
	document, err := findAuthorizedKBDocument(e, e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("Knowledge document was not found.", err)
	}

	var chunkCount int
	if err := e.App.RecordQuery(core.CollectionNameKBChunks).
		AndWhere(dbx.HashExp{"document_id": document.Id}).
		Select("count(*)").
		Row(&chunkCount); err != nil {
		return e.InternalServerError("Failed to load knowledge document status.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, map[string]any{
			"id":         document.Id,
			"status":     document.GetString("status"),
			"error":      document.GetString("error"),
			"chunkCount": chunkCount,
			"updated":    document.Get("updated"),
		})
	})
}

func kbDocumentDelete(e *core.RequestEvent) error {
	document, err := findAuthorizedKBDocument(e, e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("Knowledge document was not found.", err)
	}
	if err := e.App.Delete(document); err != nil {
		return e.BadRequestError("Failed to delete knowledge document.", err)
	}
	core.InvalidateKnowledgeQueryCacheForDocument(e.App, document.Id)

	return execAfterSuccessTx(true, e.App, func() error {
		return e.NoContent(http.StatusNoContent)
	})
}

func findAuthorizedKBDocument(e *core.RequestEvent, id string) (*core.Record, error) {
	document, err := e.App.FindRecordById(core.CollectionNameKBDocuments, id)
	if err != nil {
		return nil, err
	}
	if !e.HasSuperuserAuth() &&
		(document.GetString("owner") != e.Auth.Id || document.GetString("owner_collection") != e.Auth.Collection().Id) {
		return nil, errors.New("knowledge document owner mismatch")
	}
	return document, nil
}

type kbChatRequest struct {
	Question  string `form:"question" json:"question"`
	SessionID string `form:"sessionId" json:"sessionId"`
	TopK      int    `form:"topK" json:"topK"`
	Stream    bool   `form:"stream" json:"stream"`
}

type kbChatResponse struct {
	SessionID          string                       `json:"sessionId"`
	UserMessageID      string                       `json:"userMessageId,omitempty"`
	AssistantMessageID string                       `json:"assistantMessageId,omitempty"`
	Answer             string                       `json:"answer"`
	Sources            []core.KnowledgeSearchResult `json:"sources"`
	Model              string                       `json:"model,omitempty"`
	TokenUsage         map[string]int               `json:"tokenUsage,omitempty"`
}

func kbChat(e *core.RequestEvent) (handlerErr error) {
	request := new(kbChatRequest)
	if err := e.BindBody(request); err != nil {
		return e.BadRequestError("Failed to load the chat request.", err)
	}
	request.Question = strings.TrimSpace(request.Question)
	if request.Question == "" {
		return e.BadRequestError("A question is required.", nil)
	}
	if len(request.Question) > 8000 {
		return e.BadRequestError("The question must not exceed 8000 bytes.", nil)
	}
	trace := core.NewKnowledgeChatTrace(e.App)
	traceCompletionDeferred := false
	defer func() {
		if !traceCompletionDeferred {
			trace.Complete(handlerErr)
		}
	}()
	requestContext := core.WithKnowledgeChatTrace(e.Request.Context(), trace)

	sessionStartedAt := time.Now()
	session, sessionCreated, err := resolveKBChatSession(e, request.SessionID, request.Question)
	if err != nil {
		return firstApiError(err, e.BadRequestError("Failed to resolve the chat session.", err))
	}
	if sessionCreated {
		trace.RecordStage(core.KnowledgeStageDBWrite, sessionStartedAt)
	}
	history, err := loadKBChatHistory(e.App, session.Id, 10)
	if err != nil {
		return e.InternalServerError("Failed to load chat history.", err)
	}

	searchOptions := core.KnowledgeSearchOptions{Limit: request.TopK}
	if searchOptions.Limit <= 0 || searchOptions.Limit > 5 {
		searchOptions.Limit = 5
	}
	if !e.HasSuperuserAuth() {
		searchOptions.OwnerID = e.Auth.Id
		searchOptions.OwnerCollection = e.Auth.Collection().Id
	}
	queryResult, err := core.SearchKnowledgeBaseWithCache(requestContext, e.App, request.Question, searchOptions)
	if err != nil {
		return e.BadRequestError("Knowledge retrieval failed.", err)
	}
	sources := queryResult.Sources

	if request.Stream || strings.Contains(e.Request.Header.Get("Accept"), "text/event-stream") {
		traceCompletionDeferred = true
		return streamKBChat(
			e,
			requestContext,
			trace,
			session.Id,
			request.Question,
			history,
			sources,
			queryResult.CacheEntryID,
			queryResult.CachedAnswer,
		)
	}

	answer := core.KnowledgeGenerationResult{Text: queryResult.CachedAnswer}
	if answer.Text == "" {
		llmStartedAt := time.Now()
		answer, err = core.GenerateKnowledgeAnswer(requestContext, e.App, request.Question, history, sources)
		trace.RecordStage(core.KnowledgeStageLLMTTFT, llmStartedAt)
		trace.RecordStage(core.KnowledgeStageLLMTotal, llmStartedAt)
		if err != nil {
			return e.BadRequestError("Knowledge answer generation failed.", err)
		}
	} else {
		trace.RecordDuration(core.KnowledgeStageLLMTTFT, 0)
		trace.RecordDuration(core.KnowledgeStageLLMTotal, 0)
	}

	dbStartedAt := time.Now()
	userMessage, assistantMessage, err := persistKBChatExchange(
		e.App,
		session.Id,
		request.Question,
		answer,
		sources,
	)
	trace.RecordStage(core.KnowledgeStageDBWrite, dbStartedAt)
	if err != nil {
		return e.InternalServerError("Failed to save the chat exchange.", err)
	}
	if err := core.UpdateKnowledgeQueryCacheAnswer(e.App, queryResult.CacheEntryID, answer.Text); err != nil {
		e.App.Logger().Warn(
			"Failed to cache knowledge answer",
			slog.String("cacheEntryId", queryResult.CacheEntryID),
			slog.String("error", err.Error()),
		)
	}

	response := kbChatResponse{
		SessionID:          session.Id,
		UserMessageID:      userMessage.Id,
		AssistantMessageID: assistantMessage.Id,
		Answer:             answer.Text,
		Sources:            sources,
		Model:              answer.Model,
		TokenUsage:         answer.TokenUsage,
	}
	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, response)
	})
}

func resolveKBChatSession(e *core.RequestEvent, sessionID, question string) (*core.Record, bool, error) {
	if sessionID != "" {
		session, err := e.App.FindRecordById(core.CollectionNameKBChatSessions, sessionID)
		if err != nil {
			return nil, false, err
		}
		if !e.HasSuperuserAuth() &&
			(session.GetString("user") != e.Auth.Id || session.GetString("user_collection") != e.Auth.Collection().Id) {
			return nil, false, errors.New("chat session owner mismatch")
		}
		return session, false, nil
	}

	collection, err := e.App.FindCachedCollectionByNameOrId(core.CollectionNameKBChatSessions)
	if err != nil {
		return nil, false, err
	}
	session := core.NewRecord(collection)
	session.Set("user", e.Auth.Id)
	session.Set("user_collection", e.Auth.Collection().Id)
	title := strings.TrimSpace(question)
	if len([]rune(title)) > 80 {
		title = string([]rune(title)[:80])
	}
	session.Set("title", title)
	if err := e.App.Save(session); err != nil {
		return nil, false, err
	}
	return session, true, nil
}

func streamKBChat(
	e *core.RequestEvent,
	ctx context.Context,
	trace *core.KnowledgeChatTrace,
	sessionID string,
	question string,
	history []core.KnowledgeChatMessage,
	sources []core.KnowledgeSearchResult,
	cacheEntryID string,
	cachedAnswer string,
) error {
	e.Response.Header().Set("Content-Type", "text/event-stream")
	e.Response.Header().Set("Cache-Control", "no-cache, no-transform")
	e.Response.Header().Set("Connection", "keep-alive")
	e.Response.Header().Set("X-Accel-Buffering", "no")
	e.Response.WriteHeader(http.StatusOK)
	if err := writeKBEvent(e.Response, "ready", map[string]string{"sessionId": sessionID}); err != nil {
		trace.Complete(err)
		return nil
	}
	if err := e.Flush(); err != nil {
		trace.Complete(err)
		return nil
	}

	answer := core.KnowledgeGenerationResult{Text: cachedAnswer}
	if answer.Text != "" {
		trace.RecordDuration(core.KnowledgeStageLLMTTFT, 0)
		trace.RecordDuration(core.KnowledgeStageLLMTotal, 0)
		if err := writeKBEvent(e.Response, "token", map[string]string{"delta": answer.Text}); err != nil {
			trace.Complete(err)
			return nil
		}
		if err := e.Flush(); err != nil {
			trace.Complete(err)
			return nil
		}
	} else {
		llmStartedAt := time.Now()
		firstToken := true
		var err error
		answer, err = core.StreamKnowledgeAnswer(ctx, e.App, question, history, sources, func(delta string) error {
			if firstToken {
				trace.RecordStage(core.KnowledgeStageLLMTTFT, llmStartedAt)
				firstToken = false
			}
			if err := writeKBEvent(e.Response, "token", map[string]string{"delta": delta}); err != nil {
				return err
			}
			return e.Flush()
		})
		trace.RecordStage(core.KnowledgeStageLLMTotal, llmStartedAt)
		if firstToken {
			trace.RecordStage(core.KnowledgeStageLLMTTFT, llmStartedAt)
		}
		if err != nil {
			_ = writeKBEvent(e.Response, "error", map[string]string{"message": "Knowledge answer generation failed."})
			_ = e.Flush()
			trace.Complete(err)
			return nil
		}
	}

	response := kbChatResponse{
		SessionID:  sessionID,
		Answer:     answer.Text,
		Sources:    sources,
		Model:      answer.Model,
		TokenUsage: answer.TokenUsage,
	}
	if err := writeKBEvent(e.Response, "done", response); err != nil {
		trace.Complete(err)
		return nil
	}
	if err := e.Flush(); err != nil {
		trace.Complete(err)
		return nil
	}

	app := e.App
	go func() {
		dbStartedAt := time.Now()
		_, _, persistErr := persistKBChatExchange(app, sessionID, question, answer, sources)
		cacheErr := core.UpdateKnowledgeQueryCacheAnswer(app, cacheEntryID, answer.Text)
		trace.RecordStage(core.KnowledgeStageDBWrite, dbStartedAt)
		trace.Complete(persistErr)
		if persistErr != nil {
			app.Logger().Error(
				"Failed to asynchronously save knowledge chat exchange",
				slog.String("sessionId", sessionID),
				slog.String("error", persistErr.Error()),
			)
		}
		if cacheErr != nil {
			app.Logger().Warn(
				"Failed to cache streamed knowledge answer",
				slog.String("cacheEntryId", cacheEntryID),
				slog.String("error", cacheErr.Error()),
			)
		}
	}()

	return nil
}

func persistKBChatExchange(
	app core.App,
	sessionID string,
	question string,
	answer core.KnowledgeGenerationResult,
	sources []core.KnowledgeSearchResult,
) (*core.Record, *core.Record, error) {
	chunkIDs := make([]string, len(sources))
	for i, source := range sources {
		chunkIDs[i] = source.ChunkID
	}

	var userMessage *core.Record
	var assistantMessage *core.Record
	err := app.RunInTransaction(func(txApp core.App) error {
		var saveErr error
		userMessage, saveErr = saveKBChatMessage(
			txApp,
			sessionID,
			core.KBChatRoleUser,
			question,
			nil,
			"",
			nil,
		)
		if saveErr != nil {
			return saveErr
		}
		assistantMessage, saveErr = saveKBChatMessage(
			txApp,
			sessionID,
			core.KBChatRoleAssistant,
			answer.Text,
			chunkIDs,
			answer.Model,
			answer.TokenUsage,
		)
		if saveErr != nil {
			return saveErr
		}
		currentSession, saveErr := txApp.FindRecordById(core.CollectionNameKBChatSessions, sessionID)
		if saveErr != nil {
			return saveErr
		}
		currentSession.Set("title", currentSession.GetString("title"))
		return txApp.Save(currentSession)
	})
	return userMessage, assistantMessage, err
}

func saveKBChatMessage(
	app core.App,
	sessionID string,
	role string,
	content string,
	chunkIDs []string,
	model string,
	tokenUsage map[string]int,
) (*core.Record, error) {
	collection, err := app.FindCachedCollectionByNameOrId(core.CollectionNameKBChatMessages)
	if err != nil {
		return nil, err
	}
	message := core.NewRecord(collection)
	message.Set("session_id", sessionID)
	message.Set("role", role)
	message.Set("content", content)
	if chunkIDs != nil {
		message.Set("retrieved_chunk_ids", chunkIDs)
	}
	message.Set("model", model)
	if tokenUsage != nil {
		message.Set("token_usage", tokenUsage)
	}
	if err := app.Save(message); err != nil {
		return nil, err
	}
	return message, nil
}

func loadKBChatHistory(app core.App, sessionID string, limit int) ([]core.KnowledgeChatMessage, error) {
	if limit <= 0 {
		limit = 10
	}
	records := []*core.Record{}
	err := app.RecordQuery(core.CollectionNameKBChatMessages).
		AndWhere(dbx.HashExp{"session_id": sessionID}).
		OrderBy("created DESC").
		Limit(int64(limit)).
		All(&records)
	if err != nil {
		return nil, err
	}

	history := make([]core.KnowledgeChatMessage, len(records))
	for i, record := range records {
		history[len(records)-1-i] = core.KnowledgeChatMessage{
			Role:    record.GetString("role"),
			Content: record.GetString("content"),
		}
	}
	return history, nil
}

func kbSessionMessages(e *core.RequestEvent) error {
	session, err := e.App.FindRecordById(core.CollectionNameKBChatSessions, e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("Chat session was not found.", err)
	}
	if !e.HasSuperuserAuth() &&
		(session.GetString("user") != e.Auth.Id || session.GetString("user_collection") != e.Auth.Collection().Id) {
		return e.NotFoundError("Chat session was not found.", nil)
	}

	messages := []*core.Record{}
	if err := e.App.RecordQuery(core.CollectionNameKBChatMessages).
		AndWhere(dbx.HashExp{"session_id": session.Id}).
		OrderBy("created ASC").
		Limit(200).
		All(&messages); err != nil {
		return e.BadRequestError("Failed to load chat messages.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, map[string]any{"items": messages})
	})
}

func writeKBEvent(writer io.Writer, event string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event, raw)
	return err
}

func detectKnowledgeUploadMIME(file *filesystem.File) (string, error) {
	reader, err := file.Reader.Open()
	if err != nil {
		return "", err
	}
	defer reader.Close()
	detected, err := mimetype.DetectReader(reader)
	if err == nil && detected.String() != "application/octet-stream" {
		return strings.Split(detected.String(), ";")[0], nil
	}
	if fallback := mime.TypeByExtension(strings.ToLower(filepath.Ext(file.OriginalName))); fallback != "" {
		return strings.Split(fallback, ";")[0], nil
	}
	if err != nil {
		return "", err
	}
	return detected.String(), nil
}
