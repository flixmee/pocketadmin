package core_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// newFakeMCPToolServer starts a minimal stateless MCP server with
// "get_weather" and "secret" tools.
func newFakeMCPToolServer(t testing.TB, calls *[]string, mu *sync.Mutex) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mcp_token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params map[string]any  `json:"params"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &req)

		if len(req.ID) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2025-06-18", "serverInfo": map[string]any{"name": "weather"}}
		case "tools/list":
			result = map[string]any{"tools": []map[string]any{
				{
					"name":        "get_weather",
					"description": "Returns the weather for a city",
					"inputSchema": map[string]any{
						"$schema":    "http://json-schema.org/draft-07/schema#",
						"type":       "object",
						"properties": map[string]any{"city": map[string]any{"type": "string"}},
						"required":   []string{"city"},
					},
				},
				{"name": "secret", "description": "Must not be exposed"},
			}}
		case "tools/call":
			args, _ := req.Params["arguments"].(map[string]any)
			mu.Lock()
			*calls = append(*calls, fmt.Sprintf("%v(%v)", req.Params["name"], args["city"]))
			mu.Unlock()
			result = map[string]any{"content": []map[string]any{{"type": "text", "text": fmt.Sprintf("Sunny in %v", args["city"])}}}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
}

// fakeAIToolProvider returns a handler that requests a get_weather tool call
// on the first request and returns the final answer once it receives the tool result.
func fakeAIToolProvider(t testing.TB, provider string, bodies *[]map[string]any, mu *sync.Mutex) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)

		mu.Lock()
		*bodies = append(*bodies, body)
		round := len(*bodies)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		raw, _ := json.Marshal(body)
		hasResult := strings.Contains(string(raw), "Sunny in Hanoi")

		switch provider {
		case core.AIProviderAnthropic:
			if round == 1 {
				fmt.Fprint(w, `{"content":[{"type":"text","text":"Checking"},{"type":"tool_use","id":"tu_1","name":"get_weather","input":{"city":"Hanoi"}}],"stop_reason":"tool_use","usage":{"input_tokens":10,"output_tokens":5}}`)
			} else if hasResult {
				fmt.Fprint(w, `{"content":[{"type":"text","text":"It is sunny in Hanoi."}],"stop_reason":"end_turn","usage":{"input_tokens":20,"output_tokens":6}}`)
			} else {
				http.Error(w, "missing tool result", 400)
			}
		case core.AIProviderGemini:
			if round == 1 {
				fmt.Fprint(w, `{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"get_weather","args":{"city":"Hanoi"}}}]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5}}`)
			} else if hasResult {
				fmt.Fprint(w, `{"candidates":[{"content":{"role":"model","parts":[{"text":"It is sunny in Hanoi."}]}}],"usageMetadata":{"promptTokenCount":20,"candidatesTokenCount":6}}`)
			} else {
				http.Error(w, "missing tool result", 400)
			}
		default:
			if round == 1 {
				fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Hanoi\"}"}}]}}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`)
			} else if hasResult {
				fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"It is sunny in Hanoi."}}],"usage":{"prompt_tokens":20,"completion_tokens":6}}`)
			} else {
				http.Error(w, "missing tool result", 400)
			}
		}
	}
}

func createTestMCPServer(t testing.TB, app core.App, url string, active bool) *core.MCPServer {
	server := core.NewMCPServer()
	server.Name = "Weather"
	server.URL = url
	server.AuthToken = "mcp_token"
	server.Active = active
	if err := core.SaveMCPServer(app, server); err != nil {
		t.Fatal(err)
	}
	return server
}

func TestAutomationAIGenerateWithMCPTools(t *testing.T) {
	t.Parallel()

	for _, provider := range []string{core.AIProviderOpenAI, core.AIProviderAnthropic, core.AIProviderGemini} {
		t.Run(provider, func(t *testing.T) {
			app, _ := tests.NewTestApp()
			defer app.Cleanup()

			var mu sync.Mutex
			var toolCalls []string
			var bodies []map[string]any

			mcpServer := newFakeMCPToolServer(t, &toolCalls, &mu)
			defer mcpServer.Close()

			aiServer := httptest.NewServer(fakeAIToolProvider(t, provider, &bodies, &mu))
			defer aiServer.Close()

			app.Settings().AI.Enabled = true
			app.Settings().AI.Provider = provider
			app.Settings().AI.APIKey = "test_key"
			app.Settings().AI.Model = "test-model"
			app.Settings().AI.BaseURL = aiServer.URL

			connection := createTestMCPServer(t, app, mcpServer.URL, true)

			automation := core.NewAutomation(app)
			populateValidAutomation(automation)
			automation.SetActive(true)
			automation.SetSteps(mustParseJSONRaw(t, `[{
				"type":"ai.generate",
				"input":"What is the weather in Hanoi?",
				"systemPrompt":"You are a weather bot.",
				"mcpServers":[{"server":"`+connection.Id+`","tools":["get_weather"]}],
				"maxToolCalls":3
			}]`))
			if err := app.Save(automation); err != nil {
				t.Fatal(err)
			}
			if err := app.RunAutomationManually(automation.Id); err != nil {
				t.Fatal(err)
			}

			runs := waitForCompletedAutomationRuns(t, app, automation, 1)
			if runs[0].Status() != core.AutomationRunStatusSuccess {
				t.Fatalf("Expected success, got %q: %s", runs[0].Status(), runs[0].Error())
			}

			results := decodeStepResults(t, runs[0])
			output := results[0]["output"].(map[string]any)
			generated := output["output"].(map[string]any)
			if generated["text"] != "It is sunny in Hanoi." {
				t.Fatalf("Expected the final answer, got %#v", generated["text"])
			}

			loggedCalls, _ := output["toolCalls"].([]any)
			if len(loggedCalls) != 1 {
				t.Fatalf("Expected 1 logged tool call, got %#v", output["toolCalls"])
			}
			logged := loggedCalls[0].(map[string]any)
			if logged["server"] != "Weather" || logged["tool"] != "get_weather" || logged["output"] != "Sunny in Hanoi" || logged["isError"] != false {
				t.Fatalf("Unexpected logged tool call %#v", logged)
			}

			tokenUsage := output["tokenUsage"].(map[string]any)
			if tokenUsage["total"] != float64(41) {
				t.Fatalf("Expected accumulated token usage 41, got %#v", tokenUsage)
			}

			mu.Lock()
			defer mu.Unlock()

			if strings.Join(toolCalls, ",") != "get_weather(Hanoi)" {
				t.Fatalf("Unexpected MCP tool calls %v", toolCalls)
			}
			if len(bodies) != 2 {
				t.Fatalf("Expected 2 provider requests, got %d", len(bodies))
			}

			// only the selected tool is exposed, without the $schema keyword
			firstRequest, _ := json.Marshal(bodies[0])
			if !strings.Contains(string(firstRequest), "get_weather") || strings.Contains(string(firstRequest), "secret") {
				t.Fatalf("Expected only get_weather to be exposed, got %s", firstRequest)
			}
			if strings.Contains(string(firstRequest), "$schema") {
				t.Fatalf("Expected $schema to be removed, got %s", firstRequest)
			}

			// the system prompt is sent with every request of the tool loop
			for i, body := range bodies {
				if got := requestSystemPrompt(provider, body); got != "You are a weather bot." {
					t.Fatalf("Request %d: expected the system prompt, got %q", i+1, got)
				}
			}
		})
	}
}

func TestAutomationAIGenerateMCPToolErrors(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	var mu sync.Mutex
	var toolCalls []string
	mcpServer := newFakeMCPToolServer(t, &toolCalls, &mu)
	defer mcpServer.Close()

	app.Settings().AI.Enabled = true
	app.Settings().AI.Provider = core.AIProviderOpenAI
	app.Settings().AI.APIKey = "test_key"
	app.Settings().AI.Model = "test-model"
	app.Settings().AI.BaseURL = "http://127.0.0.1:1"

	disabled := createTestMCPServer(t, app, mcpServer.URL, false)
	enabled := createTestMCPServer(t, app, mcpServer.URL, true)

	scenarios := []struct {
		name     string
		steps    string
		expected string
	}{
		{"disabled connection", `[{"type":"ai.generate","input":"x","mcpServers":[{"server":"` + disabled.Id + `"}]}]`, "is disabled"},
		{"missing connection", `[{"type":"ai.generate","input":"x","mcpServers":[{"server":"missing"}]}]`, "doesn't exist"},
		{"missing tool", `[{"type":"ai.generate","input":"x","mcpServers":[{"server":"` + enabled.Id + `","tools":["nope"]}]}]`, `has no tool "nope"`},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			automation := core.NewAutomation(app)
			populateValidAutomation(automation)
			automation.SetActive(true)
			automation.SetSteps(mustParseJSONRaw(t, s.steps))
			if err := app.Save(automation); err != nil {
				t.Fatal(err)
			}
			err := app.RunAutomationManually(automation.Id)
			if err == nil || !strings.Contains(err.Error(), s.expected) {
				t.Fatalf("Expected failure containing %q, got %v", s.expected, err)
			}

			runs := waitForCompletedAutomationRuns(t, app, automation, 1)
			if runs[0].Status() == core.AutomationRunStatusSuccess {
				t.Fatalf("Expected failed run, got %q", runs[0].Status())
			}
		})
	}
}

func TestAutomationAIMCPToolsValidation(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	scenarios := []struct {
		name     string
		steps    string
		expected string
	}{
		{"not generate step", `[{"type":"ai.summarize","input":"x","mcpServers":[{"server":"abc"}]}]`, "only supported by the AI generate step"},
		{"invalid servers", `[{"type":"ai.generate","input":"x","mcpServers":{"server":"abc"}}]`, "must be a JSON array"},
		{"missing server", `[{"type":"ai.generate","input":"x","mcpServers":[{"tools":["a"]}]}]`, "requires a server"},
		{"too many tool calls", `[{"type":"ai.generate","input":"x","mcpServers":[{"server":"abc"}],"maxToolCalls":500}]`, "maxToolCalls must be between"},
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

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetSteps(mustParseJSONRaw(t, `[{"type":"ai.generate","input":"x","mcpServers":[{"server":"abc","tools":["a","b"]}],"maxToolCalls":5}]`))
	if err := app.Validate(automation); err != nil {
		t.Fatalf("Expected valid step, got %v", err)
	}
}
