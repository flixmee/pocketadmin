package core_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// fakeMemoryAIProvider answers each request with "answer N" and records the request bodies.
func fakeMemoryAIProvider(provider string, bodies *[]map[string]any, mu *sync.Mutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)

		mu.Lock()
		*bodies = append(*bodies, body)
		answer := fmt.Sprintf("answer %d", len(*bodies))
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch provider {
		case core.AIProviderAnthropic:
			fmt.Fprintf(w, `{"content":[{"type":"text","text":%q}],"usage":{"input_tokens":1,"output_tokens":1}}`, answer)
		case core.AIProviderGemini:
			fmt.Fprintf(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":%q}]}}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1}}`, answer)
		default:
			fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":%q}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`, answer)
		}
	}
}

// requestHistory extracts the "role: text" conversation from a provider request body.
func requestHistory(provider string, body map[string]any) []string {
	result := []string{}

	key := "messages"
	if provider == core.AIProviderGemini {
		key = "contents"
	}

	items, _ := body[key].([]any)
	for _, item := range items {
		m, _ := item.(map[string]any)
		role, _ := m["role"].(string)

		var text string
		if provider == core.AIProviderGemini {
			parts, _ := m["parts"].([]any)
			if len(parts) > 0 {
				text, _ = parts[0].(map[string]any)["text"].(string)
			}
		} else {
			text, _ = m["content"].(string)
		}

		result = append(result, role+": "+text)
	}

	return result
}

func TestAutomationAIGenerateMemory(t *testing.T) {
	t.Parallel()

	for _, provider := range []string{core.AIProviderOpenAI, core.AIProviderAnthropic, core.AIProviderGemini} {
		t.Run(provider, func(t *testing.T) {
			app, _ := tests.NewTestApp()
			defer app.Cleanup()

			var mu sync.Mutex
			var bodies []map[string]any

			server := httptest.NewServer(fakeMemoryAIProvider(provider, &bodies, &mu))
			defer server.Close()

			app.Settings().AI.Enabled = true
			app.Settings().AI.Provider = provider
			app.Settings().AI.APIKey = "test_key"
			app.Settings().AI.Model = "test-model"
			app.Settings().AI.BaseURL = server.URL

			automation := core.NewAutomation(app)
			populateValidAutomation(automation)
			automation.SetActive(true)
			automation.SetSteps(mustParseJSONRaw(t, `[{
				"type":"ai.generate",
				"input":"{{input.message}}",
				"memory":{"enabled":true,"key":"chat-{{input.chat}}","maxMessages":4}
			}]`))
			if err := app.Save(automation); err != nil {
				t.Fatal(err)
			}

			run := func(chat string, message string) map[string]any {
				result, err := app.RunAutomationManuallyWithOptions(automation.Id, core.AutomationManualRunOptions{
					Input: map[string]any{"chat": chat, "message": message},
					Wait:  true,
				})
				if err != nil {
					t.Fatal(err)
				}
				if result.Status != core.AutomationRunStatusSuccess {
					t.Fatalf("Expected success, got %q: %s", result.Status, result.Error)
				}
				raw, _ := json.Marshal(result.Output)
				output := map[string]any{}
				_ = json.Unmarshal(raw, &output)
				return output
			}

			run("A", "hi, I'm Harry")
			run("B", "hello from B")
			out := run("A", "what is my name?")
			run("A", "third")

			memoryOutput, _ := out["memory"].(map[string]any)
			if memoryOutput["key"] != "chat-A" || memoryOutput["history"] != float64(2) {
				t.Fatalf("Unexpected memory output %#v", out["memory"])
			}

			mu.Lock()
			defer mu.Unlock()

			assistant := "assistant"
			if provider == core.AIProviderGemini {
				assistant = "model"
			}

			expected := [][]string{
				{"user: hi, I'm Harry"},
				{"user: hello from B"},
				{"user: hi, I'm Harry", assistant + ": answer 1", "user: what is my name?"},
				// maxMessages = 4 -> both previous turns of chat A
				{"user: hi, I'm Harry", assistant + ": answer 1", "user: what is my name?", assistant + ": answer 3", "user: third"},
			}

			for i, exp := range expected {
				got := requestHistory(provider, bodies[i])
				if strings.Join(got, "|") != strings.Join(exp, "|") {
					t.Fatalf("Request %d: expected history\n%v\ngot\n%v", i+1, exp, got)
				}
			}

			// stored memory is pruned to maxMessages per session
			messages, err := core.FindAutomationAIMemoryMessages(app, automation.Id, "chat-A")
			if err != nil {
				t.Fatal(err)
			}
			got := []string{}
			for _, m := range messages {
				got = append(got, m.Role+": "+m.Content)
			}
			if strings.Join(got, "|") != "user: what is my name?|assistant: answer 3|user: third|assistant: answer 4" {
				t.Fatalf("Unexpected stored memory %v", got)
			}

			sessions, err := core.FindAutomationAIMemorySessions(app, automation.Id)
			if err != nil || len(sessions) != 2 {
				t.Fatalf("Expected 2 sessions, got %v (%v)", sessions, err)
			}

			// deleting the automation deletes its memory
			if err := app.Delete(automation); err != nil {
				t.Fatal(err)
			}
			sessions, _ = core.FindAutomationAIMemorySessions(app, automation.Id)
			if len(sessions) != 0 {
				t.Fatalf("Expected the memory to be deleted, got %v", sessions)
			}
		})
	}
}

func TestAutomationAIMemoryHelpers(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	// an odd limit must not start the history with an assistant message
	for i := 1; i <= 2; i++ {
		if err := core.SaveAutomationAIMemoryTurn(app, "auto1", "k", fmt.Sprintf("q%d", i), fmt.Sprintf("a%d", i), 10); err != nil {
			t.Fatal(err)
		}
	}
	if err := core.SaveAutomationAIMemoryTurn(app, "auto2", "k", "other", "other", 10); err != nil {
		t.Fatal(err)
	}

	history, err := core.LoadAutomationAIMemory(app, "auto1", "k", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 || history[0].Role != "user" || history[0].Content != "q2" || history[1].Content != "a2" {
		t.Fatalf("Unexpected history %+v", history)
	}

	if err := core.DeleteAutomationAIMemory(app, "auto1", "missing"); err != nil {
		t.Fatal(err)
	}
	if history, _ := core.LoadAutomationAIMemory(app, "auto1", "k", 10); len(history) != 4 {
		t.Fatalf("Expected other sessions to be kept, got %d messages", len(history))
	}

	if err := core.DeleteAutomationAIMemory(app, "auto1", ""); err != nil {
		t.Fatal(err)
	}
	if history, _ := core.LoadAutomationAIMemory(app, "auto1", "k", 10); len(history) != 0 {
		t.Fatalf("Expected the memory to be deleted, got %d messages", len(history))
	}
	if history, _ := core.LoadAutomationAIMemory(app, "auto2", "k", 10); len(history) != 2 {
		t.Fatalf("Expected the other automation memory to be kept, got %d messages", len(history))
	}
}

func TestAutomationAIMemoryValidation(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	scenarios := []struct {
		name     string
		steps    string
		expected string
	}{
		{"not generate step", `[{"type":"ai.summarize","input":"x","memory":{"enabled":true}}]`, "only supported by the AI generate step"},
		{"invalid memory", `[{"type":"ai.generate","input":"x","memory":true}]`, "must be a JSON object"},
		{"too many messages", `[{"type":"ai.generate","input":"x","memory":{"enabled":true,"maxMessages":1000}}]`, "maxMessages must be between"},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			automation := core.NewAutomation(app)
			populateValidAutomation(automation)
			automation.SetSteps(mustParseJSONRaw(t, s.steps))

			err := app.Validate(automation)
			if err == nil || !strings.Contains(err.Error(), s.expected) {
				t.Fatalf("Expected validation error containing %q, got %v", s.expected, err)
			}
		})
	}

	// disabled memory on other AI steps is allowed (e.g. toggled off)
	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetSteps(mustParseJSONRaw(t, `[{"type":"ai.summarize","input":"x","memory":{"enabled":false}}]`))
	if err := app.Validate(automation); err != nil {
		t.Fatalf("Expected valid step, got %v", err)
	}
}
