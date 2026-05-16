package core_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/types"
)

func TestAutomationCollectionsExist(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	scenarios := []struct {
		name           string
		expectedFields []string
	}{
		{
			name: core.CollectionNameAutomations,
			expectedFields: []string{
				"id",
				"name",
				"active",
				"triggerType",
				"collectionRef",
				"cronExpr",
				"steps",
				"notes",
				"lastRunAt",
				"lastRunStatus",
				"created",
				"updated",
			},
		},
		{
			name: core.CollectionNameCapabilities,
			expectedFields: []string{
				"id",
				"key",
				"version",
				"category",
				"icon",
				"inputSchema",
				"outputSchema",
				"authStrategy",
				"runtimeHandler",
				"configUI",
				"connectorRef",
				"requiredScopes",
				"active",
				"created",
				"updated",
			},
		},
		{
			name: core.CollectionNameAutomationRuns,
			expectedFields: []string{
				"id",
				"automationRef",
				"triggerType",
				"status",
				"input",
				"stepResults",
				"error",
				"errorStepIndex",
				"parentRunId",
				"depth",
				"dedupeKey",
				"policyDecision",
				"workflowVersionRef",
				"workflowVersionSnapshot",
				"started",
				"finished",
				"created",
				"updated",
			},
		},
		{
			name: core.CollectionNameWorkflowState,
			expectedFields: []string{
				"id",
				"automationRef",
				"runRef",
				"status",
				"currentStepIndex",
				"context",
				"checkpoints",
				"resumeToken",
				"waitingFor",
				"expires",
				"created",
				"updated",
			},
		},
		{
			name: core.CollectionNameApprovals,
			expectedFields: []string{
				"id",
				"workflowStateRef",
				"automationRef",
				"runRef",
				"stepIndex",
				"assignee",
				"role",
				"status",
				"decision",
				"comment",
				"resolved",
				"created",
				"updated",
			},
		},
		{
			name: core.CollectionNameConnectors,
			expectedFields: []string{
				"id",
				"provider",
				"authType",
				"credentials",
				"scopes",
				"rateLimits",
				"active",
				"created",
				"updated",
			},
		},
		{
			name: core.CollectionNameAutomationEvents,
			expectedFields: []string{
				"id",
				"name",
				"source",
				"subject",
				"payload",
				"occurred",
				"correlationId",
				"causationId",
				"created",
			},
		},
		{
			name: core.CollectionNameWorkflowVersions,
			expectedFields: []string{
				"id",
				"automationRef",
				"version",
				"status",
				"snapshot",
				"notes",
				"createdBy",
				"publishedBy",
				"publishedAt",
				"created",
				"updated",
			},
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			collection, err := app.FindCollectionByNameOrId(s.name)
			if err != nil {
				t.Fatal(err)
			}

			if !collection.System {
				t.Fatalf("Expected %s to be a system collection", s.name)
			}

			for _, fieldName := range s.expectedFields {
				if collection.Fields.GetByName(fieldName) == nil {
					t.Fatalf("Expected field %q to exist in %s", fieldName, s.name)
				}
			}
		})
	}
}

func TestNewCapability(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	capability := core.NewCapability(app)

	if capability.Collection().Name != core.CollectionNameCapabilities {
		t.Fatalf("Expected record with %q collection, got %q", core.CollectionNameCapabilities, capability.Collection().Name)
	}
}

func TestConnectorCredentialsHidden(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	collection, err := app.FindCollectionByNameOrId(core.CollectionNameConnectors)
	if err != nil {
		t.Fatal(err)
	}

	field := collection.Fields.GetByName("credentials")
	if field == nil || !field.GetHidden() {
		t.Fatal("Expected connector credentials field to be hidden")
	}
}

func TestCapabilityFields(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	capability := core.NewCapability(app)
	inputSchema := mustParseJSONRaw(t, `{"type":"object"}`)
	outputSchema := mustParseJSONRaw(t, `{"type":"object","properties":{"ok":{"type":"boolean"}}}`)
	configUI := mustParseJSONRaw(t, `{"fields":[]}`)

	capability.SetKey("test.capability")
	capability.SetVersion("1.2.3")
	capability.SetCategory("test")
	capability.SetIcon("bolt")
	capability.SetInputSchema(inputSchema)
	capability.SetOutputSchema(outputSchema)
	capability.SetAuthStrategy("apiKey")
	capability.SetRuntimeHandler(core.AutomationStepHTTP)
	capability.SetConfigUI(configUI)
	capability.SetActive(true)

	if capability.Key() != "test.capability" {
		t.Fatalf("Expected key to roundtrip, got %q", capability.Key())
	}
	if capability.Version() != "1.2.3" {
		t.Fatalf("Expected version to roundtrip, got %q", capability.Version())
	}
	if capability.Category() != "test" {
		t.Fatalf("Expected category to roundtrip, got %q", capability.Category())
	}
	if capability.Icon() != "bolt" {
		t.Fatalf("Expected icon to roundtrip, got %q", capability.Icon())
	}
	if capability.InputSchema().String() != inputSchema.String() {
		t.Fatalf("Expected input schema to roundtrip, got %s", capability.InputSchema())
	}
	if capability.OutputSchema().String() != outputSchema.String() {
		t.Fatalf("Expected output schema to roundtrip, got %s", capability.OutputSchema())
	}
	if capability.AuthStrategy() != "apiKey" {
		t.Fatalf("Expected auth strategy to roundtrip, got %q", capability.AuthStrategy())
	}
	if capability.RuntimeHandler() != core.AutomationStepHTTP {
		t.Fatalf("Expected runtime handler to roundtrip, got %q", capability.RuntimeHandler())
	}
	if capability.ConfigUI().String() != configUI.String() {
		t.Fatalf("Expected config UI to roundtrip, got %s", capability.ConfigUI())
	}
	if !capability.Active() {
		t.Fatal("Expected active to be true")
	}
}

func TestCapabilityValidation(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	capabilitiesCol, err := app.FindCollectionByNameOrId(core.CollectionNameCapabilities)
	if err != nil {
		t.Fatal(err)
	}

	scenarios := []struct {
		name          string
		mutate        func(c *core.Capability)
		expectedField string
	}{
		{
			name: "invalid key",
			mutate: func(c *core.Capability) {
				c.SetKey("Bad")
			},
			expectedField: "key",
		},
		{
			name: "invalid version",
			mutate: func(c *core.Capability) {
				c.SetVersion("1")
			},
			expectedField: "version",
		},
		{
			name: "missing category",
			mutate: func(c *core.Capability) {
				c.SetCategory("")
			},
			expectedField: "category",
		},
		{
			name: "invalid schema",
			mutate: func(c *core.Capability) {
				c.SetInputSchema(mustParseJSONRaw(t, `[]`))
			},
			expectedField: "inputSchema",
		},
		{
			name: "missing runtime handler",
			mutate: func(c *core.Capability) {
				c.SetRuntimeHandler("")
			},
			expectedField: "runtimeHandler",
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			capability := &core.Capability{}
			capability.SetProxyRecord(core.NewRecord(capabilitiesCol))
			populateValidCapability(capability)
			s.mutate(capability)

			err := app.Validate(capability)
			assertValidationField(t, err, s.expectedField)
		})
	}

	t.Run("valid", func(t *testing.T) {
		capability := &core.Capability{}
		capability.SetProxyRecord(core.NewRecord(capabilitiesCol))
		populateValidCapability(capability)

		if err := app.Validate(capability); err != nil {
			t.Fatalf("Expected validation to succeed, got %v", err)
		}
	})

	t.Run("duplicate key and version", func(t *testing.T) {
		first := core.NewCapability(app)
		populateValidCapability(first)
		first.SetKey("duplicate.capability")
		if err := app.Save(first); err != nil {
			t.Fatalf("Failed to create first capability: %v", err)
		}

		second := core.NewCapability(app)
		populateValidCapability(second)
		second.SetKey("duplicate.capability")

		err := app.Save(second)
		assertValidationField(t, err, "key")
	})
}

func TestNewAutomation(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	automation := core.NewAutomation(app)

	if automation.Collection().Name != core.CollectionNameAutomations {
		t.Fatalf("Expected record with %q collection, got %q", core.CollectionNameAutomations, automation.Collection().Name)
	}
}

func TestAutomationProxyRecord(t *testing.T) {
	t.Parallel()

	record := core.NewRecord(core.NewBaseCollection("test"))
	record.Id = "test_id"

	automation := core.Automation{}
	automation.SetProxyRecord(record)

	if automation.ProxyRecord() == nil || automation.ProxyRecord().Id != record.Id {
		t.Fatalf("Expected proxy record with id %q, got %v", record.Id, automation.ProxyRecord())
	}
}

func TestAutomationFields(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	automation := core.NewAutomation(app)
	now := types.NowDateTime()
	steps := mustParseJSONRaw(t, `[{"type":"condition"}]`)

	automation.SetName("Sync orders")
	automation.SetActive(true)
	automation.SetTriggerType(core.AutomationTriggerRecordCreate)
	automation.SetCollectionRef("users")
	automation.SetCronExpr("0 * * * *")
	automation.SetSteps(steps)
	automation.SetNotes("notes")
	automation.SetRaw("lastRunAt", now)
	automation.Set("lastRunStatus", core.AutomationRunStatusSuccess)
	automation.SetRaw("created", now)
	automation.SetRaw("updated", now)

	if automation.Name() != "Sync orders" {
		t.Fatalf("Expected name to roundtrip, got %q", automation.Name())
	}
	if !automation.Active() {
		t.Fatal("Expected active to be true")
	}
	if automation.TriggerType() != core.AutomationTriggerRecordCreate {
		t.Fatalf("Expected trigger type to roundtrip, got %q", automation.TriggerType())
	}
	if automation.CollectionRef() != "users" {
		t.Fatalf("Expected collectionRef to roundtrip, got %q", automation.CollectionRef())
	}
	if automation.CronExpr() != "0 * * * *" {
		t.Fatalf("Expected cronExpr to roundtrip, got %q", automation.CronExpr())
	}
	if automation.Steps().String() != steps.String() {
		t.Fatalf("Expected steps to roundtrip, got %s", automation.Steps())
	}
	if automation.Notes() != "notes" {
		t.Fatalf("Expected notes to roundtrip, got %q", automation.Notes())
	}
	if automation.LastRunAt().String() != now.String() {
		t.Fatalf("Expected lastRunAt %q, got %q", now.String(), automation.LastRunAt().String())
	}
	if automation.LastRunStatus() != core.AutomationRunStatusSuccess {
		t.Fatalf("Expected lastRunStatus to roundtrip, got %q", automation.LastRunStatus())
	}
	if automation.Created().String() != now.String() {
		t.Fatalf("Expected created %q, got %q", now.String(), automation.Created().String())
	}
	if automation.Updated().String() != now.String() {
		t.Fatalf("Expected updated %q, got %q", now.String(), automation.Updated().String())
	}
}

func TestAutomationPreValidate(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	automationsCol, err := app.FindCollectionByNameOrId(core.CollectionNameAutomations)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("no proxy record", func(t *testing.T) {
		automation := &core.Automation{}

		if err := app.Validate(automation); err == nil {
			t.Fatal("Expected collection validation error")
		}
	})

	t.Run("non-automation collection", func(t *testing.T) {
		automation := &core.Automation{}
		automation.SetProxyRecord(core.NewRecord(core.NewBaseCollection("invalid")))
		populateValidAutomation(automation)

		if err := app.Validate(automation); err == nil {
			t.Fatal("Expected collection validation error")
		}
	})

	t.Run("automation collection", func(t *testing.T) {
		automation := &core.Automation{}
		automation.SetProxyRecord(core.NewRecord(automationsCol))
		populateValidAutomation(automation)

		if err := app.Validate(automation); err != nil {
			t.Fatalf("Expected validation to succeed, got %v", err)
		}
	})
}

func TestAutomationValidation(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	automationsCol, err := app.FindCollectionByNameOrId(core.CollectionNameAutomations)
	if err != nil {
		t.Fatal(err)
	}

	usersCol, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatal(err)
	}

	viewCol, err := app.FindCollectionByNameOrId("view1")
	if err != nil {
		t.Fatal(err)
	}

	demo1Col, err := app.FindCollectionByNameOrId("demo1")
	if err != nil {
		t.Fatal(err)
	}

	inactiveCapability := core.NewCapability(app)
	populateValidCapability(inactiveCapability)
	inactiveCapability.SetKey("custom.inactive")
	inactiveCapability.SetActive(false)
	if err := app.Save(inactiveCapability); err != nil {
		t.Fatalf("Failed to create inactive capability fixture: %v", err)
	}

	activeCapability := core.NewCapability(app)
	populateValidCapability(activeCapability)
	activeCapability.SetKey("custom.http")
	activeCapability.SetRuntimeHandler(core.AutomationStepHTTP)
	activeCapability.SetActive(true)
	if err := app.Save(activeCapability); err != nil {
		t.Fatalf("Failed to create active capability fixture: %v", err)
	}

	scenarios := []struct {
		name          string
		mutate        func(a *core.Automation)
		expectedField string
	}{
		{
			name: "invalid trigger type",
			mutate: func(a *core.Automation) {
				a.SetTriggerType("bad.trigger")
			},
			expectedField: "triggerType",
		},
		{
			name: "record trigger missing collectionRef",
			mutate: func(a *core.Automation) {
				a.SetTriggerType(core.AutomationTriggerRecordCreate)
				a.SetCollectionRef("")
			},
			expectedField: "collectionRef",
		},
		{
			name: "record trigger invalid collection type",
			mutate: func(a *core.Automation) {
				a.SetTriggerType(core.AutomationTriggerRecordCreate)
				a.SetCollectionRef(viewCol.Id)
			},
			expectedField: "collectionRef",
		},
		{
			name: "cron trigger invalid expr",
			mutate: func(a *core.Automation) {
				a.SetTriggerType(core.AutomationTriggerScheduleCron)
				a.SetCronExpr("not a cron")
			},
			expectedField: "cronExpr",
		},
		{
			name: "steps must be array",
			mutate: func(a *core.Automation) {
				a.SetSteps(mustParseJSONRaw(t, `{"type":"condition"}`))
			},
			expectedField: "steps",
		},
		{
			name: "step type required",
			mutate: func(a *core.Automation) {
				a.SetSteps(mustParseJSONRaw(t, `[{}]`))
			},
			expectedField: "steps",
		},
		{
			name: "condition step missing path",
			mutate: func(a *core.Automation) {
				a.SetSteps(mustParseJSONRaw(t, `[{"type":"condition","op":"eq","value":true}]`))
			},
			expectedField: "steps",
		},
		{
			name: "http step missing url",
			mutate: func(a *core.Automation) {
				a.SetSteps(mustParseJSONRaw(t, `[{"type":"http","method":"POST"}]`))
			},
			expectedField: "steps",
		},
		{
			name: "mail step missing recipient",
			mutate: func(a *core.Automation) {
				a.SetSteps(mustParseJSONRaw(t, `[{"type":"mail.send","subject":"Test","text":"Hello"}]`))
			},
			expectedField: "steps",
		},
		{
			name: "mail attachments require record trigger",
			mutate: func(a *core.Automation) {
				a.SetSteps(mustParseJSONRaw(t, `[{
					"type":"mail.send",
					"to":["test@example.com"],
					"subject":"Test",
					"text":"Hello",
					"attachments":["file_one"]
				}]`))
			},
			expectedField: "steps",
		},
		{
			name: "mail attachments require file field",
			mutate: func(a *core.Automation) {
				a.SetTriggerType(core.AutomationTriggerRecordCreate)
				a.SetCollectionRef(demo1Col.Id)
				a.SetSteps(mustParseJSONRaw(t, `[{
					"type":"mail.send",
					"to":["test@example.com"],
					"subject":"Test",
					"text":"Hello",
					"attachments":["text"]
				}]`))
			},
			expectedField: "steps",
		},
		{
			name: "record update step missing id and filter",
			mutate: func(a *core.Automation) {
				a.SetSteps(mustParseJSONRaw(t, `[{"type":"record.update","collection":"demo2","data":{"title":"x"}}]`))
			},
			expectedField: "steps",
		},
		{
			name: "unsupported template root",
			mutate: func(a *core.Automation) {
				a.SetSteps(mustParseJSONRaw(t, `[{"type":"http","url":"https://example.com/{{unknown.id}}"}]`))
			},
			expectedField: "steps",
		},
		{
			name: "too many steps",
			mutate: func(a *core.Automation) {
				items := make([]string, core.AutomationMaxSteps+1)
				for i := range items {
					items[i] = `{"type":"condition","path":"trigger.type","op":"exists"}`
				}
				a.SetSteps(mustParseJSONRaw(t, `[`+strings.Join(items, ",")+`]`))
			},
			expectedField: "steps",
		},
		{
			name: "http timeout too large",
			mutate: func(a *core.Automation) {
				a.SetSteps(mustParseJSONRaw(t, `[{"type":"http","url":"https://example.com","timeout":61}]`))
			},
			expectedField: "steps",
		},
		{
			name: "inactive capability",
			mutate: func(a *core.Automation) {
				a.SetSteps(mustParseJSONRaw(t, `[{
					"type":"capability",
					"capability":"custom.inactive",
					"input":{"url":"https://example.com"}
				}]`))
			},
			expectedField: "steps",
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			automation := &core.Automation{}
			automation.SetProxyRecord(core.NewRecord(automationsCol))
			populateValidAutomation(automation)
			s.mutate(automation)

			err := app.Validate(automation)
			assertValidationField(t, err, s.expectedField)
		})
	}

	t.Run("valid record trigger", func(t *testing.T) {
		automation := &core.Automation{}
		automation.SetProxyRecord(core.NewRecord(automationsCol))
		populateValidAutomation(automation)
		automation.SetTriggerType(core.AutomationTriggerRecordUpdate)
		automation.SetCollectionRef(usersCol.Id)

		if err := app.Validate(automation); err != nil {
			t.Fatalf("Expected validation to succeed, got %v", err)
		}
	})

	t.Run("valid webhook trigger", func(t *testing.T) {
		automation := &core.Automation{}
		automation.SetProxyRecord(core.NewRecord(automationsCol))
		populateValidAutomation(automation)
		automation.SetTriggerType(core.AutomationTriggerWebhook)
		automation.SetSteps(mustParseJSONRaw(t, `[{"type":"condition","path":"request.method","op":"eq","value":"POST"}]`))

		if err := app.Validate(automation); err != nil {
			t.Fatalf("Expected validation to succeed, got %v", err)
		}
	})

	t.Run("valid mail step with record attachments", func(t *testing.T) {
		automation := &core.Automation{}
		automation.SetProxyRecord(core.NewRecord(automationsCol))
		populateValidAutomation(automation)
		automation.SetTriggerType(core.AutomationTriggerRecordCreate)
		automation.SetCollectionRef(demo1Col.Id)
		automation.SetSteps(mustParseJSONRaw(t, `[{
			"type":"mail.send",
			"to":["test@example.com"],
			"subject":"Order {{record.id}}",
			"text":"Hello",
			"attachments":["file_one","file_many"]
		}]`))

		if err := app.Validate(automation); err != nil {
			t.Fatalf("Expected validation to succeed, got %v", err)
		}
	})

	t.Run("valid custom capability step", func(t *testing.T) {
		automation := &core.Automation{}
		automation.SetProxyRecord(core.NewRecord(automationsCol))
		populateValidAutomation(automation)
		automation.SetSteps(mustParseJSONRaw(t, `[{
			"type":"capability",
			"capability":"custom.http",
			"input":{"url":"https://example.com"}
		}]`))

		if err := app.Validate(automation); err != nil {
			t.Fatalf("Expected validation to succeed, got %v", err)
		}
	})
}

func TestAutomationSchemas(t *testing.T) {
	t.Parallel()

	catalog := core.AutomationSchemas()

	if catalog.Limits.MaxSteps != core.AutomationMaxSteps {
		t.Fatalf("Expected max steps %d, got %d", core.AutomationMaxSteps, catalog.Limits.MaxSteps)
	}
	if catalog.Limits.HTTPMaxTimeoutSeconds != int(core.AutomationHTTPMaxTimeout.Seconds()) {
		t.Fatalf("Expected HTTP max timeout %d, got %d", int(core.AutomationHTTPMaxTimeout.Seconds()), catalog.Limits.HTTPMaxTimeoutSeconds)
	}
	if _, ok := catalog.Triggers[core.AutomationTriggerRecordCreate]; !ok {
		t.Fatalf("Expected %q trigger schema", core.AutomationTriggerRecordCreate)
	}
	if _, ok := catalog.Triggers[core.AutomationTriggerI18nMissing]; !ok {
		t.Fatalf("Expected %q trigger schema", core.AutomationTriggerI18nMissing)
	}
	if _, ok := catalog.Steps[core.AutomationStepHTTP]; !ok {
		t.Fatalf("Expected %q step schema", core.AutomationStepHTTP)
	}
	if _, ok := catalog.Steps[core.AutomationStepMailSend]; !ok {
		t.Fatalf("Expected %q step schema", core.AutomationStepMailSend)
	}
	if _, ok := catalog.Steps[core.AutomationStepCapability]; !ok {
		t.Fatalf("Expected %q step schema", core.AutomationStepCapability)
	}
	if _, ok := catalog.Capabilities[core.AutomationCapabilityHTTPRequest]; !ok {
		t.Fatalf("Expected %q capability schema", core.AutomationCapabilityHTTPRequest)
	}
}

func TestNewAutomationRun(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	run := core.NewAutomationRun(app)

	if run.Collection().Name != core.CollectionNameAutomationRuns {
		t.Fatalf("Expected record with %q collection, got %q", core.CollectionNameAutomationRuns, run.Collection().Name)
	}
}

func TestAutomationRunProxyRecord(t *testing.T) {
	t.Parallel()

	record := core.NewRecord(core.NewBaseCollection("test"))
	record.Id = "test_id"

	run := core.AutomationRun{}
	run.SetProxyRecord(record)

	if run.ProxyRecord() == nil || run.ProxyRecord().Id != record.Id {
		t.Fatalf("Expected proxy record with id %q, got %v", record.Id, run.ProxyRecord())
	}
}

func TestAutomationRunFields(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	run := core.NewAutomationRun(app)
	now := types.NowDateTime()
	input := mustParseJSONRaw(t, `{"record":{"id":"abc"}}`)
	stepResults := mustParseJSONRaw(t, `[{"ok":true}]`)

	run.SetAutomationRef("a1")
	run.SetTriggerType(core.AutomationTriggerManual)
	run.SetStatus(core.AutomationRunStatusRunning)
	run.SetInput(input)
	run.SetStepResults(stepResults)
	run.SetError("boom")
	run.SetErrorStepIndex(2)
	run.SetParentRunId("parent1")
	run.SetDepth(3)
	run.SetDedupeKey("dedupe1")
	policyDecision := mustParseJSONRaw(t, `{"allowed":true}`)
	run.SetPolicyDecision(policyDecision)
	run.SetRaw("started", now)
	run.SetRaw("finished", now)
	run.SetRaw("created", now)
	run.SetRaw("updated", now)

	if run.AutomationRef() != "a1" {
		t.Fatalf("Expected automationRef to roundtrip, got %q", run.AutomationRef())
	}
	if run.TriggerType() != core.AutomationTriggerManual {
		t.Fatalf("Expected triggerType to roundtrip, got %q", run.TriggerType())
	}
	if run.Status() != core.AutomationRunStatusRunning {
		t.Fatalf("Expected status to roundtrip, got %q", run.Status())
	}
	if run.Input().String() != input.String() {
		t.Fatalf("Expected input to roundtrip, got %s", run.Input())
	}
	if run.StepResults().String() != stepResults.String() {
		t.Fatalf("Expected stepResults to roundtrip, got %s", run.StepResults())
	}
	if run.Error() != "boom" {
		t.Fatalf("Expected error to roundtrip, got %q", run.Error())
	}
	if !run.HasErrorStepIndex() || run.ErrorStepIndex() != 2 {
		t.Fatalf("Expected errorStepIndex to roundtrip, got %v / %d", run.HasErrorStepIndex(), run.ErrorStepIndex())
	}
	if run.ParentRunId() != "parent1" {
		t.Fatalf("Expected parentRunId to roundtrip, got %q", run.ParentRunId())
	}
	if run.Depth() != 3 {
		t.Fatalf("Expected depth to roundtrip, got %d", run.Depth())
	}
	if run.DedupeKey() != "dedupe1" {
		t.Fatalf("Expected dedupeKey to roundtrip, got %q", run.DedupeKey())
	}
	if run.PolicyDecision().String() != policyDecision.String() {
		t.Fatalf("Expected policyDecision to roundtrip, got %s", run.PolicyDecision())
	}
	if run.Started().String() != now.String() {
		t.Fatalf("Expected started %q, got %q", now.String(), run.Started().String())
	}
	if run.Finished().String() != now.String() {
		t.Fatalf("Expected finished %q, got %q", now.String(), run.Finished().String())
	}
	if run.Created().String() != now.String() {
		t.Fatalf("Expected created %q, got %q", now.String(), run.Created().String())
	}
	if run.Updated().String() != now.String() {
		t.Fatalf("Expected updated %q, got %q", now.String(), run.Updated().String())
	}
}

func TestAutomationRunPreValidate(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	runsCol, err := app.FindCollectionByNameOrId(core.CollectionNameAutomationRuns)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("no proxy record", func(t *testing.T) {
		run := &core.AutomationRun{}

		if err := app.Validate(run); err == nil {
			t.Fatal("Expected collection validation error")
		}
	})

	t.Run("non-automation run collection", func(t *testing.T) {
		run := &core.AutomationRun{}
		run.SetProxyRecord(core.NewRecord(core.NewBaseCollection("invalid")))
		populateValidAutomationRun(t, app, run)

		if err := app.Validate(run); err == nil {
			t.Fatal("Expected collection validation error")
		}
	})

	t.Run("automation run collection", func(t *testing.T) {
		run := &core.AutomationRun{}
		run.SetProxyRecord(core.NewRecord(runsCol))
		populateValidAutomationRun(t, app, run)

		if err := app.Validate(run); err != nil {
			t.Fatalf("Expected validation to succeed, got %v", err)
		}
	})
}

func TestAutomationRunValidation(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	runsCol, err := app.FindCollectionByNameOrId(core.CollectionNameAutomationRuns)
	if err != nil {
		t.Fatal(err)
	}

	scenarios := []struct {
		name          string
		mutate        func(run *core.AutomationRun)
		expectedField string
	}{
		{
			name: "missing automation ref",
			mutate: func(run *core.AutomationRun) {
				run.SetAutomationRef("")
			},
			expectedField: "automationRef",
		},
		{
			name: "invalid automation ref",
			mutate: func(run *core.AutomationRun) {
				run.SetAutomationRef("missing")
			},
			expectedField: "automationRef",
		},
		{
			name: "invalid trigger type",
			mutate: func(run *core.AutomationRun) {
				run.SetTriggerType("bad.trigger")
			},
			expectedField: "triggerType",
		},
		{
			name: "invalid status",
			mutate: func(run *core.AutomationRun) {
				run.SetStatus("done-ish")
			},
			expectedField: "status",
		},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			run := &core.AutomationRun{}
			run.SetProxyRecord(core.NewRecord(runsCol))
			populateValidAutomationRun(t, app, run)
			s.mutate(run)

			err := app.Validate(run)
			assertValidationField(t, err, s.expectedField)
		})
	}
}

func populateValidAutomation(automation *core.Automation) {
	automation.SetName("Test automation")
	automation.SetTriggerType(core.AutomationTriggerManual)
	automation.SetSteps(mustParseJSONRaw(nil, `[{"type":"condition","path":"trigger.type","op":"exists"}]`))
}

func populateValidAutomationRun(t *testing.T, app core.App, run *core.AutomationRun) {
	t.Helper()

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	if err := app.Save(automation); err != nil {
		t.Fatalf("Failed to create automation fixture: %v", err)
	}

	run.SetAutomationRef(automation.Id)
	run.SetTriggerType(core.AutomationTriggerManual)
	run.SetStatus(core.AutomationRunStatusQueued)
}

func populateValidCapability(capability *core.Capability) {
	capability.SetKey("test.capability")
	capability.SetVersion("1.0.0")
	capability.SetCategory("test")
	capability.SetInputSchema(mustParseJSONRaw(nil, `{"type":"object"}`))
	capability.SetOutputSchema(mustParseJSONRaw(nil, `{"type":"object"}`))
	capability.SetRuntimeHandler(core.AutomationStepHTTP)
	capability.SetActive(true)
}

func assertValidationField(t *testing.T, err error, field string) {
	t.Helper()

	if err == nil {
		t.Fatal("Expected validation error")
	}

	var validationErrs validation.Errors
	if !errors.As(err, &validationErrs) {
		t.Fatalf("Expected validation.Errors, got %T (%v)", err, err)
	}

	if validationErrs[field] == nil {
		t.Fatalf("Expected validation error for field %q, got %v", field, validationErrs)
	}
}

func mustParseJSONRaw(t *testing.T, raw string) types.JSONRaw {
	if raw == "" {
		raw = "null"
	}

	parsed, err := types.ParseJSONRaw(raw)
	if err != nil {
		if t != nil {
			t.Fatalf("Failed to parse JSON raw %q: %v", raw, err)
		}
		panic(fmt.Sprintf("failed to parse JSON raw %q: %v", raw, err))
	}

	return parsed
}
