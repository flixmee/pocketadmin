package core

import (
	"fmt"
	"net/url"
	"strings"
)

func notifyAutomationRunCompletion(app App, automation *Automation, run *AutomationRun) error {
	if automation == nil || run == nil || !automation.NotifyOnCompletion() {
		return nil
	}

	var title string
	var message string
	var severity string

	automationName := strings.TrimSpace(automation.Name())
	if automationName == "" {
		automationName = automation.Id
	}

	switch run.Status() {
	case AutomationRunStatusSuccess:
		title = "Automation succeeded"
		message = fmt.Sprintf("Automation %q completed successfully.", automationName)
		severity = NotificationSeveritySuccess
	case AutomationRunStatusFailed:
		title = "Automation failed"
		if errMsg := strings.TrimSpace(run.Error()); errMsg != "" {
			message = fmt.Sprintf("Automation %q failed: %s", automationName, truncateAutomationNotificationMessage(errMsg))
		} else {
			message = fmt.Sprintf("Automation %q failed.", automationName)
		}
		severity = NotificationSeverityDanger
	default:
		return nil
	}

	superusersCollection, err := app.FindCachedCollectionByNameOrId(CollectionNameSuperusers)
	if err != nil {
		return err
	}

	superusers, err := app.FindAllRecords(superusersCollection)
	if err != nil {
		return err
	}

	actionURL := automationNotificationActionURL(app, automation, run)

	for _, superuser := range superusers {
		if superuser == nil {
			continue
		}

		_, err := app.CreateNotification(NotificationCreateOptions{
			RecipientCollection: superusersCollection.Id,
			RecipientRef:        superuser.Id,
			Title:               title,
			Message:             message,
			Type:                "automation",
			Severity:            severity,
			ActionURL:           actionURL,
			SourceCollection:    CollectionNameAutomationRuns,
			SourceRecord:        run.Id,
			Data: map[string]any{
				"automationId":   automation.Id,
				"automationName": automationName,
				"runId":          run.Id,
				"status":         run.Status(),
				"triggerType":    run.TriggerType(),
				"errorStepIndex": run.ErrorStepIndex(),
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func automationNotificationActionURL(app App, automation *Automation, run *AutomationRun) string {
	automationURL := "#/automations/" + automation.Id

	payload, err := decodeAutomationRunPayload(run)
	if err != nil {
		return automationURL
	}

	recordID := strings.TrimSpace(toString(payload.Record["id"]))
	if recordID == "" {
		return automationURL
	}

	collectionName := strings.TrimSpace(payload.CollectionName)
	if collectionName == "" && payload.CollectionId != "" {
		if collection, err := app.FindCachedCollectionByNameOrId(payload.CollectionId); err == nil {
			collectionName = collection.Name
		}
	}
	if collectionName == "" {
		return automationURL
	}

	values := url.Values{}
	values.Set("collection", collectionName)
	values.Set("record", recordID)

	return "/_/#/collections?" + values.Encode()
}

func truncateAutomationNotificationMessage(message string) string {
	const maxLength = 500

	if len(message) <= maxLength {
		return message
	}

	return message[:maxLength] + "..."
}
