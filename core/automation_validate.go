package core

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/tools/cron"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	StoreKeyAutomationDelaySchedulerStop = "pbAppAutomationDelaySchedulerStop"
	automationDelaySchedulerInterval     = time.Second
)

var (
	automationTriggerTypes = []string{
		AutomationTriggerRecordBeforeCreate,
		AutomationTriggerRecordBeforeUpdate,
		AutomationTriggerRecordCreate,
		AutomationTriggerRecordUpdate,
		AutomationTriggerRecordDelete,
		AutomationTriggerScheduleCron,
		AutomationTriggerWebhook,
		AutomationTriggerManual,
		AutomationTriggerI18nMissing,
		AutomationTriggerI18nPublished,
		AutomationTriggerI18nUpdated,
		AutomationTriggerI18nAIFinished,
	}
	automationStepTypes = []string{
		AutomationStepCondition,
		AutomationStepCode,
		AutomationStepHTTP,
		AutomationStepMailSend,
		AutomationStepRecordCreate,
		AutomationStepRecordUpdate,
		AutomationStepRecordDelete,
		AutomationStepResponse,
		AutomationStepCapability,
		AutomationStepWaitDelay,
		AutomationStepWaitWebhook,
		AutomationStepWaitEvent,
		AutomationStepWaitApproval,
		AutomationStepAIExtract,
		AutomationStepAIClassify,
		AutomationStepAIGenerate,
		AutomationStepAISummarize,
	}
	automationRunStatuses = []string{
		AutomationRunStatusQueued,
		AutomationRunStatusRunning,
		AutomationRunStatusWaiting,
		AutomationRunStatusSuccess,
		AutomationRunStatusFailed,
	}
	workflowStateStatuses = []string{
		WorkflowStateStatusRunning,
		WorkflowStateStatusWaiting,
		WorkflowStateStatusCompleted,
		WorkflowStateStatusFailed,
		WorkflowStateStatusExpired,
	}
	approvalStatuses = []string{
		ApprovalStatusPending,
		ApprovalStatusApproved,
		ApprovalStatusRejected,
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
	app.OnRecordValidate(CollectionNameWorkflowState).Bind(&hook.Handler[*RecordEvent]{
		Func: func(e *RecordEvent) error {
			if err := validateWorkflowStateRecord(e.App, e.Record); err != nil {
				return err
			}

			return e.Next()
		},
		Priority: 99,
	})
	app.OnRecordValidate(CollectionNameApprovals).Bind(&hook.Handler[*RecordEvent]{
		Func: func(e *RecordEvent) error {
			if err := validateApprovalRecord(e.App, e.Record); err != nil {
				return err
			}

			return e.Next()
		},
		Priority: 99,
	})

	app.OnBootstrap().Bind(&hook.Handler[*BootstrapEvent]{
		Func: func(e *BootstrapEvent) error {
			stopAutomationDelayScheduler(e.App)

			if err := e.Next(); err != nil {
				return err
			}

			if _, err := refreshAutomationRegistry(e.App); err != nil {
				return fmt.Errorf("failed to load automation registry: %w", err)
			}

			startAutomationDelayScheduler(e.App)

			return nil
		},
		Priority: 100,
	})

	app.OnTerminate().Bind(&hook.Handler[*TerminateEvent]{
		Func: func(e *TerminateEvent) error {
			stopAutomationDelayScheduler(e.App)
			return e.Next()
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

			publishRecordAutomationEvent(e.App, AutomationTriggerRecordCreate, e.Record, nil)
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

			publishRecordAutomationEvent(e.App, AutomationTriggerRecordUpdate, e.Record, e.Record.Original())
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

			publishRecordAutomationEvent(e.App, AutomationTriggerRecordDelete, e.Record, e.Record.Original())
			queueRecordAutomationRuns(e.App, AutomationTriggerRecordDelete, e.Record, e.Record.Original())

			return nil
		},
		Priority: 100,
	})

	app.OnRecordCreate().Bind(&hook.Handler[*RecordEvent]{
		Func: func(e *RecordEvent) error {
			if err := runRecordAutomationRuns(e.App, AutomationTriggerRecordBeforeCreate, e.Record, nil); err != nil {
				return err
			}

			return e.Next()
		},
		Priority: 100,
	})
	app.OnRecordUpdate().Bind(&hook.Handler[*RecordEvent]{
		Func: func(e *RecordEvent) error {
			if err := runRecordAutomationRuns(e.App, AutomationTriggerRecordBeforeUpdate, e.Record, e.Record.Original()); err != nil {
				return err
			}

			return e.Next()
		},
		Priority: 100,
	})
}

func validateAutomationRecord(app App, record *Record) error {
	triggerType := strings.TrimSpace(record.GetString("triggerType"))
	if err := validation.Validate(triggerType, validation.Required, validation.In(toAnySlice(automationTriggerTypes)...)); err != nil {
		return validation.Errors{"triggerType": err}
	}

	if isRecordAutomationTrigger(triggerType) || isI18nAutomationTrigger(triggerType) {
		collectionRef := strings.TrimSpace(record.GetString("collectionRef"))
		if err := validation.Validate(
			collectionRef,
			validation.When(triggerType != AutomationTriggerI18nPublished, validation.Required),
			validation.When(collectionRef != "", validation.By(validateCollectionId(app, CollectionTypeBase, CollectionTypeAuth))),
		); err != nil {
			return validation.Errors{"collectionRef": err}
		}
	}

	webhookMethod := NormalizeAutomationWebhookMethod(record.GetString("webhookMethod"))
	if triggerType == AutomationTriggerWebhook {
		if err := validation.Validate(webhookMethod, validation.Required, validation.By(validateAutomationWebhookMethod)); err != nil {
			return validation.Errors{"webhookMethod": err}
		}
	} else if strings.TrimSpace(record.GetString("webhookMethod")) != "" {
		if err := validation.By(validateAutomationWebhookMethod).Validate(webhookMethod); err != nil {
			return validation.Errors{"webhookMethod": err}
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

func validateAutomationWebhookMethod(value any) error {
	method := NormalizeAutomationWebhookMethod(toString(value))
	if IsAutomationWebhookMethod(method) {
		return nil
	}

	return validation.NewError("validation_invalid_webhook_method", "Unsupported webhook HTTP method.")
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

func validateWorkflowStateRecord(app App, record *Record) error {
	automationRef := strings.TrimSpace(record.GetString("automationRef"))
	if err := validation.Validate(automationRef, validation.Required, validation.By(validateRecordId(app, CollectionNameAutomations))); err != nil {
		return validation.Errors{"automationRef": err}
	}

	runRef := strings.TrimSpace(record.GetString("runRef"))
	if err := validation.Validate(runRef, validation.Required, validation.By(validateRecordId(app, CollectionNameAutomationRuns))); err != nil {
		return validation.Errors{"runRef": err}
	}

	status := strings.TrimSpace(record.GetString("status"))
	if err := validation.Validate(status, validation.Required, validation.In(toAnySlice(workflowStateStatuses)...)); err != nil {
		return validation.Errors{"status": err}
	}

	return nil
}

func validateApprovalRecord(app App, record *Record) error {
	workflowStateRef := strings.TrimSpace(record.GetString("workflowStateRef"))
	if err := validation.Validate(workflowStateRef, validation.Required, validation.By(validateRecordId(app, CollectionNameWorkflowState))); err != nil {
		return validation.Errors{"workflowStateRef": err}
	}

	automationRef := strings.TrimSpace(record.GetString("automationRef"))
	if err := validation.Validate(automationRef, validation.Required, validation.By(validateRecordId(app, CollectionNameAutomations))); err != nil {
		return validation.Errors{"automationRef": err}
	}

	runRef := strings.TrimSpace(record.GetString("runRef"))
	if err := validation.Validate(runRef, validation.Required, validation.By(validateRecordId(app, CollectionNameAutomationRuns))); err != nil {
		return validation.Errors{"runRef": err}
	}

	status := strings.TrimSpace(record.GetString("status"))
	if err := validation.Validate(status, validation.Required, validation.In(toAnySlice(approvalStatuses)...)); err != nil {
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
	if automationStepCount(steps) > AutomationMaxSteps {
		return validation.Errors{
			"steps": validation.NewError(
				"validation_automation_steps_limit",
				fmt.Sprintf("Automations can have at most %d steps.", AutomationMaxSteps),
			),
		}
	}

	stepErrs := validation.Errors{}

	validateAutomationStepList(app, automationRecord, steps, "", stepErrs)

	if len(stepErrs) > 0 {
		return validation.Errors{"steps": stepErrs}
	}

	return nil
}

func validateAutomationStepList(app App, automationRecord *Record, steps []map[string]any, prefix string, stepErrs validation.Errors) {
	for i, step := range steps {
		key := strconv.Itoa(i)
		if prefix != "" {
			key = prefix + "." + key
		}
		if step == nil {
			stepErrs[key] = validation.NewError("validation_invalid_automation_step", "Each step must be a JSON object.")
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
		if err := validateAutomationStepBranches(app, automationRecord, step, key, stepErrs); err != nil {
			fieldErrs["branches"] = err
		}

		if len(fieldErrs) > 0 {
			stepErrs[key] = fieldErrs
		}
	}
}

func automationStepCount(steps []map[string]any) int {
	count := len(steps)
	for _, step := range steps {
		for _, branch := range []string{"true", "false"} {
			count += automationStepCount(automationStepBranchSteps(step, branch))
		}
	}
	return count
}

func validateAutomationStepBranches(app App, automationRecord *Record, step map[string]any, prefix string, stepErrs validation.Errors) error {
	branchesRaw, ok := step["branches"]
	if !ok || branchesRaw == nil {
		return nil
	}
	if strings.TrimSpace(toString(step["type"])) != AutomationStepCondition && strings.TrimSpace(toString(step["type"])) != AutomationStepWaitApproval {
		return validation.NewError("validation_invalid_automation_branches", "Only condition and approval wait steps can define true/false branches.")
	}

	branches, ok := branchesRaw.(map[string]any)
	if !ok {
		return validation.NewError("validation_invalid_automation_branches", "Branches must be an object with true and false arrays.")
	}
	for key := range branches {
		if key != "true" && key != "false" {
			return validation.NewError("validation_invalid_automation_branches", `Branches can only contain "true" and "false" paths.`)
		}
	}
	for _, branch := range []string{"true", "false"} {
		rawSteps, exists := branches[branch]
		if !exists || rawSteps == nil {
			continue
		}
		items, ok := rawSteps.([]any)
		if !ok {
			return validation.NewError("validation_invalid_automation_branches", "Branch paths must be arrays of steps.")
		}
		branchSteps := make([]map[string]any, 0, len(items))
		for _, item := range items {
			branchStep, ok := item.(map[string]any)
			if !ok {
				return validation.NewError("validation_invalid_automation_branches", "Branch path entries must be step objects.")
			}
			branchSteps = append(branchSteps, branchStep)
		}
		validateAutomationStepList(app, automationRecord, branchSteps, prefix+".branches."+branch, stepErrs)
	}

	return nil
}

func validateAutomationStepDefinition(app App, automationRecord *Record, step map[string]any) error {
	switch strings.TrimSpace(toString(step["type"])) {
	case AutomationStepCondition:
		return validateAutomationConditionStep(step)
	case AutomationStepCode:
		return validateAutomationCodeStep(step)
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
	case AutomationStepResponse:
		return validateAutomationResponseStep(automationRecord, step)
	case AutomationStepCapability:
		return validateAutomationCapabilityStep(app, automationRecord, step)
	case AutomationStepWaitDelay:
		return validateAutomationWaitDelayStep(step)
	case AutomationStepWaitWebhook, AutomationStepWaitEvent:
		return validateAutomationWaitKeyStep(step)
	case AutomationStepWaitApproval:
		return validateAutomationWaitApprovalStep(step)
	case AutomationStepAIExtract, AutomationStepAIClassify, AutomationStepAIGenerate, AutomationStepAISummarize:
		return validateAutomationAIStep(step)
	default:
		return nil
	}
}

func validateAutomationCodeStep(step map[string]any) error {
	code := strings.TrimSpace(toString(step["code"]))
	if code == "" {
		return validation.NewError("validation_invalid_automation_code", "Code step requires JavaScript code.")
	}
	if len(code) > AutomationMaxTemplateStringSize {
		return validation.NewError("validation_invalid_automation_code", fmt.Sprintf("Code step exceeds %d bytes.", AutomationMaxTemplateStringSize))
	}

	return nil
}

func validateAutomationAIStep(step map[string]any) error {
	stepType := strings.TrimSpace(toString(step["type"]))
	if _, ok := step["input"]; !ok {
		return validation.NewError("validation_invalid_automation_ai", "AI step requires input.")
	}
	if schema, ok := step["schema"]; ok && schema != nil {
		if _, valid := schema.(map[string]any); !valid {
			return validation.NewError("validation_invalid_automation_ai", "AI step schema must be a JSON object.")
		}
	}
	if stepType == AutomationStepAIClassify {
		labels, ok := step["labels"].([]any)
		if !ok || len(labels) == 0 {
			return validation.NewError("validation_invalid_automation_ai", "AI classify step requires labels.")
		}
	}

	return nil
}

func validateAutomationWaitDelayStep(step map[string]any) error {
	if _, ok := step["duration"]; !ok {
		return validation.NewError("validation_invalid_automation_wait", "Wait delay step requires a duration.")
	}
	if duration, err := automationWaitDuration(step["duration"]); err != nil || duration <= 0 {
		return validation.NewError("validation_invalid_automation_wait", "Wait delay step duration must be a positive duration.")
	}

	return nil
}

func validateAutomationWaitKeyStep(step map[string]any) error {
	if strings.TrimSpace(toString(step["key"])) == "" {
		return validation.NewError("validation_invalid_automation_wait", "Wait step requires a key.")
	}

	return nil
}

func validateAutomationWaitApprovalStep(step map[string]any) error {
	if strings.TrimSpace(toString(step["role"])) == "" && strings.TrimSpace(toString(step["assignee"])) == "" {
		return validation.NewError("validation_invalid_automation_wait", "Approval wait step requires a role or assignee.")
	}

	return nil
}

func validateAutomationCapabilityStep(app App, automationRecord *Record, step map[string]any) error {
	key := automationCapabilityKey(step)
	if key == "" {
		return validation.NewError("validation_invalid_automation_capability", "Capability step requires a capability key.")
	}

	legacyStep, err := automationCapabilityLegacyStep(app, step)
	if err != nil {
		return validation.NewError("validation_invalid_automation_capability", err.Error())
	}

	return validateAutomationStepDefinition(app, automationRecord, legacyStep)
}

func validateAutomationConditionStep(step map[string]any) error {
	if conditionsRaw, ok := step["conditions"]; ok {
		conditions := []map[string]any{}
		switch list := conditionsRaw.(type) {
		case []any:
			for _, rawCondition := range list {
				condition, ok := rawCondition.(map[string]any)
				if !ok {
					return validation.NewError("validation_invalid_automation_condition", "Condition entries must be objects.")
				}
				conditions = append(conditions, condition)
			}
		case []map[string]any:
			conditions = append(conditions, list...)
		default:
			return validation.NewError("validation_invalid_automation_condition", "Condition step requires at least one condition.")
		}
		if len(conditions) == 0 {
			return validation.NewError("validation_invalid_automation_condition", "Condition step requires at least one condition.")
		}

		match := strings.TrimSpace(toString(step["match"]))
		if match != "" && match != "and" && match != "or" {
			return validation.NewError("validation_invalid_automation_condition", `Condition match must be either "and" or "or".`)
		}

		for _, condition := range conditions {
			if err := validateAutomationConditionRule(condition); err != nil {
				return err
			}
		}

		return nil
	}

	return validateAutomationConditionRule(step)
}

func validateAutomationConditionRule(step map[string]any) error {
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
		automationConditionOpEmpty,
		automationConditionOpNotEmpty,
		automationConditionOpStartsWith,
		automationConditionOpEndsWith,
		automationConditionOpNotStartsWith,
		automationConditionOpNotEndsWith,
		automationConditionOpContains,
	)); err != nil {
		return err
	}

	if op != automationConditionOpExists && op != automationConditionOpEmpty && op != automationConditionOpNotEmpty {
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
		duration := automationStepDuration(timeout)
		if duration <= 0 {
			return validation.NewError("validation_invalid_automation_http", "HTTP step timeout must be greater than zero.")
		}
		if duration > AutomationHTTPMaxTimeout {
			return validation.NewError(
				"validation_invalid_automation_http",
				fmt.Sprintf("HTTP step timeout must be %d seconds or less.", int(AutomationHTTPMaxTimeout.Seconds())),
			)
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
	if triggerType != AutomationTriggerRecordCreate &&
		triggerType != AutomationTriggerRecordUpdate &&
		triggerType != AutomationTriggerRecordBeforeCreate &&
		triggerType != AutomationTriggerRecordBeforeUpdate {
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

func validateAutomationResponseStep(automationRecord *Record, step map[string]any) error {
	if strings.TrimSpace(automationRecord.GetString("triggerType")) != AutomationTriggerWebhook {
		return validation.NewError("validation_invalid_automation_response", "Response steps require a webhook trigger.")
	}

	if _, err := automationResponseStatusCode(step["statusCode"], http.StatusOK); err != nil {
		return validation.NewError("validation_invalid_automation_response", err.Error())
	}

	if headers, ok := step["headers"]; ok {
		if _, valid := headers.(map[string]any); !valid {
			return validation.NewError("validation_invalid_automation_response", "Response step headers must be a JSON object.")
		}
	}

	return nil
}

func isRecordAutomationTrigger(triggerType string) bool {
	switch triggerType {
	case AutomationTriggerRecordBeforeCreate,
		AutomationTriggerRecordBeforeUpdate,
		AutomationTriggerRecordCreate,
		AutomationTriggerRecordUpdate,
		AutomationTriggerRecordDelete:
		return true
	default:
		return false
	}
}

func isI18nAutomationTrigger(triggerType string) bool {
	switch triggerType {
	case AutomationTriggerI18nMissing,
		AutomationTriggerI18nPublished,
		AutomationTriggerI18nUpdated,
		AutomationTriggerI18nAIFinished:
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
