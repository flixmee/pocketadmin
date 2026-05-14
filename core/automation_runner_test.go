package core_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/cron"
	"github.com/pocketbase/pocketbase/tools/filesystem"
	"github.com/pocketbase/pocketbase/tools/types"
)

func TestAutomationCronSyncOnSave(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetActive(true)
	automation.SetTriggerType(core.AutomationTriggerScheduleCron)
	automation.SetCronExpr("0 * * * *")

	if err := app.Save(automation); err != nil {
		t.Fatal(err)
	}

	job := findAutomationCronJob(app, automation.Id)
	if job == nil {
		t.Fatal("Expected automation cron job to be registered")
	}

	if job.Expression() != "0 * * * *" {
		t.Fatalf("Expected cron expression to be registered, got %q", job.Expression())
	}

	automation.SetActive(false)
	if err := app.Save(automation); err != nil {
		t.Fatal(err)
	}

	if findAutomationCronJob(app, automation.Id) != nil {
		t.Fatal("Expected automation cron job to be removed after deactivation")
	}
}

func TestAutomationCronRunCreatesRunLog(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetActive(true)
	automation.SetTriggerType(core.AutomationTriggerScheduleCron)
	automation.SetCronExpr("0 * * * *")

	if err := app.Save(automation); err != nil {
		t.Fatal(err)
	}

	job := findAutomationCronJob(app, automation.Id)
	if job == nil {
		t.Fatal("Expected automation cron job to be registered")
	}

	job.Run()

	runs := waitForCompletedAutomationRuns(t, app, automation, 1)
	if runs[0].TriggerType() != core.AutomationTriggerScheduleCron {
		t.Fatalf("Expected trigger type %q, got %q", core.AutomationTriggerScheduleCron, runs[0].TriggerType())
	}
	if runs[0].Status() != core.AutomationRunStatusSuccess {
		t.Fatalf("Expected successful run, got %q", runs[0].Status())
	}

	latestAutomation, err := app.FindAutomationById(automation.Id)
	if err != nil {
		t.Fatal(err)
	}
	if latestAutomation.LastRunStatus() != core.AutomationRunStatusSuccess {
		t.Fatalf("Expected lastRunStatus %q, got %q", core.AutomationRunStatusSuccess, latestAutomation.LastRunStatus())
	}
	if latestAutomation.LastRunAt().IsZero() {
		t.Fatal("Expected lastRunAt to be set")
	}
}

func TestAutomationRecordTriggersCreateRunLogs(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	collection, err := app.FindCollectionByNameOrId("demo2")
	if err != nil {
		t.Fatal(err)
	}

	createAutomation := newRecordTriggerAutomation(t, app, collection.Id, core.AutomationTriggerRecordCreate)
	updateAutomation := newRecordTriggerAutomation(t, app, collection.Id, core.AutomationTriggerRecordUpdate)
	deleteAutomation := newRecordTriggerAutomation(t, app, collection.Id, core.AutomationTriggerRecordDelete)

	record := core.NewRecord(collection)
	record.Set("title", "automation_create")
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}

	waitForAutomationRuns(t, app, createAutomation, 1)

	record.Set("title", "automation_update")
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}

	waitForAutomationRuns(t, app, updateAutomation, 1)

	if err := app.Delete(record); err != nil {
		t.Fatal(err)
	}

	waitForAutomationRuns(t, app, deleteAutomation, 1)
}

func TestAutomationI18nTriggersCreateRunLogs(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	locale := core.NewLocale(app)
	locale.SetCode("vi")
	locale.SetName("Vietnamese")
	locale.SetEnabled(true)
	if err := app.Save(locale); err != nil {
		t.Fatal(err)
	}

	collection := core.NewBaseCollection("automation_i18n_posts")
	collection.Fields.Add(&core.TextField{Name: "title"})
	collection.I18n.Enabled = true
	collection.I18n.DefaultLocale = core.DefaultLocaleCode
	collection.I18n.LocalizedFields = []string{"title"}
	if err := app.Save(collection); err != nil {
		t.Fatal(err)
	}

	missingAutomation := newRecordTriggerAutomation(t, app, collection.Id, core.AutomationTriggerI18nMissing)

	record := core.NewRecord(collection)
	record.Set("title", "Missing vi")
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}

	runs := waitForCompletedAutomationRuns(t, app, missingAutomation, 1)
	if runs[0].TriggerType() != core.AutomationTriggerI18nMissing {
		t.Fatalf("Expected i18n missing trigger, got %q", runs[0].TriggerType())
	}
	if !strings.Contains(runs[0].Input().String(), `"missingLocales":["vi"]`) {
		t.Fatalf("Expected missing locale payload, got %s", runs[0].Input().String())
	}
}

func TestAutomationI18nAIFinishedTriggerFromTranslationJob(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	locale := core.NewLocale(app)
	locale.SetCode("vi")
	locale.SetName("Vietnamese")
	locale.SetEnabled(true)
	if err := app.Save(locale); err != nil {
		t.Fatal(err)
	}

	collection := core.NewBaseCollection("automation_i18n_jobs")
	collection.Fields.Add(&core.TextField{Name: "title"})
	collection.I18n.Enabled = true
	collection.I18n.DefaultLocale = core.DefaultLocaleCode
	collection.I18n.LocalizedFields = []string{"title"}
	if err := app.Save(collection); err != nil {
		t.Fatal(err)
	}

	automation := newRecordTriggerAutomation(t, app, collection.Id, core.AutomationTriggerI18nAIFinished)

	job := core.NewTranslationJob(app)
	job.SetCollectionRef(collection.Id)
	job.SetSourceRecordId("source123")
	job.SetSourceLocale(core.DefaultLocaleCode)
	job.SetTargetLocale("vi")
	job.SetStatus(core.TranslationJobStatusPending)
	if err := app.Save(job); err != nil {
		t.Fatal(err)
	}

	job.SetStatus(core.TranslationJobStatusFinished)
	if err := app.Save(job); err != nil {
		t.Fatal(err)
	}

	runs := waitForCompletedAutomationRuns(t, app, automation, 1)
	if runs[0].TriggerType() != core.AutomationTriggerI18nAIFinished {
		t.Fatalf("Expected AI finished trigger, got %q", runs[0].TriggerType())
	}
	if !strings.Contains(runs[0].Input().String(), `"translationJobId":"`+job.Id+`"`) {
		t.Fatalf("Expected translation job payload, got %s", runs[0].Input().String())
	}
}

func TestAutomationRecordTriggerSkippedOnRollback(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	collection, err := app.FindCollectionByNameOrId("demo2")
	if err != nil {
		t.Fatal(err)
	}

	automation := newRecordTriggerAutomation(t, app, collection.Id, core.AutomationTriggerRecordCreate)

	txErr := app.RunInTransaction(func(txApp core.App) error {
		record := core.NewRecord(collection)
		record.Set("title", "automation_rollback")

		if err := txApp.Save(record); err != nil {
			return err
		}

		return errors.New("rollback")
	})
	if txErr == nil {
		t.Fatal("Expected transaction rollback error")
	}

	time.Sleep(200 * time.Millisecond)

	runs, err := app.FindAllAutomationRunsByAutomation(automation)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 0 {
		t.Fatalf("Expected no runs after rollback, got %d", len(runs))
	}
}

func TestAutomationConditionStepStopsRemainingSteps(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	httpCalls := 0
	app.Store().Set(core.StoreKeyAutomationHTTPDoer, automationHTTPDoerFunc(func(req *http.Request) (*http.Response, error) {
		httpCalls++
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	}))

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetActive(true)
	automation.SetTriggerType(core.AutomationTriggerScheduleCron)
	automation.SetCronExpr("0 * * * *")
	automation.SetSteps(mustParseJSONRaw(t, `[
		{"type":"condition","path":"record.id","op":"exists"},
		{"type":"http","url":"https://example.com/hooks"}
	]`))

	if err := app.Save(automation); err != nil {
		t.Fatal(err)
	}

	job := findAutomationCronJob(app, automation.Id)
	if job == nil {
		t.Fatal("Expected automation cron job to be registered")
	}

	job.Run()

	runs := waitForCompletedAutomationRuns(t, app, automation, 1)
	if runs[0].Status() != core.AutomationRunStatusSuccess {
		t.Fatalf("Expected successful run, got %q", runs[0].Status())
	}
	if httpCalls != 0 {
		t.Fatalf("Expected condition step to stop the workflow before HTTP execution, got %d HTTP calls", httpCalls)
	}

	results := decodeStepResults(t, runs[0])
	if len(results) != 1 {
		t.Fatalf("Expected 1 recorded step result, got %d", len(results))
	}
	if results[0]["status"] != "stopped" {
		t.Fatalf("Expected stopped condition step, got %v", results[0]["status"])
	}
}

func TestAutomationConditionStringOperators(t *testing.T) {
	t.Parallel()

	scenarios := []struct {
		name               string
		op                 string
		expectedValue      string
		recordTitle        string
		expectHTTPCalls    int
		expectedStepStatus string
	}{
		{
			name:               "starts with",
			op:                 "startsWith",
			expectedValue:      "phase3",
			recordTitle:        "phase3_condition_title",
			expectHTTPCalls:    1,
			expectedStepStatus: "success",
		},
		{
			name:               "ends with",
			op:                 "endsWith",
			expectedValue:      "title",
			recordTitle:        "phase3_condition_title",
			expectHTTPCalls:    1,
			expectedStepStatus: "success",
		},
		{
			name:               "does not start with",
			op:                 "notStartsWith",
			expectedValue:      "other",
			recordTitle:        "phase3_condition_title",
			expectHTTPCalls:    1,
			expectedStepStatus: "success",
		},
		{
			name:               "does not end with",
			op:                 "notEndsWith",
			expectedValue:      "other",
			recordTitle:        "phase3_condition_title",
			expectHTTPCalls:    1,
			expectedStepStatus: "success",
		},
		{
			name:               "contains",
			op:                 "contains",
			expectedValue:      "condition",
			recordTitle:        "phase3_condition_title",
			expectHTTPCalls:    1,
			expectedStepStatus: "success",
		},
		{
			name:               "starts with mismatch stops workflow",
			op:                 "startsWith",
			expectedValue:      "other",
			recordTitle:        "phase3_condition_title",
			expectHTTPCalls:    0,
			expectedStepStatus: "stopped",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()

			app, _ := tests.NewTestApp()
			defer app.Cleanup()

			httpCalls := 0
			app.Store().Set(core.StoreKeyAutomationHTTPDoer, automationHTTPDoerFunc(func(req *http.Request) (*http.Response, error) {
				httpCalls++
				return &http.Response{
					StatusCode: http.StatusNoContent,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
				}, nil
			}))

			collection, err := app.FindCollectionByNameOrId("demo2")
			if err != nil {
				t.Fatal(err)
			}

			automation := core.NewAutomation(app)
			populateValidAutomation(automation)
			automation.SetActive(true)
			automation.SetTriggerType(core.AutomationTriggerRecordCreate)
			automation.SetCollectionRef(collection.Id)
			automation.SetSteps(mustParseJSONRaw(t, fmt.Sprintf(`[
				{"type":"condition","path":"record.title","op":"%s","value":%q},
				{"type":"http","url":"https://example.com/hooks"}
			]`, scenario.op, scenario.expectedValue)))

			if err := app.Save(automation); err != nil {
				t.Fatal(err)
			}

			record := core.NewRecord(collection)
			record.Set("title", scenario.recordTitle)
			if err := app.Save(record); err != nil {
				t.Fatal(err)
			}

			runs := waitForCompletedAutomationRuns(t, app, automation, 1)
			if runs[0].Status() != core.AutomationRunStatusSuccess {
				t.Fatalf("Expected successful run, got %q", runs[0].Status())
			}
			if httpCalls != scenario.expectHTTPCalls {
				t.Fatalf("Expected %d HTTP calls, got %d", scenario.expectHTTPCalls, httpCalls)
			}

			results := decodeStepResults(t, runs[0])
			if len(results) == 0 {
				t.Fatal("Expected at least one step result")
			}
			if results[0]["status"] != scenario.expectedStepStatus {
				t.Fatalf("Expected first step status %q, got %v", scenario.expectedStepStatus, results[0]["status"])
			}
		})
	}
}

func TestAutomationWebhookRunExposesRequestTemplateData(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	gotMethod := ""
	gotURL := ""
	gotHeader := ""
	gotBody := ""
	httpCalls := 0

	app.Store().Set(core.StoreKeyAutomationHTTPDoer, automationHTTPDoerFunc(func(req *http.Request) (*http.Response, error) {
		httpCalls++
		gotMethod = req.Method
		gotURL = req.URL.String()
		gotHeader = req.Header.Get("X-Source-Ip")

		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		gotBody = string(body)

		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	}))

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetActive(true)
	automation.SetTriggerType(core.AutomationTriggerWebhook)
	automation.SetSteps(mustParseJSONRaw(t, `[
		{"type":"condition","path":"request.headers.x_automation_event","op":"eq","value":"invoice.paid"},
		{
			"type":"http",
			"method":"POST",
			"url":"https://example.com/hooks/{{request.query.tenant}}",
			"headers":{"X-Source-Ip":"{{request.remoteIP}}"},
			"body":{"trigger":"{{trigger.type}}","event":"{{request.body.event}}","path":"{{request.path}}"}
		}
	]`))

	if err := app.Save(automation); err != nil {
		t.Fatal(err)
	}

	_, err := app.RunAutomationWebhook(automation.Id, &core.AutomationWebhookRequest{
		Method:   http.MethodPost,
		Path:     "/api/automation-webhooks/" + automation.Id,
		Query:    map[string]string{"tenant": "acme"},
		Headers:  map[string]string{"X-Automation-Event": "invoice.paid"},
		Body:     map[string]any{"event": "invoice.paid"},
		RemoteIP: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("Expected webhook run to succeed, got %v", err)
	}

	runs := waitForCompletedAutomationRuns(t, app, automation, 1)
	if runs[0].Status() != core.AutomationRunStatusSuccess {
		t.Fatalf("Expected successful run, got %q", runs[0].Status())
	}
	if httpCalls != 1 {
		t.Fatalf("Expected 1 HTTP call, got %d", httpCalls)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("Expected HTTP method %q, got %q", http.MethodPost, gotMethod)
	}
	if gotURL != "https://example.com/hooks/acme" {
		t.Fatalf("Expected rendered URL, got %q", gotURL)
	}
	if gotHeader != "127.0.0.1" {
		t.Fatalf("Expected rendered request header, got %q", gotHeader)
	}
	if !strings.Contains(gotBody, `"trigger":"webhook"`) || !strings.Contains(gotBody, `"event":"invoice.paid"`) {
		t.Fatalf("Expected rendered request body, got %q", gotBody)
	}

	input := decodeAutomationRunInput(t, runs[0])
	request, ok := input["request"].(map[string]any)
	if !ok {
		t.Fatalf("Expected request payload in run input, got %#v", input["request"])
	}
	if request["method"] != http.MethodPost {
		t.Fatalf("Expected request method %q, got %#v", http.MethodPost, request["method"])
	}
	if request["path"] != "/api/automation-webhooks/"+automation.Id {
		t.Fatalf("Expected request path in run input, got %#v", request["path"])
	}
}

func TestAutomationRunReplayUsesStoredWebhookPayload(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	httpCalls := 0
	gotMethod := ""
	gotURL := ""

	app.Store().Set(core.StoreKeyAutomationHTTPDoer, automationHTTPDoerFunc(func(req *http.Request) (*http.Response, error) {
		httpCalls++
		gotMethod = req.Method
		gotURL = req.URL.String()

		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	}))

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetActive(true)
	automation.SetTriggerType(core.AutomationTriggerWebhook)
	automation.SetSteps(mustParseJSONRaw(t, `[
		{"type":"condition","path":"request.headers.x_automation_event","op":"eq","value":"invoice.paid"},
		{"type":"http","method":"POST","url":"https://example.com/hooks/{{request.query.tenant}}"}
	]`))

	if err := app.Save(automation); err != nil {
		t.Fatal(err)
	}

	if _, err := app.RunAutomationWebhook(automation.Id, &core.AutomationWebhookRequest{
		Method:   http.MethodPost,
		Path:     "/api/automation-webhooks/" + automation.Id,
		Query:    map[string]string{"tenant": "acme"},
		Headers:  map[string]string{"X-Automation-Event": "invoice.paid"},
		Body:     map[string]any{"event": "invoice.paid"},
		RemoteIP: "127.0.0.1",
	}); err != nil {
		t.Fatalf("Expected webhook run to succeed, got %v", err)
	}

	originalRuns := waitForCompletedAutomationRuns(t, app, automation, 1)
	originalRun := originalRuns[0]

	if err := app.RunAutomationFromRun(originalRun.Id); err != nil {
		t.Fatalf("Expected replayed run to succeed, got %v", err)
	}

	runs := waitForCompletedAutomationRuns(t, app, automation, 2)
	replayedRun := runs[0]
	if replayedRun.Id == originalRun.Id {
		t.Fatalf("Expected a new run id, got the original %q", replayedRun.Id)
	}
	if replayedRun.TriggerType() != core.AutomationTriggerWebhook {
		t.Fatalf("Expected replay trigger type %q, got %q", core.AutomationTriggerWebhook, replayedRun.TriggerType())
	}
	if replayedRun.Status() != core.AutomationRunStatusSuccess {
		t.Fatalf("Expected successful replay run, got %q", replayedRun.Status())
	}
	if httpCalls != 2 {
		t.Fatalf("Expected 2 HTTP calls across original and replayed runs, got %d", httpCalls)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("Expected replayed HTTP method %q, got %q", http.MethodPost, gotMethod)
	}
	if gotURL != "https://example.com/hooks/acme" {
		t.Fatalf("Expected replayed URL to preserve request query data, got %q", gotURL)
	}

	input := decodeAutomationRunInput(t, replayedRun)
	request, ok := input["request"].(map[string]any)
	if !ok {
		t.Fatalf("Expected request payload in replayed run input, got %#v", input["request"])
	}
	if request["path"] != "/api/automation-webhooks/"+automation.Id {
		t.Fatalf("Expected replayed request path, got %#v", request["path"])
	}
}

func TestAutomationHTTPStepExecutesRequest(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	gotMethod := ""
	gotURL := ""
	gotHeader := ""
	gotBody := ""

	app.Store().Set(core.StoreKeyAutomationHTTPDoer, automationHTTPDoerFunc(func(req *http.Request) (*http.Response, error) {
		gotMethod = req.Method
		gotURL = req.URL.String()
		gotHeader = req.Header.Get("X-Automation")

		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		gotBody = string(body)

		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	}))

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetName("phase3_http")
	automation.SetActive(true)
	automation.SetTriggerType(core.AutomationTriggerScheduleCron)
	automation.SetCronExpr("0 * * * *")
	automation.SetSteps(mustParseJSONRaw(t, `[
		{
			"type":"http",
			"method":"post",
			"url":"https://example.com/hooks",
			"headers":{"X-Automation":"{{trigger.type}}"},
			"body":{"name":"{{automation.name}}","trigger":"{{trigger.type}}"}
		}
	]`))

	if err := app.Save(automation); err != nil {
		t.Fatal(err)
	}

	job := findAutomationCronJob(app, automation.Id)
	if job == nil {
		t.Fatal("Expected automation cron job to be registered")
	}

	job.Run()

	runs := waitForCompletedAutomationRuns(t, app, automation, 1)
	if runs[0].Status() != core.AutomationRunStatusSuccess {
		t.Fatalf("Expected successful run, got %q", runs[0].Status())
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("Expected POST request, got %q", gotMethod)
	}
	if gotURL != "https://example.com/hooks" {
		t.Fatalf("Expected request url to roundtrip, got %q", gotURL)
	}
	if gotHeader != core.AutomationTriggerScheduleCron {
		t.Fatalf("Expected trigger header %q, got %q", core.AutomationTriggerScheduleCron, gotHeader)
	}
	if !strings.Contains(gotBody, `"name":"phase3_http"`) || !strings.Contains(gotBody, `"trigger":"schedule.cron"`) {
		t.Fatalf("Expected rendered JSON body, got %q", gotBody)
	}
}

func TestAutomationMailStepSendsMessageWithRecordAttachments(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	collection, err := app.FindCollectionByNameOrId("demo1")
	if err != nil {
		t.Fatal(err)
	}

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetActive(true)
	automation.SetTriggerType(core.AutomationTriggerRecordCreate)
	automation.SetCollectionRef(collection.Id)
	automation.SetSteps(mustParseJSONRaw(t, `[
		{
			"type":"mail.send",
			"to":["customer@example.com","{{record.email}}"],
			"cc":["ops@example.com"],
			"subject":"Files for {{record.text}}",
			"text":"Record {{record.id}} attached",
			"attachments":["file_one","file_many"]
		}
	]`))

	if err := app.Save(automation); err != nil {
		t.Fatal(err)
	}

	fileOne, err := filesystem.NewFileFromBytes([]byte("file one content"), "invoice.txt")
	if err != nil {
		t.Fatal(err)
	}
	fileTwo, err := filesystem.NewFileFromBytes([]byte("file two content"), "terms.txt")
	if err != nil {
		t.Fatal(err)
	}

	record := core.NewRecord(collection)
	record.Set("text", "phase_mail")
	record.Set("email", "record@example.com")
	record.Set("file_one", fileOne)
	record.Set("file_many", []any{fileTwo})
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}

	runs := waitForCompletedAutomationRuns(t, app, automation, 1)
	if runs[0].Status() != core.AutomationRunStatusSuccess {
		t.Fatalf("Expected successful run, got %q", runs[0].Status())
	}

	message := app.TestMailer.LastMessage()
	if len(message.To) != 2 {
		t.Fatalf("Expected 2 recipients, got %d", len(message.To))
	}
	if message.To[0].Address != "customer@example.com" || message.To[1].Address != "record@example.com" {
		t.Fatalf("Unexpected recipients: %#v", message.To)
	}
	if len(message.Cc) != 1 || message.Cc[0].Address != "ops@example.com" {
		t.Fatalf("Unexpected cc recipients: %#v", message.Cc)
	}
	if message.Subject != "Files for phase_mail" {
		t.Fatalf("Expected rendered subject, got %q", message.Subject)
	}
	if message.Text != "Record "+record.Id+" attached" {
		t.Fatalf("Expected rendered text body, got %q", message.Text)
	}
	if message.From.Address != app.Settings().Meta.SenderAddress {
		t.Fatalf("Expected default sender address %q, got %q", app.Settings().Meta.SenderAddress, message.From.Address)
	}
	if len(message.Attachments) != 2 {
		t.Fatalf("Expected 2 attachments, got %d", len(message.Attachments))
	}

	attachmentContent := map[string]string{}
	for name, reader := range message.Attachments {
		content, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("Failed to read attachment %q: %v", name, err)
		}
		attachmentContent[name] = string(content)
	}

	if attachmentContent["invoice.txt"] != "file one content" {
		t.Fatalf("Expected invoice attachment content, got %#v", attachmentContent)
	}
	if attachmentContent["terms.txt"] != "file two content" {
		t.Fatalf("Expected terms attachment content, got %#v", attachmentContent)
	}
}

func TestAutomationFailedRunStoresErrorStepIndexAndTiming(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	app.Store().Set(core.StoreKeyAutomationHTTPDoer, automationHTTPDoerFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader("upstream failure")),
			Header:     make(http.Header),
		}, nil
	}))

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetActive(true)
	automation.SetTriggerType(core.AutomationTriggerScheduleCron)
	automation.SetCronExpr("0 * * * *")
	automation.SetSteps(mustParseJSONRaw(t, `[{"type":"http","url":"https://example.com/fail"}]`))

	if err := app.Save(automation); err != nil {
		t.Fatal(err)
	}

	job := findAutomationCronJob(app, automation.Id)
	if job == nil {
		t.Fatal("Expected automation cron job to be registered")
	}

	job.Run()

	runs := waitForCompletedAutomationRuns(t, app, automation, 1)
	run := runs[0]
	if run.Status() != core.AutomationRunStatusFailed {
		t.Fatalf("Expected failed run, got %q", run.Status())
	}
	if run.Error() == "" {
		t.Fatal("Expected failed run to store an error")
	}
	if !run.HasErrorStepIndex() || run.ErrorStepIndex() != 0 {
		t.Fatalf("Expected failed run errorStepIndex 0, got has=%v value=%d", run.HasErrorStepIndex(), run.ErrorStepIndex())
	}

	results := decodeStepResults(t, run)
	if len(results) != 1 {
		t.Fatalf("Expected 1 step result, got %d", len(results))
	}
	if results[0]["status"] != "failed" {
		t.Fatalf("Expected failed step status, got %v", results[0]["status"])
	}
	if results[0]["started"] == "" || results[0]["finished"] == "" {
		t.Fatalf("Expected started and finished step timestamps, got %v", results[0])
	}
	if duration, ok := results[0]["durationMs"].(float64); !ok || duration < 0 {
		t.Fatalf("Expected non-negative durationMs, got %v", results[0]["durationMs"])
	}
}

func TestAutomationRecordCreateStepUsesTriggerTemplates(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	sourceCollection, err := app.FindCollectionByNameOrId("demo2")
	if err != nil {
		t.Fatal(err)
	}

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetActive(true)
	automation.SetTriggerType(core.AutomationTriggerRecordCreate)
	automation.SetCollectionRef(sourceCollection.Id)
	automation.SetSteps(mustParseJSONRaw(t, `[
		{
			"type":"record.create",
			"collection":"demo1",
			"data":{"text":"copied_{{record.title}}"}
		}
	]`))

	if err := app.Save(automation); err != nil {
		t.Fatal(err)
	}

	sourceRecord := core.NewRecord(sourceCollection)
	sourceRecord.Set("title", "phase3_create_template")
	if err := app.Save(sourceRecord); err != nil {
		t.Fatal(err)
	}

	runs := waitForCompletedAutomationRuns(t, app, automation, 1)
	if runs[0].Status() != core.AutomationRunStatusSuccess {
		t.Fatalf("Expected successful run, got %q", runs[0].Status())
	}

	createdRecord, err := app.FindFirstRecordByData("demo1", "text", "copied_phase3_create_template")
	if err != nil {
		t.Fatalf("Expected record.create step to persist target record: %v", err)
	}
	if createdRecord.GetString("text") != "copied_phase3_create_template" {
		t.Fatalf("Expected created record text to match template, got %q", createdRecord.GetString("text"))
	}
}

func TestAutomationTemplatesEvaluateJavaScriptExpressions(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	sourceCollection, err := app.FindCollectionByNameOrId("demo2")
	if err != nil {
		t.Fatal(err)
	}

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetActive(true)
	automation.SetTriggerType(core.AutomationTriggerRecordCreate)
	automation.SetCollectionRef(sourceCollection.Id)
	automation.SetSteps(mustParseJSONRaw(t, `[
		{
			"type":"record.create",
			"collection":"demo1",
			"data":{"text":"prefix_{{ record.title.replace(\"a\", \"\") }}"}
		}
	]`))

	if err := app.Save(automation); err != nil {
		t.Fatal(err)
	}

	sourceRecord := core.NewRecord(sourceCollection)
	sourceRecord.Set("title", "banana")
	if err := app.Save(sourceRecord); err != nil {
		t.Fatal(err)
	}

	runs := waitForCompletedAutomationRuns(t, app, automation, 1)
	if runs[0].Status() != core.AutomationRunStatusSuccess {
		t.Fatalf("Expected successful run, got %q", runs[0].Status())
	}

	createdRecord, err := app.FindFirstRecordByData("demo1", "text", "prefix_bnana")
	if err != nil {
		t.Fatalf("Expected record.create step to persist JS template result: %v", err)
	}
	if createdRecord.GetString("text") != "prefix_bnana" {
		t.Fatalf("Expected interpolated JS template result, got %q", createdRecord.GetString("text"))
	}
}

func TestAutomationStepsExposePreviousOutputsToTemplates(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	sourceCollection, err := app.FindCollectionByNameOrId("demo2")
	if err != nil {
		t.Fatal(err)
	}

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetActive(true)
	automation.SetTriggerType(core.AutomationTriggerRecordCreate)
	automation.SetCollectionRef(sourceCollection.Id)
	automation.SetSteps(mustParseJSONRaw(t, `[
		{
			"type":"record.create",
			"collection":"demo1",
			"data":{"text":"first_{{record.title}}"}
		},
		{
			"type":"record.create",
			"collection":"demo1",
			"data":{"text":"second_{{steps[0].output.text}}_{{prevStep.output.id.length > 0}}"}
		}
	]`))

	if err := app.Save(automation); err != nil {
		t.Fatal(err)
	}

	sourceRecord := core.NewRecord(sourceCollection)
	sourceRecord.Set("title", "previous_output")
	if err := app.Save(sourceRecord); err != nil {
		t.Fatal(err)
	}

	runs := waitForCompletedAutomationRuns(t, app, automation, 1)
	if runs[0].Status() != core.AutomationRunStatusSuccess {
		t.Fatalf("Expected successful run, got %q", runs[0].Status())
	}

	createdRecord, err := app.FindFirstRecordByData("demo1", "text", "second_first_previous_output_true")
	if err != nil {
		t.Fatalf("Expected second record.create step to use previous step output: %v", err)
	}
	if createdRecord.GetString("text") != "second_first_previous_output_true" {
		t.Fatalf("Expected chained template result, got %q", createdRecord.GetString("text"))
	}

	results := decodeStepResults(t, runs[0])
	if len(results) != 2 {
		t.Fatalf("Expected 2 step results, got %d", len(results))
	}
	output, ok := results[0]["output"].(map[string]any)
	if !ok || output["text"] != "first_previous_output" {
		t.Fatalf("Expected first step output to be recorded, got %v", results[0]["output"])
	}
}

func TestAutomationRecordUpdateStep(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	collection, err := app.FindCollectionByNameOrId("demo2")
	if err != nil {
		t.Fatal(err)
	}

	target := core.NewRecord(collection)
	target.Set("title", "phase3_update_source")
	if err := app.Save(target); err != nil {
		t.Fatal(err)
	}

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetName("phase3_update")
	automation.SetActive(true)
	automation.SetTriggerType(core.AutomationTriggerScheduleCron)
	automation.SetCronExpr("0 * * * *")
	automation.SetSteps(mustParseJSONRaw(t, `[{
		"type":"record.update",
		"collection":"demo2",
		"id":"`+target.Id+`",
		"data":{"title":"updated_{{automation.name}}"}
	}]`))

	if err := app.Save(automation); err != nil {
		t.Fatal(err)
	}

	job := findAutomationCronJob(app, automation.Id)
	if job == nil {
		t.Fatal("Expected automation cron job to be registered")
	}

	job.Run()

	runs := waitForCompletedAutomationRuns(t, app, automation, 1)
	if runs[0].Status() != core.AutomationRunStatusSuccess {
		t.Fatalf("Expected successful run, got %q", runs[0].Status())
	}

	updated, err := app.FindRecordById(collection, target.Id)
	if err != nil {
		t.Fatal(err)
	}
	if updated.GetString("title") != "updated_phase3_update" {
		t.Fatalf("Expected updated record title, got %q", updated.GetString("title"))
	}
}

func TestAutomationRecordDeleteStep(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	collection, err := app.FindCollectionByNameOrId("demo2")
	if err != nil {
		t.Fatal(err)
	}

	target := core.NewRecord(collection)
	target.Set("title", "phase3_delete_source")
	if err := app.Save(target); err != nil {
		t.Fatal(err)
	}

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetActive(true)
	automation.SetTriggerType(core.AutomationTriggerScheduleCron)
	automation.SetCronExpr("0 * * * *")
	automation.SetSteps(mustParseJSONRaw(t, `[{
		"type":"record.delete",
		"collection":"demo2",
		"id":"`+target.Id+`"
	}]`))

	if err := app.Save(automation); err != nil {
		t.Fatal(err)
	}

	job := findAutomationCronJob(app, automation.Id)
	if job == nil {
		t.Fatal("Expected automation cron job to be registered")
	}

	job.Run()

	runs := waitForCompletedAutomationRuns(t, app, automation, 1)
	if runs[0].Status() != core.AutomationRunStatusSuccess {
		t.Fatalf("Expected successful run, got %q", runs[0].Status())
	}

	if _, err := app.FindRecordById(collection, target.Id); err == nil {
		t.Fatal("Expected record.delete step to remove the target record")
	}
}

func newRecordTriggerAutomation(t *testing.T, app *tests.TestApp, collectionId string, triggerType string) *core.Automation {
	t.Helper()

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetActive(true)
	automation.SetTriggerType(triggerType)
	automation.SetCollectionRef(collectionId)

	if err := app.Save(automation); err != nil {
		t.Fatalf("Failed to create record trigger automation: %v", err)
	}

	return automation
}

func waitForAutomationRuns(t *testing.T, app *tests.TestApp, automation *core.Automation, expected int) []*core.AutomationRun {
	t.Helper()

	var runs []*core.AutomationRun
	var err error

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runs, err = app.FindAllAutomationRunsByAutomation(automation)
		if err == nil && len(runs) >= expected {
			return runs
		}
		if err == nil && expected == 0 && len(runs) == 0 {
			return runs
		}

		time.Sleep(50 * time.Millisecond)
	}

	if err != nil {
		t.Fatalf("Failed to fetch automation runs: %v", err)
	}

	t.Fatalf("Expected at least %d automation runs, got %d", expected, len(runs))
	return nil
}

func waitForCompletedAutomationRuns(t *testing.T, app *tests.TestApp, automation *core.Automation, expected int) []*core.AutomationRun {
	t.Helper()

	var runs []*core.AutomationRun
	var err error

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runs, err = app.FindAllAutomationRunsByAutomation(automation)
		if err == nil && len(runs) >= expected && hasCompletedAutomationRun(runs, expected) {
			return runs
		}

		time.Sleep(50 * time.Millisecond)
	}

	if err != nil {
		t.Fatalf("Failed to fetch automation runs: %v", err)
	}

	t.Fatalf("Expected at least %d completed automation runs, got %d", expected, len(runs))
	return nil
}

func hasCompletedAutomationRun(runs []*core.AutomationRun, expected int) bool {
	if expected <= 0 {
		return true
	}

	limit := min(expected, len(runs))
	for i := 0; i < limit; i++ {
		status := runs[i].Status()
		if status == core.AutomationRunStatusSuccess || status == core.AutomationRunStatusFailed {
			return true
		}
	}

	return false
}

func findAutomationCronJob(app *tests.TestApp, automationID string) *cron.Job {
	for _, job := range app.Cron().Jobs() {
		if job.Id() == "__pbAutomation__"+automationID {
			return job
		}
	}

	return nil
}

func decodeStepResults(t *testing.T, run *core.AutomationRun) []map[string]any {
	t.Helper()

	raw := strings.TrimSpace(run.StepResults().String())
	if raw == "" || raw == "null" {
		return nil
	}

	result := []map[string]any{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("Failed to decode step results: %v", err)
	}

	return result
}

func decodeAutomationRunInput(t *testing.T, run *core.AutomationRun) map[string]any {
	t.Helper()

	raw := strings.TrimSpace(run.Input().String())
	if raw == "" || raw == "null" {
		return nil
	}

	result := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("Failed to decode automation run input: %v", err)
	}

	return result
}

func TestAutomationRunStartedFieldSet(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetActive(true)
	automation.SetTriggerType(core.AutomationTriggerScheduleCron)
	automation.SetCronExpr("0 * * * *")

	if err := app.Save(automation); err != nil {
		t.Fatal(err)
	}

	job := findAutomationCronJob(app, automation.Id)
	if job == nil {
		t.Fatal("Expected automation cron job to be registered")
	}

	job.Run()

	runs := waitForAutomationRuns(t, app, automation, 1)
	if runs[0].Started().IsZero() {
		t.Fatal("Expected started field to be set")
	}
	if runs[0].Finished().Time().Before(types.NowDateTime().Add(-10 * time.Second).Time()) {
		t.Fatal("Expected finished field to be recent")
	}
}

type automationHTTPDoerFunc func(req *http.Request) (*http.Response, error)

func (f automationHTTPDoerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}
