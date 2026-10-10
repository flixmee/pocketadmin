package core_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

type knowledgeRoundTripper func(*http.Request) (*http.Response, error)

func (fn knowledgeRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestSettingsKnowledgeProvider(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	app.Settings().AI = core.AIConfig{
		Enabled:        true,
		Provider:       core.AIProviderOpenAI,
		APIKey:         "secret",
		Model:          "chat-model",
		EmbeddingModel: "embedding-model",
		BaseURL:        "https://provider.example/v1",
	}
	client := &http.Client{Transport: knowledgeRoundTripper(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("Expected bearer token, got %q", request.Header.Get("Authorization"))
		}
		var body string
		switch request.URL.Path {
		case "/v1/embeddings":
			body = `{"data":[{"index":1,"embedding":[0,1]},{"index":0,"embedding":[1,0]}]}`
		case "/v1/chat/completions":
			body = `{"choices":[{"message":{"content":"Grounded answer [1]."}}],"usage":{"prompt_tokens":9,"completion_tokens":4,"total_tokens":13}}`
		default:
			t.Fatalf("Unexpected provider path %q", request.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})}
	provider := &core.SettingsKnowledgeProvider{App: app, Client: client}

	vectors, err := provider.Embed(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 2 || vectors[0][0] != 1 || vectors[1][1] != 1 {
		t.Fatalf("Unexpected ordered embeddings: %#v", vectors)
	}

	answer, err := provider.Generate(context.Background(), "prompt")
	if err != nil {
		t.Fatal(err)
	}
	if answer.Text != "Grounded answer [1]." || answer.Model != "chat-model" || answer.TokenUsage["total"] != 13 {
		t.Fatalf("Unexpected generation response: %#v", answer)
	}
}

func TestSettingsKnowledgeProviderEmbeddingModelByProvider(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	app.Settings().AI = core.AIConfig{
		Enabled:  true,
		Provider: core.AIProviderGemini,
		APIKey:   "secret",
		BaseURL:  "https://provider.example/v1beta",
	}

	requestCount := 0
	client := &http.Client{Transport: knowledgeRoundTripper(func(request *http.Request) (*http.Response, error) {
		requestCount++
		if request.URL.Path != "/v1beta/models/gemini-embedding-2:batchEmbedContents" {
			t.Fatalf("Unexpected Gemini embedding path %q", request.URL.Path)
		}
		if request.URL.Query().Get("key") != "secret" {
			t.Fatalf("Expected Gemini API key query parameter")
		}

		raw, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !strings.Contains(string(raw), `"model":"models/gemini-embedding-2"`) {
			t.Fatalf("Expected provider-specific embedding model in %s", raw)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"embeddings":[{"values":[1,0]}]}`)),
			Request:    request,
		}, nil
	})}
	provider := &core.SettingsKnowledgeProvider{App: app, Client: client}

	vectors, err := provider.Embed(context.Background(), []string{"question"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 1 || len(vectors[0]) != 2 {
		t.Fatalf("Unexpected Gemini embeddings: %#v", vectors)
	}

	app.Settings().AI.Provider = core.AIProviderCustom
	app.Settings().AI.EmbeddingModel = ""
	_, err = provider.Embed(context.Background(), []string{"question"})
	if err == nil || !strings.Contains(err.Error(), "embedding model is required") {
		t.Fatalf("Expected missing custom embedding model error, got %v", err)
	}
	if requestCount != 1 {
		t.Fatalf("Expected missing custom model to fail before a provider request")
	}
}

func TestSettingsKnowledgeProviderGenerateStream(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	app.Settings().AI = core.AIConfig{
		Enabled:  true,
		Provider: core.AIProviderOpenAI,
		APIKey:   "secret",
		Model:    "chat-model",
		BaseURL:  "https://provider.example/v1",
	}
	client := &http.Client{Transport: knowledgeRoundTripper(func(request *http.Request) (*http.Response, error) {
		raw, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !strings.Contains(string(raw), `"stream":true`) {
			t.Fatalf("Expected a streaming request, got %s", raw)
		}
		body := strings.Join([]string{
			`data: {"model":"stream-model","choices":[{"delta":{"content":"Grounded "}}]}`,
			"",
			`data: {"choices":[{"delta":{"content":"answer [1]."}}]}`,
			"",
			`data: {"choices":[],"usage":{"prompt_tokens":9,"completion_tokens":4,"total_tokens":13}}`,
			"",
			"data: [DONE]",
			"",
		}, "\n")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})}
	provider := &core.SettingsKnowledgeProvider{App: app, Client: client}
	deltas := []string{}
	result, err := provider.GenerateStream(context.Background(), "prompt", func(delta string) error {
		deltas = append(deltas, delta)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(deltas, "") != "Grounded answer [1]." {
		t.Fatalf("Unexpected streamed deltas: %#v", deltas)
	}
	if result.Text != "Grounded answer [1]." || result.Model != "stream-model" || result.TokenUsage["total"] != 13 {
		t.Fatalf("Unexpected streaming result: %#v", result)
	}
}
