package core_test

import (
	"fmt"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestAutomationExecutionDataManualRunReturnsMergedData(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	automation := newManualAutomationForTest(t, app, "execution-data", []core.AutomationManualInput{
		{Name: "amount", Type: core.AutomationManualInputNumber, Required: true},
	}, `[
		{"type":"code","code":"return { doubled: input.amount * 2, tags: ['a', 'b'] }"},
		{"type":"execution.data","data":{"total":"{{steps[0].output.doubled}}","status":"draft","tags":"{{steps[0].output.tags}}"}},
		{"type":"code","code":"return { after: true }"},
		{"type":"execution.data","data":{"status":"done","label":"total is {{steps[0].output.doubled}}"}}
	]`)

	result, err := app.RunAutomationManuallyWithOptions(automation.Id, core.AutomationManualRunOptions{
		Wait:  true,
		Input: map[string]any{"amount": 4},
	})
	if err != nil {
		t.Fatalf("Expected successful run, got %v", err)
	}
	if result.Status != core.AutomationRunStatusSuccess {
		t.Fatalf("Expected success status, got %+v", result)
	}

	output, ok := result.Output.(map[string]any)
	if !ok {
		t.Fatalf("Expected map output, got %#v", result.Output)
	}

	// whole templates keep their type, later steps override earlier keys
	if fmt.Sprint(output["total"]) != "8" {
		t.Fatalf("Expected total 8, got %#v", output["total"])
	}
	if output["status"] != "done" {
		t.Fatalf("Expected later step to override status, got %#v", output["status"])
	}
	if output["label"] != "total is 8" {
		t.Fatalf("Expected interpolated label, got %#v", output["label"])
	}
	if tags, ok := output["tags"].([]any); !ok || len(tags) != 2 {
		t.Fatalf("Expected tags array to be preserved, got %#v", output["tags"])
	}
	if _, ok := output["after"]; ok {
		t.Fatalf("Expected only execution data to be returned, got %#v", output)
	}
}

func TestAutomationExecutionDataTakesPrecedenceOverResponse(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	automation := newManualAutomationForTest(t, app, "execution-data-response", nil, `[
		{"type":"execution.data","data":{"source":"execution data"}},
		{"type":"response","body":{"source":"response"}}
	]`)

	result, err := app.RunAutomationManuallyWithOptions(automation.Id, core.AutomationManualRunOptions{Wait: true})
	if err != nil {
		t.Fatalf("Expected successful run, got %v", err)
	}

	output, _ := result.Output.(map[string]any)
	if output["source"] != "execution data" {
		t.Fatalf("Expected execution data output, got %#v", result.Output)
	}
}

func TestAutomationExecutionDataReturnedToCaller(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	child := newManualAutomationForTest(t, app, "child", []core.AutomationManualInput{
		{Name: "name", Type: core.AutomationManualInputText, Required: true},
	}, `[
		{"type":"execution.data","data":{"greeting":"Hello {{input.name}}","caller":"{{trigger.caller.automationName}}"}}
	]`)
	parent := newManualAutomationForTest(t, app, "parent", nil, `[
		{"type":"automation.call","automation":"`+child.Id+`","mode":"wait","input":{"name":"Ada"}},
		{"type":"execution.data","data":{"message":"{{steps[0].output.output.greeting}}","caller":"{{steps[0].output.output.caller}}"}}
	]`)

	result, err := app.RunAutomationManuallyWithOptions(parent.Id, core.AutomationManualRunOptions{Wait: true})
	if err != nil {
		t.Fatalf("Expected successful parent run, got %v", err)
	}

	output, _ := result.Output.(map[string]any)
	if output["message"] != "Hello Ada" || output["caller"] != "parent" {
		t.Fatalf("Expected child execution data to reach the parent, got %#v", result.Output)
	}
}

func TestAutomationExecutionDataValidation(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	scenarios := []struct {
		name        string
		triggerType string
		steps       string
		expectError bool
	}{
		{"valid", core.AutomationTriggerManual, `[{"type":"execution.data","data":{"a":"1"}}]`, false},
		{"missing data", core.AutomationTriggerManual, `[{"type":"execution.data"}]`, true},
		{"non-object data", core.AutomationTriggerManual, `[{"type":"execution.data","data":"x"}]`, true},
		{"empty data", core.AutomationTriggerManual, `[{"type":"execution.data","data":{}}]`, true},
		{"blank key", core.AutomationTriggerManual, `[{"type":"execution.data","data":{" ":"x"}}]`, true},
		{"non-manual trigger", core.AutomationTriggerWebhook, `[{"type":"execution.data","data":{"a":"1"}}]`, true},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			automation := core.NewAutomation(app)
			populateValidAutomation(automation)
			automation.SetTriggerType(s.triggerType)
			automation.SetSteps(mustParseJSONRaw(t, s.steps))

			err := app.Save(automation)
			if s.expectError && err == nil {
				t.Fatal("Expected validation error")
			}
			if !s.expectError && err != nil {
				t.Fatalf("Expected no validation error, got %v", err)
			}
		})
	}
}
