package core

import (
	"context"
	"errors"
	"strings"

	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	CollectionNameTranslationJobs = "_translationJobs"

	TranslationJobStatusPending   = "pending"
	TranslationJobStatusRunning   = "running"
	TranslationJobStatusFinished  = "finished"
	TranslationJobStatusFailed    = "failed"
	TranslationJobStatusCancelled = "cancelled"
)

var translationJobStatuses = []string{
	TranslationJobStatusPending,
	TranslationJobStatusRunning,
	TranslationJobStatusFinished,
	TranslationJobStatusFailed,
	TranslationJobStatusCancelled,
}

var (
	_ Model        = (*TranslationJob)(nil)
	_ PreValidator = (*TranslationJob)(nil)
	_ RecordProxy  = (*TranslationJob)(nil)
)

type TranslationJob struct {
	*Record
}

func NewTranslationJob(app App) *TranslationJob {
	m := &TranslationJob{}

	c, err := app.FindCachedCollectionByNameOrId(CollectionNameTranslationJobs)
	if err != nil {
		c = NewBaseCollection("@__invalid_translation_jobs__")
	}

	m.Record = NewRecord(c)

	return m
}

func (m *TranslationJob) PreValidate(ctx context.Context, app App) error {
	if m.Record == nil || m.Record.Collection().Name != CollectionNameTranslationJobs {
		return errors.New("missing or invalid TranslationJob ProxyRecord")
	}

	return nil
}

func (m *TranslationJob) ProxyRecord() *Record {
	return m.Record
}

func (m *TranslationJob) SetProxyRecord(record *Record) {
	m.Record = record
}

func (m *TranslationJob) CollectionRef() string {
	return m.GetString("collectionRef")
}

func (m *TranslationJob) SetCollectionRef(collectionRef string) {
	m.Set("collectionRef", strings.TrimSpace(collectionRef))
}

func (m *TranslationJob) SourceRecordId() string {
	return m.GetString("sourceRecordId")
}

func (m *TranslationJob) SetSourceRecordId(recordId string) {
	m.Set("sourceRecordId", strings.TrimSpace(recordId))
}

func (m *TranslationJob) TargetRecordId() string {
	return m.GetString("targetRecordId")
}

func (m *TranslationJob) SetTargetRecordId(recordId string) {
	m.Set("targetRecordId", strings.TrimSpace(recordId))
}

func (m *TranslationJob) SourceLocale() string {
	return m.GetString("sourceLocale")
}

func (m *TranslationJob) SetSourceLocale(locale string) {
	m.Set("sourceLocale", normalizeLocaleCode(locale))
}

func (m *TranslationJob) TargetLocale() string {
	return m.GetString("targetLocale")
}

func (m *TranslationJob) SetTargetLocale(locale string) {
	m.Set("targetLocale", normalizeLocaleCode(locale))
}

func (m *TranslationJob) Status() string {
	return m.GetString("status")
}

func (m *TranslationJob) SetStatus(status string) {
	m.Set("status", strings.TrimSpace(status))
}

func (m *TranslationJob) Provider() string {
	return m.GetString("provider")
}

func (m *TranslationJob) SetProvider(provider string) {
	m.Set("provider", strings.TrimSpace(provider))
}

func (m *TranslationJob) Model() string {
	return m.GetString("model")
}

func (m *TranslationJob) SetModel(model string) {
	m.Set("model", strings.TrimSpace(model))
}

func (m *TranslationJob) Error() string {
	return m.GetString("error")
}

func (m *TranslationJob) SetError(message string) {
	m.Set("error", strings.TrimSpace(message))
}

func (m *TranslationJob) Result() types.JSONRaw {
	raw, _ := m.GetRaw("result").(types.JSONRaw)
	return raw
}

func (m *TranslationJob) SetResult(result types.JSONRaw) {
	m.Set("result", result)
}

func (app *BaseApp) FindTranslationJobById(id string) (*TranslationJob, error) {
	result := &TranslationJob{}

	err := app.RecordQuery(CollectionNameTranslationJobs).
		AndWhere(dbx.HashExp{"id": id}).
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (app *BaseApp) registerTranslationJobHooks() {
	app.OnRecordValidate(CollectionNameTranslationJobs).Bind(&hook.Handler[*RecordEvent]{
		Id:       "pbI18nTranslationJobValidate",
		Priority: -10,
		Func: func(e *RecordEvent) error {
			if err := validateTranslationJobRecord(e.App, e.Record); err != nil {
				return err
			}

			return e.Next()
		},
	})

	app.OnRecordAfterUpdateSuccess(CollectionNameTranslationJobs).Bind(&hook.Handler[*RecordEvent]{
		Id: "pbI18nTranslationJobAutomation",
		Func: func(e *RecordEvent) error {
			if err := e.Next(); err != nil {
				return err
			}

			if e.Record.GetString("status") == TranslationJobStatusFinished &&
				e.Record.Original().GetString("status") != TranslationJobStatusFinished {
				queueI18nAutomationRuns(e.App, AutomationTriggerI18nAIFinished, nil, map[string]any{
					"collectionId":     e.Record.GetString("collectionRef"),
					"sourceRecordId":   e.Record.GetString("sourceRecordId"),
					"targetRecordId":   e.Record.GetString("targetRecordId"),
					"sourceLocale":     e.Record.GetString("sourceLocale"),
					"targetLocale":     e.Record.GetString("targetLocale"),
					"translationJobId": e.Record.Id,
					"provider":         e.Record.GetString("provider"),
					"model":            e.Record.GetString("model"),
				})
			}

			return nil
		},
	})
}

func validateTranslationJobRecord(app App, record *Record) error {
	errs := validation.Errors{}

	collectionRef := strings.TrimSpace(record.GetString("collectionRef"))
	if err := validation.Validate(collectionRef, validation.Required, validation.By(validateCollectionId(app, CollectionTypeBase, CollectionTypeAuth))); err != nil {
		errs["collectionRef"] = err
	}
	if err := validation.Validate(record.GetString("sourceRecordId"), validation.Required); err != nil {
		errs["sourceRecordId"] = err
	}
	if err := validation.Validate(record.GetString("sourceLocale"), validation.Required, validation.By(func(value any) error {
		return ValidateLocaleCode(value.(string))
	})); err != nil {
		errs["sourceLocale"] = err
	}
	if err := validation.Validate(record.GetString("targetLocale"), validation.Required, validation.By(func(value any) error {
		return ValidateLocaleCode(value.(string))
	})); err != nil {
		errs["targetLocale"] = err
	}

	status := strings.TrimSpace(record.GetString("status"))
	if status == "" {
		record.Set("status", TranslationJobStatusPending)
		status = TranslationJobStatusPending
	}
	if err := validation.Validate(status, validation.In(toAnySlice(translationJobStatuses)...)); err != nil {
		errs["status"] = err
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}
