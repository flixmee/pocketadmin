package apis_test

import (
	"net/http"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func createAIMemoryAutomationFixture(t testing.TB, app core.App) *core.Automation {
	automation := core.NewAutomation(app)
	automation.Id = "aimemorytest001"
	automation.SetName("Memory")
	automation.SetTriggerType(core.AutomationTriggerManual)
	automation.Set("steps", `[{"type":"ai.generate","input":"x","memory":{"enabled":true}}]`)
	if err := app.Save(automation); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"chat-1", "chat-2"} {
		if err := core.SaveAutomationAIMemoryTurn(app, automation.Id, key, "hello "+key, "hi "+key, 20); err != nil {
			t.Fatal(err)
		}
	}

	return automation
}

func TestAutomationAIMemoryAPI(t *testing.T) {
	t.Parallel()

	superuser := map[string]string{"Authorization": testSuperuserAuthHeader}

	scenarios := []tests.ApiScenario{
		{
			Name:            "list unauthorized",
			Method:          http.MethodGet,
			URL:             "/api/automations/aimemorytest001/ai-memory",
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:           "list sessions",
			Method:         http.MethodGet,
			URL:            "/api/automations/aimemorytest001/ai-memory",
			Headers:        superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) { createAIMemoryAutomationFixture(t, app) },
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"key":"chat-1"`,
				`"key":"chat-2"`,
				`"messages":2`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:           "view session messages",
			Method:         http.MethodGet,
			URL:            "/api/automations/aimemorytest001/ai-memory?key=chat-2",
			Headers:        superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) { createAIMemoryAutomationFixture(t, app) },
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"key":"chat-2"`,
				`"messages":[{"role":"user","content":"hello chat-2"},{"role":"assistant","content":"hi chat-2"}]`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:           "clear a single session",
			Method:         http.MethodDelete,
			URL:            "/api/automations/aimemorytest001/ai-memory?key=chat-1",
			Headers:        superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) { createAIMemoryAutomationFixture(t, app) },
			ExpectedStatus: 204,
			ExpectedEvents: map[string]int{"*": 0},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				sessions, _ := core.FindAutomationAIMemorySessions(app, "aimemorytest001")
				if len(sessions) != 1 || sessions[0].Key != "chat-2" {
					t.Fatalf("Expected only chat-2 to remain, got %v", sessions)
				}
			},
		},
		{
			Name:           "clear all",
			Method:         http.MethodDelete,
			URL:            "/api/automations/aimemorytest001/ai-memory",
			Headers:        superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) { createAIMemoryAutomationFixture(t, app) },
			ExpectedStatus: 204,
			ExpectedEvents: map[string]int{"*": 0},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				sessions, _ := core.FindAutomationAIMemorySessions(app, "aimemorytest001")
				if len(sessions) != 0 {
					t.Fatalf("Expected no sessions, got %v", sessions)
				}
			},
		},
		{
			Name:            "missing automation",
			Method:          http.MethodDelete,
			URL:             "/api/automations/missing/ai-memory",
			Headers:         superuser,
			ExpectedStatus:  404,
			ExpectedContent: []string{`"status":404`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}
