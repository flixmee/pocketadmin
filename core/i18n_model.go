package core

import (
	"context"
	"errors"
	"regexp"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	CollectionNameLocales    = "_locales"
	CollectionNameI18nGroups = "_i18nGroups"

	DefaultLocaleCode = "en"

	FieldNameI18nGroupId = "i18n_group_id"
	FieldNameLocale      = "locale"
	FieldNameLocaleLinks = "localeLinks"
	FieldNameIsSource    = "is_source"
)

var localeCodeRegex = regexp.MustCompile(`^[a-z]{2,3}(-[A-Z0-9]{2,8})?$`)

var (
	_ Model        = (*Locale)(nil)
	_ PreValidator = (*Locale)(nil)
	_ RecordProxy  = (*Locale)(nil)

	_ Model        = (*I18nGroup)(nil)
	_ PreValidator = (*I18nGroup)(nil)
	_ RecordProxy  = (*I18nGroup)(nil)
)

type Locale struct {
	*Record
}

func NewLocale(app App) *Locale {
	m := &Locale{}

	c, err := app.FindCachedCollectionByNameOrId(CollectionNameLocales)
	if err != nil {
		c = NewBaseCollection("@__invalid_locales__")
	}

	m.Record = NewRecord(c)

	return m
}

func (m *Locale) PreValidate(ctx context.Context, app App) error {
	if m.Record == nil || m.Record.Collection().Name != CollectionNameLocales {
		return errors.New("missing or invalid Locale ProxyRecord")
	}

	return nil
}

func (m *Locale) ProxyRecord() *Record {
	return m.Record
}

func (m *Locale) SetProxyRecord(record *Record) {
	m.Record = record
}

func (m *Locale) Code() string {
	return m.GetString("code")
}

func (m *Locale) SetCode(code string) {
	m.Set("code", normalizeLocaleCode(code))
}

func (m *Locale) Name() string {
	return m.GetString("label")
}

func (m *Locale) SetName(name string) {
	m.Set("label", strings.TrimSpace(name))
}

func (m *Locale) Enabled() bool {
	return m.GetBool("enabled")
}

func (m *Locale) SetEnabled(enabled bool) {
	m.Set("enabled", enabled)
}

func (m *Locale) IsDefault() bool {
	return m.GetBool("is_default")
}

func (m *Locale) SetIsDefault(isDefault bool) {
	m.Set("is_default", isDefault)
	if isDefault {
		m.SetEnabled(true)
	}
}

type I18nGroup struct {
	*Record
}

func NewI18nGroup(app App) *I18nGroup {
	m := &I18nGroup{}

	c, err := app.FindCachedCollectionByNameOrId(CollectionNameI18nGroups)
	if err != nil {
		c = NewBaseCollection("@__invalid_i18n_groups__")
	}

	m.Record = NewRecord(c)

	return m
}

func (m *I18nGroup) PreValidate(ctx context.Context, app App) error {
	if m.Record == nil || m.Record.Collection().Name != CollectionNameI18nGroups {
		return errors.New("missing or invalid I18nGroup ProxyRecord")
	}

	return nil
}

func (m *I18nGroup) ProxyRecord() *Record {
	return m.Record
}

func (m *I18nGroup) SetProxyRecord(record *Record) {
	m.Record = record
}

func (m *I18nGroup) CollectionName() string {
	return m.GetString("collectionRef")
}

func (m *I18nGroup) SetCollectionName(collectionName string) {
	m.Set("collectionRef", collectionName)
}

func (m *I18nGroup) DefaultLocale() string {
	return m.GetString("defaultLocale")
}

func (m *I18nGroup) SetDefaultLocale(locale string) {
	m.Set("defaultLocale", normalizeLocaleCode(locale))
}

func (app *BaseApp) FindLocaleByCode(code string) (*Locale, error) {
	result := &Locale{}

	err := app.RecordQuery(CollectionNameLocales).
		AndWhere(dbx.HashExp{"code": normalizeLocaleCode(code)}).
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (app *BaseApp) FindAllLocales() ([]*Locale, error) {
	result := []*Locale{}

	err := app.RecordQuery(CollectionNameLocales).
		OrderBy("is_default DESC").
		AndOrderBy("code ASC").
		All(&result)

	return result, err
}

func (app *BaseApp) FindEnabledLocales() ([]*Locale, error) {
	result := []*Locale{}

	err := app.RecordQuery(CollectionNameLocales).
		AndWhere(dbx.HashExp{"enabled": true}).
		OrderBy("is_default DESC").
		AndOrderBy("code ASC").
		All(&result)

	return result, err
}

func (app *BaseApp) DefaultLocaleCode() string {
	result := &Locale{}

	err := app.RecordQuery(CollectionNameLocales).
		AndWhere(dbx.HashExp{"is_default": true, "enabled": true}).
		Limit(1).
		One(result)
	if err != nil || result.Code() == "" {
		return DefaultLocaleCode
	}

	return result.Code()
}

func (app *BaseApp) FindI18nGroupById(id string) (*I18nGroup, error) {
	result := &I18nGroup{}

	err := app.RecordQuery(CollectionNameI18nGroups).
		AndWhere(dbx.HashExp{"id": id}).
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func ValidateLocaleCode(code string) error {
	code = normalizeLocaleCode(code)
	return validation.Validate(code, validation.Required, validation.Match(localeCodeRegex))
}

func normalizeLocaleCode(code string) string {
	return strings.TrimSpace(code)
}

func (c *Collection) I18nEnabled() bool {
	return c != nil && c.IsBase() && c.I18n.Enabled
}

func (c *Collection) I18nDefaultLocale(app App) string {
	if c != nil && c.I18n.DefaultLocale != "" {
		return normalizeLocaleCode(c.I18n.DefaultLocale)
	}
	if app != nil {
		return app.DefaultLocaleCode()
	}
	return DefaultLocaleCode
}

func (c *Collection) initI18nFields() {
	if !c.I18nEnabled() {
		return
	}

	c.ensureI18nTextField(FieldNameI18nGroupId, true)
	c.ensureI18nTextField(FieldNameLocale, true)

	sourceField, _ := c.Fields.GetByName(FieldNameIsSource).(*BoolField)
	if sourceField == nil {
		c.Fields.Add(&BoolField{
			Name:   FieldNameIsSource,
			System: true,
			Hidden: true,
		})
	} else {
		sourceField.System = true
		sourceField.Hidden = true
	}

	c.AddIndex("idx_"+c.Name+"_i18n_unique", true, "`"+FieldNameI18nGroupId+"`, `"+FieldNameLocale+"`", "")
	c.AddIndex("idx_"+c.Name+"_locale", false, "`"+FieldNameLocale+"`", "")
	c.AddIndex("idx_"+c.Name+"_i18n_group", false, "`"+FieldNameI18nGroupId+"`", "")

	if c.Fields.GetByName("slug") != nil {
		c.AddIndex("idx_"+c.Name+"_slug_locale", false, "`slug`, `"+FieldNameLocale+"`", "")
	}
}

func (c *Collection) ensureI18nTextField(name string, required bool) {
	field, _ := c.Fields.GetByName(name).(*TextField)
	if field == nil {
		c.Fields.Add(&TextField{
			Name:     name,
			System:   true,
			Hidden:   true,
			Required: required,
		})
		return
	}

	field.System = true
	field.Hidden = true
	field.Required = required
}

func (app *BaseApp) registerI18nHooks() {
	app.OnRecordCreate().Bind(&hook.Handler[*RecordEvent]{
		Id:       "pbI18nRecordCreate",
		Priority: -10,
		Func: func(e *RecordEvent) error {
			if err := e.App.PrepareI18nRecord(e.Record); err != nil {
				return err
			}

			return e.Next()
		},
	})

	app.OnRecordValidate().Bind(&hook.Handler[*RecordEvent]{
		Id:       "pbI18nRecordValidate",
		Priority: -10,
		Func: func(e *RecordEvent) error {
			if err := e.App.ValidateI18nRecord(e.Record); err != nil {
				return err
			}

			return e.Next()
		},
	})

	app.OnRecordAfterDeleteSuccess().Bind(&hook.Handler[*RecordEvent]{
		Id: "pbI18nRecordDelete",
		Func: func(e *RecordEvent) error {
			if err := e.App.CleanupI18nGroup(e.Record); err != nil {
				return err
			}

			return e.Next()
		},
	})

	app.OnRecordValidate(CollectionNameLocales).Bind(&hook.Handler[*RecordEvent]{
		Id:       "pbI18nLocaleValidate",
		Priority: -10,
		Func: func(e *RecordEvent) error {
			if err := validateLocaleRecord(e.App, e.Record); err != nil {
				return err
			}

			return e.Next()
		},
	})

	app.OnRecordCreateExecute(CollectionNameLocales).Bind(localeSaveHandler)
	app.OnRecordUpdateExecute(CollectionNameLocales).Bind(localeSaveHandler)
}

var localeSaveHandler = &hook.Handler[*RecordEvent]{
	Id: "pbI18nLocaleSave",
	Func: func(e *RecordEvent) error {
		if e.Record.GetBool("is_default") {
			if _, err := e.App.DB().
				Update(e.Record.Collection().Name, map[string]any{"is_default": false}, dbx.NewExp("[[id]] != {:id}", dbx.Params{"id": e.Record.Id})).
				Execute(); err != nil {
				return err
			}
			e.Record.Set("enabled", true)
		}

		return e.Next()
	},
}

func validateLocaleRecord(app App, record *Record) error {
	if record.Collection().Name != CollectionNameLocales {
		return nil
	}

	record.Set("code", normalizeLocaleCode(record.GetString("code")))
	record.Set("label", strings.TrimSpace(record.GetString("label")))
	if record.GetBool("is_default") {
		record.Set("enabled", true)
	}

	errs := validation.Errors{}
	if err := ValidateLocaleCode(record.GetString("code")); err != nil {
		errs["code"] = err
	}
	if record.GetString("label") == "" {
		errs["label"] = validation.NewError("validation_required", "Missing locale name.")
	}
	if !record.GetBool("enabled") && record.GetBool("is_default") {
		errs["enabled"] = validation.NewError("validation_default_locale_must_be_enabled", "The default locale must be enabled.")
	}
	if !record.IsNew() && record.Original().GetBool("is_default") && !record.GetBool("is_default") {
		errs["is_default"] = validation.NewError("validation_default_locale_required", "Another locale must be made default before unsetting the current default locale.")
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}

func (app *BaseApp) PrepareI18nRecord(record *Record) error {
	collection := record.Collection()
	if record == nil || collection == nil || !collection.I18nEnabled() {
		return nil
	}

	if record.GetString(FieldNameLocale) == "" {
		record.Set(FieldNameLocale, collection.I18nDefaultLocale(app))
	}

	if record.GetString(FieldNameI18nGroupId) == "" {
		group := NewI18nGroup(app)
		group.SetCollectionName(collection.Name)
		group.SetDefaultLocale(collection.I18nDefaultLocale(app))
		if err := app.Save(group); err != nil {
			return err
		}

		record.Set(FieldNameI18nGroupId, group.Id)
		record.Set(FieldNameIsSource, true)
	}

	return nil
}

func (app *BaseApp) ValidateI18nRecord(record *Record) error {
	if record == nil || record.Collection() == nil || !record.Collection().I18nEnabled() {
		return nil
	}

	locale := normalizeLocaleCode(record.GetString(FieldNameLocale))
	record.Set(FieldNameLocale, locale)

	errs := validation.Errors{}
	if err := ValidateLocaleCode(locale); err != nil {
		errs[FieldNameLocale] = err
	} else if localeRecord, err := app.FindLocaleByCode(locale); err != nil || !localeRecord.Enabled() {
		errs[FieldNameLocale] = validation.NewError("validation_unknown_locale", "Unknown or disabled locale.")
	}

	groupId := record.GetString(FieldNameI18nGroupId)
	if groupId == "" {
		errs[FieldNameI18nGroupId] = validation.NewError("validation_required", "Missing i18n group.")
	} else if group, err := app.FindI18nGroupById(groupId); err != nil || group.CollectionName() != record.Collection().Name {
		errs[FieldNameI18nGroupId] = validation.NewError("validation_invalid_i18n_group", "Missing or invalid i18n group.")
	}

	if len(errs) > 0 {
		return errs
	}

	var duplicateId string
	err := app.RecordQuery(record.Collection()).
		Select("id").
		AndWhere(dbx.HashExp{
			FieldNameI18nGroupId: groupId,
			FieldNameLocale:      locale,
		}).
		AndWhere(dbx.NewExp("[[id]] != {:id}", dbx.Params{"id": record.Id})).
		Limit(1).
		Row(&duplicateId)
	if err == nil && duplicateId != "" {
		return validation.Errors{
			FieldNameLocale: validation.NewError("validation_duplicated_i18n_locale", "The translation locale already exists for this record group."),
		}
	}

	return nil
}

func (app *BaseApp) CleanupI18nGroup(record *Record) error {
	if record == nil || record.Collection() == nil || !record.Collection().I18nEnabled() {
		return nil
	}

	groupId := record.GetString(FieldNameI18nGroupId)
	if groupId == "" {
		return nil
	}

	var total int
	err := app.RecordQuery(record.Collection()).
		Select("count(*)").
		AndWhere(dbx.HashExp{FieldNameI18nGroupId: groupId}).
		Row(&total)
	if err != nil || total > 0 {
		return err
	}

	group, err := app.FindI18nGroupById(groupId)
	if err != nil {
		return nil
	}

	return app.Delete(group)
}

func (app *BaseApp) EnsureDefaultLocale() error {
	var total int
	if err := app.RecordQuery(CollectionNameLocales).Select("count(*)").Row(&total); err != nil {
		return err
	}
	if total > 0 {
		return nil
	}

	locale := NewLocale(app)
	locale.SetCode(DefaultLocaleCode)
	locale.SetName("English")
	locale.SetEnabled(true)
	locale.SetIsDefault(true)

	return app.Save(locale)
}

func (app *BaseApp) BackfillI18nCollectionRecords(newCollection *Collection, oldCollection *Collection) error {
	if newCollection == nil || !newCollection.I18nEnabled() {
		return nil
	}
	if oldCollection != nil && oldCollection.I18nEnabled() {
		return nil
	}

	rows := []struct {
		Id          string `db:"id"`
		I18nGroupId string `db:"i18n_group_id"`
		Locale      string `db:"locale"`
	}{}
	err := app.DB().
		Select("id", FieldNameI18nGroupId, FieldNameLocale).
		From(newCollection.Name).
		All(&rows)
	if err != nil {
		return err
	}

	defaultLocale := newCollection.I18nDefaultLocale(app)
	for _, row := range rows {
		if row.I18nGroupId != "" && row.Locale != "" {
			continue
		}

		group := NewI18nGroup(app)
		group.SetCollectionName(newCollection.Name)
		group.SetDefaultLocale(defaultLocale)
		if err := app.Save(group); err != nil {
			return err
		}

		_, err := app.DB().
			Update(newCollection.Name, map[string]any{
				FieldNameI18nGroupId: group.Id,
				FieldNameLocale:      defaultLocale,
				FieldNameIsSource:    true,
			}, dbx.HashExp{"id": row.Id}).
			Execute()
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Locale) Created() types.DateTime {
	return m.GetDateTime("created")
}

func (m *Locale) Updated() types.DateTime {
	return m.GetDateTime("updated")
}
