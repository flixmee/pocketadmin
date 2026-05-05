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

func TestAutomationCreate(t *testing.T) {
	t.Parallel()

	validBody := `{
		"name":"API create automation",
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
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"name":"API create automation"`,
				`"triggerType":"manual"`,
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
			Body:   strings.NewReader(`{"active":false,"notes":"updated through api"}`),
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
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"autoapi00000003"`,
				`"active":false`,
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
