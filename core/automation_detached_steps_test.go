package core_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestAutomationDetachedStepsArePersistedButNotExecuted(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetActive(true)
	automation.SetTriggerType(core.AutomationTriggerManual)
	automation.SetSteps(mustParseJSONRaw(t, `[{"type":"code","code":"return { ran: 'connected' }"}]`))
	// detached steps are editor drafts - they may be incomplete and never run
	automation.SetDetachedSteps(mustParseJSONRaw(t, `[[{"type":"code","code":"throw new Error('detached step executed')"},{"type":"http"}]]`))
	if err := app.Save(automation); err != nil {
		t.Fatalf("Expected automation with detached steps to save, got %v", err)
	}

	reloaded, err := app.FindAutomationById(automation.Id)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reloaded.DetachedSteps().String(), "detached step executed") {
		t.Fatalf("Expected detached steps to be persisted, got %s", reloaded.DetachedSteps().String())
	}

	result, err := app.RunAutomationManuallyWithOptions(automation.Id, core.AutomationManualRunOptions{Wait: true})
	if err != nil {
		t.Fatalf("Expected run to ignore detached steps, got %v", err)
	}
	if output, _ := result.Output.(map[string]any); fmt.Sprint(output["ran"]) != "connected" {
		t.Fatalf("Expected only the connected step to run, got %#v", result.Output)
	}
}

func TestAutomationDetachedStepsValidation(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	scenarios := []struct {
		name  string
		value string
	}{
		{"not a list of groups", `[{"type":"code"}]`},
		{"unknown step type", `[[{"type":"unknown"}]]`},
		{"missing step type", `[[{"code":"return {}"}]]`},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			automation := core.NewAutomation(app)
			populateValidAutomation(automation)
			automation.SetTriggerType(core.AutomationTriggerManual)
			automation.SetSteps(mustParseJSONRaw(t, `[{"type":"code","code":"return {}"}]`))
			automation.SetDetachedSteps(mustParseJSONRaw(t, s.value))
			if err := app.Save(automation); err == nil || !strings.Contains(err.Error(), "detachedSteps") {
				t.Fatalf("Expected detachedSteps validation error, got %v", err)
			}
		})
	}
}
