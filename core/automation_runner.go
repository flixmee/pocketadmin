package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/inflector"
	"github.com/pocketbase/pocketbase/tools/routine"
	"github.com/pocketbase/pocketbase/tools/types"
)

type automationTriggerPayload struct {
	TriggerType    string         `json:"triggerType"`
	CollectionId   string         `json:"collectionId,omitempty"`
	CollectionName string         `json:"collectionName,omitempty"`
	Request        map[string]any `json:"request,omitempty"`
	I18n           map[string]any `json:"i18n,omitempty"`
	Record         map[string]any `json:"record,omitempty"`
	RecordOriginal map[string]any `json:"recordOriginal,omitempty"`
	triggerRecord  *Record        `json:"-"`
	originalRecord *Record        `json:"-"`
}

// AutomationWebhookRequest defines the normalized inbound webhook request payload.
type AutomationWebhookRequest struct {
	Method   string            `json:"method"`
	Path     string            `json:"path,omitempty"`
	Query    map[string]string `json:"query,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	Body     any               `json:"body,omitempty"`
	RemoteIP string            `json:"remoteIP,omitempty"`
}

// AutomationWebhookResponse defines the response returned to the webhook caller.
type AutomationWebhookResponse struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       any               `json:"body,omitempty"`
}

type automationStepResult struct {
	Index      int            `json:"index"`
	Type       string         `json:"type"`
	Status     string         `json:"status"`
	Output     any            `json:"output,omitempty"`
	Error      string         `json:"error,omitempty"`
	Started    types.DateTime `json:"started"`
	Finished   types.DateTime `json:"finished"`
	DurationMs int64          `json:"durationMs"`
}

func queueRecordAutomationRuns(app App, triggerType string, record *Record, original *Record) {
	if record == nil || record.Collection() == nil {
		return
	}

	if shouldSkipAutomationTriggerCollection(record.Collection().Name) {
		return
	}

	registry, err := getAutomationRegistry(app)
	if err != nil {
		app.Logger().Warn("Failed to load automation registry", "error", err)
		return
	}

	collectionRegistry := registry.ByTriggerScope[triggerType]
	if collectionRegistry == nil {
		return
	}

	automations := collectionRegistry[record.Collection().Id]
	if len(automations) == 0 {
		return
	}

	payload := newAutomationTriggerPayload(triggerType, record, original)

	for _, automation := range automations {
		if automation == nil {
			continue
		}

		nextAutomation := automation
		routine.FireAndForget(func() {
			if err := runAutomation(app, nextAutomation, payload); err != nil {
				app.Logger().Warn(
					"Failed to execute record-triggered automation",
					"automationId", nextAutomation.Id,
					"triggerType", triggerType,
					"error", err,
				)
			}
		})
	}
}

func queueI18nAutomationRuns(app App, triggerType string, record *Record, i18n map[string]any) {
	if i18n == nil {
		i18n = map[string]any{}
	}

	registry, err := getAutomationRegistry(app)
	if err != nil {
		app.Logger().Warn("Failed to load automation registry", "error", err)
		return
	}

	var collectionId string
	var collectionName string
	var triggerRecord *Record
	if record != nil && record.Collection() != nil {
		collectionId = record.Collection().Id
		collectionName = record.Collection().Name
		triggerRecord = record
	}
	if collectionId == "" {
		collectionId = toString(i18n["collectionId"])
	}
	if collectionName == "" && collectionId != "" {
		if collection, err := app.FindCachedCollectionByNameOrId(collectionId); err == nil {
			collectionName = collection.Name
		}
	}

	automations := []*Automation{}
	if collectionId != "" && registry.ByTriggerScope[triggerType] != nil {
		automations = append(automations, registry.ByTriggerScope[triggerType][collectionId]...)
	}
	automations = append(automations, registry.ByTrigger[triggerType]...)
	if len(automations) == 0 {
		return
	}

	payload := automationTriggerPayload{
		TriggerType:    triggerType,
		CollectionId:   collectionId,
		CollectionName: collectionName,
		I18n:           i18n,
		triggerRecord:  triggerRecord,
	}
	if triggerRecord != nil {
		payload.Record = automationTemplateRecordData(triggerRecord)
	}

	seen := map[string]struct{}{}
	for _, automation := range automations {
		if automation == nil {
			continue
		}
		if _, ok := seen[automation.Id]; ok {
			continue
		}
		seen[automation.Id] = struct{}{}

		nextAutomation := automation
		routine.FireAndForget(func() {
			if err := runAutomation(app, nextAutomation, payload); err != nil {
				app.Logger().Warn(
					"Failed to execute i18n-triggered automation",
					"automationId", nextAutomation.Id,
					"triggerType", triggerType,
					"error", err,
				)
			}
		})
	}
}

func runAutomationByID(app App, automationID string, payload automationTriggerPayload) error {
	registry, err := getAutomationRegistry(app)
	if err != nil {
		return err
	}

	automation := registry.ByID[automationID]
	if automation == nil {
		return fmt.Errorf("missing or inactive automation %q", automationID)
	}

	return runAutomation(app, automation, payload)
}

// RunAutomationManually runs the specified automation using the manual trigger type.
func (app *BaseApp) RunAutomationManually(automationID string) error {
	automation, err := app.FindAutomationById(automationID)
	if err != nil {
		return err
	}

	return runAutomation(app, automation, automationTriggerPayload{
		TriggerType: AutomationTriggerManual,
	})
}

// RunAutomationFromRun reruns the specified stored automation run using its saved trigger payload.
func (app *BaseApp) RunAutomationFromRun(runID string) error {
	run, err := app.FindAutomationRunById(runID)
	if err != nil {
		return err
	}

	automation, err := app.FindAutomationById(run.AutomationRef())
	if err != nil {
		return err
	}

	payload, err := decodeAutomationRunPayload(run)
	if err != nil {
		return err
	}

	return runAutomation(app, automation, payload)
}

// RunAutomationWebhook runs the specified active automation using the webhook trigger type.
func (app *BaseApp) RunAutomationWebhook(automationID string, request *AutomationWebhookRequest) (*AutomationWebhookResponse, error) {
	registry, err := getAutomationRegistry(app)
	if err != nil {
		return nil, err
	}

	automation := registry.ByID[automationID]
	if automation == nil || automation.TriggerType() != AutomationTriggerWebhook {
		return nil, fmt.Errorf("missing active webhook automation %q", automationID)
	}

	ctx, err := runAutomationWithContext(app, automation, automationTriggerPayload{
		TriggerType: AutomationTriggerWebhook,
		Request:     automationWebhookRequestData(request),
	})
	if err != nil {
		return nil, err
	}
	if ctx != nil && ctx.WebhookResponse != nil {
		return ctx.WebhookResponse, nil
	}

	return &AutomationWebhookResponse{StatusCode: http.StatusNoContent}, nil
}

func runAutomation(app App, automation *Automation, payload automationTriggerPayload) (err error) {
	_, err = runAutomationWithContext(app, automation, payload)
	return err
}

func runAutomationWithContext(app App, automation *Automation, payload automationTriggerPayload) (ctx *automationExecutionContext, err error) {
	if automation == nil {
		return nil, errors.New("missing automation")
	}

	var run *AutomationRun
	var stepResults []automationStepResult

	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("automation execution panicked: %v", rec)

			app.Logger().Error(
				"Automation execution panic",
				"automationId", automation.Id,
				"triggerType", payload.TriggerType,
				"error", err.Error(),
				"stack", string(debug.Stack()),
			)

			if run != nil {
				if finishErr := finalizeAutomationRun(app, automation, run, stepResults, err); finishErr != nil {
					app.Logger().Error(
						"Failed to persist automation panic state",
						"automationId", automation.Id,
						"runId", run.Id,
						"error", finishErr,
					)
				}
			}
		}
	}()

	inputRaw, err := toJSONRaw(payload)
	if err != nil {
		return nil, err
	}

	run = NewAutomationRun(app)
	run.SetAutomationRef(automation.Id)
	run.SetTriggerType(payload.TriggerType)
	run.SetStatus(AutomationRunStatusQueued)
	run.SetInput(inputRaw)
	run.ClearErrorStepIndex()

	if err := app.Save(run); err != nil {
		return nil, err
	}

	run.SetStatus(AutomationRunStatusRunning)
	if err := app.Save(run); err != nil {
		return nil, err
	}

	ctx = newAutomationExecutionContext(app, automation, run, payload)
	stepResults, err = executeAutomationSteps(ctx)
	if finishErr := finalizeAutomationRun(app, automation, run, stepResults, err); finishErr != nil {
		return ctx, finishErr
	}

	return ctx, err
}

func finalizeAutomationRun(app App, automation *Automation, run *AutomationRun, stepResults []automationStepResult, execErr error) error {
	if len(stepResults) > 0 {
		stepResultsRaw, err := toJSONRaw(stepResults)
		if err != nil {
			return err
		}
		run.SetStepResults(stepResultsRaw)
	}

	run.SetRaw("finished", types.NowDateTime())
	if execErr != nil {
		run.SetStatus(AutomationRunStatusFailed)
		run.SetError(execErr.Error())

		if index, ok := failedAutomationStepIndex(stepResults); ok {
			run.SetErrorStepIndex(index)
		} else {
			run.ClearErrorStepIndex()
		}
	} else {
		run.SetStatus(AutomationRunStatusSuccess)
		run.SetError("")
		run.ClearErrorStepIndex()
	}

	if err := app.Save(run); err != nil {
		return err
	}

	if err := updateAutomationLastRunState(app, automation.Id, run.Status(), run.Finished()); err != nil {
		app.Logger().Warn(
			"Failed to update automation last run state",
			"automationId", automation.Id,
			"error", err,
		)
	}

	return nil
}

func failedAutomationStepIndex(stepResults []automationStepResult) (int, bool) {
	for i := len(stepResults) - 1; i >= 0; i-- {
		if stepResults[i].Status == automationStepStatusFailed {
			return stepResults[i].Index, true
		}
	}

	return 0, false
}

func executeAutomationSteps(ctx *automationExecutionContext) ([]automationStepResult, error) {
	steps, err := decodeAutomationSteps(ctx.Automation.Steps().String())
	if err != nil {
		return nil, err
	}

	results := make([]automationStepResult, 0, len(steps))

	for i, step := range steps {
		stepType := strings.TrimSpace(toString(step["type"]))
		started := types.NowDateTime()
		status, output, err := executeAutomationStep(ctx, step)
		finished := types.NowDateTime()
		result := automationStepResult{
			Index:      i,
			Type:       stepType,
			Status:     status,
			Output:     output,
			Started:    started,
			Finished:   finished,
			DurationMs: finished.Sub(started).Milliseconds(),
		}

		if err != nil {
			result.Status = automationStepStatusFailed
			result.Error = err.Error()
			results = append(results, result)
			return results, err
		}

		results = append(results, result)
		ctx.appendStepTemplateResult(result)

		if stepType == AutomationStepResponse {
			return results, nil
		}

		if status == automationStepStatusStopped {
			return results, nil
		}
	}

	return results, nil
}

func updateAutomationLastRunState(app App, automationID string, status string, finished types.DateTime) error {
	automation, err := findAutomationByID(app, automationID)
	if err != nil {
		return err
	}

	automation.Set("lastRunStatus", status)
	automation.SetRaw("lastRunAt", finished)

	return app.Save(automation)
}

func newAutomationTriggerPayload(triggerType string, record *Record, original *Record) automationTriggerPayload {
	payload := automationTriggerPayload{
		TriggerType: triggerType,
	}

	if record != nil && record.Collection() != nil {
		payload.CollectionId = record.Collection().Id
		payload.CollectionName = record.Collection().Name
		payload.Record = automationTemplateRecordData(record)
		payload.triggerRecord = record
	}

	if original != nil {
		payload.RecordOriginal = automationTemplateRecordData(original)
		payload.originalRecord = original
	}

	return payload
}

func decodeAutomationRunPayload(run *AutomationRun) (automationTriggerPayload, error) {
	if run == nil {
		return automationTriggerPayload{}, errors.New("missing automation run")
	}

	raw := strings.TrimSpace(run.Input().String())
	if raw == "" {
		return automationTriggerPayload{}, fmt.Errorf("automation run %q is missing input payload", run.Id)
	}

	payload := automationTriggerPayload{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return automationTriggerPayload{}, fmt.Errorf("failed to decode automation run %q payload: %w", run.Id, err)
	}

	if strings.TrimSpace(payload.TriggerType) == "" {
		payload.TriggerType = run.TriggerType()
	}

	return payload, nil
}

func shouldSkipAutomationTriggerCollection(collectionName string) bool {
	switch collectionName {
	case CollectionNameAutomations,
		CollectionNameAutomationRuns,
		CollectionNameLocales,
		CollectionNameI18nGroups,
		CollectionNameTranslationJobs:
		return true
	default:
		return false
	}
}

func automationWebhookRequestData(request *AutomationWebhookRequest) map[string]any {
	if request == nil {
		return nil
	}

	data := map[string]any{}

	if method := strings.TrimSpace(request.Method); method != "" {
		data["method"] = method
	}
	if path := strings.TrimSpace(request.Path); path != "" {
		data["path"] = path
	}
	if remoteIP := strings.TrimSpace(request.RemoteIP); remoteIP != "" {
		data["remoteIP"] = remoteIP
	}
	if len(request.Query) > 0 {
		data["query"] = stringMapToAnyMap(request.Query)
	}
	if len(request.Headers) > 0 {
		headers := make(map[string]any, len(request.Headers))
		for key, value := range request.Headers {
			headers[inflector.Snakecase(key)] = value
		}
		data["headers"] = headers
	}
	if request.Body != nil {
		data["body"] = request.Body
	}

	if len(data) == 0 {
		return nil
	}

	return data
}

func stringMapToAnyMap(values map[string]string) map[string]any {
	if len(values) == 0 {
		return nil
	}

	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = value
	}

	return result
}

func toJSONRaw(value any) (types.JSONRaw, error) {
	return types.ParseJSONRaw(value)
}

func toString(value any) string {
	if value == nil {
		return ""
	}

	if v, ok := value.(string); ok {
		return v
	}

	return fmt.Sprint(value)
}

func findAutomationByID(app App, id string) (*Automation, error) {
	result := &Automation{}

	err := app.RecordQuery(CollectionNameAutomations).
		AndWhere(dbx.HashExp{"id": id}).
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
