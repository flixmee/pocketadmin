package apis

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

func TestFetchAIModels(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		name      string
		config    core.AIConfig
		handler   func(*testing.T) http.HandlerFunc
		expect    []settingsAIModel
		expectErr bool
	}{
		{
			name: "openai",
			config: core.AIConfig{
				Provider: core.AIProviderOpenAI,
				APIKey:   "test_key",
			},
			handler: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/models" {
						t.Fatalf("Expected path /models, got %q", r.URL.Path)
					}
					if r.Header.Get("Authorization") != "Bearer test_key" {
						t.Fatalf("Expected authorization header, got %q", r.Header.Get("Authorization"))
					}
					w.Write([]byte(`{"data":[{"id":"gpt-test"},{"id":"gpt-alpha"}]}`))
				}
			},
			expect: []settingsAIModel{
				{Id: "gpt-alpha", Label: "gpt-alpha"},
				{Id: "gpt-test", Label: "gpt-test"},
			},
		},
		{
			name: "gemini",
			config: core.AIConfig{
				Provider: core.AIProviderGemini,
				APIKey:   "test_key",
			},
			handler: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Query().Get("key") != "test_key" {
						t.Fatalf("Expected key query param, got %q", r.URL.Query().Get("key"))
					}
					w.Write([]byte(`{"models":[{"name":"models/gemini-test","displayName":"Gemini Test"}]}`))
				}
			},
			expect: []settingsAIModel{
				{Id: "gemini-test", Label: "Gemini Test"},
			},
		},
		{
			name: "anthropic",
			config: core.AIConfig{
				Provider: core.AIProviderAnthropic,
				APIKey:   "test_key",
			},
			handler: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					if r.Header.Get("x-api-key") != "test_key" {
						t.Fatalf("Expected x-api-key header, got %q", r.Header.Get("x-api-key"))
					}
					if r.Header.Get("anthropic-version") == "" {
						t.Fatal("Expected anthropic-version header")
					}
					w.Write([]byte(`{"data":[{"id":"claude-test","display_name":"Claude Test"}]}`))
				}
			},
			expect: []settingsAIModel{
				{Id: "claude-test", Label: "Claude Test"},
			},
		},
		{
			name: "provider error",
			config: core.AIConfig{
				Provider: core.AIProviderOpenAI,
				APIKey:   "test_key",
			},
			handler: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					http.Error(w, "bad key", http.StatusUnauthorized)
				}
			},
			expectErr: true,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(scenario.handler(t))
			defer server.Close()

			scenario.config.BaseURL = server.URL
			result, err := fetchAIModels(context.Background(), server.Client(), scenario.config)
			if scenario.expectErr {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			if len(result) != len(scenario.expect) {
				t.Fatalf("Expected %d models, got %d: %#v", len(scenario.expect), len(result), result)
			}
			for i := range scenario.expect {
				if result[i] != scenario.expect[i] {
					t.Fatalf("Expected model %d %#v, got %#v", i, scenario.expect[i], result[i])
				}
			}
		})
	}
}
