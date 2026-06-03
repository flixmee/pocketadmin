package apis_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	testSuperuserAuthHeader = "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY"
	testRegularAuthHeader   = "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjRxMXhsY2xtZmxva3UzMyIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoiX3BiX3VzZXJzX2F1dGhfIiwiZXhwIjoyNTI0NjA0NDYxLCJyZWZyZXNoYWJsZSI6dHJ1ZX0.ZT3F0Z3iM-xbGgSG3LEKiEzHrPHr8t8IuHLZGGNuxLo"
)

func TestAutomationsList(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "unauthorized",
			Method:          http.MethodGet,
			URL:             "/api/automations",
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as regular user",
			Method: http.MethodGet,
			URL:    "/api/automations",
			Headers: map[string]string{
				"Authorization": testRegularAuthHeader,
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser",
			Method: http.MethodGet,
			URL:    "/api/automations",
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createAutomationFixture(t, app, "autoapi00000001", "API list automation")
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"name":"API list automation"`,
				`"triggerType":"manual"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestAutomationSchemas(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "unauthorized",
			Method:          http.MethodGet,
			URL:             "/api/automations/schemas",
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as regular user",
			Method: http.MethodGet,
			URL:    "/api/automations/schemas",
			Headers: map[string]string{
				"Authorization": testRegularAuthHeader,
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser",
			Method: http.MethodGet,
			URL:    "/api/automations/schemas",
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"triggers":{`,
				`"record.create"`,
				`"telegram.message"`,
				`"i18n.translation_missing"`,
				`"steps":{`,
				`"http"`,
				`"mail.send"`,
				`"capability"`,
				`"capabilities":{`,
				`"http.request"`,
				`"limits":{`,
				`"maxSteps":100`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestAutomationCreate(t *testing.T) {
	t.Parallel()

	validBody := `{
		"name":"API create automation",
		"tag":"ops",
		"active":true,
		"triggerType":"manual",
		"steps":[{"type":"condition","path":"trigger.type","op":"exists"}]
	}`

	scenarios := []tests.ApiScenario{
		{
			Name:            "unauthorized",
			Method:          http.MethodPost,
			URL:             "/api/automations",
			Body:            strings.NewReader(validBody),
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as regular user",
			Method: http.MethodPost,
			URL:    "/api/automations",
			Body:   strings.NewReader(validBody),
			Headers: map[string]string{
				"Authorization": testRegularAuthHeader,
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as superuser invalid body",
			Method: http.MethodPost,
			URL:    "/api/automations",
			Body:   strings.NewReader(`{"active":true}`),
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"data":{`,
				`"name":{"code":"validation_required"`,
			},
		},
		{
			Name:   "authorized as superuser valid body",
			Method: http.MethodPost,
			URL:    "/api/automations",
			Body:   strings.NewReader(validBody),
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				record, err := app.FindFirstRecordByData(core.CollectionNameAutomations, "name", "API create automation")
				if err != nil {
					t.Fatalf("Expected created automation to be persisted: %v", err)
				}
				if record.GetString("triggerType") != core.AutomationTriggerManual {
					t.Fatalf("Expected created automation trigger type %q, got %q", core.AutomationTriggerManual, record.GetString("triggerType"))
				}
				if record.GetString("tag") != "ops" {
					t.Fatalf("Expected created automation tag %q, got %q", "ops", record.GetString("tag"))
				}
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"name":"API create automation"`,
				`"tag":"ops"`,
				`"triggerType":"manual"`,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestAutomationDryRun(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "unauthorized",
			Method:          http.MethodPost,
			URL:             "/api/automations/autoapi00000061/dry-run",
			Body:            strings.NewReader(`{}`),
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser",
			Method: http.MethodPost,
			URL:    "/api/automations/autoapi00000061/dry-run",
			Body:   strings.NewReader(`{"triggerType":"manual"}`),
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createAutomationFixture(t, app, "autoapi00000061", "API dry-run automation")
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"automationId":"autoapi00000061"`,
				`"triggerType":"manual"`,
				`"status":"success"`,
				`"stepResults":[`,
				`"type":"condition"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestAutomationPublish(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "unauthorized",
			Method:          http.MethodPost,
			URL:             "/api/automations/autoapi00000062/publish",
			Body:            strings.NewReader(`{"notes":"v1"}`),
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as regular user",
			Method: http.MethodPost,
			URL:    "/api/automations/autoapi00000062/publish",
			Body:   strings.NewReader(`{"notes":"v1"}`),
			Headers: map[string]string{
				"Authorization": testRegularAuthHeader,
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser",
			Method: http.MethodPost,
			URL:    "/api/automations/autoapi00000062/publish",
			Body:   strings.NewReader(`{"notes":"v1"}`),
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createAutomationFixture(t, app, "autoapi00000062", "API publish automation")
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"automationRef":"autoapi00000062"`,
				`"version":1`,
				`"status":"published"`,
				`"notes":"v1"`,
			},
			ExpectedEvents: map[string]int{
				"OnRecordValidate":           1,
				"OnRecordCreate":             1,
				"OnRecordCreateExecute":      1,
				"OnRecordAfterCreateSuccess": 1,
				"OnModelValidate":            1,
				"OnModelCreate":              1,
				"OnModelCreateExecute":       1,
				"OnModelAfterCreateSuccess":  1,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestAutomationViewUpdateDelete(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:   "view authorized as superuser",
			Method: http.MethodGet,
			URL:    "/api/automations/autoapi00000002",
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createAutomationFixture(t, app, "autoapi00000002", "API view automation")
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"autoapi00000002"`,
				`"name":"API view automation"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:   "update authorized as superuser",
			Method: http.MethodPatch,
			URL:    "/api/automations/autoapi00000003",
			Body:   strings.NewReader(`{"active":false,"tag":"ops","notes":"updated through api"}`),
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createAutomationFixture(t, app, "autoapi00000003", "API update automation")
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				record, err := app.FindRecordById(core.CollectionNameAutomations, "autoapi00000003")
				if err != nil {
					t.Fatalf("Expected updated automation to exist: %v", err)
				}
				if record.GetBool("active") {
					t.Fatal("Expected updated automation to be inactive")
				}
				if record.GetString("notes") != "updated through api" {
					t.Fatalf("Expected updated notes, got %q", record.GetString("notes"))
				}
				if record.GetString("tag") != "ops" {
					t.Fatalf("Expected updated tag, got %q", record.GetString("tag"))
				}
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"autoapi00000003"`,
				`"active":false`,
				`"tag":"ops"`,
				`"notes":"updated through api"`,
			},
		},
		{
			Name:   "delete authorized as superuser",
			Method: http.MethodDelete,
			URL:    "/api/automations/autoapi00000004",
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createAutomationFixture(t, app, "autoapi00000004", "API delete automation")
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				if _, err := app.FindRecordById(core.CollectionNameAutomations, "autoapi00000004"); err == nil {
					t.Fatal("Expected automation to be deleted")
				}
			},
			ExpectedStatus: 204,
		},
		{
			Name:   "missing automation",
			Method: http.MethodGet,
			URL:    "/api/automations/missing",
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestAutomationRun(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "unauthorized",
			Method:          http.MethodPost,
			URL:             "/api/automations/autoapi00000005/run",
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as superuser missing automation",
			Method: http.MethodPost,
			URL:    "/api/automations/missing/run",
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as superuser existing automation",
			Method: http.MethodPost,
			URL:    "/api/automations/autoapi00000005/run",
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			Delay: 100 * time.Millisecond,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createAutomationFixture(t, app, "autoapi00000005", "API run automation")
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				automationRecord, err := app.FindRecordById(core.CollectionNameAutomations, "autoapi00000005")
				if err != nil {
					t.Fatalf("Expected automation to exist: %v", err)
				}

				automation := &core.Automation{}
				automation.SetProxyRecord(automationRecord)

				waitForAutomationRunsAPI(t, app, automation, 1)
			},
			ExpectedStatus: 204,
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestAutomationRunRerun(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:           "unauthorized",
			Method:         http.MethodPost,
			URL:            "/api/automations/autoapi00000053/runs/runapi000000053/rerun",
			ExpectedStatus: 401,
			ExpectedContent: []string{
				`"data":{}`,
			},
		},
		{
			Name:   "authorized as superuser missing run",
			Method: http.MethodPost,
			URL:    "/api/automations/autoapi00000053/runs/missing/rerun",
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createWebhookAutomationFixture(t, app, "autoapi00000053", "API rerun automation")
			},
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as superuser existing run",
			Method: http.MethodPost,
			URL:    "/api/automations/autoapi00000054/runs/runapi000000054/rerun",
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			Delay: 100 * time.Millisecond,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				automation := createWebhookAutomationFixture(t, app, "autoapi00000054", "API rerun automation")

				run := core.NewAutomationRun(app)
				run.SetRaw("id", "runapi000000054")
				run.SetAutomationRef(automation.Id)
				run.SetTriggerType(core.AutomationTriggerWebhook)
				run.SetStatus(core.AutomationRunStatusSuccess)
				run.SetInput(mustAutomationJSONRaw(t, `{
					"triggerType":"webhook",
					"request":{
						"method":"POST",
						"path":"/api/automation-webhooks/autoapi00000054",
						"query":{"tenant":"acme"},
						"headers":{"x_automation_event":"invoice.paid"},
						"body":{"event":"invoice.paid"},
						"remoteIP":"127.0.0.1"
					}
				}`))

				if err := app.Save(run); err != nil {
					t.Fatalf("Failed to create automation run fixture: %v", err)
				}
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				automationRecord, err := app.FindRecordById(core.CollectionNameAutomations, "autoapi00000054")
				if err != nil {
					t.Fatalf("Expected automation to exist: %v", err)
				}

				automation := &core.Automation{}
				automation.SetProxyRecord(automationRecord)

				runs := waitForAutomationRunsAPI(t, app, automation, 2)
				latest := runs[0]
				if latest.Id == "runapi000000054" {
					t.Fatalf("Expected rerun to create a new run id, got %q", latest.Id)
				}
				if latest.TriggerType() != core.AutomationTriggerWebhook {
					t.Fatalf("Expected rerun trigger type %q, got %q", core.AutomationTriggerWebhook, latest.TriggerType())
				}

				input := decodeAutomationRunInputAPI(t, latest)
				request, ok := input["request"].(map[string]any)
				if !ok {
					t.Fatalf("Expected request payload in rerun input, got %#v", input["request"])
				}
				if request["path"] != "/api/automation-webhooks/autoapi00000054" {
					t.Fatalf("Expected rerun request path, got %#v", request["path"])
				}
			},
			ExpectedStatus: 204,
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestAutomationWebhook(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "missing automation",
			Method:          http.MethodPost,
			URL:             "/api/automation-webhooks/missing",
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "non-webhook automation",
			Method: http.MethodPost,
			URL:    "/api/automation-webhooks/autoapi00000050",
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createAutomationFixture(t, app, "autoapi00000050", "API non-webhook automation")
			},
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "invalid webhook json",
			Method: http.MethodPost,
			URL:    "/api/automation-webhooks/autoapi00000051",
			Body:   strings.NewReader(`{"event":`),
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createWebhookAutomationFixture(t, app, "autoapi00000051", "API invalid webhook automation")
			},
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"message":"Failed to load webhook request."`,
			},
		},
		{
			Name:   "valid webhook automation",
			Method: http.MethodPost,
			URL:    "/api/automation-webhooks/autoapi00000052?tenant=acme",
			Body:   strings.NewReader(`{"event":"invoice.paid"}`),
			Headers: map[string]string{
				"Content-Type":       "application/json",
				"X-Automation-Event": "invoice.paid",
			},
			Delay: 100 * time.Millisecond,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createWebhookAutomationFixture(t, app, "autoapi00000052", "API webhook automation")
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				automationRecord, err := app.FindRecordById(core.CollectionNameAutomations, "autoapi00000052")
				if err != nil {
					t.Fatalf("Expected webhook automation to exist: %v", err)
				}

				automation := &core.Automation{}
				automation.SetProxyRecord(automationRecord)

				runs := waitForAutomationRunsAPI(t, app, automation, 1)
				run := runs[0]
				if run.TriggerType() != core.AutomationTriggerWebhook {
					t.Fatalf("Expected webhook trigger type %q, got %q", core.AutomationTriggerWebhook, run.TriggerType())
				}

				input := decodeAutomationRunInputAPI(t, run)
				request, ok := input["request"].(map[string]any)
				if !ok {
					t.Fatalf("Expected request payload in automation run input, got %#v", input["request"])
				}
				if request["method"] != http.MethodPost {
					t.Fatalf("Expected request method %q, got %#v", http.MethodPost, request["method"])
				}
				if request["path"] != "/api/automation-webhooks/autoapi00000052" {
					t.Fatalf("Expected request path, got %#v", request["path"])
				}

				headers, ok := request["headers"].(map[string]any)
				if !ok || headers["x_automation_event"] != "invoice.paid" {
					t.Fatalf("Expected normalized request headers, got %#v", request["headers"])
				}

				query, ok := request["query"].(map[string]any)
				if !ok || query["tenant"] != "acme" {
					t.Fatalf("Expected request query payload, got %#v", request["query"])
				}

				body, ok := request["body"].(map[string]any)
				if !ok || body["event"] != "invoice.paid" {
					t.Fatalf("Expected request body payload, got %#v", request["body"])
				}
			},
			ExpectedStatus: 204,
		},
		{
			Name:   "webhook response step",
			Method: http.MethodPost,
			URL:    "/api/automation-webhooks/autoapi00000055?tenant=acme",
			Body:   strings.NewReader(`{"event":"invoice.paid"}`),
			Headers: map[string]string{
				"Content-Type":       "application/json",
				"X-Automation-Event": "invoice.paid",
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				automation := createWebhookAutomationFixture(t, app, "autoapi00000055", "API webhook response automation")
				automation.SetSteps(mustAutomationJSONRaw(t, `[
					{"type":"condition","path":"request.headers.x_automation_event","op":"eq","value":"invoice.paid"},
					{"type":"response","statusCode":202,"headers":{"X-Automation-Tenant":"{{request.query.tenant}}"},"body":{"ok":true,"event":"{{request.body.event}}","matched":"{{prevStep.output.matched}}"}}
				]`))
				if err := app.Save(automation); err != nil {
					t.Fatalf("Failed to update webhook automation fixture: %v", err)
				}
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				if res.Header.Get("X-Automation-Tenant") != "acme" {
					t.Fatalf("Expected rendered response header, got %q", res.Header.Get("X-Automation-Tenant"))
				}
			},
			ExpectedStatus: 202,
			ExpectedContent: []string{
				`"ok":true`,
				`"event":"invoice.paid"`,
				`"matched":true`,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestAutomationTelegramWebhook(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "telegram credentials disabled",
			Method:          http.MethodPost,
			URL:             "/api/automation-telegram/token_123",
			Body:            strings.NewReader(`{"update_id":901,"message":{"text":"/start"}}`),
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "invalid token",
			Method: http.MethodPost,
			URL:    "/api/automation-telegram/wrong_token",
			Body:   strings.NewReader(`{"update_id":901,"message":{"text":"/start"}}`),
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				app.Settings().Credentials.Telegram.Enabled = true
				app.Settings().Credentials.Telegram.AccessToken = "token_123"
			},
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "invalid Telegram json",
			Method: http.MethodPost,
			URL:    "/api/automation-telegram/token_123",
			Body:   strings.NewReader(`{"update_id":`),
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				app.Settings().Credentials.Telegram.Enabled = true
				app.Settings().Credentials.Telegram.AccessToken = "token_123"
			},
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"message":"Failed to load Telegram update."`,
			},
		},
		{
			Name:   "valid Telegram message update",
			Method: http.MethodPost,
			URL:    "/api/automation-telegram/token_123",
			Body: strings.NewReader(`{
				"update_id":901,
				"message":{
					"message_id":11,
					"text":"/start",
					"chat":{"id":12345,"username":"test_chat"},
					"from":{"id":54321,"username":"sender"}
				}
			}`),
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Delay: 100 * time.Millisecond,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				app.Settings().Credentials.Telegram.Enabled = true
				app.Settings().Credentials.Telegram.AccessToken = "token_123"
				createTelegramAutomationFixture(t, app, "autoapi00000056", "API Telegram automation")
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				automationRecord, err := app.FindRecordById(core.CollectionNameAutomations, "autoapi00000056")
				if err != nil {
					t.Fatalf("Expected Telegram automation to exist: %v", err)
				}

				automation := &core.Automation{}
				automation.SetProxyRecord(automationRecord)

				runs := waitForAutomationRunsAPI(t, app, automation, 1)
				run := runs[0]
				if run.TriggerType() != core.AutomationTriggerTelegramMessage {
					t.Fatalf("Expected Telegram trigger type %q, got %q", core.AutomationTriggerTelegramMessage, run.TriggerType())
				}

				input := decodeAutomationRunInputAPI(t, run)
				telegram, ok := input["telegram"].(map[string]any)
				if !ok {
					t.Fatalf("Expected Telegram payload in automation run input, got %#v", input["telegram"])
				}
				if telegram["updateId"] != "901" || telegram["text"] != "/start" {
					t.Fatalf("Expected normalized Telegram update payload, got %#v", telegram)
				}
			},
			ExpectedStatus: 204,
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestAutomationRunsList(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:   "authorized as superuser",
			Method: http.MethodGet,
			URL:    "/api/automations/autoapi00000006/runs",
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				automation := createAutomationFixture(t, app, "autoapi00000006", "API runs automation")
				if err := app.RunAutomationManually(automation.Id); err != nil {
					t.Fatalf("Failed to create manual automation run fixture: %v", err)
				}
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"automationRef":"autoapi00000006"`,
				`"triggerType":"manual"`,
				`"status":"success"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser with limit",
			Method: http.MethodGet,
			URL:    "/api/automations/autoapi00000007/runs?limit=1",
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				automation := createAutomationFixture(t, app, "autoapi00000007", "API recent runs automation")
				if err := app.RunAutomationManually(automation.Id); err != nil {
					t.Fatalf("Failed to create first automation run fixture: %v", err)
				}
				if err := app.RunAutomationManually(automation.Id); err != nil {
					t.Fatalf("Failed to create second automation run fixture: %v", err)
				}
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				defer res.Body.Close()

				runs := []map[string]any{}
				if err := json.NewDecoder(res.Body).Decode(&runs); err != nil {
					t.Fatalf("Failed to decode runs response: %v", err)
				}
				if len(runs) != 1 {
					t.Fatalf("Expected 1 run due to limit=1, got %d", len(runs))
				}
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"automationRef":"autoapi00000007"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:   "invalid query params",
			Method: http.MethodGet,
			URL:    "/api/automations/autoapi00000008/runs?limit=-1",
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createAutomationFixture(t, app, "autoapi00000008", "API invalid runs query automation")
			},
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"message":"Invalid automation runs query params."`,
			},
		},
		{
			Name:   "missing automation",
			Method: http.MethodGet,
			URL:    "/api/automations/missing/runs",
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestAutomationRunsClear(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:   "authorized as superuser",
			Method: http.MethodDelete,
			URL:    "/api/automations/autoapi00000009/runs",
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				automation := createAutomationFixture(t, app, "autoapi00000009", "API clear runs automation")
				otherAutomation := createAutomationFixture(t, app, "autoapi00000010", "API keep runs automation")

				if err := app.RunAutomationManually(automation.Id); err != nil {
					t.Fatalf("Failed to create automation run fixture: %v", err)
				}
				if err := app.RunAutomationManually(otherAutomation.Id); err != nil {
					t.Fatalf("Failed to create other automation run fixture: %v", err)
				}
				waitForAutomationRunsAPI(t, app, automation, 1)
				waitForAutomationRunsAPI(t, app, otherAutomation, 1)
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				automation, err := app.FindAutomationById("autoapi00000009")
				if err != nil {
					t.Fatalf("Failed to reload automation fixture: %v", err)
				}
				runs, err := app.FindAllAutomationRunsByAutomation(automation)
				if err != nil {
					t.Fatalf("Failed to load automation runs: %v", err)
				}
				if len(runs) != 0 {
					t.Fatalf("Expected cleared automation runs, got %d", len(runs))
				}

				otherAutomation, err := app.FindAutomationById("autoapi00000010")
				if err != nil {
					t.Fatalf("Failed to reload other automation fixture: %v", err)
				}
				otherRuns, err := app.FindAllAutomationRunsByAutomation(otherAutomation)
				if err != nil {
					t.Fatalf("Failed to load other automation runs: %v", err)
				}
				if len(otherRuns) != 1 {
					t.Fatalf("Expected other automation runs to remain, got %d", len(otherRuns))
				}
			},
			ExpectedStatus: 204,
			ExpectedEvents: map[string]int{
				"*":                          0,
				"OnModelDelete":              1,
				"OnModelDeleteExecute":       1,
				"OnModelAfterDeleteSuccess":  1,
				"OnRecordDelete":             1,
				"OnRecordDeleteExecute":      1,
				"OnRecordAfterDeleteSuccess": 1,
			},
		},
		{
			Name:   "missing automation",
			Method: http.MethodDelete,
			URL:    "/api/automations/missing/runs",
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			ExpectedStatus:  404,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:           "unauthorized",
			Method:         http.MethodDelete,
			URL:            "/api/automations/autoapi00000009/runs",
			ExpectedStatus: 401,
			ExpectedContent: []string{
				`"message":"The request requires valid record authorization token."`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestAutomationApprovalsList(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "unauthorized",
			Method:          http.MethodGet,
			URL:             "/api/automations/approvals",
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as regular user",
			Method: http.MethodGet,
			URL:    "/api/automations/approvals",
			Headers: map[string]string{
				"Authorization": testRegularAuthHeader,
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser",
			Method: http.MethodGet,
			URL:    "/api/automations/approvals?status=pending",
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createApprovalFixture(t, app)
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"approvalapi0001"`,
				`"role":"manager"`,
				`"status":"pending"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestAutomationApprovalDecision(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "unauthorized",
			Method:          http.MethodPost,
			URL:             "/api/automations/approvals/approvalapi0001/decision",
			Body:            strings.NewReader(`{"decision":"rejected"}`),
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as regular user",
			Method: http.MethodPost,
			URL:    "/api/automations/approvals/approvalapi0001/decision",
			Body:   strings.NewReader(`{"decision":"rejected"}`),
			Headers: map[string]string{
				"Authorization": testRegularAuthHeader,
			},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized as superuser",
			Method: http.MethodPost,
			URL:    "/api/automations/approvals/approvalapi0001/decision",
			Body:   strings.NewReader(`{"decision":"rejected","comment":"No"}`),
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createApprovalFixture(t, app)
			},
			ExpectedStatus: 204,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				approval, err := app.FindApprovalById("approvalapi0001")
				if err != nil {
					t.Fatalf("Expected resolved approval, got error: %v", err)
				}
				if approval.Status() != core.ApprovalStatusRejected || approval.Comment() != "No" {
					t.Fatalf("Expected rejected approval with comment, got status=%q comment=%q", approval.Status(), approval.Comment())
				}

				for i := 0; i < 20; i++ {
					run, err := app.FindAutomationRunById("runapproval0001")
					if err != nil {
						t.Fatalf("Expected automation run, got error: %v", err)
					}
					if run.Status() == core.AutomationRunStatusFailed {
						return
					}
					time.Sleep(25 * time.Millisecond)
				}

				t.Fatal("Expected approval workflow continuation to fail the run in the background")
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func createAutomationFixture(t testing.TB, app *tests.TestApp, id string, name string) *core.Automation {
	t.Helper()

	automation := core.NewAutomation(app)
	automation.SetRaw("id", id)
	automation.SetName(name)
	automation.SetActive(true)
	automation.SetTriggerType(core.AutomationTriggerManual)
	automation.SetSteps(mustAutomationJSONRaw(t, `[{"type":"condition","path":"trigger.type","op":"exists"}]`))

	if err := app.Save(automation); err != nil {
		t.Fatalf("Failed to create automation fixture: %v", err)
	}

	return automation
}

func createWebhookAutomationFixture(t testing.TB, app *tests.TestApp, id string, name string) *core.Automation {
	t.Helper()

	automation := createAutomationFixture(t, app, id, name)
	automation.SetTriggerType(core.AutomationTriggerWebhook)
	automation.SetSteps(mustAutomationJSONRaw(t, `[{"type":"condition","path":"request.headers.x_automation_event","op":"eq","value":"invoice.paid"}]`))

	if err := app.Save(automation); err != nil {
		t.Fatalf("Failed to create webhook automation fixture: %v", err)
	}

	return automation
}

func createTelegramAutomationFixture(t testing.TB, app *tests.TestApp, id string, name string) *core.Automation {
	t.Helper()

	automation := createAutomationFixture(t, app, id, name)
	automation.SetTriggerType(core.AutomationTriggerTelegramMessage)
	automation.SetSteps(mustAutomationJSONRaw(t, `[{"type":"condition","path":"telegram.text","op":"eq","value":"/start"}]`))

	if err := app.Save(automation); err != nil {
		t.Fatalf("Failed to create Telegram automation fixture: %v", err)
	}

	return automation
}

func createApprovalFixture(t testing.TB, app *tests.TestApp) *core.Approval {
	t.Helper()

	automation := createAutomationFixture(t, app, "autoapproval001", "Approval API automation")

	run := core.NewAutomationRun(app)
	run.SetRaw("id", "runapproval0001")
	run.SetAutomationRef(automation.Id)
	run.SetTriggerType(core.AutomationTriggerManual)
	run.SetStatus(core.AutomationRunStatusWaiting)
	run.SetInput(mustAutomationJSONRaw(t, `{"triggerType":"manual"}`))
	run.ClearErrorStepIndex()
	if err := app.Save(run); err != nil {
		t.Fatalf("Failed to create automation run fixture: %v", err)
	}

	state := core.NewWorkflowState(app)
	state.SetRaw("id", "stateapproval01")
	state.SetAutomationRef(automation.Id)
	state.SetRunRef(run.Id)
	state.SetStatus(core.WorkflowStateStatusWaiting)
	state.SetCurrentStepIndex(0)
	state.SetResumeToken("approval_resume_token")
	state.SetContext(mustAutomationJSONRaw(t, `{"triggerType":"manual"}`))
	state.SetCheckpoints(mustAutomationJSONRaw(t, `[]`))
	state.SetWaitingFor(mustAutomationJSONRaw(t, `{"type":"wait.approval","approvalId":"approvalapi0001"}`))
	if err := app.Save(state); err != nil {
		t.Fatalf("Failed to create workflow state fixture: %v", err)
	}

	approval := core.NewApproval(app)
	approval.SetRaw("id", "approvalapi0001")
	approval.SetWorkflowStateRef(state.Id)
	approval.SetAutomationRef(automation.Id)
	approval.SetRunRef(run.Id)
	approval.SetStepIndex(0)
	approval.SetRole("manager")
	approval.SetStatus(core.ApprovalStatusPending)
	if err := app.Save(approval); err != nil {
		t.Fatalf("Failed to create approval fixture: %v", err)
	}

	return approval
}

func mustAutomationJSONRaw(t testing.TB, raw string) types.JSONRaw {
	t.Helper()

	parsed, err := types.ParseJSONRaw(raw)
	if err != nil {
		t.Fatalf("Failed to parse automation JSON raw %q: %v", raw, err)
	}

	return parsed
}

func waitForAutomationRunsAPI(t testing.TB, app *tests.TestApp, automation *core.Automation, expected int) []*core.AutomationRun {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runs, err := app.FindAllAutomationRunsByAutomation(automation)
		if err == nil && len(runs) >= expected {
			return runs
		}
		if err == nil && expected == 0 && len(runs) == 0 {
			return runs
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("Expected at least %d automation runs for %s", expected, automation.Id)
	return nil
}

func decodeAutomationRunInputAPI(t testing.TB, run *core.AutomationRun) map[string]any {
	t.Helper()

	result := map[string]any{}
	if err := json.Unmarshal([]byte(run.Input().String()), &result); err != nil {
		t.Fatalf("Failed to decode automation run input: %v", err)
	}

	return result
}
