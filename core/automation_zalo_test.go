package core_test

import (
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func setupZaloTestApp(t *testing.T) *tests.TestApp {
	t.Helper()

	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}

	app.Settings().Credentials.Zalo.Enabled = true
	app.Settings().Credentials.Zalo.BridgeURL = "http://127.0.0.1:1" // offline bridge - messages stay pending
	app.Settings().Credentials.Zalo.Secret = "test-bridge-secret-123"
	if err := core.EnsureZaloCollections(app); err != nil {
		t.Fatal(err)
	}

	return app
}

func saveZaloMessage(t *testing.T, app core.App, msgId string, text string, isSelf bool) *core.Record {
	t.Helper()

	messages, err := app.FindCollectionByNameOrId(core.ZaloCollectionMessages)
	if err != nil {
		t.Fatal(err)
	}

	record := core.NewRecord(messages)
	record.Set("msg_id", msgId)
	record.Set("thread_id", "thread-42")
	record.Set("thread_type", "group")
	record.Set("sender_id", "sender-1")
	record.Set("sender_name", "Alice")
	record.Set("text", text)
	record.Set("is_self", isSelf)
	if err := app.Save(record); err != nil {
		t.Fatal(err)
	}

	return record
}

func findZaloOutbox(t *testing.T, app core.App) []*core.Record {
	t.Helper()

	records, err := app.FindAllRecords(core.ZaloCollectionOutbox)
	if err != nil {
		t.Fatal(err)
	}

	return records
}

func TestAutomationZaloMessageTriggerAndReply(t *testing.T) {
	t.Parallel()

	app := setupZaloTestApp(t)
	defer app.Cleanup()

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetTriggerType(core.AutomationTriggerZaloMessage)
	automation.SetActive(true)
	automation.SetSteps(mustParseJSONRaw(t, `[{
		"type":"zalo.send",
		"threadId":"{{zalo.threadId}}",
		"threadType":"{{zalo.threadType}}",
		"text":"Hi {{zalo.senderName}}, you said: {{zalo.text}}",
		"replyTo":"{{zalo.messageId}}"
	}]`))
	if err := app.Save(automation); err != nil {
		t.Fatal(err)
	}

	// own messages (e.g. the replies sent by the bridge) don't trigger automations
	saveZaloMessage(t, app, "self-1", "from me", true)

	incoming := saveZaloMessage(t, app, "msg-1", "hello", false)

	runs := waitForCompletedAutomationRuns(t, app, automation, 1)
	if runs[0].Status() != core.AutomationRunStatusSuccess {
		t.Fatalf("Expected success, got %q: %s", runs[0].Status(), runs[0].Error())
	}

	results := decodeStepResults(t, runs[0])
	output := results[0]["output"].(map[string]any)
	if output["queued"] != true || output["status"] != "pending" || output["threadType"] != "group" {
		t.Fatalf("Unexpected step output %#v", output)
	}

	outbox := findZaloOutbox(t, app)
	if len(outbox) != 1 {
		t.Fatalf("Expected 1 outbox record, got %d", len(outbox))
	}
	reply := outbox[0]
	if reply.GetString("thread_id") != "thread-42" || reply.GetString("thread_type") != "group" ||
		reply.GetString("text") != "Hi Alice, you said: hello" || reply.GetString("reply_to") != incoming.Id ||
		reply.GetString("status") != "pending" || reply.Id != output["outboxId"] {
		t.Fatalf("Unexpected outbox record %v", reply.PublicExport())
	}

	// give the (ignored) self message a chance to be wrongly processed
	time.Sleep(200 * time.Millisecond)
	if runs, _ := app.FindAllRecords(core.CollectionNameAutomationRuns); len(runs) != 1 {
		t.Fatalf("Expected only 1 automation run, got %d", len(runs))
	}
}

func TestAutomationZaloSendStep(t *testing.T) {
	t.Parallel()

	newAutomation := func(t *testing.T, app *tests.TestApp, steps string) *core.Automation {
		automation := core.NewAutomation(app)
		populateValidAutomation(automation)
		automation.SetActive(true)
		automation.SetSteps(mustParseJSONRaw(t, steps))
		if err := app.Save(automation); err != nil {
			t.Fatal(err)
		}
		return automation
	}

	// simulates the bridge processing the pending outbox records
	simulateBridge := func(app *tests.TestApp, status string, errMessage string) {
		go func() {
			for i := 0; i < 100; i++ {
				time.Sleep(50 * time.Millisecond)
				records, _ := app.FindAllRecords(core.ZaloCollectionOutbox)
				for _, r := range records {
					if r.GetString("status") != "pending" {
						continue
					}
					r.Set("status", status)
					r.Set("error", errMessage)
					r.Set("zalo_msg_id", "zalo-999")
					r.Set("resolved_thread_id", "resolved-1")
					_ = app.Save(r)
					return
				}
			}
		}()
	}

	t.Run("send by phone and wait for delivery", func(t *testing.T) {
		app := setupZaloTestApp(t)
		defer app.Cleanup()

		automation := newAutomation(t, app, `[{"type":"zalo.send","phone":"0901234567","text":"Order confirmed","waitForDelivery":true}]`)
		simulateBridge(app, "sent", "")

		result, err := app.RunAutomationManuallyWithOptions(automation.Id, core.AutomationManualRunOptions{Wait: true})
		if err != nil {
			t.Fatal(err)
		}
		if result.Status != core.AutomationRunStatusSuccess {
			t.Fatalf("Expected success, got %q: %s", result.Status, result.Error)
		}

		output, _ := result.Output.(map[string]any)
		if output["status"] != "sent" || output["zaloMsgId"] != "zalo-999" || output["threadId"] != "resolved-1" {
			t.Fatalf("Unexpected output %#v", result.Output)
		}

		outbox := findZaloOutbox(t, app)
		if len(outbox) != 1 || outbox[0].GetString("phone") != "0901234567" || outbox[0].GetString("thread_id") != "" {
			t.Fatalf("Unexpected outbox records %v", outbox)
		}
	})

	t.Run("delivery failure fails the step", func(t *testing.T) {
		app := setupZaloTestApp(t)
		defer app.Cleanup()

		automation := newAutomation(t, app, `[{"type":"zalo.send","threadId":"123","text":"hi","waitForDelivery":true}]`)
		simulateBridge(app, "failed", "account locked")

		result, err := app.RunAutomationManuallyWithOptions(automation.Id, core.AutomationManualRunOptions{Wait: true})
		if result == nil {
			t.Fatal(err)
		}
		if result.Status != core.AutomationRunStatusFailed || !strings.Contains(result.Error, "account locked") {
			t.Fatalf("Expected failed run with the bridge error, got %q: %s", result.Status, result.Error)
		}
	})

	t.Run("disabled integration", func(t *testing.T) {
		app := setupZaloTestApp(t)
		defer app.Cleanup()

		automation := newAutomation(t, app, `[{"type":"zalo.send","threadId":"123","text":"hi"}]`)
		app.Settings().Credentials.Zalo.Enabled = false

		result, err := app.RunAutomationManuallyWithOptions(automation.Id, core.AutomationManualRunOptions{Wait: true})
		if result == nil {
			t.Fatal(err)
		}
		if result.Status != core.AutomationRunStatusFailed || !strings.Contains(result.Error, "not enabled") {
			t.Fatalf("Expected failed run, got %q: %s", result.Status, result.Error)
		}
	})
}

func TestAutomationZaloSendValidation(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	scenarios := []struct {
		name     string
		steps    string
		expected string
	}{
		{"missing recipient", `[{"type":"zalo.send","text":"hi"}]`, "thread ID or a phone number"},
		{"missing text", `[{"type":"zalo.send","threadId":"123"}]`, "requires a message"},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			automation := core.NewAutomation(app)
			populateValidAutomation(automation)
			automation.SetSteps(mustParseJSONRaw(t, s.steps))

			err := app.Validate(automation)
			if err == nil || !strings.Contains(err.Error(), s.expected) {
				t.Fatalf("Expected validation error containing %q, got %v", s.expected, err)
			}
		})
	}

	automation := core.NewAutomation(app)
	populateValidAutomation(automation)
	automation.SetTriggerType(core.AutomationTriggerZaloMessage)
	automation.SetSteps(mustParseJSONRaw(t, `[{"type":"zalo.send","phone":"0901234567","text":"hi"}]`))
	if err := app.Validate(automation); err != nil {
		t.Fatalf("Expected valid automation, got %v", err)
	}
}
