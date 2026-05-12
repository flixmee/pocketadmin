package apis

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/dbutils"
	"github.com/pocketbase/pocketbase/tools/router"
)

func bindI18nApi(app core.App, rg *router.RouterGroup[*core.RequestEvent]) {
	subGroup := rg.Group("/locales").Bind(RequireSuperuserAuth())
	subGroup.GET("", localesList)
	subGroup.POST("", localeCreate)
	subGroup.GET("/{id}", localeView)
	subGroup.PATCH("/{id}", localeUpdate)
	subGroup.DELETE("/{id}", localeDelete)
}

func localesList(e *core.RequestEvent) error {
	locales, err := e.App.FindAllLocales()
	if err != nil {
		return e.BadRequestError("Failed to load locales.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, locales)
	})
}

func localeView(e *core.RequestEvent) error {
	locale, err := findLocaleForAPI(e.App, e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("Missing or invalid locale.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, locale)
	})
}

func localeCreate(e *core.RequestEvent) error {
	body := map[string]any{}
	if err := e.BindBody(&body); err != nil {
		return e.BadRequestError("Failed to load the submitted data due to invalid formatting.", err)
	}

	locale := core.NewLocale(e.App)
	applyLocaleBody(locale, body)

	if err := e.App.Save(locale); err != nil {
		return e.BadRequestError("Failed to create locale.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, locale)
	})
}

func localeUpdate(e *core.RequestEvent) error {
	locale, err := findLocaleForAPI(e.App, e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("Missing or invalid locale.", err)
	}

	body := map[string]any{}
	if err := e.BindBody(&body); err != nil {
		return e.BadRequestError("Failed to load the submitted data due to invalid formatting.", err)
	}

	applyLocaleBody(locale, body)

	if err := e.App.Save(locale); err != nil {
		return e.BadRequestError("Failed to update locale.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, locale)
	})
}

func localeDelete(e *core.RequestEvent) error {
	locale, err := findLocaleForAPI(e.App, e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("Missing or invalid locale.", err)
	}
	if locale.IsDefault() {
		return e.BadRequestError("Failed to delete locale.", validation.NewError("validation_default_locale_delete", "The default locale cannot be deleted."))
	}

	if err := e.App.Delete(locale); err != nil {
		return e.BadRequestError("Failed to delete locale.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.NoContent(http.StatusNoContent)
	})
}

func findLocaleForAPI(app core.App, idOrCode string) (*core.Locale, error) {
	locale := &core.Locale{}
	err := app.RecordQuery(core.CollectionNameLocales).
		AndWhere(dbx.Or(
			dbx.HashExp{"id": idOrCode},
			dbx.HashExp{"code": idOrCode},
		)).
		Limit(1).
		One(locale)
	if err != nil {
		return nil, err
	}

	return locale, nil
}

func applyLocaleBody(locale *core.Locale, body map[string]any) {
	if v, ok := body["code"]; ok {
		locale.SetCode(fmt.Sprint(v))
	}
	if v, ok := body["name"]; ok {
		locale.SetName(fmt.Sprint(v))
	}
	if v, ok := body["enabled"]; ok {
		locale.SetEnabled(v == true || fmt.Sprint(v) == "true" || fmt.Sprint(v) == "1")
	}
	if v, ok := body["is_default"]; ok {
		locale.SetIsDefault(v == true || fmt.Sprint(v) == "true" || fmt.Sprint(v) == "1")
	}
}

func applyI18nListQuery(app core.App, collection *core.Collection, query *dbx.SelectQuery, params url.Values) (url.Values, error) {
	cleanParams := cloneURLValues(params)
	cleanParams.Del("locale")
	cleanParams.Del("fallback")

	if collection == nil || !collection.I18nEnabled() {
		return cleanParams, nil
	}

	localeCode := params.Get("locale")
	if localeCode == "" {
		localeCode = collection.I18nDefaultLocale(app)
	}

	locale, err := app.FindLocaleByCode(localeCode)
	if err != nil || !locale.Enabled() {
		return cleanParams, validation.NewError("validation_unknown_locale", "Unknown or disabled locale.")
	}

	defaultLocale := collection.I18nDefaultLocale(app)
	fallback := params.Get("fallback") == "true" || params.Get("fallback") == "1"
	if !fallback || locale.Code() == defaultLocale {
		query.AndWhere(dbx.NewExp(fmt.Sprintf("{{%s}}.[[%s]] = {:locale}", collection.Name, core.FieldNameLocale), dbx.Params{
			"locale": locale.Code(),
		}))
		return cleanParams, nil
	}

	query.AndWhere(dbx.NewExp(
		fmt.Sprintf(
			`(
				{{%[1]s}}.[[%[2]s]] = {:locale}
				OR (
					{{%[1]s}}.[[%[2]s]] = {:defaultLocale}
					AND NOT EXISTS (
						SELECT 1 FROM {{%[1]s}} i18n_requested
						WHERE i18n_requested.[[%[3]s]] = {{%[1]s}}.[[%[3]s]]
						AND i18n_requested.[[%[2]s]] = {:locale}
					)
				)
			)`,
			collection.Name,
			core.FieldNameLocale,
			core.FieldNameI18nGroupId,
		),
		dbx.Params{
			"locale":        locale.Code(),
			"defaultLocale": defaultLocale,
		},
	))

	return cleanParams, nil
}

func cloneURLValues(values url.Values) url.Values {
	result := url.Values{}
	for k, vals := range values {
		for _, v := range vals {
			result.Add(k, v)
		}
	}
	return result
}

func recordTranslations(e *core.RequestEvent) error {
	collection, err := e.App.FindCachedCollectionByNameOrId(e.Request.PathValue("collection"))
	if err != nil || collection == nil || !collection.I18nEnabled() {
		return e.NotFoundError("Missing localized collection context.", err)
	}

	record, err := e.App.FindRecordById(collection, e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("Missing or invalid record.", err)
	}

	translations, err := loadRecordTranslations(e.App, collection, record)
	if err != nil {
		return e.BadRequestError("Failed to load record translations.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, translations)
	})
}

func recordTranslationCreate(e *core.RequestEvent) error {
	requestInfo, err := e.RequestInfo()
	if err != nil {
		return firstApiError(err, e.BadRequestError("", err))
	}
	if !requestInfo.HasSuperuserAuth() {
		return e.ForbiddenError("Only superusers can perform this action.", nil)
	}

	collection, err := e.App.FindCachedCollectionByNameOrId(e.Request.PathValue("collection"))
	if err != nil || collection == nil || !collection.I18nEnabled() {
		return e.NotFoundError("Missing localized collection context.", err)
	}

	source, err := e.App.FindRecordById(collection, e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("Missing or invalid source record.", err)
	}

	body := struct {
		Locale string `json:"locale"`
	}{}
	if err := json.NewDecoder(e.Request.Body).Decode(&body); err != nil {
		return e.BadRequestError("Failed to load the submitted data due to invalid formatting.", err)
	}

	locale, err := e.App.FindLocaleByCode(body.Locale)
	if err != nil || !locale.Enabled() {
		return e.BadRequestError("Failed to create translation.", validation.NewError("validation_unknown_locale", "Unknown or disabled locale."))
	}

	record := core.NewRecord(collection)
	for _, field := range collection.Fields {
		name := field.GetName()
		if name == core.FieldNameId || name == core.FieldNameI18nGroupId || name == core.FieldNameLocale || name == core.FieldNameIsSource {
			continue
		}
		if field.GetSystem() {
			continue
		}
		record.Set(name, source.Get(name))
	}
	prepareUniqueTranslationSlugs(e.App, collection, source, record, locale.Code())
	record.Set(core.FieldNameI18nGroupId, source.GetString(core.FieldNameI18nGroupId))
	record.Set(core.FieldNameLocale, locale.Code())
	record.Set(core.FieldNameIsSource, false)

	if err := e.App.Save(record); err != nil {
		return e.BadRequestError("Failed to create translation.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, record)
	})
}

func prepareUniqueTranslationSlugs(app core.App, collection *core.Collection, source *core.Record, record *core.Record, locale string) {
	for _, field := range collection.Fields {
		slugField, ok := field.(*core.SlugField)
		if !ok {
			continue
		}

		if _, ok := dbutils.FindSingleColumnUniqueIndex(collection.Indexes, slugField.Name); !ok {
			continue
		}

		slug := uniqueTranslationSlug(app, collection, slugField, source.GetString(slugField.Name), locale)
		if slug != "" {
			record.Set(slugField.Name, slug)
		}
	}
}

func uniqueTranslationSlug(app core.App, collection *core.Collection, field *core.SlugField, sourceSlug string, locale string) string {
	if sourceSlug == "" {
		return ""
	}

	suffix := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(locale), "_", "-"))
	if suffix == "" {
		return sourceSlug
	}

	for i := 0; i < 10; i++ {
		candidateSuffix := suffix
		if i > 0 {
			candidateSuffix = fmt.Sprintf("%s-%d", suffix, i+1)
		}

		candidate := translationSlugCandidate(sourceSlug, candidateSuffix, field.Max)
		_, err := app.FindFirstRecordByFilter(collection.Id, field.Name+"={:slug}", dbx.Params{"slug": candidate})
		if err == sql.ErrNoRows {
			return candidate
		}
		if err != nil {
			break
		}
	}

	return translationSlugCandidate(sourceSlug, suffix, field.Max)
}

func translationSlugCandidate(base string, suffix string, max int) string {
	if suffix == "" {
		return base
	}

	if max == 0 {
		max = 5000
	}

	tail := "-" + suffix
	baseRunes := []rune(base)
	tailRunes := []rune(tail)
	if max > 0 && len(baseRunes)+len(tailRunes) > max {
		limit := max - len(tailRunes)
		if limit <= 0 {
			suffixRunes := []rune(strings.Trim(suffix, "-"))
			if len(suffixRunes) > max {
				return strings.Trim(string(suffixRunes[:max]), "-")
			}
			return string(suffixRunes)
		}

		base = strings.Trim(string(baseRunes[:limit]), "-")
	}

	if base == "" {
		return strings.Trim(suffix, "-")
	}

	return base + tail
}

type i18nTranslationInfo struct {
	Locale    string `json:"locale"`
	Id        string `json:"id,omitempty"`
	IsSource  bool   `json:"is_source"`
	IsMissing bool   `json:"is_missing"`
}

func loadRecordTranslations(app core.App, collection *core.Collection, record *core.Record) ([]i18nTranslationInfo, error) {
	groupId := record.GetString(core.FieldNameI18nGroupId)
	if groupId == "" {
		return nil, sql.ErrNoRows
	}

	records := []*core.Record{}
	err := app.RecordQuery(collection).
		AndWhere(dbx.HashExp{core.FieldNameI18nGroupId: groupId}).
		OrderBy(core.FieldNameLocale + " ASC").
		All(&records)
	if err != nil {
		return nil, err
	}

	byLocale := map[string]*core.Record{}
	for _, item := range records {
		byLocale[item.GetString(core.FieldNameLocale)] = item
	}

	locales, err := app.FindEnabledLocales()
	if err != nil {
		return nil, err
	}

	result := make([]i18nTranslationInfo, 0, len(locales))
	for _, locale := range locales {
		info := i18nTranslationInfo{Locale: locale.Code(), IsMissing: true}
		if item := byLocale[locale.Code()]; item != nil {
			info.Id = item.Id
			info.IsSource = item.GetBool(core.FieldNameIsSource)
			info.IsMissing = false
		}
		result = append(result, info)
	}

	return result, nil
}
