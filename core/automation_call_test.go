package core_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func newManualAutomationForTest(t *testing.T, app *tests.TestApp, name string, inputs []core.AutomationManualInput, steps string) *core.Automation {
	t.Helper()

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetName(name)
	automation.SetActive(true)
	automation.SetTriggerType(core.AutomationTriggerManual)
	automation.SetManualInputs(inputs)
	automation.SetSteps(mustParseJSONRaw(t, steps))
	if err := app.Save(automation); err != nil {
		t.Fatalf("Failed to create automation %q: %v", name, err)
	}

	return automation
}

func newDoublerAutomationForTest(t *testing.T, app *tests.TestApp) *core.Automation {
	t.Helper()

	return newManualAutomationForTest(t, app, "doubler", []core.AutomationManualInput{
		{Name: "amount", Type: core.AutomationManualInputNumber, Required: true},
		{Name: "label", Type: core.AutomationManualInputText, Default: "none"},
	}, `[
		{"type":"code","code":"return { doubled: input.amount * 2, label: input.label, caller: trigger.caller ? trigger.caller.automationName : '' }"},
		{"type":"response","body":{"total":"{{steps[0].output.doubled}}","label":"{{steps[0].output.label}}","caller":"{{steps[0].output.caller}}"}}
	]`)
}

func TestAutomationManualRunValidatesAndCoercesInput(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	automation := newDoublerAutomationForTest(t, app)

	_, err := app.RunAutomationManuallyWithOptions(automation.Id, core.AutomationManualRunOptions{Wait: true})
	var validationErrs validation.Errors
	if !errors.As(err, &validationErrs) || validationErrs["amount"] == nil {
		t.Fatalf("Expected amount validation error, got %v", err)
	}
	if runs, _ := app.FindAllAutomationRunsByAutomation(automation); len(runs) != 0 {
		t.Fatalf("Expected no runs for invalid input, got %d", len(runs))
	}

	_, err = app.RunAutomationManuallyWithOptions(automation.Id, core.AutomationManualRunOptions{
		Wait:  true,
		Input: map[string]any{"amount": "abc"},
	})
	if !errors.As(err, &validationErrs) || validationErrs["amount"] == nil {
		t.Fatalf("Expected amount type error, got %v", err)
	}

	result, err := app.RunAutomationManuallyWithOptions(automation.Id, core.AutomationManualRunOptions{
		Wait:  true,
		Input: map[string]any{"amount": "21", "extra": "kept"},
	})
	if err != nil {
		t.Fatalf("Expected successful run, got %v", err)
	}
	if result.Status != core.AutomationRunStatusSuccess || result.RunId == "" {
		t.Fatalf("Expected successful run result with run id, got %+v", result)
	}

	output, ok := result.Output.(map[string]any)
	if !ok || fmt.Sprint(output["total"]) != "42" || output["label"] != "none" {
		t.Fatalf("Expected response body output, got %#v", result.Output)
	}

	run, err := app.FindAutomationRunById(result.RunId)
	if err != nil {
		t.Fatalf("Expected run %q to be stored: %v", result.RunId, err)
	}
	input, _ := decodeAutomationRunInput(t, run)["input"].(map[string]any)
	if input["amount"] != float64(21) || input["extra"] != "kept" {
		t.Fatalf("Expected coerced input to be stored, got %#v", input)
	}
}

func TestAutomationManualRunInBackgroundReturnsRunId(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	automation := newDoublerAutomationForTest(t, app)

	result, err := app.RunAutomationManuallyWithOptions(automation.Id, core.AutomationManualRunOptions{
		Input: map[string]any{"amount": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != core.AutomationRunStatusQueued || result.RunId == "" {
		t.Fatalf("Expected queued result with run id, got %+v", result)
	}

	runs := waitForCompletedAutomationRuns(t, app, automation, 1)
	if runs[0].Id != result.RunId {
		t.Fatalf("Expected run id %q, got %q", result.RunId, runs[0].Id)
	}
}

func TestAutomationCallStepWaitReturnsChildOutput(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	child := newDoublerAutomationForTest(t, app)
	parent := newManualAutomationForTest(t, app, "parent", []core.AutomationManualInput{
		{Name: "base", Type: core.AutomationManualInputNumber, Required: true},
	}, `[
		{"type":"automation.call","automation":"`+child.Id+`","mode":"wait","input":{"amount":"{{input.base}}","label":"from parent"}},
		{"type":"code","code":"return { result: steps[0].output.output.total, label: prevStep.output.output.label, caller: steps[0].output.output.caller }"}
	]`)

	result, err := app.RunAutomationManuallyWithOptions(parent.Id, core.AutomationManualRunOptions{
		Wait:  true,
		Input: map[string]any{"base": 5},
	})
	if err != nil {
		t.Fatalf("Expected successful parent run, got %v", err)
	}

	output, _ := result.Output.(map[string]any)
	if fmt.Sprint(output["result"]) != "10" || output["label"] != "from parent" || output["caller"] != "parent" {
		t.Fatalf("Expected child output to be available to the parent, got %#v", result.Output)
	}

	childRuns := waitForCompletedAutomationRuns(t, app, child, 1)
	if childRuns[0].ParentRunId() != result.RunId {
		t.Fatalf("Expected child parentRunId %q, got %q", result.RunId, childRuns[0].ParentRunId())
	}
	if childRuns[0].Depth() != 1 {
		t.Fatalf("Expected child depth 1, got %d", childRuns[0].Depth())
	}
}

func TestAutomationCallStepParallelQueuesChildRun(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	child := newDoublerAutomationForTest(t, app)
	parent := newManualAutomationForTest(t, app, "parent", nil, `[
		{"type":"automation.call","automation":"`+child.Id+`","mode":"parallel","input":{"amount":3}}
	]`)

	result, err := app.RunAutomationManuallyWithOptions(parent.Id, core.AutomationManualRunOptions{Wait: true})
	if err != nil {
		t.Fatalf("Expected successful parent run, got %v", err)
	}

	output, _ := result.Output.(map[string]any)
	if output["mode"] != core.AutomationCallModeParallel || output["status"] != core.AutomationRunStatusQueued {
		t.Fatalf("Expected queued parallel call output, got %#v", result.Output)
	}
	childRunId, _ := output["runId"].(string)
	if childRunId == "" {
		t.Fatal("Expected the parallel call to return the child run id")
	}

	childRuns := waitForCompletedAutomationRuns(t, app, child, 1)
	if childRuns[0].Id != childRunId {
		t.Fatalf("Expected child run %q, got %q", childRunId, childRuns[0].Id)
	}
	if childRuns[0].Status() != core.AutomationRunStatusSuccess {
		t.Fatalf("Expected child run success, got %q (%s)", childRuns[0].Status(), childRuns[0].Error())
	}
	if childRuns[0].ParentRunId() != result.RunId {
		t.Fatalf("Expected child parentRunId %q, got %q", result.RunId, childRuns[0].ParentRunId())
	}
}

func TestAutomationCallStepFailures(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	failing := newManualAutomationForTest(t, app, "failing", nil, `[
		{"type":"code","code":"throw new Error('boom')"}
	]`)

	// failing child fails the parent by default
	parent := newManualAutomationForTest(t, app, "strict parent", nil, `[
		{"type":"automation.call","automation":"`+failing.Id+`"}
	]`)
	if _, err := app.RunAutomationManuallyWithOptions(parent.Id, core.AutomationManualRunOptions{Wait: true}); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Expected parent to fail with the child error, got %v", err)
	}

	// continueOnError keeps the parent running
	lenient := newManualAutomationForTest(t, app, "lenient parent", nil, `[
		{"type":"automation.call","automation":"`+failing.Id+`","continueOnError":true},
		{"type":"code","code":"return { childStatus: prevStep.output.status }"}
	]`)
	result, err := app.RunAutomationManuallyWithOptions(lenient.Id, core.AutomationManualRunOptions{Wait: true})
	if err != nil {
		t.Fatalf("Expected lenient parent to succeed, got %v", err)
	}
	if output, _ := result.Output.(map[string]any); output["childStatus"] != core.AutomationRunStatusFailed {
		t.Fatalf("Expected failed child status to be exposed, got %#v", result.Output)
	}

	// inactive targets can't be called
	failing.SetActive(false)
	if err := app.Save(failing); err != nil {
		t.Fatal(err)
	}
	if _, err := app.RunAutomationManuallyWithOptions(parent.Id, core.AutomationManualRunOptions{Wait: true}); err == nil || !strings.Contains(err.Error(), "inactive") {
		t.Fatalf("Expected inactive target error, got %v", err)
	}
}

func TestAutomationCallStepValidation(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	webhook := core.NewAutomation(app)
	populateValidAutomation(webhook)
	webhook.SetTriggerType(core.AutomationTriggerWebhook)
	webhook.SetSteps(mustParseJSONRaw(t, `[{"type":"code","code":"return {}"}]`))
	if err := app.Save(webhook); err != nil {
		t.Fatal(err)
	}

	scenarios := []struct {
		name  string
		steps string
	}{
		{"missing target", `[{"type":"automation.call"}]`},
		{"unknown target", `[{"type":"automation.call","automation":"missing"}]`},
		{"non-manual target", `[{"type":"automation.call","automation":"` + webhook.Id + `"}]`},
		{"invalid mode", `[{"type":"automation.call","automation":"{{input.target}}","mode":"later"}]`},
		{"invalid input", `[{"type":"automation.call","automation":"{{input.target}}","input":"x"}]`},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			automation := core.NewAutomation(app)
			populateValidAutomation(automation)
			automation.SetTriggerType(core.AutomationTriggerManual)
			automation.SetSteps(mustParseJSONRaw(t, s.steps))
			if err := app.Save(automation); err == nil {
				t.Fatal("Expected validation error")
			}
		})
	}

	t.Run("invalid manual inputs", func(t *testing.T) {
		automation := core.NewAutomation(app)
		populateValidAutomation(automation)
		automation.SetTriggerType(core.AutomationTriggerManual)
		automation.SetManualInputs([]core.AutomationManualInput{
			{Name: "a", Type: core.AutomationManualInputText},
			{Name: "a", Type: core.AutomationManualInputText},
		})
		automation.SetSteps(mustParseJSONRaw(t, `[{"type":"code","code":"return {}"}]`))
		if err := app.Save(automation); err == nil {
			t.Fatal("Expected duplicate input validation error")
		}
	})
}

func TestAutomationCallStepRecursionStopsAtMaxDepth(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	app.Store().Set(core.StoreKeyAutomationPolicyConfig, core.AutomationPolicyConfig{
		MaxDepth:          2,
		MaxRunsPerMinute:  100,
		MaxConcurrentRuns: 10,
	})

	automation := newManualAutomationForTest(t, app, "recursive", nil, `[{"type":"code","code":"return {}"}]`)
	automation.SetSteps(mustParseJSONRaw(t, `[{"type":"automation.call","automation":"`+automation.Id+`"}]`))
	if err := app.Save(automation); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := app.RunAutomationManuallyWithOptions(automation.Id, core.AutomationManualRunOptions{Wait: true})
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "max_depth_exceeded") {
			t.Fatalf("Expected max depth error, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Recursive call didn't stop")
	}
}

func TestAutomationCallStepDryRunDoesNotExecuteChild(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	child := newDoublerAutomationForTest(t, app)
	parent := newManualAutomationForTest(t, app, "parent", nil, `[
		{"type":"automation.call","automation":"`+child.Id+`","input":{"amount":"{{input.value}}"}}
	]`)

	result, err := app.RunAutomationDryRun(parent.Id, map[string]any{"input": map[string]any{"value": 4}})
	if err != nil {
		t.Fatalf("Expected dry run to succeed, got %v", err)
	}

	output, _ := result.StepResults[0].Output.(map[string]any)
	input, _ := output["input"].(map[string]any)
	if output["dryRun"] != true || input["amount"] != float64(4) {
		t.Fatalf("Expected dry run preview with rendered input, got %#v", result.StepResults[0].Output)
	}

	if runs, _ := app.FindAllAutomationRunsByAutomation(child); len(runs) != 0 {
		t.Fatalf("Expected no child runs during dry run, got %d", len(runs))
	}
}
