package core

import (
	"encoding/json"
	"fmt"
	"strings"

	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/tools/types"
)

// DetachedSteps returns the editor-only groups of steps that are not
// connected to the automation flow.
//
// Detached steps are kept so that the workflow editor can restore the
// unconnected canvas nodes, but they are never executed.
func (m *Automation) DetachedSteps() types.JSONRaw {
	raw, _ := m.GetRaw("detachedSteps").(types.JSONRaw)
	return raw
}

// SetDetachedSteps updates the serialized detached step groups.
func (m *Automation) SetDetachedSteps(raw types.JSONRaw) {
	m.Set("detachedSteps", raw)
}

func validateAutomationDetachedSteps(value any) error {
	var raw []byte
	switch v := value.(type) {
	case nil:
		return nil
	case types.JSONRaw:
		raw = v
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return validation.NewError("validation_invalid_detached_steps", "Unconnected steps must be a list of step groups.")
		}
		raw = encoded
	}

	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}

	groups := [][]map[string]any{}
	if err := json.Unmarshal([]byte(trimmed), &groups); err != nil {
		return validation.NewError("validation_invalid_detached_steps", "Unconnected steps must be a list of step groups.")
	}

	total := 0
	for _, group := range groups {
		total += automationStepCount(group)
		for _, step := range group {
			stepType := strings.TrimSpace(toString(step["type"]))
			if err := validation.In(toAnySlice(automationStepTypes)...).Validate(stepType); err != nil || stepType == "" {
				return validation.NewError("validation_invalid_detached_steps", fmt.Sprintf("Unconnected step has an unsupported type %q.", stepType))
			}
		}
	}

	if total > AutomationMaxSteps {
		return validation.NewError("validation_invalid_detached_steps", fmt.Sprintf("Unconnected steps cannot exceed %d steps.", AutomationMaxSteps))
	}

	return nil
}
