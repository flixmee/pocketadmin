package core

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/types"
)

const CollectionNameCapabilities = "_capabilities"

var (
	capabilityKeyPattern     = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)+$`)
	capabilityVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
)

var (
	_ Model        = (*Capability)(nil)
	_ PreValidator = (*Capability)(nil)
	_ RecordProxy  = (*Capability)(nil)
)

// Capability defines a Record proxy for the capability registry collection.
type Capability struct {
	*Record
}

func NewCapability(app App) *Capability {
	m := &Capability{}

	c, err := app.FindCachedCollectionByNameOrId(CollectionNameCapabilities)
	if err != nil {
		c = NewBaseCollection("@__invalid_capabilities__")
	}

	m.Record = NewRecord(c)

	return m
}

func (m *Capability) PreValidate(ctx context.Context, app App) error {
	if m.Record == nil || m.Record.Collection().Name != CollectionNameCapabilities {
		return errors.New("missing or invalid Capability ProxyRecord")
	}

	return nil
}

func (m *Capability) ProxyRecord() *Record {
	return m.Record
}

func (m *Capability) SetProxyRecord(record *Record) {
	m.Record = record
}

func (m *Capability) Key() string {
	return m.GetString("key")
}

func (m *Capability) SetKey(key string) {
	m.Set("key", strings.TrimSpace(key))
}

func (m *Capability) Version() string {
	return m.GetString("version")
}

func (m *Capability) SetVersion(version string) {
	m.Set("version", strings.TrimSpace(version))
}

func (m *Capability) Category() string {
	return m.GetString("category")
}

func (m *Capability) SetCategory(category string) {
	m.Set("category", strings.TrimSpace(category))
}

func (m *Capability) Icon() string {
	return m.GetString("icon")
}

func (m *Capability) SetIcon(icon string) {
	m.Set("icon", strings.TrimSpace(icon))
}

func (m *Capability) InputSchema() types.JSONRaw {
	raw, _ := m.GetRaw("inputSchema").(types.JSONRaw)
	return raw
}

func (m *Capability) SetInputSchema(schema types.JSONRaw) {
	m.Set("inputSchema", schema)
}

func (m *Capability) OutputSchema() types.JSONRaw {
	raw, _ := m.GetRaw("outputSchema").(types.JSONRaw)
	return raw
}

func (m *Capability) SetOutputSchema(schema types.JSONRaw) {
	m.Set("outputSchema", schema)
}

func (m *Capability) AuthStrategy() string {
	return m.GetString("authStrategy")
}

func (m *Capability) SetAuthStrategy(strategy string) {
	m.Set("authStrategy", strings.TrimSpace(strategy))
}

func (m *Capability) RuntimeHandler() string {
	return m.GetString("runtimeHandler")
}

func (m *Capability) SetRuntimeHandler(handler string) {
	m.Set("runtimeHandler", strings.TrimSpace(handler))
}

func (m *Capability) ConfigUI() types.JSONRaw {
	raw, _ := m.GetRaw("configUI").(types.JSONRaw)
	return raw
}

func (m *Capability) SetConfigUI(config types.JSONRaw) {
	m.Set("configUI", config)
}

func (m *Capability) ConnectorRef() string {
	return m.GetString("connectorRef")
}

func (m *Capability) SetConnectorRef(id string) {
	m.Set("connectorRef", strings.TrimSpace(id))
}

func (m *Capability) RequiredScopes() types.JSONRaw {
	raw, _ := m.GetRaw("requiredScopes").(types.JSONRaw)
	return raw
}

func (m *Capability) SetRequiredScopes(scopes types.JSONRaw) {
	m.Set("requiredScopes", scopes)
}

func (m *Capability) Active() bool {
	return m.GetBool("active")
}

func (m *Capability) SetActive(active bool) {
	m.Set("active", active)
}

func (app *BaseApp) FindCapabilityById(id string) (*Capability, error) {
	result := &Capability{}

	err := app.RecordQuery(CollectionNameCapabilities).
		AndWhere(dbx.HashExp{"id": id}).
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (app *BaseApp) FindCapabilityByKey(key string) (*Capability, error) {
	result := &Capability{}

	err := app.RecordQuery(CollectionNameCapabilities).
		AndWhere(dbx.HashExp{"key": strings.TrimSpace(key), "active": true}).
		OrderBy("version DESC").
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (app *BaseApp) FindAllActiveCapabilities() ([]*Capability, error) {
	result := []*Capability{}

	err := app.RecordQuery(CollectionNameCapabilities).
		AndWhere(dbx.HashExp{"active": true}).
		OrderBy("category ASC", "key ASC", "version DESC").
		All(&result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (app *BaseApp) registerCapabilityHooks() {
	app.OnRecordValidate(CollectionNameCapabilities).Bind(&hook.Handler[*RecordEvent]{
		Id:       "pbAutomationCapabilityValidate",
		Priority: -10,
		Func: func(e *RecordEvent) error {
			if err := validateCapabilityRecord(e.App, e.Record); err != nil {
				return err
			}

			return e.Next()
		},
	})
}

func validateCapabilityRecord(app App, record *Record) error {
	errs := validation.Errors{}

	key := strings.TrimSpace(record.GetString("key"))
	if err := validation.Validate(key, validation.Required, validation.Match(capabilityKeyPattern)); err != nil {
		errs["key"] = err
	}

	version := strings.TrimSpace(record.GetString("version"))
	if err := validation.Validate(version, validation.Required, validation.Match(capabilityVersionPattern)); err != nil {
		errs["version"] = err
	}

	if err := validation.Validate(strings.TrimSpace(record.GetString("category")), validation.Required); err != nil {
		errs["category"] = err
	}

	if err := validateCapabilityJSONSchemaField(record, "inputSchema", false); err != nil {
		errs["inputSchema"] = err
	}
	if err := validateCapabilityJSONSchemaField(record, "outputSchema", false); err != nil {
		errs["outputSchema"] = err
	}
	if err := validateCapabilityJSONSchemaField(record, "configUI", true); err != nil {
		errs["configUI"] = err
	}
	if err := validateConnectorJSONArray(record, "requiredScopes", true); err != nil {
		errs["requiredScopes"] = err
	}

	if err := validation.Validate(strings.TrimSpace(record.GetString("runtimeHandler")), validation.Required); err != nil {
		errs["runtimeHandler"] = err
	}

	if key != "" && version != "" {
		existing := &Capability{}
		query := app.RecordQuery(CollectionNameCapabilities).
			AndWhere(dbx.HashExp{"key": key, "version": version}).
			Limit(1)
		if record.Id != "" {
			query.AndWhere(dbx.Not(dbx.HashExp{"id": record.Id}))
		}
		if err := query.One(existing); err == nil {
			errs["key"] = validation.NewError("validation_duplicate_capability", "Capability key and version must be unique.")
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func validateCapabilityJSONSchemaField(record *Record, field string, optional bool) error {
	raw, ok := record.GetRaw(field).(types.JSONRaw)
	if !ok || strings.TrimSpace(raw.String()) == "" || strings.TrimSpace(raw.String()) == "null" {
		if optional {
			return nil
		}
		return validation.NewError("validation_required", "Missing required value.")
	}

	value := map[string]any{}
	if err := json.Unmarshal([]byte(raw.String()), &value); err != nil {
		return validation.NewError("validation_invalid_json_schema", "Schema must be a JSON object.")
	}

	return nil
}
