package apis_test

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

type apiTestKnowledgeProvider struct{}

const (
	testKnowledgeOwnerID         = "4q1xlclmfloku33"
	testKnowledgeOwnerCollection = "_pb_users_auth_"
	testKnowledgeOwnerAuthHeader = "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwiZXhwIjoyNTI0NjA0NDYxLCJyZWZyZXNoYWJsZSI6dHJ1ZX0.ZT3F0Z3iM-xbGgSG3LEKiEzHrPHr8t8IuHLZGGNuxLo"
	testKnowledgeOtherAuthHeader = "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6Im9hcDY0MGNvdDR5cnUycyIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwiZXhwIjoyNTI0NjA0NDYxLCJyZWZyZXNoYWJsZSI6dHJ1ZX0.GfJo6EHIobgas_AXt-M-tj5IoQendPnrkMSe9ExuSEY"
)

func (apiTestKnowledgeProvider) KnowledgeCacheKey() string {
	return "api-test-knowledge-provider"
}

func (apiTestKnowledgeProvider) Embed(_ context.Context, input []string) ([][]float32, error) {
	result := make([][]float32, len(input))
	for i, text := range input {
		if strings.Contains(strings.ToLower(text), "password") {
			result[i] = []float32{1, 0}
		} else {
			result[i] = []float32{0, 1}
		}
	}
	return result, nil
}

func (apiTestKnowledgeProvider) Generate(_ context.Context, _ string) (core.KnowledgeGenerationResult, error) {
	return core.KnowledgeGenerationResult{
		Text:       "Open account settings and use Reset password [1].",
		Model:      "test-model",
		TokenUsage: map[string]int{"input": 10, "output": 8, "total": 18},
	}, nil
}

func (apiTestKnowledgeProvider) GenerateStream(
	_ context.Context,
	_ string,
	onDelta func(string) error,
) (core.KnowledgeGenerationResult, error) {
	for _, delta := range []string{"Open account settings ", "and use Reset password [1]."} {
		if err := onDelta(delta); err != nil {
			return core.KnowledgeGenerationResult{}, err
		}
	}
	return core.KnowledgeGenerationResult{
		Text:       "Open account settings and use Reset password [1].",
		Model:      "test-model",
		TokenUsage: map[string]int{"input": 10, "output": 8, "total": 18},
	}, nil
}

func TestKnowledgeDocumentsAuth(t *testing.T) {
	unauthorized := tests.ApiScenario{
		Name:            "unauthorized list",
		Method:          http.MethodGet,
		URL:             "/api/kb/documents",
		ExpectedStatus:  http.StatusUnauthorized,
		ExpectedContent: []string{`"data":{}`},
	}
	unauthorized.Test(t)

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("title", "Password guide"); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("file", "password-guide.md")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("Reset your password from account settings.")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	upload := tests.ApiScenario{
		Name:   "authorized upload",
		Method: http.MethodPost,
		URL:    "/api/kb/documents",
		Body:   body,
		Headers: map[string]string{
			"Authorization": testSuperuserAuthHeader,
			"Content-Type":  writer.FormDataContentType(),
		},
		TestAppFactory: newKnowledgeAPITestApp,
		ExpectedStatus: http.StatusAccepted,
		ExpectedContent: []string{
			`"title":"Password guide"`,
			`"status":"pending"`,
			`"mime_type":"text/plain"`,
		},
	}
	upload.Test(t)
}

func TestKnowledgeChat(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:   "retrieves, answers, and persists",
		Method: http.MethodPost,
		URL:    "/api/kb/chat",
		Body:   strings.NewReader(`{"question":"How do I reset my password?","topK":3}`),
		Headers: map[string]string{
			"Authorization": testSuperuserAuthHeader,
		},
		TestAppFactory: newKnowledgeAPITestApp,
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
			createKnowledgeAPIFixture(t, app)
		},
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`"answer":"Open account settings and use Reset password [1]."`,
			`"documentTitle":"Password help"`,
			`"model":"test-model"`,
			`"total":18`,
		},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, _ *http.Response) {
			messages := []*core.Record{}
			if err := app.RecordQuery(core.CollectionNameKBChatMessages).All(&messages); err != nil {
				t.Fatal(err)
			}
			if len(messages) != 2 {
				t.Fatalf("Expected 2 persisted messages, got %d", len(messages))
			}
		},
	}
	scenario.Test(t)
}

func TestKnowledgeChatExactCachedAnswer(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:   "reuses optional exact cached answer",
		Method: http.MethodPost,
		URL:    "/api/kb/chat",
		Body:   strings.NewReader(`{"question":"cached password question","topK":3}`),
		Headers: map[string]string{
			"Authorization": testSuperuserAuthHeader,
		},
		TestAppFactory: newKnowledgeAPITestApp,
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
			createKnowledgeAPIFixture(t, app)
			app.Settings().AI.KnowledgeCacheAnswers = true
			result, err := core.SearchKnowledgeBaseWithCache(
				context.Background(),
				app,
				"cached password question",
				core.KnowledgeSearchOptions{Limit: 3},
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.CacheEntryID == "" {
				t.Fatal("Expected a cache entry")
			}
			if err := core.UpdateKnowledgeQueryCacheAnswer(app, result.CacheEntryID, "Cached literal answer [1]."); err != nil {
				t.Fatal(err)
			}
		},
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`"answer":"Cached literal answer [1]."`,
			`"documentTitle":"Password help"`,
		},
		NotExpectedContent: []string{
			`"model":"test-model"`,
			`Open account settings`,
		},
	}
	scenario.Test(t)
}

func TestKnowledgeChatSSE(t *testing.T) {
	scenario := tests.ApiScenario{
		Name:   "event stream",
		Method: http.MethodPost,
		URL:    "/api/kb/chat",
		Body:   strings.NewReader(`{"question":"password help","stream":true}`),
		Headers: map[string]string{
			"Authorization": testSuperuserAuthHeader,
			"Accept":        "text/event-stream",
		},
		TestAppFactory: newKnowledgeAPITestApp,
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
			createKnowledgeAPIFixture(t, app)
		},
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			"event: ready",
			"event: token",
			`"delta":"Open account settings "`,
			"event: done",
			`"documentTitle":"Password help"`,
		},
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, _ *http.Response) {
			deadline := time.Now().Add(2 * time.Second)
			for {
				messages := []*core.Record{}
				if err := app.RecordQuery(core.CollectionNameKBChatMessages).All(&messages); err != nil {
					t.Fatal(err)
				}
				if len(messages) == 2 {
					return
				}
				if time.Now().After(deadline) {
					t.Fatalf("Expected asynchronous chat persistence, got %d messages", len(messages))
				}
				time.Sleep(10 * time.Millisecond)
			}
		},
	}
	scenario.Test(t)
}

func TestKnowledgeDocumentStatusAndDelete(t *testing.T) {
	status := tests.ApiScenario{
		Name:   "document status",
		Method: http.MethodGet,
		URL:    "/api/kb/documents/kbdocapi0000001/status",
		Headers: map[string]string{
			"Authorization": testSuperuserAuthHeader,
		},
		TestAppFactory: newKnowledgeAPITestApp,
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
			createKnowledgeAPIFixture(t, app)
		},
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`"status":"indexed"`,
			`"chunkCount":1`,
		},
	}
	status.Test(t)

	deleteScenario := tests.ApiScenario{
		Name:   "document delete cascades chunks",
		Method: http.MethodDelete,
		URL:    "/api/kb/documents/kbdocapi0000001",
		Headers: map[string]string{
			"Authorization": testSuperuserAuthHeader,
		},
		TestAppFactory: newKnowledgeAPITestApp,
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
			createKnowledgeAPIFixture(t, app)
		},
		ExpectedStatus: http.StatusNoContent,
		AfterTestFunc: func(t testing.TB, app *tests.TestApp, _ *http.Response) {
			if _, err := app.FindRecordById(core.CollectionNameKBDocuments, "kbdocapi0000001"); err == nil {
				t.Fatal("Expected the document to be deleted")
			}
			chunks := []*core.Record{}
			if err := app.RecordQuery(core.CollectionNameKBChunks).All(&chunks); err != nil {
				t.Fatal(err)
			}
			if len(chunks) != 0 {
				t.Fatalf("Expected chunk cascade delete, got %d chunks", len(chunks))
			}
		},
	}
	deleteScenario.Test(t)
}

func TestKnowledgeOwnershipAndSessionMessages(t *testing.T) {
	documentStatus := tests.ApiScenario{
		Name:   "owner can read document status",
		Method: http.MethodGet,
		URL:    "/api/kb/documents/kbdocapi0000001/status",
		Headers: map[string]string{
			"Authorization": testKnowledgeOwnerAuthHeader,
		},
		TestAppFactory: newKnowledgeAPITestApp,
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
			createKnowledgeAPIFixture(t, app)
		},
		ExpectedStatus:  http.StatusOK,
		ExpectedContent: []string{`"status":"indexed"`},
	}
	documentStatus.Test(t)

	otherUserStatus := documentStatus
	otherUserStatus.Name = "other user cannot read document status"
	otherUserStatus.Headers = map[string]string{"Authorization": testKnowledgeOtherAuthHeader}
	otherUserStatus.ExpectedStatus = http.StatusNotFound
	otherUserStatus.ExpectedContent = []string{`"status":404`}
	otherUserStatus.Test(t)

	sessionMessages := tests.ApiScenario{
		Name:   "owner can read session messages",
		Method: http.MethodGet,
		URL:    "/api/kb/sessions/kbsessionapi001/messages",
		Headers: map[string]string{
			"Authorization": testKnowledgeOwnerAuthHeader,
		},
		TestAppFactory: newKnowledgeAPITestApp,
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, _ *core.ServeEvent) {
			createKnowledgeSessionFixture(t, app)
		},
		ExpectedStatus: http.StatusOK,
		ExpectedContent: []string{
			`"role":"user"`,
			`"content":"How do I reset my password?"`,
		},
	}
	sessionMessages.Test(t)

	otherUserMessages := sessionMessages
	otherUserMessages.Name = "other user cannot read session messages"
	otherUserMessages.Headers = map[string]string{"Authorization": testKnowledgeOtherAuthHeader}
	otherUserMessages.ExpectedStatus = http.StatusNotFound
	otherUserMessages.ExpectedContent = []string{`"status":404`}
	otherUserMessages.Test(t)
}

func newKnowledgeAPITestApp(t testing.TB) *tests.TestApp {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	app.Store().Set(core.StoreKeyKnowledgeProvider, apiTestKnowledgeProvider{})
	return app
}

func createKnowledgeAPIFixture(t testing.TB, app *tests.TestApp) {
	t.Helper()
	documents, err := app.FindCollectionByNameOrId(core.CollectionNameKBDocuments)
	if err != nil {
		t.Fatal(err)
	}
	upload, err := filesystem.NewFileFromBytes([]byte("Password help source"), "password.md")
	if err != nil {
		t.Fatal(err)
	}
	document := core.NewRecord(documents)
	document.SetRaw("id", "kbdocapi0000001")
	document.Set("title", "Password help")
	document.Set("file", upload)
	document.Set("mime_type", "text/markdown")
	document.Set("status", core.KBDocumentStatusIndexed)
	document.Set("owner", testKnowledgeOwnerID)
	document.Set("owner_collection", testKnowledgeOwnerCollection)
	if err := app.Save(document); err != nil {
		t.Fatal(err)
	}

	chunks, err := app.FindCollectionByNameOrId(core.CollectionNameKBChunks)
	if err != nil {
		t.Fatal(err)
	}
	chunk := core.NewRecord(chunks)
	chunk.SetRaw("id", "kbchunkapi00001")
	chunk.Set("document_id", document.Id)
	chunk.Set("content", "To reset your password, open account settings and select Reset password.")
	chunk.Set("chunk_index", 0)
	chunk.Set("embedding", []float32{1, 0})
	chunk.Set("token_count", 12)
	if err := app.Save(chunk); err != nil {
		t.Fatal(err)
	}
	if _, err := core.SearchKnowledgeBase(context.Background(), app, "password", core.KnowledgeSearchOptions{}); err != nil {
		t.Fatalf("Invalid knowledge API fixture: %v", err)
	}
}

func createKnowledgeSessionFixture(t testing.TB, app *tests.TestApp) {
	t.Helper()
	sessions, err := app.FindCollectionByNameOrId(core.CollectionNameKBChatSessions)
	if err != nil {
		t.Fatal(err)
	}
	session := core.NewRecord(sessions)
	session.SetRaw("id", "kbsessionapi001")
	session.Set("user", testKnowledgeOwnerID)
	session.Set("user_collection", testKnowledgeOwnerCollection)
	session.Set("title", "Password help")
	if err := app.Save(session); err != nil {
		t.Fatal(err)
	}

	messages, err := app.FindCollectionByNameOrId(core.CollectionNameKBChatMessages)
	if err != nil {
		t.Fatal(err)
	}
	message := core.NewRecord(messages)
	message.SetRaw("id", "kbmessageapi001")
	message.Set("session_id", session.Id)
	message.Set("role", core.KBChatRoleUser)
	message.Set("content", "How do I reset my password?")
	if err := app.Save(message); err != nil {
		t.Fatal(err)
	}
}
