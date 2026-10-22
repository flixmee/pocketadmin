package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/tools/routine"
)

// AutomationZaloDeliveryTimeout is how long the "Send Zalo" step waits
// for the delivery result when waitForDelivery is enabled.
var AutomationZaloDeliveryTimeout = 60 * time.Second

const automationZaloDeliveryPoll = 500 * time.Millisecond

// QueueAutomationZaloMessage queues all active "On Zalo message" automations
// for a message saved by the zalo-bridge in the zalo_messages collection.
//
// Messages sent by the connected account itself are ignored (to avoid
// automations replying to their own messages).
func QueueAutomationZaloMessage(app App, message *Record) (int, error) {
	if message == nil || message.GetBool("is_self") {
		return 0, nil
	}

	registry, err := getAutomationRegistry(app)
	if err != nil {
		return 0, err
	}

	automations := registry.ByTrigger[AutomationTriggerZaloMessage]
	if len(automations) == 0 {
		return 0, nil
	}

	payload := automationZaloTriggerPayload(message)
	inheritAutomationPolicyContext(app, &payload)
	_, _ = app.PublishAutomationEvent(AutomationEventEnvelope{
		Name:    AutomationTriggerZaloMessage,
		Source:  "zalo",
		Subject: toString(payload.Zalo["threadId"]),
		Payload: map[string]any{
			"zalo": payload.Zalo,
		},
	})

	queued := 0
	for _, automation := range automations {
		if automation == nil {
			continue
		}
		queued++

		nextAutomation := automation
		nextPayload := payload
		routine.FireAndForget(func() {
			if err := runAutomation(app, nextAutomation, nextPayload); err != nil {
				app.Logger().Warn(
					"Failed to execute Zalo message automation",
					"automationId", nextAutomation.Id,
					"error", err,
				)
			}
		})
	}

	return queued, nil
}

func automationZaloTriggerPayload(message *Record) automationTriggerPayload {
	zalo := map[string]any{
		"messageId":  message.Id,
		"msgId":      message.GetString("msg_id"),
		"threadId":   message.GetString("thread_id"),
		"threadType": message.GetString("thread_type"),
		"senderId":   message.GetString("sender_id"),
		"senderName": message.GetString("sender_name"),
		"msgType":    message.GetString("msg_type"),
		"text":       message.GetString("text"),
		"isSelf":     message.GetBool("is_self"),
		"ts":         message.GetFloat("zalo_ts"),
		"content":    message.Get("content"),
		"quote":      message.Get("quote"),
		"message":    automationTemplateRecordData(message),
	}

	return automationTriggerPayload{
		TriggerType: AutomationTriggerZaloMessage,
		DedupeKey:   AutomationTriggerZaloMessage + ":" + message.GetString("msg_id"),
		Zalo:        zalo,
	}
}

// executeAutomationZaloStep queues a message in the zalo_outbox collection
// (the zalo-bridge sends it) and optionally waits for the delivery result.
func executeAutomationZaloStep(ctx *automationExecutionContext, step map[string]any) (map[string]any, error) {
	if !ctx.App.Settings().Credentials.Zalo.Enabled {
		return nil, fmt.Errorf("the Zalo integration is not enabled (Settings > Zalo)")
	}

	outbox, err := ctx.App.FindCollectionByNameOrId(ZaloCollectionOutbox)
	if err != nil {
		return nil, fmt.Errorf("the %s collection doesn't exist - enable Zalo in Settings > Zalo", ZaloCollectionOutbox)
	}

	fields := map[string]string{}
	for _, key := range []string{"threadId", "threadType", "phone", "text", "replyTo"} {
		value, err := renderAutomationTelegramString(ctx, step[key])
		if err != nil {
			return nil, err
		}
		fields[key] = strings.TrimSpace(value)
	}

	if fields["threadId"] == "" && fields["phone"] == "" {
		return nil, fmt.Errorf("Zalo step requires a thread ID or a phone number")
	}
	if fields["text"] == "" {
		return nil, fmt.Errorf("Zalo step requires a message")
	}

	threadType := fields["threadType"]
	if threadType != "group" {
		threadType = "user"
	}

	record := NewRecord(outbox)
	record.Set("thread_id", fields["threadId"])
	if fields["threadId"] != "" {
		record.Set("thread_type", threadType)
	}
	record.Set("phone", fields["phone"])
	record.Set("text", fields["text"])
	record.Set("reply_to", fields["replyTo"])
	record.Set("status", "pending")

	// note: saving triggers the outbox hook that notifies the bridge
	if err := ctx.App.Save(record); err != nil {
		return nil, fmt.Errorf("failed to queue the Zalo message: %w", err)
	}

	output := map[string]any{
		"queued":     true,
		"outboxId":   record.Id,
		"status":     "pending",
		"threadId":   fields["threadId"],
		"threadType": record.GetString("thread_type"),
		"phone":      fields["phone"],
	}

	if !automationTelegramBool(step["waitForDelivery"]) {
		return output, nil
	}

	deadline := time.Now().Add(AutomationZaloDeliveryTimeout)
	for time.Now().Before(deadline) {
		time.Sleep(automationZaloDeliveryPoll)

		current, err := ctx.App.FindRecordById(outbox, record.Id)
		if err != nil {
			return nil, fmt.Errorf("failed to load the Zalo outbox record: %w", err)
		}

		switch current.GetString("status") {
		case "sent":
			output["status"] = "sent"
			output["zaloMsgId"] = current.GetString("zalo_msg_id")
			output["threadId"] = current.GetString("resolved_thread_id")
			return output, nil
		case "failed":
			return nil, fmt.Errorf("failed to send the Zalo message: %s", current.GetString("error"))
		}
	}

	return nil, fmt.Errorf(
		"the Zalo message %s was not delivered within %s - check that the bridge is online and logged in (Settings > Zalo)",
		record.Id, AutomationZaloDeliveryTimeout,
	)
}
