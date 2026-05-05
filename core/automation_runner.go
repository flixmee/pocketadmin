package core

import (
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/routine"
	"github.com/pocketbase/pocketbase/tools/types"
)

type automationTriggerPayload struct {
	TriggerType    string         `json:"triggerType"`
	CollectionId   string         `json:"collectionId,omitempty"`
	CollectionName string         `json:"collectionName,omitempty"`
	Record         map[string]any `json:"record,omitempty"`
	RecordOriginal map[string]any `json:"recordOriginal,omitempty"`
	triggerRecord  *Record        `json:"-"`
	originalRecord *Record        `json:"-"`
}

type automationStepResult struct {
	Index      int            `json:"index"`
	Type       string         `json:"type"`
	Status     string         `json:"status"`
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

func runAutomation(app App, automation *Automation, payload automationTriggerPayload) (err error) {
	if automation == nil {
		return errors.New("missing automation")
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
		return err
	}

	run = NewAutomationRun(app)
	run.SetAutomationRef(automation.Id)
	run.SetTriggerType(payload.TriggerType)
	run.SetStatus(AutomationRunStatusQueued)
	run.SetInput(inputRaw)
	run.ClearErrorStepIndex()

	if err := app.Save(run); err != nil {
		return err
	}

	run.SetStatus(AutomationRunStatusRunning)
	if err := app.Save(run); err != nil {
		return err
	}

	stepResults, err = executeAutomationSteps(newAutomationExecutionContext(app, automation, run, payload))
	if finishErr := finalizeAutomationRun(app, automation, run, stepResults, err); finishErr != nil {
		return finishErr
	}

	return err
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
		status, err := executeAutomationStep(ctx, step)
		finished := types.NowDateTime()
		result := automationStepResult{
			Index:      i,
			Type:       stepType,
			Status:     status,
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

func shouldSkipAutomationTriggerCollection(collectionName string) bool {
	return collectionName == CollectionNameAutomations || collectionName == CollectionNameAutomationRuns
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
