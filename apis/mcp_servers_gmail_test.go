package apis_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func createGmailServerFixture(t testing.TB, app core.App) *core.MCPServer {
	server := core.NewMCPServer()
	server.Id = "mcpgmailtest001"
	server.Name = "Inbox"
	server.Kind = core.MCPServerKindGmail
	server.ClientId = "cid"
	server.ClientSecret = "csecret"
	if err := core.SaveMCPServer(app, server); err != nil {
		t.Fatal(err)
	}

	return server
}

// note: not parallel because it overrides the global Google endpoints
func TestMCPServersGmailOAuth(t *testing.T) {
	google := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			_ = r.ParseForm()
			if r.Form.Get("code") != "good_code" {
				w.WriteHeader(400)
				fmt.Fprint(w, `{"error":"invalid_grant"}`)
				return
			}
			fmt.Fprint(w, `{"access_token":"at","refresh_token":"rt","expires_in":3600}`)
		case "/userinfo":
			fmt.Fprint(w, `{"email":"inbox@example.com"}`)
		}
	}))
	defer google.Close()

	original := core.GoogleEndpoints
	core.GoogleEndpoints.AuthURL = google.URL + "/auth"
	core.GoogleEndpoints.TokenURL = google.URL + "/token"
	core.GoogleEndpoints.UserInfoURL = google.URL + "/userinfo"
	defer func() { core.GoogleEndpoints = original }()

	superuser := map[string]string{"Authorization": testSuperuserAuthHeader}

	scenarios := []tests.ApiScenario{
		{
			Name:           "create gmail connection without url",
			Method:         http.MethodPost,
			URL:            "/api/mcp/servers",
			Headers:        superuser,
			Body:           strings.NewReader(`{"name":"Inbox","kind":"gmail","clientId":"cid","clientSecret":"csecret"}`),
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"kind":"gmail"`,
				`"clientId":"cid"`,
				`"hasClientSecret":true`,
				`"connected":false`,
			},
			NotExpectedContent: []string{`csecret`},
			ExpectedEvents:     map[string]int{"*": 0},
		},
		{
			Name:            "invalid kind",
			Method:          http.MethodPost,
			URL:             "/api/mcp/servers",
			Headers:         superuser,
			Body:            strings.NewReader(`{"name":"X","kind":"ftp"}`),
			ExpectedStatus:  400,
			ExpectedContent: []string{`"kind":{`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:           "oauth start",
			Method:         http.MethodPost,
			URL:            "/api/mcp/servers/mcpgmailtest001/oauth/start",
			Headers:        superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) { createGmailServerFixture(t, app) },
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"authURL":"` + google.URL + `/auth?`,
				`client_id=cid`,
				`access_type=offline`,
				`"redirectURL":"http://localhost:8090/api/mcp/oauth/google/callback"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:            "oauth start unauthorized",
			Method:          http.MethodPost,
			URL:             "/api/mcp/servers/mcpgmailtest001/oauth/start",
			BeforeTestFunc:  func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) { createGmailServerFixture(t, app) },
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "callback with invalid state",
			Method:          http.MethodGet,
			URL:             "/api/mcp/oauth/google/callback?state=nope&code=good_code",
			ExpectedStatus:  400,
			ExpectedContent: []string{`Connection failed`, `invalid or expired OAuth state`, `pocketadmin:mcp-oauth`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "callback success",
			Method: http.MethodGet,
			URL:    "/api/mcp/oauth/google/callback?state=fixedstate&code=good_code",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createGmailServerFixture(t, app)
				// replicate NewMCPServerOAuthState with a known state value
				state := core.NewMCPServerOAuthState(app, "mcpgmailtest001")
				app.Store().Set("pbMCPOAuthState_fixedstate", app.Store().Get("pbMCPOAuthState_"+state))
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`Account connected`, `inbox@example.com`, `"success":true`},
			ExpectedEvents:  map[string]int{"*": 0},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				server, _ := core.FindMCPServerById(app, "mcpgmailtest001")
				if server.OAuth.RefreshToken != "rt" || server.OAuth.Email != "inbox@example.com" {
					t.Fatalf("Expected the tokens to be stored, got %+v", server.OAuth)
				}
			},
		},
		{
			Name:   "callback with denied consent",
			Method: http.MethodGet,
			URL:    "/api/mcp/oauth/google/callback?state=fixedstate&error=access_denied",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createGmailServerFixture(t, app)
				state := core.NewMCPServerOAuthState(app, "mcpgmailtest001")
				app.Store().Set("pbMCPOAuthState_fixedstate", app.Store().Get("pbMCPOAuthState_"+state))
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`access_denied`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "disconnect",
			Method:  http.MethodPost,
			URL:     "/api/mcp/servers/mcpgmailtest001/oauth/disconnect",
			Headers: superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				server := createGmailServerFixture(t, app)
				server.OAuth = core.MCPServerOAuth{RefreshToken: "rt", Email: "inbox@example.com"}
				if err := core.SaveMCPServerOAuth(app, server); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"connected":false`, `"account":""`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}
