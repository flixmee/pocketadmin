package core

import (
	"context"
	"errors"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/types"
)

const CollectionNameConnectors = "_connectors"

const (
	AutomationConnectorAuthAPIKey = "apiKey"
	AutomationConnectorAuthBearer = "bearer"
)

var (
	_ Model        = (*Connector)(nil)
	_ PreValidator = (*Connector)(nil)
	_ RecordProxy  = (*Connector)(nil)
)

type Connector struct {
	*Record
}

func NewConnector(app App) *Connector {
	m := &Connector{}

	c, err := app.FindCachedCollectionByNameOrId(CollectionNameConnectors)
	if err != nil {
		c = NewBaseCollection("@__invalid_connectors__")
	}

	m.Record = NewRecord(c)

	return m
}

func (m *Connector) PreValidate(ctx context.Context, app App) error {
	if m.Record == nil || m.Record.Collection().Name != CollectionNameConnectors {
		return errors.New("missing or invalid Connector ProxyRecord")
	}

	return nil
}

func (m *Connector) ProxyRecord() *Record {
	return m.Record
}

func (m *Connector) SetProxyRecord(record *Record) {
	m.Record = record
}

func (m *Connector) Provider() string {
	return m.GetString("provider")
}

func (m *Connector) SetProvider(provider string) {
	m.Set("provider", strings.TrimSpace(provider))
}

func (m *Connector) AuthType() string {
	return m.GetString("authType")
}

func (m *Connector) SetAuthType(authType string) {
	m.Set("authType", strings.TrimSpace(authType))
}

func (m *Connector) Credentials() types.JSONRaw {
	raw, _ := m.GetRaw("credentials").(types.JSONRaw)
	return raw
}

func (m *Connector) SetCredentials(credentials types.JSONRaw) {
	m.Set("credentials", credentials)
}

func (m *Connector) Scopes() types.JSONRaw {
	raw, _ := m.GetRaw("scopes").(types.JSONRaw)
	return raw
}

func (m *Connector) SetScopes(scopes types.JSONRaw) {
	m.Set("scopes", scopes)
}

func (m *Connector) RateLimits() types.JSONRaw {
	raw, _ := m.GetRaw("rateLimits").(types.JSONRaw)
	return raw
}

func (m *Connector) SetRateLimits(rateLimits types.JSONRaw) {
	m.Set("rateLimits", rateLimits)
}

func (m *Connector) Active() bool {
	return m.GetBool("active")
}

func (m *Connector) SetActive(active bool) {
	m.Set("active", active)
}

func (app *BaseApp) FindConnectorById(id string) (*Connector, error) {
	result := &Connector{}
	err := app.RecordQuery(CollectionNameConnectors).
		AndWhere(dbx.HashExp{"id": id}).
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (app *BaseApp) registerConnectorHooks() {
	app.OnRecordValidate(CollectionNameConnectors).Bind(&hook.Handler[*RecordEvent]{
		Id:       "pbAutomationConnectorValidate",
		Priority: -10,
		Func: func(e *RecordEvent) error {
			if err := validateConnectorRecord(e.Record); err != nil {
				return err
			}

			return e.Next()
		},
	})
}

func validateConnectorRecord(record *Record) error {
	errs := validation.Errors{}

	if err := validation.Validate(strings.TrimSpace(record.GetString("provider")), validation.Required); err != nil {
		errs["provider"] = err
	}

	authType := strings.TrimSpace(record.GetString("authType"))
	if err := validation.Validate(authType, validation.Required, validation.In(AutomationConnectorAuthAPIKey, AutomationConnectorAuthBearer)); err != nil {
		errs["authType"] = err
	}

	if err := validateConnectorJSONArray(record, "scopes", true); err != nil {
		errs["scopes"] = err
	}
	if err := validateConnectorJSONObject(record, "credentials", false); err != nil {
		errs["credentials"] = err
	}
	if err := validateConnectorJSONObject(record, "rateLimits", true); err != nil {
		errs["rateLimits"] = err
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}
