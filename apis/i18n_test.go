package apis_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestLocalesList(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "unauthorized",
			Method:          http.MethodGet,
			URL:             "/api/locales",
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized",
			Method: http.MethodGet,
			URL:    "/api/locales",
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"code":"en"`,
				`"is_default":true`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestI18nRecordLocaleListFallbackAndTranslations(t *testing.T) {
	t.Parallel()

	sourceId := ""
	translationId := ""

	setup := func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
		createTestLocale(t, app, "vi", "Vietnamese")
		createTestLocale(t, app, "ja", "Japanese")

		collection := core.NewBaseCollection("i18n_api_posts")
		collection.Fields.Add(&core.TextField{Name: "title"})
		collection.ListRule = ptr("")
		collection.ViewRule = ptr("")
		collection.I18n.Enabled = true
		collection.I18n.DefaultLocale = core.DefaultLocaleCode
		collection.I18n.LocalizedFields = []string{"title"}
		if err := app.Save(collection); err != nil {
			t.Fatalf("Failed to create localized collection: %v", err)
		}

		source := core.NewRecord(collection)
		source.Set(core.FieldNameId, "i18nsource00001")
		source.Set("title", "Hello")
		if err := app.Save(source); err != nil {
			t.Fatalf("Failed to create source record: %v", err)
		}
		sourceId = source.Id

		translation := core.NewRecord(collection)
		translation.Set(core.FieldNameId, "i18ntrans000001")
		translation.Set("title", "Xin chao")
		translation.Set(core.FieldNameI18nGroupId, source.GetString(core.FieldNameI18nGroupId))
		translation.Set(core.FieldNameLocale, "vi")
		if err := app.Save(translation); err != nil {
			t.Fatalf("Failed to create translation record: %v", err)
		}
		translationId = translation.Id
	}

	scenarios := []tests.ApiScenario{
		{
			Name:           "locale filter",
			Method:         http.MethodGet,
			URL:            "/api/collections/i18n_api_posts/records?locale=vi",
			BeforeTestFunc: setup,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"title":"Xin chao"`,
			},
			NotExpectedContent: []string{`"title":"Hello"`},
		},
		{
			Name:           "fallback to default locale",
			Method:         http.MethodGet,
			URL:            "/api/collections/i18n_api_posts/records?locale=ja&fallback=true",
			BeforeTestFunc: setup,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"title":"Hello"`,
			},
			NotExpectedContent: []string{`"title":"Xin chao"`},
		},
		{
			Name:           "translations metadata",
			Method:         http.MethodGet,
			URL:            "/api/collections/i18n_api_posts/records/source/translations",
			BeforeTestFunc: setup,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"locale":"en"`,
				`"locale":"vi"`,
				`"locale":"ja"`,
				`"is_missing":true`,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				_ = sourceId
			},
		},
		{
			Name:           "record view locale metadata",
			Method:         http.MethodGet,
			URL:            "/api/collections/i18n_api_posts/records/source",
			BeforeTestFunc: setup,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"locale":"vi"`,
				`"localeLinks":[`,
				`"id":"i18nsource00001","locale":"en"`,
				`"id":"i18ntrans000001","locale":"vi"`,
			},
			NotExpectedContent: []string{
				`"i18n_group_id"`,
				`"is_source"`,
			},
		},
	}

	for _, scenario := range scenarios {
		if scenario.Name == "translations metadata" {
			baseSetup := scenario.BeforeTestFunc
			scenario.BeforeTestFunc = func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				baseSetup(t, app, e)
				scenario.URL = "/api/collections/i18n_api_posts/records/" + sourceId + "/translations"
			}
		}
		if scenario.Name == "record view locale metadata" {
			baseSetup := scenario.BeforeTestFunc
			scenario.BeforeTestFunc = func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				baseSetup(t, app, e)
				scenario.URL = "/api/collections/i18n_api_posts/records/" + translationId
			}
		}
		scenario.Test(t)
	}
}

func TestI18nRecordTranslationCreateWithUniqueSlug(t *testing.T) {
	t.Parallel()

	sourceId := ""

	setup := func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
		createTestLocale(t, app, "vi", "Vietnamese")

		collection := core.NewBaseCollection("i18n_api_slug_posts")
		collection.Fields.Add(&core.TextField{Name: "title"})
		collection.Fields.Add(&core.SlugField{Name: "slug", AttachedField: "title", Required: true})
		collection.AddIndex("idx_i18n_api_slug_posts_slug", true, "`slug`", "")
		collection.ListRule = ptr("")
		collection.ViewRule = ptr("")
		collection.I18n.Enabled = true
		collection.I18n.DefaultLocale = core.DefaultLocaleCode
		collection.I18n.LocalizedFields = []string{"title", "slug"}
		if err := app.Save(collection); err != nil {
			t.Fatalf("Failed to create localized collection: %v", err)
		}

		source := core.NewRecord(collection)
		source.Set("title", "Hello World")
		if err := app.Save(source); err != nil {
			t.Fatalf("Failed to create source record: %v", err)
		}
		sourceId = source.Id
	}

	scenario := tests.ApiScenario{
		Name:           "create translation with unique slug",
		Method:         http.MethodPost,
		URL:            "/api/collections/i18n_api_slug_posts/records/source/translations",
		Body:           strings.NewReader(`{"locale":"vi"}`),
		BeforeTestFunc: setup,
		Headers: map[string]string{
			"Authorization": testSuperuserAuthHeader,
		},
		ExpectedStatus: 200,
		ExpectedContent: []string{
			`"slug":"hello-world-vi"`,
		},
		NotExpectedContent: []string{
			`"code":"validation_not_unique"`,
		},
	}

	baseSetup := scenario.BeforeTestFunc
	scenario.BeforeTestFunc = func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
		baseSetup(t, app, e)
		scenario.URL = "/api/collections/i18n_api_slug_posts/records/" + sourceId + "/translations"
	}

	scenario.Test(t)
}

func createTestLocale(t testing.TB, app *tests.TestApp, code string, name string) {
	t.Helper()

	locale := core.NewLocale(app)
	locale.SetCode(code)
	locale.SetName(name)
	locale.SetEnabled(true)
	if err := app.Save(locale); err != nil {
		t.Fatalf("Failed to create locale %q: %v", code, err)
	}
}

func ptr(v string) *string {
	return &v
}
