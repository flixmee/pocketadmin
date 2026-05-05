package core

import (
	"fmt"
	"strconv"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/tools/cron"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/types"
)

var (
	automationTriggerTypes = []string{
		AutomationTriggerRecordCreate,
		AutomationTriggerRecordUpdate,
		AutomationTriggerRecordDelete,
		AutomationTriggerScheduleCron,
		AutomationTriggerManual,
	}
	automationStepTypes = []string{
		AutomationStepCondition,
		AutomationStepHTTP,
		AutomationStepMailSend,
		AutomationStepRecordCreate,
		AutomationStepRecordUpdate,
		AutomationStepRecordDelete,
	}
	automationRunStatuses = []string{
		AutomationRunStatusQueued,
		AutomationRunStatusRunning,
		AutomationRunStatusSuccess,
		AutomationRunStatusFailed,
	}
)

func (app *BaseApp) registerAutomationHooks() {
	app.OnRecordValidate(CollectionNameAutomations).Bind(&hook.Handler[*RecordEvent]{
		Func: func(e *RecordEvent) error {
			if err := validateAutomationRecord(e.App, e.Record); err != nil {
				return err
			}

			return e.Next()
		},
		Priority: 99,
	})

	app.OnRecordValidate(CollectionNameAutomationRuns).Bind(&hook.Handler[*RecordEvent]{
		Func: func(e *RecordEvent) error {
			if err := validateAutomationRunRecord(e.App, e.Record); err != nil {
				return err
			}

			return e.Next()
		},
		Priority: 99,
	})

	app.OnBootstrap().Bind(&hook.Handler[*BootstrapEvent]{
		Func: func(e *BootstrapEvent) error {
			if err := e.Next(); err != nil {
				return err
			}

			if _, err := refreshAutomationRegistry(e.App); err != nil {
				return fmt.Errorf("failed to load automation registry: %w", err)
			}

			return nil
		},
		Priority: 100,
	})

	onAutomationChange := func(e *RecordEvent) error {
		if err := e.Next(); err != nil {
			return err
		}

		_, err := refreshAutomationRegistry(e.App)
		if err != nil {
			return fmt.Errorf("failed to refresh automation registry: %w", err)
		}

		return nil
	}

	app.OnRecordAfterCreateSuccess(CollectionNameAutomations).Bind(&hook.Handler[*RecordEvent]{
		Func:     onAutomationChange,
		Priority: 99,
	})
	app.OnRecordAfterUpdateSuccess(CollectionNameAutomations).Bind(&hook.Handler[*RecordEvent]{
		Func:     onAutomationChange,
		Priority: 99,
	})
	app.OnRecordAfterDeleteSuccess(CollectionNameAutomations).Bind(&hook.Handler[*RecordEvent]{
		Func:     onAutomationChange,
		Priority: 99,
	})

	app.OnRecordAfterCreateSuccess().Bind(&hook.Handler[*RecordEvent]{
		Func: func(e *RecordEvent) error {
			if err := e.Next(); err != nil {
				return err
			}

			queueRecordAutomationRuns(e.App, AutomationTriggerRecordCreate, e.Record, nil)

			return nil
		},
		Priority: 100,
	})
	app.OnRecordAfterUpdateSuccess().Bind(&hook.Handler[*RecordEvent]{
		Func: func(e *RecordEvent) error {
			if err := e.Next(); err != nil {
				return err
			}

			queueRecordAutomationRuns(e.App, AutomationTriggerRecordUpdate, e.Record, e.Record.Original())

			return nil
		},
		Priority: 100,
	})
	app.OnRecordAfterDeleteSuccess().Bind(&hook.Handler[*RecordEvent]{
		Func: func(e *RecordEvent) error {
			if err := e.Next(); err != nil {
				return err
			}

			queueRecordAutomationRuns(e.App, AutomationTriggerRecordDelete, e.Record, e.Record.Original())

			return nil
		},
		Priority: 100,
	})
}

func validateAutomationRecord(app App, record *Record) error {
	triggerType := strings.TrimSpace(record.GetString("triggerType"))
	if err := validation.Validate(triggerType, validation.Required, validation.In(toAnySlice(automationTriggerTypes)...)); err != nil {
		return validation.Errors{"triggerType": err}
	}

	if isRecordAutomationTrigger(triggerType) {
		collectionRef := strings.TrimSpace(record.GetString("collectionRef"))
		if err := validation.Validate(
			collectionRef,
			validation.Required,
			validation.By(validateCollectionId(app, CollectionTypeBase, CollectionTypeAuth)),
		); err != nil {
			return validation.Errors{"collectionRef": err}
		}
	}

	cronExpr := strings.TrimSpace(record.GetString("cronExpr"))
	if triggerType == AutomationTriggerScheduleCron {
		if err := validation.Required.Validate(cronExpr); err != nil {
			return validation.Errors{"cronExpr": err}
		}
	}
	if cronExpr != "" {
		if _, err := cron.NewSchedule(cronExpr); err != nil {
			return validation.Errors{
				"cronExpr": validation.NewError("validation_invalid_cron", err.Error()),
			}
		}
	}

	if err := validateAutomationSteps(app, record); err != nil {
		return err
	}

	return nil
}

func validateAutomationRunRecord(app App, record *Record) error {
	automationRef := strings.TrimSpace(record.GetString("automationRef"))
	if err := validation.Validate(automationRef, validation.Required, validation.By(validateRecordId(app, CollectionNameAutomations))); err != nil {
		return validation.Errors{"automationRef": err}
	}

	triggerType := strings.TrimSpace(record.GetString("triggerType"))
	if err := validation.Validate(triggerType, validation.Required, validation.In(toAnySlice(automationTriggerTypes)...)); err != nil {
		return validation.Errors{"triggerType": err}
	}

	status := strings.TrimSpace(record.GetString("status"))
	if err := validation.Validate(status, validation.Required, validation.In(toAnySlice(automationRunStatuses)...)); err != nil {
		return validation.Errors{"status": err}
	}

	return nil
}

func validateAutomationSteps(app App, automationRecord *Record) error {
	value := automationRecord.GetRaw("steps")
	raw, ok := value.(types.JSONRaw)
	if !ok {
		return validation.Errors{"steps": validation.NewError("validation_invalid_automation_steps", "Steps must be a JSON array.")}
	}

	steps, err := decodeAutomationSteps(raw.String())
	if err != nil {
		return validation.Errors{"steps": validation.NewError("validation_invalid_automation_steps", "Steps must be a JSON array.")}
	}
	if len(steps) == 0 {
		return nil
	}

	stepErrs := validation.Errors{}

	for i, step := range steps {
		if step == nil {
			stepErrs[strconv.Itoa(i)] = validation.NewError("validation_invalid_automation_step", "Each step must be a JSON object.")
			continue
		}

		typeVal, _ := step["type"].(string)
		typeVal = strings.TrimSpace(typeVal)

		fieldErrs := validation.Errors{}
		if err := validation.Validate(typeVal, validation.Required, validation.In(toAnySlice(automationStepTypes)...)); err != nil {
			fieldErrs["type"] = err
		}
		if err := validateAutomationTemplateRoots(step); err != nil {
			fieldErrs["template"] = validation.NewError("validation_invalid_automation_template", err.Error())
		}
		if err := validateAutomationStepDefinition(app, automationRecord, step); err != nil {
			fieldErrs["config"] = err
		}

		if len(fieldErrs) > 0 {
			stepErrs[strconv.Itoa(i)] = fieldErrs
		}
	}

	if len(stepErrs) > 0 {
		return validation.Errors{"steps": stepErrs}
	}

	return nil
}

func validateAutomationStepDefinition(app App, automationRecord *Record, step map[string]any) error {
	switch strings.TrimSpace(toString(step["type"])) {
	case AutomationStepCondition:
		return validateAutomationConditionStep(step)
	case AutomationStepHTTP:
		return validateAutomationHTTPStep(step)
	case AutomationStepMailSend:
		return validateAutomationMailStep(app, automationRecord, step)
	case AutomationStepRecordCreate:
		return validateAutomationRecordCreateStep(step)
	case AutomationStepRecordUpdate:
		return validateAutomationRecordUpdateStep(step)
	case AutomationStepRecordDelete:
		return validateAutomationRecordDeleteStep(step)
	default:
		return nil
	}
}

func validateAutomationConditionStep(step map[string]any) error {
	path := strings.TrimSpace(toString(step["path"]))
	if path == "" {
		path = strings.TrimSpace(toString(step["field"]))
	}
	if path == "" {
		return validation.NewError("validation_invalid_automation_condition", "Condition step requires a path.")
	}

	op := strings.TrimSpace(toString(step["op"]))
	if err := validation.Validate(op, validation.Required, validation.In(
		automationConditionOpEq,
		automationConditionOpNeq,
		automationConditionOpIn,
		automationConditionOpExists,
	)); err != nil {
		return err
	}

	if op != automationConditionOpExists {
		if _, ok := step["value"]; !ok {
			return validation.NewError("validation_invalid_automation_condition", "Condition step requires a value.")
		}
	}

	return nil
}

func validateAutomationHTTPStep(step map[string]any) error {
	if strings.TrimSpace(toString(step["url"])) == "" {
		return validation.NewError("validation_invalid_automation_http", "HTTP step requires a url.")
	}

	if headers, ok := step["headers"]; ok {
		if _, valid := headers.(map[string]any); !valid {
			return validation.NewError("validation_invalid_automation_http", "HTTP step headers must be a JSON object.")
		}
	}

	if timeout, ok := step["timeout"]; ok {
		if automationStepDuration(timeout) <= 0 {
			return validation.NewError("validation_invalid_automation_http", "HTTP step timeout must be greater than zero.")
		}
	}

	return nil
}

func validateAutomationMailStep(app App, automationRecord *Record, step map[string]any) error {
	toValues, err := normalizeAutomationStringValues(step["to"])
	if err != nil || len(toValues) == 0 {
		return validation.NewError("validation_invalid_automation_mail", "Mail step requires at least one recipient.")
	}

	subject := strings.TrimSpace(toString(step["subject"]))
	if subject == "" {
		return validation.NewError("validation_invalid_automation_mail", "Mail step requires a subject.")
	}

	text := strings.TrimSpace(toString(step["text"]))
	html := strings.TrimSpace(toString(step["html"]))
	if text == "" && html == "" {
		return validation.NewError("validation_invalid_automation_mail", "Mail step requires text or html content.")
	}

	for _, fieldName := range []string{"cc", "bcc"} {
		if _, exists := step[fieldName]; !exists {
			continue
		}

		if _, err := normalizeAutomationStringValues(step[fieldName]); err != nil {
			return validation.NewError("validation_invalid_automation_mail", "Mail step address lists must contain only strings.")
		}
	}

	attachmentFields, err := normalizeAutomationAttachmentFields(step["attachments"])
	if err != nil {
		return validation.NewError("validation_invalid_automation_mail", "Mail step attachments must be a list of file field names.")
	}
	if len(attachmentFields) == 0 {
		return nil
	}

	triggerType := strings.TrimSpace(automationRecord.GetString("triggerType"))
	if triggerType != AutomationTriggerRecordCreate && triggerType != AutomationTriggerRecordUpdate {
		return validation.NewError(
			"validation_invalid_automation_mail",
			"Mail step attachments require a record create or record update trigger.",
		)
	}

	collectionRef := strings.TrimSpace(automationRecord.GetString("collectionRef"))
	if collectionRef == "" {
		return validation.NewError("validation_invalid_automation_mail", "Mail step attachments require a trigger collection.")
	}

	collection, err := app.FindCollectionByNameOrId(collectionRef)
	if err != nil {
		return validation.NewError("validation_invalid_automation_mail", "Mail step attachments require a valid trigger collection.")
	}

	for _, fieldName := range attachmentFields {
		field, ok := collection.Fields.GetByName(fieldName).(*FileField)
		if !ok || field == nil {
			return validation.NewError(
				"validation_invalid_automation_mail",
				fmt.Sprintf("Mail step attachment field %q must be a file field on the trigger collection.", fieldName),
			)
		}
	}

	return nil
}

func validateAutomationRecordCreateStep(step map[string]any) error {
	if strings.TrimSpace(toString(step["collection"])) == "" {
		return validation.NewError("validation_invalid_automation_record", "Record step requires a collection.")
	}

	if _, ok := step["data"].(map[string]any); !ok {
		return validation.NewError("validation_invalid_automation_record", "Record create step requires a data object.")
	}

	return nil
}

func validateAutomationRecordUpdateStep(step map[string]any) error {
	if err := validateAutomationRecordCreateStep(step); err != nil {
		return err
	}

	if strings.TrimSpace(toString(step["id"])) == "" && strings.TrimSpace(toString(step["filter"])) == "" {
		return validation.NewError("validation_invalid_automation_record", "Record update step requires either id or filter.")
	}

	return nil
}

func validateAutomationRecordDeleteStep(step map[string]any) error {
	if strings.TrimSpace(toString(step["collection"])) == "" {
		return validation.NewError("validation_invalid_automation_record", "Record step requires a collection.")
	}

	if strings.TrimSpace(toString(step["id"])) == "" && strings.TrimSpace(toString(step["filter"])) == "" {
		return validation.NewError("validation_invalid_automation_record", "Record delete step requires either id or filter.")
	}

	return nil
}

func isRecordAutomationTrigger(triggerType string) bool {
	switch triggerType {
	case AutomationTriggerRecordCreate, AutomationTriggerRecordUpdate, AutomationTriggerRecordDelete:
		return true
	default:
		return false
	}
}

func toAnySlice(items []string) []any {
	result := make([]any, len(items))
	for i, item := range items {
		result[i] = item
	}

	return result
}
