package apis_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/types"
)

const testMCPToken = "pbmcp_testtoken0000000000000000000000000000"

func createMCPKeyFixture(t testing.TB, app core.App, token string, modify func(key *core.MCPKey)) *core.MCPKey {
	key, _ := core.NewMCPKey()
	key.Name = "test"
	key.KeyHash = core.HashMCPKeyToken(token)
	key.KeyPrefix = token[:12]
	key.Permissions = core.MCPPermissions

	if modify != nil {
		modify(key)
	}

	if err := core.SaveMCPKey(app, key); err != nil {
		t.Fatal(err)
	}

	return key
}

func mcpCallBody(tool string, args map[string]any) io.Reader {
	raw, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": tool, "arguments": args},
	})

	return strings.NewReader(string(raw))
}

func TestMCPKeysManagement(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "list unauthorized",
			Method:          http.MethodGet,
			URL:             "/api/mcp/keys",
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "list as superuser",
			Method:  http.MethodGet,
			URL:     "/api/mcp/keys",
			Headers: map[string]string{"Authorization": testSuperuserAuthHeader},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"name":"test"`,
				`"keyPrefix":"pbmcp_testto"`,
				`"permissions":["collections.read","collections.write","records.read","records.write"]`,
			},
			NotExpectedContent: []string{`keyHash`},
			ExpectedEvents:     map[string]int{"*": 0},
		},
		{
			Name:           "create with invalid data",
			Method:         http.MethodPost,
			URL:            "/api/mcp/keys",
			Headers:        map[string]string{"Authorization": testSuperuserAuthHeader},
			Body:           strings.NewReader(`{"name":"","permissions":["unknown"],"collections":["_pb_users_auth_x"]}`),
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"collections":{`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:           "create with system collection",
			Method:         http.MethodPost,
			URL:            "/api/mcp/keys",
			Headers:        map[string]string{"Authorization": testSuperuserAuthHeader},
			Body:           strings.NewReader(`{"name":"test","collections":["` + core.CollectionNameSuperusers + `"]}`),
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"collections":{`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:           "create valid",
			Method:         http.MethodPost,
			URL:            "/api/mcp/keys",
			Headers:        map[string]string{"Authorization": testSuperuserAuthHeader},
			Body:           strings.NewReader(`{"name":" my key ","permissions":["records.read","records.read"],"collections":["sz5l5z67tg7gku0"],"expiresAt":"2099-01-01 00:00:00.000Z"}`),
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"token":"pbmcp_`,
				`"name":"my key"`,
				`"permissions":["records.read"]`,
				`"collections":["sz5l5z67tg7gku0"]`,
				`"active":true`,
				`"expiresAt":"2099-01-01 00:00:00.000Z"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				keys, err := core.FindAllMCPKeys(app)
				if err != nil || len(keys) != 1 {
					t.Fatalf("Expected 1 stored key, got %d (%v)", len(keys), err)
				}
			},
		},
		{
			Name:    "update",
			Method:  http.MethodPatch,
			URL:     "/api/mcp/keys/mcpkeytest00001",
			Headers: map[string]string{"Authorization": testSuperuserAuthHeader},
			Body:    strings.NewReader(`{"name":"renamed","active":false,"expiresAt":""}`),
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, func(key *core.MCPKey) {
					key.Id = "mcpkeytest00001"
					key.ExpiresAt = types.NowDateTime().Add(time.Hour)
				})
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"name":"renamed"`,
				`"active":false`,
				`"expiresAt":""`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:    "regenerate",
			Method:  http.MethodPost,
			URL:     "/api/mcp/keys/mcpkeytest00001/regenerate",
			Headers: map[string]string{"Authorization": testSuperuserAuthHeader},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, func(key *core.MCPKey) {
					key.Id = "mcpkeytest00001"
				})
			},
			ExpectedStatus:     200,
			ExpectedContent:    []string{`"token":"pbmcp_`},
			NotExpectedContent: []string{`"keyPrefix":"pbmcp_testto"`},
			ExpectedEvents:     map[string]int{"*": 0},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				if _, err := core.FindActiveMCPKeyByToken(app, testMCPToken); err == nil {
					t.Fatal("Expected the old token to be invalidated")
				}
			},
		},
		{
			Name:    "delete",
			Method:  http.MethodDelete,
			URL:     "/api/mcp/keys/mcpkeytest00001",
			Headers: map[string]string{"Authorization": testSuperuserAuthHeader},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, func(key *core.MCPKey) {
					key.Id = "mcpkeytest00001"
				})
			},
			ExpectedStatus: 204,
			ExpectedEvents: map[string]int{"*": 0},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				if _, err := core.FindMCPKeyById(app, "mcpkeytest00001"); err == nil {
					t.Fatal("Expected the key to be deleted")
				}
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestMCPEndpoint(t *testing.T) {
	t.Parallel()

	bearer := map[string]string{"Authorization": "Bearer " + testMCPToken}

	scenarios := []tests.ApiScenario{
		{
			Name:            "missing key",
			Method:          http.MethodPost,
			URL:             "/api/mcp",
			Body:            strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`),
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "superuser token is not accepted",
			Method: http.MethodPost,
			URL:    "/api/mcp",
			Body:   strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`),
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "disabled key",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, func(key *core.MCPKey) { key.Active = false })
			},
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "expired key",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}`),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, func(key *core.MCPKey) {
					key.ExpiresAt = types.NowDateTime().Add(-time.Hour)
				})
			},
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "initialize (X-API-Key header)",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    strings.NewReader(`{"jsonrpc":"2.0","id":"a1","method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`),
			Headers: map[string]string{"X-API-Key": testMCPToken},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"a1"`,
				`"protocolVersion":"2025-06-18"`,
				`"tools":{"listChanged":false}`,
				`"name":"pocketadmin"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				key, err := core.FindActiveMCPKeyByToken(app, testMCPToken)
				if err != nil || key.LastUsedAt.IsZero() {
					t.Fatalf("Expected lastUsedAt to be set, got %v (%v)", key, err)
				}
			},
		},
		{
			Name:    "notification",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus: 202,
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:    "unknown method",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"resources/list"}`),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"id":2`, `"code":-32601`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "batch",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    strings.NewReader(`[{"jsonrpc":"2.0","id":1,"method":"ping"},{"jsonrpc":"2.0","method":"notifications/initialized"},{"jsonrpc":"2.0","id":2,"method":"ping"}]`),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`[{"jsonrpc":"2.0","id":1,"result":{}},{"jsonrpc":"2.0","id":2,"result":{}}]`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "tools/list filtered by permissions",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, func(key *core.MCPKey) {
					key.Permissions = []string{core.MCPPermissionRecordsRead}
				})
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"name":"list_records"`,
				`"name":"get_record"`,
				`"inputSchema":{`,
			},
			NotExpectedContent: []string{
				`"name":"create_record"`,
				`"name":"list_collections"`,
				`"name":"delete_collection"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:    "calling a tool without permission",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("delete_record", map[string]any{"collection": "demo2", "id": "llvuca81nly1qls"}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, func(key *core.MCPKey) {
					key.Permissions = []string{core.MCPPermissionRecordsRead}
				})
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"code":-32602`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "list_collections hides system collections",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("list_collections", nil),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"isError":false`,
				`\"name\":\"demo2\"`,
				`\"name\":\"users\"`,
			},
			NotExpectedContent: []string{
				`\"name\":\"` + core.CollectionNameSuperusers + `\"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:    "list_records",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("list_records", map[string]any{"collection": "demo2", "filter": "title != 'test2'", "sort": "-title"}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"isError":false`,
				`\"totalItems\":2`,
				`\"id\":\"0yxhwia2amd8gec\"`,
				`\"id\":\"llvuca81nly1qls\"`,
			},
			NotExpectedContent: []string{`achvryl401bhse3`},
			ExpectedEvents:     map[string]int{"*": 0},
		},
		{
			Name:    "list_records on a system collection",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("list_records", map[string]any{"collection": core.CollectionNameSuperusers}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"isError":true`, `not accessible`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "list_records outside the key allowlist",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("list_records", map[string]any{"collection": "demo1"}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, func(key *core.MCPKey) {
					key.Collections = []string{"sz5l5z67tg7gku0"} // demo2
				})
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"isError":true`, `not accessible`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "create_collection with an allowlist",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("create_collection", map[string]any{"data": map[string]any{"name": "mcp_new"}}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, func(key *core.MCPKey) {
					key.Collections = []string{"sz5l5z67tg7gku0"}
				})
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"isError":true`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "create_collection",
			Method: http.MethodPost,
			URL:    "/api/mcp",
			Body: mcpCallBody("create_collection", map[string]any{"data": map[string]any{
				"name":   "mcp_new",
				"type":   "base",
				"system": true,
				"fields": []any{map[string]any{"name": "title", "type": "text", "required": true}},
			}}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"isError":false`, `\"name\":\"mcp_new\"`, `\"system\":false`},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				collection, err := app.FindCollectionByNameOrId("mcp_new")
				if err != nil {
					t.Fatal(err)
				}
				if collection.Fields.GetByName("title") == nil {
					t.Fatal("Expected title field")
				}
			},
		},
		{
			Name:    "create_record validation error",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("create_record", map[string]any{"collection": "demo2", "data": map[string]any{}}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"isError":true`, `Validation failed`, `title`},
		},
		{
			Name:    "create_record",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("create_record", map[string]any{"collection": "demo2", "data": `{"title":"from mcp"}`}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"isError":false`, `\"title\":\"from mcp\"`},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				if _, err := app.FindFirstRecordByData("demo2", "title", "from mcp"); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			Name:    "update_record",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("update_record", map[string]any{"collection": "demo2", "id": "llvuca81nly1qls", "data": map[string]any{"title": "updated", "id": "other"}}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"isError":false`, `\"id\":\"llvuca81nly1qls\"`, `\"title\":\"updated\"`},
		},
		{
			Name:    "delete_record",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("delete_record", map[string]any{"collection": "demo2", "id": "llvuca81nly1qls"}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"isError":false`, `\"deleted\":true`},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				if _, err := app.FindRecordById("demo2", "llvuca81nly1qls"); err == nil {
					t.Fatal("Expected the record to be deleted")
				}
			},
		},
		{
			Name:            "GET is not allowed",
			Method:          http.MethodGet,
			URL:             "/api/mcp",
			ExpectedStatus:  405,
			ExpectedContent: []string{`"status":405`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}
