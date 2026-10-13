package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/tools/routine"
	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	// AutomationStepCall runs another manual-trigger automation.
	AutomationStepCall = "automation.call"

	AutomationCallModeWait     = "wait"
	AutomationCallModeParallel = "parallel"

	AutomationManualInputText    = "text"
	AutomationManualInputNumber  = "number"
	AutomationManualInputBoolean = "boolean"
	AutomationManualInputJSON    = "json"

	AutomationMaxManualInputs = 50
)

var (
	automationManualInputTypes = []string{
		AutomationManualInputText,
		AutomationManualInputNumber,
		AutomationManualInputBoolean,
		AutomationManualInputJSON,
	}
	automationManualInputNamePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

// AutomationManualInput describes a single input field accepted by a
// manual-trigger automation (either when run from the UI/API or when
// called by another automation).
type AutomationManualInput struct {
	Name        string `json:"name"`
	Label       string `json:"label,omitempty"`
	Type        string `json:"type"`
	Required    bool   `json:"required,omitempty"`
	Default     any    `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
}

// AutomationManualRunOptions defines the options for running a manual automation.
type AutomationManualRunOptions struct {
	// Input is the data exposed to the automation as `input`.
	Input map[string]any `json:"input,omitempty"`

	// Wait blocks until the run finishes and returns its output.
	// When false the run is executed in the background.
	Wait bool `json:"wait,omitempty"`
}

// AutomationCallResult describes the outcome of a manual run or an automation call.
type AutomationCallResult struct {
	AutomationId   string `json:"automationId"`
	AutomationName string `json:"automationName,omitempty"`
	RunId          string `json:"runId"`
	Mode           string `json:"mode"`
	Status         string `json:"status"`
	Output         any    `json:"output,omitempty"`
	Error          string `json:"error,omitempty"`
}

// ManualInputs returns the configured manual trigger input fields.
func (m *Automation) ManualInputs() []AutomationManualInput {
	inputs, _ := decodeAutomationManualInputs(m.GetRaw("manualInputs"))
	return inputs
}

// SetManualInputs updates the manual trigger input fields.
func (m *Automation) SetManualInputs(inputs []AutomationManualInput) {
	if inputs == nil {
		inputs = []AutomationManualInput{}
	}
	raw, _ := toJSONRaw(inputs)
	m.Set("manualInputs", raw)
}

func automationManualInputsRaw(automation *Automation) types.JSONRaw {
	if automation == nil || automation.TriggerType() != AutomationTriggerManual {
		return nil
	}

	inputs := automation.ManualInputs()
	if len(inputs) == 0 {
		return nil
	}

	raw, _ := toJSONRaw(inputs)
	return raw
}

func decodeAutomationManualInputs(value any) ([]AutomationManualInput, error) {
	var raw []byte
	switch v := value.(type) {
	case nil:
		return nil, nil
	case types.JSONRaw:
		raw = v
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		raw = encoded
	}

	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}

	inputs := []AutomationManualInput{}
	if err := json.Unmarshal([]byte(trimmed), &inputs); err != nil {
		return nil, err
	}

	return inputs, nil
}

func validateAutomationManualInputs(value any) error {
	inputs, err := decodeAutomationManualInputs(value)
	if err != nil {
		return validation.NewError("validation_invalid_manual_inputs", "Manual inputs must be a list of input definitions.")
	}

	if len(inputs) > AutomationMaxManualInputs {
		return validation.NewError("validation_invalid_manual_inputs", fmt.Sprintf("Manual inputs cannot exceed %d fields.", AutomationMaxManualInputs))
	}

	seen := map[string]struct{}{}
	for i, input := range inputs {
		name := strings.TrimSpace(input.Name)
		if !automationManualInputNamePattern.MatchString(name) {
			return validation.NewError(
				"validation_invalid_manual_input_name",
				fmt.Sprintf("Input %d: name must start with a letter or underscore and contain only letters, digits and underscores.", i+1),
			)
		}
		if _, ok := seen[name]; ok {
			return validation.NewError("validation_duplicate_manual_input", fmt.Sprintf("Input %q is defined more than once.", name))
		}
		seen[name] = struct{}{}

		if err := validation.In(toAnySlice(automationManualInputTypes)...).Validate(input.Type); err != nil || input.Type == "" {
			return validation.NewError("validation_invalid_manual_input_type", fmt.Sprintf("Input %q has an unsupported type.", name))
		}

		if input.Default != nil {
			if _, err := coerceAutomationManualInputValue(input, input.Default); err != nil {
				return validation.NewError("validation_invalid_manual_input_default", fmt.Sprintf("Input %q default value: %s", name, err.Error()))
			}
		}
	}

	return nil
}

// prepareAutomationManualInput applies the declared defaults, checks the
// required fields and coerces the values to their declared types.
//
// Keys that are not declared are passed through unchanged.
func prepareAutomationManualInput(automation *Automation, input map[string]any) (map[string]any, error) {
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}

	if automation == nil {
		return result, nil
	}

	errs := validation.Errors{}
	for _, def := range automation.ManualInputs() {
		value, exists := result[def.Name]
		if !exists || isEmptyAutomationManualInputValue(value) {
			if def.Default != nil {
				value = def.Default
			} else if def.Required {
				errs[def.Name] = validation.NewError("validation_required", "Cannot be blank.")
				continue
			} else {
				if exists {
					result[def.Name] = nil
				}
				continue
			}
		}

		coerced, err := coerceAutomationManualInputValue(def, value)
		if err != nil {
			errs[def.Name] = validation.NewError("validation_invalid_input_value", err.Error())
			continue
		}
		result[def.Name] = coerced
	}

	if len(errs) > 0 {
		return nil, errs
	}

	return result, nil
}

func isEmptyAutomationManualInputValue(value any) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}

func coerceAutomationManualInputValue(def AutomationManualInput, value any) (any, error) {
	switch def.Type {
	case AutomationManualInputText:
		switch v := value.(type) {
		case string:
			return v, nil
		case bool, float64, float32, int, int64, json.Number:
			return fmt.Sprint(v), nil
		default:
			return nil, errors.New("must be a text value")
		}
	case AutomationManualInputNumber:
		switch v := value.(type) {
		case float64:
			return v, nil
		case float32:
			return float64(v), nil
		case int:
			return float64(v), nil
		case int64:
			return float64(v), nil
		case json.Number:
			return v.Float64()
		case string:
			number, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				return nil, errors.New("must be a number")
			}
			return number, nil
		default:
			return nil, errors.New("must be a number")
		}
	case AutomationManualInputBoolean:
		switch v := value.(type) {
		case bool:
			return v, nil
		case string:
			parsed, err := strconv.ParseBool(strings.TrimSpace(v))
			if err != nil {
				return nil, errors.New("must be a boolean")
			}
			return parsed, nil
		case float64:
			return v != 0, nil
		default:
			return nil, errors.New("must be a boolean")
		}
	case AutomationManualInputJSON:
		// plain JSON strings coming from forms are decoded when possible
		if text, ok := value.(string); ok {
			trimmed := strings.TrimSpace(text)
			if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
				var decoded any
				if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
					return nil, errors.New("must be valid JSON")
				}
				return decoded, nil
			}
		}
		return value, nil
	default:
		return value, nil
	}
}

// RunAutomationManuallyWithOptions runs the specified automation using the
// manual trigger type and the provided input data.
//
// Input validation errors are returned before the run is created.
func (app *BaseApp) RunAutomationManuallyWithOptions(automationID string, options AutomationManualRunOptions) (*AutomationCallResult, error) {
	automation, err := app.FindAutomationById(automationID)
	if err != nil {
		return nil, err
	}

	input, err := prepareAutomationManualInput(automation, options.Input)
	if err != nil {
		return nil, err
	}

	payload := automationTriggerPayload{
		TriggerType: AutomationTriggerManual,
		Input:       input,
		runId:       GenerateDefaultRandomId(),
	}

	if !options.Wait {
		routine.FireAndForget(func() {
			if err := runAutomation(app, automation, payload); err != nil {
				app.Logger().Warn(
					"Failed to execute automation manual run",
					"automationId", automation.Id,
					"error", err,
				)
			}
		})

		return &AutomationCallResult{
			AutomationId:   automation.Id,
			AutomationName: automation.Name(),
			RunId:          payload.runId,
			Mode:           AutomationCallModeParallel,
			Status:         AutomationRunStatusQueued,
		}, nil
	}

	result := runAutomationCall(app, automation, payload)
	if result.Error != "" {
		return result, errors.New(result.Error)
	}

	return result, nil
}

// runAutomationCall synchronously runs the automation and summarizes its result.
//
// The currently active policy context (if any) is restored afterwards so
// that the calling run keeps its own context.
func runAutomationCall(app App, automation *Automation, payload automationTriggerPayload) *AutomationCallResult {
	previous, hadPrevious := app.Store().Get(storeKeyAutomationPolicyContext).(automationPolicyContext)
	defer func() {
		if hadPrevious {
			app.Store().Set(storeKeyAutomationPolicyContext, previous)
		}
	}()

	result := &AutomationCallResult{
		AutomationId:   automation.Id,
		AutomationName: automation.Name(),
		RunId:          payload.runId,
		Mode:           AutomationCallModeWait,
	}

	runCtx, err := runAutomationWithContext(app, automation, payload)
	if runCtx != nil && runCtx.Run != nil {
		result.RunId = runCtx.Run.Id
		result.Status = runCtx.Run.Status()
		result.Output = automationCallOutput(runCtx)
	}
	if err != nil {
		result.Status = AutomationRunStatusFailed
		result.Error = err.Error()
	}
	if result.Status == "" {
		result.Status = AutomationRunStatusSuccess
	}

	return result
}

// automationCallOutput returns the explicit response body (if a response step
// was executed) or the output of the last executed step.
func automationCallOutput(ctx *automationExecutionContext) any {
	if data := automationExecutionData(ctx); data != nil {
		return data
	}

	if ctx.WebhookResponse != nil {
		return ctx.WebhookResponse.Body
	}

	if prevStep, ok := ctx.TemplateData["prevStep"].(map[string]any); ok {
		return prevStep["output"]
	}

	return nil
}

type automationCallStepConfig struct {
	Target          *Automation
	Mode            string
	Input           map[string]any
	ContinueOnError bool
}

func resolveAutomationCallStep(ctx *automationExecutionContext, step map[string]any) (*automationCallStepConfig, error) {
	targetRef, err := renderAutomationTemplateString(strings.TrimSpace(toString(step["automation"])), ctx.TemplateData)
	if err != nil {
		return nil, err
	}
	targetId := strings.TrimSpace(toString(targetRef))
	if targetId == "" {
		return nil, errors.New("call step requires an automation")
	}

	target, err := findAutomationByID(ctx.App, targetId)
	if err != nil {
		return nil, fmt.Errorf("missing automation %q", targetId)
	}
	if target.TriggerType() != AutomationTriggerManual {
		return nil, fmt.Errorf("automation %q must use the manual trigger to be called", target.Name())
	}
	if !target.Active() {
		return nil, fmt.Errorf("automation %q is inactive", target.Name())
	}

	mode, err := normalizeAutomationCallMode(step["mode"])
	if err != nil {
		return nil, err
	}

	rawInput := step["input"]
	if rawInput == nil {
		rawInput = map[string]any{}
	}
	if _, ok := rawInput.(map[string]any); !ok {
		return nil, errors.New("call step input must be a JSON object")
	}
	rendered, err := renderAutomationTemplateValue(rawInput, ctx.TemplateData)
	if err != nil {
		return nil, err
	}
	renderedInput, _ := rendered.(map[string]any)

	input, err := prepareAutomationManualInput(target, renderedInput)
	if err != nil {
		return nil, fmt.Errorf("invalid input for automation %q: %s", target.Name(), formatAutomationValidationErrors(err))
	}

	continueOnError, _ := step["continueOnError"].(bool)

	return &automationCallStepConfig{
		Target:          target,
		Mode:            mode,
		Input:           input,
		ContinueOnError: continueOnError,
	}, nil
}

func normalizeAutomationCallMode(value any) (string, error) {
	mode := strings.TrimSpace(toString(value))
	switch mode {
	case "", AutomationCallModeWait:
		return AutomationCallModeWait, nil
	case AutomationCallModeParallel:
		return AutomationCallModeParallel, nil
	default:
		return "", fmt.Errorf("unsupported call mode %q", mode)
	}
}

func executeAutomationCallStep(ctx *automationExecutionContext, step map[string]any) (map[string]any, error) {
	config, err := resolveAutomationCallStep(ctx, step)
	if err != nil {
		return nil, err
	}

	parentRunId := ""
	depth := 1
	if ctx.Run != nil {
		parentRunId = ctx.Run.Id
		depth = ctx.Run.Depth() + 1
	}

	payload := automationTriggerPayload{
		TriggerType: AutomationTriggerManual,
		ParentRunId: parentRunId,
		Depth:       depth,
		Input:       config.Input,
		Caller: map[string]any{
			"automationId":   ctx.Automation.Id,
			"automationName": ctx.Automation.Name(),
			"runId":          parentRunId,
			"mode":           config.Mode,
		},
		runId: GenerateDefaultRandomId(),
	}

	if config.Mode == AutomationCallModeParallel {
		startAutomationCallInBackground(ctx.App, config.Target, payload)

		return automationCallResultOutput(&AutomationCallResult{
			AutomationId:   config.Target.Id,
			AutomationName: config.Target.Name(),
			RunId:          payload.runId,
			Mode:           AutomationCallModeParallel,
			Status:         AutomationRunStatusQueued,
		}), nil
	}

	result := runAutomationCall(ctx.App, config.Target, payload)
	output := automationCallResultOutput(result)
	if result.Error != "" && !config.ContinueOnError {
		return output, fmt.Errorf("called automation %q failed: %s", config.Target.Name(), result.Error)
	}

	return output, nil
}

// startAutomationCallInBackground runs the automation in a separate goroutine.
//
// When invoked from within a transaction the run is deferred until the
// transaction completes successfully to avoid using a stale tx app.
func startAutomationCallInBackground(app App, automation *Automation, payload automationTriggerPayload) {
	fire := func(runApp App) {
		routine.FireAndForget(func() {
			if err := runAutomation(runApp, automation, payload); err != nil {
				runApp.Logger().Warn(
					"Failed to execute called automation",
					"automationId", automation.Id,
					"parentRunId", payload.ParentRunId,
					"error", err,
				)
			}
		})
	}

	if txInfo := app.TxInfo(); txInfo != nil && txInfo.parent != nil {
		parent := txInfo.parent
		txInfo.OnComplete(func(txErr error) error {
			if txErr == nil {
				fire(parent)
			}
			return nil
		})
		return
	}

	fire(app)
}

func automationCallResultOutput(result *AutomationCallResult) map[string]any {
	output := map[string]any{
		"automationId":   result.AutomationId,
		"automationName": result.AutomationName,
		"runId":          result.RunId,
		"mode":           result.Mode,
		"status":         result.Status,
	}
	if result.Output != nil {
		output["output"] = result.Output
	}
	if result.Error != "" {
		output["error"] = result.Error
	}

	return output
}

func previewAutomationCallStep(ctx *automationExecutionContext, step map[string]any) (map[string]any, error) {
	config, err := resolveAutomationCallStep(ctx, step)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"dryRun":         true,
		"automationId":   config.Target.Id,
		"automationName": config.Target.Name(),
		"mode":           config.Mode,
		"input":          config.Input,
	}, nil
}

func validateAutomationCallStep(app App, step map[string]any) error {
	targetId := strings.TrimSpace(toString(step["automation"]))
	if targetId == "" {
		return validation.NewError("validation_invalid_automation_call", "Call step requires an automation.")
	}

	// dynamic targets are resolved at runtime
	if !strings.Contains(targetId, "{{") {
		target, err := findAutomationByID(app, targetId)
		if err != nil {
			return validation.NewError("validation_invalid_automation_call", "The selected automation doesn't exist.")
		}
		if target.TriggerType() != AutomationTriggerManual {
			return validation.NewError("validation_invalid_automation_call", "Only automations with a manual trigger can be called.")
		}
	}

	if _, err := normalizeAutomationCallMode(step["mode"]); err != nil {
		return validation.NewError("validation_invalid_automation_call", "Call mode must be either wait or parallel.")
	}

	if input, ok := step["input"]; ok && input != nil {
		if _, valid := input.(map[string]any); !valid {
			return validation.NewError("validation_invalid_automation_call", "Call step input must be a JSON object.")
		}
	}

	if value, ok := step["continueOnError"]; ok && value != nil {
		if _, valid := value.(bool); !valid {
			return validation.NewError("validation_invalid_automation_call", "Call step continueOnError must be a boolean.")
		}
	}

	return nil
}

func formatAutomationValidationErrors(err error) string {
	var errs validation.Errors
	if !errors.As(err, &errs) {
		return err.Error()
	}

	keys := make([]string, 0, len(errs))
	for key := range errs {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+": "+errs[key].Error())
	}

	return strings.Join(parts, "; ")
}

func automationCallStepSchema() AutomationSchema {
	return AutomationSchema{
		Key:      AutomationStepCall,
		Label:    "Call automation",
		Category: "control",
		InputSchema: objectSchema(map[string]any{
			"type":            constStringSchema(AutomationStepCall),
			"automation":      stringSchema(),
			"mode":            enumSchema([]string{AutomationCallModeWait, AutomationCallModeParallel}),
			"input":           objectSchema(nil),
			"continueOnError": boolSchema(),
		}),
		OutputSchema: objectSchema(map[string]any{
			"automationId":   stringSchema(),
			"automationName": stringSchema(),
			"runId":          stringSchema(),
			"mode":           stringSchema(),
			"status":         stringSchema(),
			"output":         map[string]any{},
			"error":          stringSchema(),
		}),
	}
}

func manualTriggerSchema() AutomationSchema {
	schema := simpleTriggerSchema(AutomationTriggerManual, "Manual")
	schema.OutputSchema = objectSchema(map[string]any{
		"trigger": objectSchema(map[string]any{
			"type":   stringSchema(),
			"input":  objectSchema(nil),
			"caller": objectSchema(nil),
		}),
		"input": objectSchema(nil),
	})
	return schema
}
