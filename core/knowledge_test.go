package core_test

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

type testKnowledgeProvider struct{}

type testKnowledgeSearchBackend struct {
	options core.KnowledgeSearchOptions
}

type cachedTestKnowledgeProvider struct {
	embedCalls atomic.Int32
}

func (provider *cachedTestKnowledgeProvider) KnowledgeCacheKey() string {
	return "cached-test-provider"
}

func (provider *cachedTestKnowledgeProvider) Embed(_ context.Context, input []string) ([][]float32, error) {
	provider.embedCalls.Add(1)
	result := make([][]float32, len(input))
	for i := range input {
		result[i] = []float32{1, 0}
	}
	return result, nil
}

func (*cachedTestKnowledgeProvider) Generate(
	_ context.Context,
	_ string,
) (core.KnowledgeGenerationResult, error) {
	return core.KnowledgeGenerationResult{Text: "answer"}, nil
}

func (backend *testKnowledgeSearchBackend) Search(
	_ context.Context,
	_ string,
	options core.KnowledgeSearchOptions,
) ([]core.KnowledgeSearchResult, error) {
	backend.options = options
	return []core.KnowledgeSearchResult{{ChunkID: "custom-result"}}, nil
}

func (testKnowledgeProvider) Embed(_ context.Context, input []string) ([][]float32, error) {
	result := make([][]float32, len(input))
	for i, text := range input {
		text = strings.ToLower(text)
		if strings.Contains(text, "password") || strings.Contains(text, "reset") {
			result[i] = []float32{1, 0}
		} else {
			result[i] = []float32{0, 1}
		}
	}
	return result, nil
}

func (testKnowledgeProvider) Generate(_ context.Context, prompt string) (core.KnowledgeGenerationResult, error) {
	return core.KnowledgeGenerationResult{Text: "Use the reset link [1].", Model: "test"}, nil
}

func TestExtractKnowledgeText(t *testing.T) {
	t.Parallel()

	docx := bytes.NewBuffer(nil)
	archive := zip.NewWriter(docx)
	document, err := archive.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := document.Write([]byte(`<w:document xmlns:w="urn:test"><w:body><w:p><w:r><w:t>DOCX heading</w:t></w:r></w:p><w:p><w:r><w:t>Second paragraph</w:t></w:r></w:p></w:body></w:document>`)); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	scenarios := []struct {
		name     string
		filename string
		mime     string
		input    []byte
		contains []string
	}{
		{
			name:     "markdown",
			filename: "guide.md",
			mime:     "text/markdown",
			input:    []byte("# Heading\n\nReset your password."),
			contains: []string{"# Heading", "Reset your password."},
		},
		{
			name:     "html",
			filename: "guide.html",
			mime:     "text/html",
			input:    []byte(`<h1>Guide</h1><script>hidden()</script><p>Reset &amp; retry.</p>`),
			contains: []string{"Guide", "Reset & retry."},
		},
		{
			name:     "docx",
			filename: "guide.docx",
			mime:     "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			input:    docx.Bytes(),
			contains: []string{"DOCX heading", "Second paragraph"},
		},
		{
			name:     "pdf",
			filename: "guide.pdf",
			mime:     "application/pdf",
			input:    []byte("%PDF-1.4\n1 0 obj << /Length 44 >> stream\nBT (Password reset guide) Tj T* (Try again) Tj ET\nendstream\nendobj\n%%EOF"),
			contains: []string{"Password reset guide", "Try again"},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			text, err := core.ExtractKnowledgeText(scenario.filename, scenario.mime, bytes.NewReader(scenario.input))
			if err != nil {
				t.Fatal(err)
			}
			for _, expected := range scenario.contains {
				if !strings.Contains(text, expected) {
					t.Fatalf("Expected extracted text to contain %q, got %q", expected, text)
				}
			}
			if strings.Contains(text, "hidden()") {
				t.Fatalf("Expected script content to be omitted, got %q", text)
			}
		})
	}
}

func TestChunkKnowledgeText(t *testing.T) {
	t.Parallel()

	paragraphs := make([]string, 8)
	for i := range paragraphs {
		paragraphs[i] = fmt.Sprintf("Paragraph %d contains several useful support words for chunk testing.", i)
	}
	chunks, err := core.ChunkKnowledgeText(strings.Join(paragraphs, "\n\n"), 25, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 3 {
		t.Fatalf("Expected at least 3 chunks, got %d", len(chunks))
	}
	for i, chunk := range chunks {
		if chunk.Index != i {
			t.Fatalf("Expected chunk index %d, got %d", i, chunk.Index)
		}
		if chunk.TokenCount == 0 || chunk.TokenCount > 25 {
			t.Fatalf("Unexpected chunk token count %d for %q", chunk.TokenCount, chunk.Content)
		}
	}

	punctuated := strings.Repeat("word! ", 30)
	chunks, err = core.ChunkKnowledgeText(punctuated, 10, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatalf("Expected punctuated text to split into multiple chunks, got %d", len(chunks))
	}
	for _, chunk := range chunks {
		if chunk.TokenCount > 10 {
			t.Fatalf("Expected chunk to stay within the token limit, got %d for %q", chunk.TokenCount, chunk.Content)
		}
	}
}

func TestKnowledgeIngestAndSearch(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	app.Store().Set(core.StoreKeyKnowledgeProvider, testKnowledgeProvider{})

	collection, err := app.FindCollectionByNameOrId(core.CollectionNameKBDocuments)
	if err != nil {
		t.Fatal(err)
	}
	upload, err := filesystem.NewFileFromBytes(
		[]byte("Password help\n\nTo reset your password, open account settings and select Reset password."),
		"password-help.md",
	)
	if err != nil {
		t.Fatal(err)
	}
	document := core.NewRecord(collection)
	document.Set("title", "Password help")
	document.Set("file", upload)
	document.Set("mime_type", "text/markdown")
	document.Set("status", core.KBDocumentStatusIndexed) // avoid the async create hook in this synchronous test
	document.Set("owner", "user-one")
	document.Set("owner_collection", "users")
	if err := app.Save(document); err != nil {
		t.Fatal(err)
	}

	if err := core.IngestKnowledgeDocument(context.Background(), app, document.Id); err != nil {
		t.Fatal(err)
	}
	indexed, err := app.FindRecordById(core.CollectionNameKBDocuments, document.Id)
	if err != nil {
		t.Fatal(err)
	}
	if indexed.GetString("status") != core.KBDocumentStatusIndexed {
		t.Fatalf("Expected indexed status, got %q (%s)", indexed.GetString("status"), indexed.GetString("error"))
	}

	results, err := core.SearchKnowledgeBase(context.Background(), app, "How do I reset my password?", core.KnowledgeSearchOptions{
		OwnerID:         "user-one",
		OwnerCollection: "users",
		Limit:           5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("Expected at least one search result")
	}
	if results[0].DocumentID != document.Id || !strings.Contains(results[0].Content, "Reset password") {
		t.Fatalf("Unexpected first result: %#v", results[0])
	}

	otherResults, err := core.SearchKnowledgeBase(context.Background(), app, "password", core.KnowledgeSearchOptions{
		OwnerID:         "different-user",
		OwnerCollection: "users",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(otherResults) != 0 {
		t.Fatalf("Expected owner isolation, got %#v", otherResults)
	}

	backend := new(testKnowledgeSearchBackend)
	app.Store().Set(core.StoreKeyKnowledgeSearchBackend, backend)
	customResults, err := core.SearchKnowledgeBase(context.Background(), app, "password", core.KnowledgeSearchOptions{
		OwnerID:         "user-one",
		OwnerCollection: "users",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(customResults) != 1 || customResults[0].ChunkID != "custom-result" || backend.options.OwnerID != "user-one" {
		t.Fatalf("Unexpected custom search result or owner options: %#v %#v", customResults, backend.options)
	}
}

func TestKnowledgeSearchCachesAndInvalidation(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()
	provider := new(cachedTestKnowledgeProvider)
	app.Store().Set(core.StoreKeyKnowledgeProvider, provider)

	collection, err := app.FindCollectionByNameOrId(core.CollectionNameKBDocuments)
	if err != nil {
		t.Fatal(err)
	}
	upload, err := filesystem.NewFileFromBytes([]byte("Password reset instructions."), "cache.md")
	if err != nil {
		t.Fatal(err)
	}
	document := core.NewRecord(collection)
	document.Set("title", "Cache test")
	document.Set("file", upload)
	document.Set("mime_type", "text/markdown")
	document.Set("status", core.KBDocumentStatusIndexed)
	document.Set("owner", "cache-user")
	document.Set("owner_collection", "users")
	if err := app.Save(document); err != nil {
		t.Fatal(err)
	}
	if err := core.IngestKnowledgeDocument(context.Background(), app, document.Id); err != nil {
		t.Fatal(err)
	}
	provider.embedCalls.Store(0)

	options := core.KnowledgeSearchOptions{Limit: 5}
	first, err := core.SearchKnowledgeBase(context.Background(), app, "password reset", options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := core.SearchKnowledgeBase(context.Background(), app, "password reset", options)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || len(second) != 1 || provider.embedCalls.Load() != 1 {
		t.Fatalf("Expected cached search and embedding, got %d/%d results and %d calls", len(first), len(second), provider.embedCalls.Load())
	}

	core.InvalidateKnowledgeSearchCache(app)
	if _, err := core.SearchKnowledgeBase(context.Background(), app, "password reset", options); err != nil {
		t.Fatal(err)
	}
	if provider.embedCalls.Load() != 1 {
		t.Fatalf("Expected query embedding cache to survive result invalidation, got %d calls", provider.embedCalls.Load())
	}
}

func TestBuildKnowledgePromptBoundsContext(t *testing.T) {
	history := make([]core.KnowledgeChatMessage, 12)
	for i := range history {
		history[i] = core.KnowledgeChatMessage{Role: "user", Content: fmt.Sprintf("history-%d", i)}
	}
	sources := make([]core.KnowledgeSearchResult, 7)
	for i := range sources {
		sources[i] = core.KnowledgeSearchResult{
			DocumentID:    fmt.Sprintf("document-%d", i),
			DocumentTitle: fmt.Sprintf("Source %d", i),
			ChunkIndex:    i,
			Content:       fmt.Sprintf("content-%d", i),
		}
	}

	prompt := core.BuildKnowledgePrompt("question", history, sources)
	if strings.Contains(prompt, "User: history-0\n") || strings.Contains(prompt, "User: history-1\n") {
		t.Fatalf("Expected oldest history messages to be omitted: %s", prompt)
	}
	if !strings.Contains(prompt, "history-2") || !strings.Contains(prompt, "history-11") {
		t.Fatalf("Expected latest history messages to remain: %s", prompt)
	}
	if strings.Contains(prompt, "content-5") || strings.Contains(prompt, "content-6") {
		t.Fatalf("Expected prompt sources to be capped at five: %s", prompt)
	}
	if !strings.Contains(prompt, "content-4") {
		t.Fatalf("Expected fifth source in prompt: %s", prompt)
	}
}

func TestKnowledgeLatencyStats(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	trace := core.NewKnowledgeChatTrace(app)
	trace.RecordDuration(core.KnowledgeStageEmbedQuery, 12*time.Millisecond)
	trace.RecordDuration(core.KnowledgeStageLLMTotal, 45*time.Millisecond)
	trace.Complete(nil)
	trace.Complete(nil)

	stats := core.KnowledgeLatencyStats(app)
	if stats.Requests != 1 || stats.Stages[core.KnowledgeStageEmbedQuery].Samples != 1 {
		t.Fatalf("Unexpected latency stats: %#v", stats)
	}
	if stats.Stages[core.KnowledgeStageLLMTotal].P50MS != 45 {
		t.Fatalf("Unexpected LLM latency percentile: %#v", stats.Stages[core.KnowledgeStageLLMTotal])
	}
}
