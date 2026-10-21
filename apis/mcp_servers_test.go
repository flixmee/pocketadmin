package apis_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func createMCPServerFixture(t testing.TB, app core.App, url string) *core.MCPServer {
	server := core.NewMCPServer()
	server.Id = "mcpsrvtest00001"
	server.Name = "Remote"
	server.URL = url
	server.AuthToken = "remote_secret"
	if err := core.SaveMCPServer(app, server); err != nil {
		t.Fatal(err)
	}

	return server
}

func TestMCPServersManagement(t *testing.T) {
	t.Parallel()

	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer remote_secret" || r.Header.Get("X-Tenant") != "acme" {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}

		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &req)

		if len(req.ID) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		var result any = map[string]any{"protocolVersion": "2025-06-18", "serverInfo": map[string]any{"name": "remote"}}
		if req.Method == "tools/list" {
			result = map[string]any{"tools": []map[string]any{{"name": "search", "description": "Searches docs"}}}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer remote.Close()

	superuser := map[string]string{"Authorization": testSuperuserAuthHeader}

	scenarios := []tests.ApiScenario{
		{
			Name:            "list unauthorized",
			Method:          http.MethodGet,
			URL:             "/api/mcp/servers",
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:           "list as superuser",
			Method:         http.MethodGet,
			URL:            "/api/mcp/servers",
			Headers:        superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) { createMCPServerFixture(t, app, remote.URL) },
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"mcpsrvtest00001"`,
				`"name":"Remote"`,
				`"hasAuthToken":true`,
			},
			NotExpectedContent: []string{`remote_secret`, `authToken"`},
			ExpectedEvents:     map[string]int{"*": 0},
		},
		{
			Name:            "create invalid",
			Method:          http.MethodPost,
			URL:             "/api/mcp/servers",
			Headers:         superuser,
			Body:            strings.NewReader(`{"name":"","url":"ftp://example.com","headers":{"bad header":"x"}}`),
			ExpectedStatus:  400,
			ExpectedContent: []string{`"name":{`, `"url":{`, `"headers":{`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:               "create",
			Method:             http.MethodPost,
			URL:                "/api/mcp/servers",
			Headers:            superuser,
			Body:               strings.NewReader(`{"name":" Docs ","url":"https://example.com/mcp","authToken":"abc","headers":{"X-Tenant":"acme"}}`),
			ExpectedStatus:     200,
			ExpectedContent:    []string{`"name":"Docs"`, `"hasAuthToken":true`, `"headers":{"X-Tenant":"acme"}`, `"active":true`},
			NotExpectedContent: []string{`"abc"`},
			ExpectedEvents:     map[string]int{"*": 0},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				servers, err := core.FindAllMCPServers(app)
				if err != nil || len(servers) != 1 || servers[0].AuthToken != "abc" {
					t.Fatalf("Expected 1 stored server with token, got %v (%v)", servers, err)
				}
			},
		},
		{
			Name:            "update keeps the token when omitted",
			Method:          http.MethodPatch,
			URL:             "/api/mcp/servers/mcpsrvtest00001",
			Headers:         superuser,
			Body:            strings.NewReader(`{"name":"Renamed","active":false}`),
			BeforeTestFunc:  func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) { createMCPServerFixture(t, app, remote.URL) },
			ExpectedStatus:  200,
			ExpectedContent: []string{`"name":"Renamed"`, `"active":false`, `"hasAuthToken":true`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "update clears the token when empty",
			Method:          http.MethodPatch,
			URL:             "/api/mcp/servers/mcpsrvtest00001",
			Headers:         superuser,
			Body:            strings.NewReader(`{"authToken":""}`),
			BeforeTestFunc:  func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) { createMCPServerFixture(t, app, remote.URL) },
			ExpectedStatus:  200,
			ExpectedContent: []string{`"hasAuthToken":false`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "tools",
			Method:  http.MethodGet,
			URL:     "/api/mcp/servers/mcpsrvtest00001/tools",
			Headers: superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				server := createMCPServerFixture(t, app, remote.URL)
				server.Headers = map[string]string{"X-Tenant": "acme"}
				if err := core.SaveMCPServer(app, server); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"tools":[{"name":"search","title":"","description":"Searches docs"}]`, `"server":{"name":"remote"}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "tools with connection error",
			Method:          http.MethodGet,
			URL:             "/api/mcp/servers/mcpsrvtest00001/tools",
			Headers:         superuser,
			BeforeTestFunc:  func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) { createMCPServerFixture(t, app, remote.URL) },
			ExpectedStatus:  400,
			ExpectedContent: []string{`401`, `invalid credentials`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:           "delete",
			Method:         http.MethodDelete,
			URL:            "/api/mcp/servers/mcpsrvtest00001",
			Headers:        superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) { createMCPServerFixture(t, app, remote.URL) },
			ExpectedStatus: 204,
			ExpectedEvents: map[string]int{"*": 0},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				if _, err := core.FindMCPServerById(app, "mcpsrvtest00001"); err == nil {
					t.Fatal("Expected the server to be deleted")
				}
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}
