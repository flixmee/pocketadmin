package core

import (
	"net/url"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/types"
)

var notificationSeverities = []any{
	NotificationSeverityInfo,
	NotificationSeveritySuccess,
	NotificationSeverityWarning,
	NotificationSeverityDanger,
}

func (app *BaseApp) registerNotificationHooks() {
	app.OnRecordValidate(CollectionNameNotifications).Bind(&hook.Handler[*RecordEvent]{
		Func: func(e *RecordEvent) error {
			normalizeNotificationRecord(e.Record)

			if err := validateNotificationRecord(e.Record); err != nil {
				return err
			}

			return e.Next()
		},
		Priority: 100,
	})
}

func normalizeNotificationRecord(record *Record) {
	record.Set("recipientCollection", strings.TrimSpace(record.GetString("recipientCollection")))
	record.Set("recipientRef", strings.TrimSpace(record.GetString("recipientRef")))
	record.Set("title", strings.TrimSpace(record.GetString("title")))
	record.Set("message", strings.TrimSpace(record.GetString("message")))
	record.Set("type", strings.TrimSpace(record.GetString("type")))
	record.Set("severity", strings.TrimSpace(record.GetString("severity")))
	record.Set("actionUrl", strings.TrimSpace(record.GetString("actionUrl")))
	record.Set("sourceCollection", strings.TrimSpace(record.GetString("sourceCollection")))
	record.Set("sourceRecord", strings.TrimSpace(record.GetString("sourceRecord")))

	if record.GetString("severity") == "" {
		record.Set("severity", NotificationSeverityInfo)
	}

	if record.GetBool("read") {
		if record.GetDateTime("readAt").IsZero() {
			record.Set("readAt", types.NowDateTime())
		}
	} else {
		record.Set("readAt", "")
	}
}

func validateNotificationRecord(record *Record) error {
	if err := validation.Validate(record.GetString("recipientCollection"), validation.Required, validation.Length(1, 255)); err != nil {
		return validation.Errors{"recipientCollection": err}
	}
	if err := validation.Validate(record.GetString("recipientRef"), validation.Required, validation.Length(1, 255)); err != nil {
		return validation.Errors{"recipientRef": err}
	}
	if err := validation.Validate(record.GetString("title"), validation.Required, validation.Length(1, 255)); err != nil {
		return validation.Errors{"title": err}
	}
	if err := validation.Validate(record.GetString("type"), validation.Length(0, 80)); err != nil {
		return validation.Errors{"type": err}
	}
	if err := validation.Validate(record.GetString("severity"), validation.Required, validation.In(notificationSeverities...)); err != nil {
		return validation.Errors{"severity": err}
	}
	if err := validation.Validate(record.GetString("actionUrl"), validation.By(validateNotificationActionURL)); err != nil {
		return validation.Errors{"actionUrl": err}
	}

	return nil
}

func validateNotificationActionURL(value any) error {
	raw, _ := value.(string)
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "#/") {
		return nil
	}

	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return validation.NewError("validation_invalid_notification_action_url", "Must be a valid internal route or URL.")
	}

	if parsed.Scheme != "" && parsed.Host == "" {
		return validation.NewError("validation_invalid_notification_action_url", "Must be a valid internal route or URL.")
	}

	return nil
}
