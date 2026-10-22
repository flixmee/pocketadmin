package apis_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// regular "users" auth record token (4q1xlclmfloku33)
const testZaloUserAuthHeader = "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwiZXhwIjoyNTI0NjA0NDYxLCJyZWZyZXNoYWJsZSI6dHJ1ZX0.ZT3F0Z3iM-xbGgSG3LEKiEzHrPHr8t8IuHLZGGNuxLo"

const testZaloSecret = "test-bridge-secret-123"

type fakeZaloBridge struct {
	mu    sync.Mutex
	calls []string
}

func (f *fakeZaloBridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Header.Get("X-Bridge-Secret") != testZaloSecret {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"unauthorized"}`)
		return
	}

	f.mu.Lock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	f.mu.Unlock()

	switch r.Method + " " + r.URL.Path {
	case "GET /status":
		fmt.Fprint(w, `{"status":"logged_in","message":"active","accountId":"123","accountName":"Shop","connected":true}`)
	case "POST /login":
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, `{"ok":true,"message":"creating QR"}`)
	case "POST /logout":
		fmt.Fprint(w, `{"ok":true}`)
	case "POST /kick":
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, `{"ok":true}`)
	default:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"not found"}`)
	}
}

func (f *fakeZaloBridge) calledWith(call string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if c == call {
			return true
		}
	}
	return false
}

func enableZalo(t testing.TB, app core.App, bridgeURL string, secret string) {
	app.Settings().Credentials.Zalo.Enabled = true
	app.Settings().Credentials.Zalo.BridgeURL = bridgeURL
	app.Settings().Credentials.Zalo.Secret = secret
	if err := core.EnsureZaloCollections(app); err != nil {
		t.Fatal(err)
	}
}

func TestZaloApi(t *testing.T) {
	t.Parallel()

	bridge := &fakeZaloBridge{}
	server := httptest.NewServer(bridge)
	defer server.Close()

	superuser := map[string]string{"Authorization": testSuperuserAuthHeader}
	user := map[string]string{"Authorization": testZaloUserAuthHeader}

	scenarios := []tests.ApiScenario{
		{
			Name:            "status unauthorized",
			Method:          http.MethodGet,
			URL:             "/api/zalo/status",
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "status when disabled",
			Method:          http.MethodGet,
			URL:             "/api/zalo/status",
			Headers:         user,
			ExpectedStatus:  503,
			ExpectedContent: []string{`"status":"disabled"`, `"connected":false`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "status proxied for regular users",
			Method:  http.MethodGet,
			URL:     "/api/zalo/status",
			Headers: user,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				enableZalo(t, app, server.URL, testZaloSecret)
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"status":"logged_in"`,
				`"accountName":"Shop"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:    "status with offline bridge",
			Method:  http.MethodGet,
			URL:     "/api/zalo/status",
			Headers: user,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				enableZalo(t, app, "http://127.0.0.1:1", testZaloSecret)
			},
			ExpectedStatus:  503,
			ExpectedContent: []string{`"status":"bridge_offline"`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "login requires superuser",
			Method:  http.MethodPost,
			URL:     "/api/zalo/login",
			Headers: user,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				enableZalo(t, app, server.URL, testZaloSecret)
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "login proxied",
			Method:  http.MethodPost,
			URL:     "/api/zalo/login",
			Headers: superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				enableZalo(t, app, server.URL, testZaloSecret)
			},
			ExpectedStatus:  202,
			ExpectedContent: []string{`"message":"creating QR"`},
			ExpectedEvents:  map[string]int{"*": 0},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				if !bridge.calledWith("POST /login") {
					t.Fatal("Expected the bridge /login to be called")
				}
			},
		},
		{
			Name:    "logout proxied",
			Method:  http.MethodPost,
			URL:     "/api/zalo/logout",
			Headers: superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				enableZalo(t, app, server.URL, testZaloSecret)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"ok":true`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "unknown action",
			Method:  http.MethodPost,
			URL:     "/api/zalo/restart",
			Headers: superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				enableZalo(t, app, server.URL, testZaloSecret)
			},
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "overview with online bridge and session",
			Method:  http.MethodGet,
			URL:     "/api/zalo/overview",
			Headers: superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				enableZalo(t, app, server.URL, testZaloSecret)

				sessions, _ := app.FindCollectionByNameOrId(core.ZaloCollectionSession)
				record := core.NewRecord(sessions)
				record.Id = core.ZaloSessionRecordId
				record.Set("status", "qr_pending")
				record.Set("qr_image", "data:image/png;base64,AAA")
				record.Set("credentials", map[string]any{"cookie": "secret-cookie"})
				if err := app.Save(record); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"enabled":true`,
				`"hasSecret":true`,
				`"online":true`,
				`"zalo_outbox":true`,
				`"status":"qr_pending"`,
				`"qr_image":"data:image/png;base64,AAA"`,
			},
			NotExpectedContent: []string{testZaloSecret, `secret-cookie`},
			ExpectedEvents:     map[string]int{"*": 0},
		},
		{
			Name:    "overview with wrong secret",
			Method:  http.MethodGet,
			URL:     "/api/zalo/overview",
			Headers: superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				enableZalo(t, app, server.URL, "wrong-secret-0000000")
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"online":false`, `BRIDGE_SECRET`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "overview with stopped bridge",
			Method:  http.MethodGet,
			URL:     "/api/zalo/overview",
			Headers: superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				enableZalo(t, app, "http://127.0.0.1:1", testZaloSecret)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"online":false`, `not running`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "overview when disabled",
			Method:          http.MethodGet,
			URL:             "/api/zalo/overview",
			Headers:         superuser,
			ExpectedStatus:  200,
			ExpectedContent: []string{`"enabled":false`, `"bridgeURL":"http://127.0.0.1:3939"`, `"zalo_session":false`, `"session":null`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "enable without secret",
			Method:          http.MethodPost,
			URL:             "/api/zalo/settings",
			Headers:         superuser,
			Body:            strings.NewReader(`{"enabled":true}`),
			ExpectedStatus:  400,
			ExpectedContent: []string{`"secret":{`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "enable with a short secret",
			Method:          http.MethodPost,
			URL:             "/api/zalo/settings",
			Headers:         superuser,
			Body:            strings.NewReader(`{"enabled":true,"secret":"short"}`),
			ExpectedStatus:  400,
			ExpectedContent: []string{`"secret":{`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "enable creates the collections",
			Method:          http.MethodPost,
			URL:             "/api/zalo/settings",
			Headers:         superuser,
			Body:            strings.NewReader(`{"enabled":true,"bridgeURL":"` + server.URL + `","secret":"` + testZaloSecret + `"}`),
			ExpectedStatus:  200,
			ExpectedContent: []string{`"enabled":true`, `"online":true`, `"zalo_session":true`, `"zalo_messages":true`, `"zalo_outbox":true`},
			ExpectedEvents: map[string]int{
				"*":                              0,
				"OnSettingsReload":               1,
				"OnCollectionValidate":           3,
				"OnCollectionCreate":             3,
				"OnCollectionCreateExecute":      3,
				"OnCollectionAfterCreateSuccess": 3,
				"OnModelValidate":                4,
				"OnModelCreate":                  3,
				"OnModelCreateExecute":           3,
				"OnModelAfterCreateSuccess":      3,
				"OnModelUpdate":                  1,
				"OnModelUpdateExecute":           1,
				"OnModelAfterUpdateSuccess":      1,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				if app.Settings().Credentials.Zalo.Secret != testZaloSecret {
					t.Fatal("Expected the secret to be saved")
				}
			},
		},
		{
			Name:    "update keeps the secret when omitted",
			Method:  http.MethodPost,
			URL:     "/api/zalo/settings",
			Headers: superuser,
			Body:    strings.NewReader(`{"enabled":false,"secret":""}`),
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				enableZalo(t, app, server.URL, testZaloSecret)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"enabled":false`, `"hasSecret":true`},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				if app.Settings().Credentials.Zalo.Secret != testZaloSecret {
					t.Fatal("Expected the secret to be kept")
				}
			},
		},
		{
			Name:    "settings API doesn't expose the secret",
			Method:  http.MethodGet,
			URL:     "/api/settings",
			Headers: superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				enableZalo(t, app, server.URL, testZaloSecret)
			},
			ExpectedStatus:     200,
			ExpectedContent:    []string{`"zalo":{"enabled":true,"bridgeURL":"` + server.URL + `"}`},
			NotExpectedContent: []string{testZaloSecret},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestZaloOutbox(t *testing.T) {
	t.Parallel()

	bridge := &fakeZaloBridge{}
	server := httptest.NewServer(bridge)
	defer server.Close()

	user := map[string]string{"Authorization": testZaloUserAuthHeader}

	scenarios := []tests.ApiScenario{
		{
			Name:    "outbox requires a thread or phone",
			Method:  http.MethodPost,
			URL:     "/api/collections/zalo_outbox/records",
			Headers: user,
			Body:    strings.NewReader(`{"text":"hello"}`),
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				enableZalo(t, app, server.URL, testZaloSecret)
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`thread_id or phone`},
		},
		{
			Name:    "outbox requires text or attachments",
			Method:  http.MethodPost,
			URL:     "/api/collections/zalo_outbox/records",
			Headers: user,
			Body:    strings.NewReader(`{"thread_id":"123"}`),
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				enableZalo(t, app, server.URL, testZaloSecret)
			},
			ExpectedStatus:  400,
			ExpectedContent: []string{`text or attachments`},
		},
		{
			Name:    "outbox create normalizes the record and kicks the bridge",
			Method:  http.MethodPost,
			URL:     "/api/collections/zalo_outbox/records",
			Headers: user,
			Body:    strings.NewReader(`{"thread_id":"123","text":"hello","status":"sent","zalo_msg_id":"forged"}`),
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				enableZalo(t, app, server.URL, testZaloSecret)
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"status":"pending"`,
				`"thread_type":"user"`,
				`"zalo_msg_id":""`,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				// the kick is sent in the background
				for i := 0; i < 50 && !bridge.calledWith("POST /kick"); i++ {
					time.Sleep(20 * time.Millisecond)
				}
				if !bridge.calledWith("POST /kick") {
					t.Fatal("Expected the bridge /kick to be called")
				}
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestEnsureZaloCollectionsKeepsExisting(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	// an existing (e.g. JS migration created) collection with a custom rule is kept as is
	existing := core.NewBaseCollection(core.ZaloCollectionSession)
	existing.ListRule = nil
	if err := app.Save(existing); err != nil {
		t.Fatal(err)
	}

	if err := core.EnsureZaloCollections(app); err != nil {
		t.Fatal(err)
	}
	// idempotent
	if err := core.EnsureZaloCollections(app); err != nil {
		t.Fatal(err)
	}

	session, _ := app.FindCollectionByNameOrId(core.ZaloCollectionSession)
	if session.ListRule != nil || session.Fields.GetByName("qr_image") != nil {
		t.Fatal("Expected the existing session collection to be left untouched")
	}

	outbox, err := app.FindCollectionByNameOrId(core.ZaloCollectionOutbox)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(outbox)
	messages, _ := app.FindCollectionByNameOrId(core.ZaloCollectionMessages)
	if !strings.Contains(string(raw), `"collectionId":"`+messages.Id+`"`) || !strings.Contains(string(raw), `"protected":true`) {
		t.Fatalf("Unexpected outbox schema %s", raw)
	}
}
