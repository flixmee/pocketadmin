package apis_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

const testSuperuserAuthToken = "eyJhbGciOiJIUzI1NiJ9.eyJpZCI6InN5d2JoZWNuaDQ2cmhtMCIsInR5cGUiOiJhdXRoIiwiY29sbGVjdGlvbklkIjoicGJjXzMxNDI2MzU4MjMiLCJleHAiOjI1MjQ2MDQ0NjEsInJlZnJlc2hhYmxlIjp0cnVlfQ.UXgO3j-0BumcugrFjbd7j0M4MQvbrLggLlcu_YNGjoY"

func TestMediaManagerAPI(t *testing.T) {
	t.Parallel()

	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	router := newTestRouter(t, app)

	medias, err := app.FindCollectionByNameOrId(core.CollectionNameMedias)
	if err != nil {
		t.Fatal(err)
	}

	rootFolder := core.NewRecord(medias)
	rootFolder.Set("name", "API Root")
	rootFolder.Set("kind", core.MediaKindFolder)
	if err := app.Save(rootFolder); err != nil {
		t.Fatal(err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("kind", core.MediaKindFile); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("parent", rootFolder.Id); err != nil {
		t.Fatal(err)
	}

	part, err := writer.CreateFormFile("file", "api-file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("api upload body")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/collections/_medias/records", body)
	createReq.Header.Set("Content-Type", writer.FormDataContentType())
	createReq.Header.Set("Authorization", testSuperuserAuthToken)
	createRes := httptest.NewRecorder()
	router.ServeHTTP(createRes, createReq)

	if createRes.Code != http.StatusOK {
		t.Fatalf("expected upload status 200, got %d: %s", createRes.Code, createRes.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	createdID := created["id"].(string)
	storedFileName := created["file"].(string)
	if created["name"] != "api-file.txt" {
		t.Fatalf("expected original filename as media name, got %v", created["name"])
	}

	listReq := httptest.NewRequest(
		http.MethodGet,
		"/api/collections/_medias/records?filter="+url.QueryEscape(`parent="`+rootFolder.Id+`"`)+"&sort=-kind,name",
		nil,
	)
	listReq.Header.Set("Authorization", testSuperuserAuthToken)
	listRes := httptest.NewRecorder()
	router.ServeHTTP(listRes, listReq)

	if listRes.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d: %s", listRes.Code, listRes.Body.String())
	}

	if !strings.Contains(listRes.Body.String(), createdID) {
		t.Fatalf("expected uploaded media in list response: %s", listRes.Body.String())
	}

	tokenReq := httptest.NewRequest(http.MethodPost, "/api/files/token", nil)
	tokenReq.Header.Set("Authorization", testSuperuserAuthToken)
	tokenRes := httptest.NewRecorder()
	router.ServeHTTP(tokenRes, tokenReq)

	if tokenRes.Code != http.StatusOK {
		t.Fatalf("expected file token status 200, got %d: %s", tokenRes.Code, tokenRes.Body.String())
	}

	var tokenResponse map[string]string
	if err := json.Unmarshal(tokenRes.Body.Bytes(), &tokenResponse); err != nil {
		t.Fatal(err)
	}

	downloadReq := httptest.NewRequest(
		http.MethodGet,
		"/api/files/_medias/"+createdID+"/"+storedFileName+"?token="+tokenResponse["token"],
		nil,
	)
	downloadRes := httptest.NewRecorder()
	router.ServeHTTP(downloadRes, downloadReq)

	if downloadRes.Code != http.StatusOK {
		t.Fatalf("expected download status 200, got %d: %s", downloadRes.Code, downloadRes.Body.String())
	}
}

func newTestRouter(t *testing.T, app *tests.TestApp) http.Handler {
	t.Helper()

	baseRouter, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}

	serveEvent := &core.ServeEvent{
		App:    app,
		Router: baseRouter,
	}

	if err := app.OnServe().Trigger(serveEvent, func(e *core.ServeEvent) error {
		return e.Next()
	}); err != nil {
		t.Fatal(err)
	}

	mux, err := baseRouter.BuildMux()
	if err != nil {
		t.Fatal(err)
	}

	return mux
}
