package core

import (
	"encoding/json"
	"strings"

	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/tools/types"
)

func validateConnectorJSONObject(record *Record, field string, optional bool) error {
	raw, ok := record.GetRaw(field).(types.JSONRaw)
	if !ok || strings.TrimSpace(raw.String()) == "" || strings.TrimSpace(raw.String()) == "null" {
		if optional {
			return nil
		}
		return validation.NewError("validation_required", "Missing required value.")
	}

	var decoded any
	if err := json.Unmarshal([]byte(raw.String()), &decoded); err != nil {
		return validation.NewError("validation_invalid_json", "Invalid JSON value.")
	}
	if _, ok := decoded.(map[string]any); !ok {
		return validation.NewError("validation_invalid_json_object", "Value must be a JSON object.")
	}

	return nil
}

func validateConnectorJSONArray(record *Record, field string, optional bool) error {
	raw, ok := record.GetRaw(field).(types.JSONRaw)
	if !ok || strings.TrimSpace(raw.String()) == "" || strings.TrimSpace(raw.String()) == "null" {
		if optional {
			return nil
		}
		return validation.NewError("validation_required", "Missing required value.")
	}

	var decoded any
	if err := json.Unmarshal([]byte(raw.String()), &decoded); err != nil {
		return validation.NewError("validation_invalid_json", "Invalid JSON value.")
	}
	if _, ok := decoded.([]any); !ok {
		return validation.NewError("validation_invalid_json_array", "Value must be a JSON array.")
	}

	return nil
}
