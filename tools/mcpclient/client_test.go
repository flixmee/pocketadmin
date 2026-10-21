package mcpclient_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/pocketbase/pocketbase/tools/mcpclient"
)

type fakeServer struct {
	sse       bool
	sessionID string

	mu       sync.Mutex
	methods  []string
	sessions []string
	deleted  bool
	auth     string
}

func (s *fakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.auth = r.Header.Get("Authorization")

	if r.Method == http.MethodDelete {
		s.deleted = true
		return
	}

	var req struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params map[string]any  `json:"params"`
	}
	raw, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(raw, &req)

	s.methods = append(s.methods, req.Method)
	s.sessions = append(s.sessions, r.Header.Get("Mcp-Session-Id"))

	if len(req.ID) == 0 {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	var result any
	switch req.Method {
	case "initialize":
		if s.sessionID != "" {
			w.Header().Set("Mcp-Session-Id", s.sessionID)
		}
		result = map[string]any{"protocolVersion": mcpclient.ProtocolVersion, "serverInfo": map[string]any{"name": "fake"}}
	case "tools/list":
		if req.Params["cursor"] == "page2" {
			result = map[string]any{"tools": []map[string]any{{"name": "echo", "inputSchema": map[string]any{"type": "object"}}}}
		} else {
			result = map[string]any{"tools": []map[string]any{{"name": "add", "description": "Adds numbers"}}, "nextCursor": "page2"}
		}
	case "tools/call":
		args, _ := req.Params["arguments"].(map[string]any)
		switch req.Params["name"] {
		case "echo":
			result = map[string]any{"content": []map[string]any{{"type": "text", "text": fmt.Sprint(args["text"])}}}
		case "fail":
			result = map[string]any{"content": []map[string]any{{"type": "text", "text": "boom"}}, "isError": true}
		default:
			writeJSON(w, s.sse, map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32602, "message": "Unknown tool"}})
			return
		}
	}

	writeJSON(w, s.sse, map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
}

func writeJSON(w http.ResponseWriter, sse bool, payload any) {
	raw, _ := json.Marshal(payload)

	if !sse {
		w.Header().Set("Content-Type", "application/json")
		w.Write(raw)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	// an unrelated server notification before the actual response
	fmt.Fprint(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/message\",\"params\":{}}\n\n")
	fmt.Fprintf(w, "id: 1\nevent: message\ndata: %s\n\n", raw)
}

func TestClient(t *testing.T) {
	for _, sse := range []bool{false, true} {
		t.Run(fmt.Sprintf("sse_%v", sse), func(t *testing.T) {
			fake := &fakeServer{sse: sse, sessionID: "sess-1"}
			server := httptest.NewServer(fake)
			defer server.Close()

			client := mcpclient.New(server.URL, map[string]string{"Authorization": "Bearer secret"})
			ctx := context.Background()

			tools, err := client.ListTools(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(tools) != 2 || tools[0].Name != "add" || tools[1].Name != "echo" {
				t.Fatalf("Unexpected tools %+v", tools)
			}

			result, err := client.CallTool(ctx, "echo", map[string]any{"text": "hello"})
			if err != nil {
				t.Fatal(err)
			}
			if result.IsError || result.Text() != "hello" {
				t.Fatalf("Unexpected result %+v", result)
			}

			result, err = client.CallTool(ctx, "fail", nil)
			if err != nil {
				t.Fatal(err)
			}
			if !result.IsError || result.Text() != "boom" {
				t.Fatalf("Expected tool error result, got %+v", result)
			}

			_, err = client.CallTool(ctx, "missing", nil)
			if err == nil || !strings.Contains(err.Error(), "Unknown tool") {
				t.Fatalf("Expected RPC error, got %v", err)
			}

			if err := client.Close(ctx); err != nil {
				t.Fatal(err)
			}

			fake.mu.Lock()
			defer fake.mu.Unlock()

			expectedMethods := "initialize,notifications/initialized,tools/list,tools/list,tools/call,tools/call,tools/call"
			if got := strings.Join(fake.methods, ","); got != expectedMethods {
				t.Fatalf("Expected methods %s, got %s", expectedMethods, got)
			}
			if fake.sessions[0] != "" || fake.sessions[1] != "sess-1" || fake.sessions[len(fake.sessions)-1] != "sess-1" {
				t.Fatalf("Unexpected session headers %v", fake.sessions)
			}
			if !fake.deleted {
				t.Fatal("Expected the session to be terminated")
			}
			if fake.auth != "Bearer secret" {
				t.Fatalf("Expected custom headers to be sent, got %q", fake.auth)
			}
		})
	}
}

func TestClientHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "invalid key", http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := mcpclient.New(server.URL, nil).ListTools(context.Background())
	if err == nil || !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "invalid key") {
		t.Fatalf("Expected 401 error, got %v", err)
	}
}

func TestCallToolResultText(t *testing.T) {
	result := &mcpclient.CallToolResult{
		Content: []mcpclient.Content{
			{Type: "text", Text: "a"},
			{Type: "image", MimeType: "image/png", Data: "..."},
			{Type: "resource", Resource: json.RawMessage(`{"uri":"file:///x","text":"b"}`)},
		},
	}

	if got := result.Text(); got != "a\n[image content (image/png) omitted]\nb" {
		t.Fatalf("Unexpected text %q", got)
	}

	structured := &mcpclient.CallToolResult{StructuredContent: map[string]any{"x": 1}}
	if got := structured.Text(); got != `{"x":1}` {
		t.Fatalf("Unexpected structured text %q", got)
	}
}
