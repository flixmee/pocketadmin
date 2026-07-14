package apis_test

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

const collectionPresetSuperuserToken = "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY"

func TestCollectionPresetsList(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "unauthorized",
			Method:          http.MethodGet,
			URL:             "/api/collection-presets",
			ExpectedStatus:  http.StatusUnauthorized,
			ExpectedContent: []string{`"data":{}`},
		},
		{
			Name:   "authorized as superuser",
			Method: http.MethodGet,
			URL:    "/api/collection-presets",
			Headers: map[string]string{
				"Authorization": collectionPresetSuperuserToken,
			},
			ExpectedStatus: http.StatusOK,
			ExpectedContent: []string{
				`"id":"blog"`,
				`"name":"Blog"`,
				`"version":"1.0.0"`,
				`"collections":["categories","tags","posts"]`,
				`"collectionCount":3`,
				`"id":"ecommerce"`,
				`"name":"E-commerce"`,
				`"collections":["product_categories","products"`,
				`"collectionCount":13`,
				`"id":"jobs-career"`,
				`"name":"Jobs \u0026 Career"`,
				`"collections":["companies","jobs","job_categories","applicants","resumes","applications","skills","locations"]`,
				`"collectionCount":8`,
			},
		},
	}

	for i := range scenarios {
		scenarios[i].Test(t)
	}
}

func TestCollectionPresetPreview(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:   "missing preset",
			Method: http.MethodPost,
			URL:    "/api/collection-presets/missing/preview",
			Body:   strings.NewReader(`{"prefix":""}`),
			Headers: map[string]string{
				"Authorization": collectionPresetSuperuserToken,
			},
			ExpectedStatus:  http.StatusNotFound,
			ExpectedContent: []string{`"message":"Collection preset not found."`},
		},
		{
			Name:   "invalid prefix",
			Method: http.MethodPost,
			URL:    "/api/collection-presets/blog/preview",
			Body:   strings.NewReader(`{"prefix":"invalid prefix"}`),
			Headers: map[string]string{
				"Authorization": collectionPresetSuperuserToken,
			},
			ExpectedStatus: http.StatusBadRequest,
			ExpectedContent: []string{
				`"preset":{"code":"validation_collection_preset"`,
				`Prefix must start with a letter`,
			},
		},
		{
			Name:   "resolved prefixed preview",
			Method: http.MethodPost,
			URL:    "/api/collection-presets/blog/preview",
			Body:   strings.NewReader(`{"prefix":"site"}`),
			Headers: map[string]string{
				"Authorization": collectionPresetSuperuserToken,
			},
			ExpectedStatus: http.StatusOK,
			ExpectedContent: []string{
				`"prefix":"site"`,
				`"name":"site_categories"`,
				`"name":"site_tags"`,
				`"name":"site_posts"`,
				`"targetCollection":"site_categories"`,
				`"targetCollection":"site_tags"`,
				`"targetCollection":"_superusers"`,
				`"conflicts":[]`,
				`"canImport":true`,
			},
			NotExpectedContent: []string{`collectionRef`},
		},
		{
			Name:   "resolved jobs-career preview",
			Method: http.MethodPost,
			URL:    "/api/collection-presets/jobs-career/preview",
			Body:   strings.NewReader(`{"prefix":"career"}`),
			Headers: map[string]string{
				"Authorization": collectionPresetSuperuserToken,
			},
			ExpectedStatus: http.StatusOK,
			ExpectedContent: []string{
				`"prefix":"career"`,
				`"name":"career_companies"`,
				`"name":"career_jobs"`,
				`"name":"career_applicants"`,
				`"name":"career_locations"`,
				`"targetCollection":"career_applicants"`,
				`"conflicts":[]`,
				`"canImport":true`,
			},
			NotExpectedContent: []string{`collectionRef`},
		},
	}

	for i := range scenarios {
		scenarios[i].Test(t)
	}
}

func TestCollectionPresetImport(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:   "successful prefixed import",
			Method: http.MethodPost,
			URL:    "/api/collection-presets/blog/import",
			Body:   strings.NewReader(`{"prefix":"site"}`),
			Headers: map[string]string{
				"Authorization": collectionPresetSuperuserToken,
			},
			ExpectedStatus: http.StatusNoContent,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				categories := findCollection(t, app, "site_categories")
				tags := findCollection(t, app, "site_tags")
				posts := findCollection(t, app, "site_posts")
				if categories.CollectionGroup != "Blog" || tags.CollectionGroup != "Blog" || posts.CollectionGroup != "Blog" {
					t.Fatalf("expected imported collections in Blog group")
				}

				category, _ := posts.Fields.GetByName("category").(*core.RelationField)
				if category == nil || category.CollectionId != categories.Id {
					t.Fatalf("unexpected category relation: %#v", category)
				}
				tagsField, _ := posts.Fields.GetByName("tags").(*core.RelationField)
				if tagsField == nil || tagsField.CollectionId != tags.Id {
					t.Fatalf("unexpected tags relation: %#v", tagsField)
				}
				author, _ := posts.Fields.GetByName("author").(*core.RelationField)
				superusers := findCollection(t, app, core.CollectionNameSuperusers)
				if author == nil || author.CollectionId != superusers.Id {
					t.Fatalf("unexpected author relation: %#v", author)
				}
				cover, _ := posts.Fields.GetByName("cover").(*core.MediaField)
				if cover == nil || cover.MaxSelect != 1 || len(cover.MimeTypes) != 4 {
					t.Fatalf("unexpected cover media field: %#v", cover)
				}
			},
		},
		{
			Name:   "conflict leaves other preset collections unchanged",
			Method: http.MethodPost,
			URL:    "/api/collection-presets/blog/import",
			Body:   strings.NewReader(`{"prefix":""}`),
			Headers: map[string]string{
				"Authorization": collectionPresetSuperuserToken,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				if err := app.Save(core.NewBaseCollection("categories")); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus: http.StatusConflict,
			ExpectedContent: []string{
				`"message":"Collection preset has conflicts."`,
				`"conflicts":{"code":"validation_collection_preset_conflict"`,
				`Collection name \"categories\" already exists.`,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				assertMissingCollection(t, app, "tags")
				assertMissingCollection(t, app, "posts")
			},
		},
		{
			Name:   "transaction rollback on import validation failure",
			Method: http.MethodPost,
			URL:    "/api/collection-presets/blog/import",
			Body:   strings.NewReader(`{"prefix":"rollback"}`),
			Headers: map[string]string{
				"Authorization": collectionPresetSuperuserToken,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				app.OnCollectionsImportRequest().BindFunc(func(event *core.CollectionsImportRequestEvent) error {
					event.CollectionsData[2]["fields"] = []map[string]any{{
						"name": "invalid field name",
						"type": "text",
					}}
					return event.Next()
				})
			},
			ExpectedStatus: http.StatusBadRequest,
			ExpectedContent: []string{
				`"collections":{"code":"validation_collections_import_failure"`,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				assertMissingCollection(t, app, "rollback_categories")
				assertMissingCollection(t, app, "rollback_tags")
				assertMissingCollection(t, app, "rollback_posts")
			},
		},
	}

	for i := range scenarios {
		scenarios[i].Test(t)
	}
}

func findCollection(t testing.TB, app *tests.TestApp, name string) *core.Collection {
	t.Helper()
	collection, err := app.FindCollectionByNameOrId(name)
	if err != nil {
		t.Fatalf("find collection %q: %v", name, err)
	}
	return collection
}

func assertMissingCollection(t testing.TB, app *tests.TestApp, name string) {
	t.Helper()
	_, err := app.FindCollectionByNameOrId(name)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected collection %q to be missing, got %v", name, err)
	}
}
