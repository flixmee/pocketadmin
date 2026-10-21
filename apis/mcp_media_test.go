package apis_test

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// 1x1 transparent png
var testMCPPng, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==")

func findTestMediaByPath(t testing.TB, app core.App, mediaPath string) *core.Record {
	record, err := core.ResolveMediaRecordByPath(app, mediaPath)
	if err != nil {
		t.Fatalf("Expected media %q to exist: %v", mediaPath, err)
	}

	return record
}

func TestMCPMediaTools(t *testing.T) {
	t.Parallel()

	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/images/photo":
			w.Header().Set("Content-Type", "image/png")
			w.Write(testMCPPng)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer remote.Close()

	bearer := map[string]string{"Authorization": "Bearer " + testMCPToken}
	pngBase64 := base64.StdEncoding.EncodeToString(testMCPPng)

	scenarios := []tests.ApiScenario{
		{
			Name:    "tools/list without media permissions",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, func(key *core.MCPKey) {
					key.Permissions = []string{core.MCPPermissionRecordsRead, core.MCPPermissionRecordsWrite}
				})
			},
			ExpectedStatus:     200,
			ExpectedContent:    []string{`"name":"create_record"`},
			NotExpectedContent: []string{`"name":"list_media"`, `"name":"upload_media"`},
			ExpectedEvents:     map[string]int{"*": 0},
		},
		{
			Name:    "tools/list with media permissions",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, func(key *core.MCPKey) {
					key.Permissions = []string{core.MCPPermissionMediaRead, core.MCPPermissionMediaWrite}
				})
			},
			ExpectedStatus:     200,
			ExpectedContent:    []string{`"name":"list_media"`, `"name":"upload_media"`},
			NotExpectedContent: []string{`"name":"create_record"`},
			ExpectedEvents:     map[string]int{"*": 0},
		},
		{
			Name:    "upload_media without permission",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("upload_media", map[string]any{"name": "cover.png", "content": pngBase64}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, func(key *core.MCPKey) {
					key.Permissions = []string{core.MCPPermissionMediaRead}
				})
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"code":-32602`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "upload_media requires exactly one source",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("upload_media", map[string]any{"name": "cover.png", "content": pngBase64, "url": remote.URL + "/images/photo"}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"isError":true`, `exactly one of`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "upload_media with invalid base64",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("upload_media", map[string]any{"name": "cover.png", "content": "%%%not-base64"}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"isError":true`, `not valid base64`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "upload_media base64 into new nested folder",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("upload_media", map[string]any{"name": "cover.png", "folder": "posts/covers", "content": "data:image/png;base64," + pngBase64}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"isError":false`,
				`\"name\":\"cover.png\"`,
				`\"path\":\"/posts/covers/cover.png\"`,
				`\"mime\":\"image/png\"`,
				`\"url\":\"/api/files/`,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				folder := findTestMediaByPath(t, app, "/posts/covers")
				if folder.GetString("kind") != core.MediaKindFolder {
					t.Fatalf("Expected /posts/covers to be a folder, got %q", folder.GetString("kind"))
				}

				file := findTestMediaByPath(t, app, "/posts/covers/cover.png")
				if file.GetString("kind") != core.MediaKindFile || file.GetString("file") == "" || file.GetInt("size") != len(testMCPPng) {
					t.Fatalf("Unexpected uploaded media record: %v", file.PublicExport())
				}
			},
		},
		{
			Name:    "upload_media renames on name conflict",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("upload_media", map[string]any{"name": "cover.png", "content": pngBase64}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)

				medias, err := app.FindCollectionByNameOrId(core.CollectionNameMedias)
				if err != nil {
					t.Fatal(err)
				}
				existing := core.NewRecord(medias)
				existing.Set("kind", core.MediaKindFolder)
				existing.Set("name", "cover.png")
				if err := app.Save(existing); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"isError":false`,
				`\"name\":\"cover-1.png\"`,
				`\"path\":\"/cover-1.png\"`,
			},
		},
		{
			Name:    "upload_media from url",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("upload_media", map[string]any{"url": remote.URL + "/images/photo", "folder": "/remote"}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"isError":false`,
				`\"path\":\"/remote/photo.png\"`,
				`\"mime\":\"image/png\"`,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				findTestMediaByPath(t, app, "/remote/photo.png")
			},
		},
		{
			Name:    "upload_media from failing url",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("upload_media", map[string]any{"url": remote.URL + "/missing.png"}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"isError":true`, `status 404`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "upload_media rejects non http urls",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("upload_media", map[string]any{"url": "file:///etc/passwd"}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"isError":true`, `http(s) URL`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "list_media in folder",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("list_media", map[string]any{"folder": "/gallery"}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, func(key *core.MCPKey) {
					key.Permissions = []string{core.MCPPermissionMediaRead}
				})

				medias, err := app.FindCollectionByNameOrId(core.CollectionNameMedias)
				if err != nil {
					t.Fatal(err)
				}

				folder := core.NewRecord(medias)
				folder.Set("kind", core.MediaKindFolder)
				folder.Set("name", "gallery")
				if err := app.Save(folder); err != nil {
					t.Fatal(err)
				}

				sub := core.NewRecord(medias)
				sub.Set("kind", core.MediaKindFolder)
				sub.Set("name", "nested")
				sub.Set("parent", folder.Id)
				if err := app.Save(sub); err != nil {
					t.Fatal(err)
				}

				// outside of the listed folder
				other := core.NewRecord(medias)
				other.Set("kind", core.MediaKindFolder)
				other.Set("name", "other")
				if err := app.Save(other); err != nil {
					t.Fatal(err)
				}
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"isError":false`,
				`\"folder\":\"/gallery\"`,
				`\"path\":\"/gallery/nested\"`,
			},
			NotExpectedContent: []string{`\"name\":\"other\"`},
			ExpectedEvents:     map[string]int{"*": 0},
		},
		{
			Name:    "list_media missing folder",
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("list_media", map[string]any{"folder": "/missing"}),
			Headers: bearer,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"isError":false`, `\"folder\":\"/missing\"`, `\"exists\":false`, `\"items\":[]`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestMCPMediaSearch(t *testing.T) {
	t.Parallel()

	scenario := tests.ApiScenario{
		Name:    "list_media search across folders",
		Method:  http.MethodPost,
		URL:     "/api/mcp",
		Body:    mcpCallBody("list_media", map[string]any{"search": "banner"}),
		Headers: map[string]string{"Authorization": "Bearer " + testMCPToken},
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			createMCPKeyFixture(t, app, testMCPToken, nil)

			medias, err := app.FindCollectionByNameOrId(core.CollectionNameMedias)
			if err != nil {
				t.Fatal(err)
			}

			parent := core.NewRecord(medias)
			parent.Set("kind", core.MediaKindFolder)
			parent.Set("name", "deep")
			if err := app.Save(parent); err != nil {
				t.Fatal(err)
			}

			match := core.NewRecord(medias)
			match.Set("kind", core.MediaKindFolder)
			match.Set("name", "home-banner")
			match.Set("parent", parent.Id)
			if err := app.Save(match); err != nil {
				t.Fatal(err)
			}
		},
		ExpectedStatus: 200,
		ExpectedContent: []string{
			`"isError":false`,
			`\"path\":\"/deep/home-banner\"`,
		},
		NotExpectedContent: []string{`\"name\":\"deep\"`},
		ExpectedEvents:     map[string]int{"*": 0},
	}

	scenario.Test(t)
}

func TestMCPUploadMediaContentFormats(t *testing.T) {
	t.Parallel()

	const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><rect width="10" height="10" fill="red"/></svg>`

	std := base64.StdEncoding.EncodeToString(testMCPPng)
	urlSafeRaw := base64.RawURLEncoding.EncodeToString(testMCPPng)

	// wrapped in lines and with literal (double JSON encoded) escapes
	wrapped := ""
	for i := 0; i < len(std); i += 20 {
		wrapped += std[i:min(i+20, len(std))] + `\n`
	}

	scenarios := []struct {
		name         string
		args         map[string]any
		expectedPath string
		expectedMime string
		expectedSize int
	}{
		{"standard base64", map[string]any{"name": "a.png", "content": std}, "/a.png", "image/png", len(testMCPPng)},
		{"url-safe base64 without padding", map[string]any{"name": "b.png", "content": urlSafeRaw}, "/b.png", "image/png", len(testMCPPng)},
		{"line wrapped with escaped newlines", map[string]any{"name": "c.png", "content": "  " + wrapped + "\n"}, "/c.png", "image/png", len(testMCPPng)},
		{"base64 data uri with charset", map[string]any{"name": "d.png", "content": "data:image/png;charset=binary;base64," + std}, "/d.png", "image/png", len(testMCPPng)},
		{"raw svg markup", map[string]any{"name": "e.svg", "content": svg}, "/e.svg", "image/svg+xml", len(svg)},
		{"utf8 svg data uri", map[string]any{"name": "f.svg", "content": "data:image/svg+xml;utf8," + url.PathEscape(svg)}, "/f.svg", "image/svg+xml", len(svg)},
		{"explicit text encoding", map[string]any{"name": "g.md", "content": "# Hello", "encoding": "text"}, "/g.md", "text/plain; charset=utf-8", len("# Hello")},
		{"nested missing folders", map[string]any{"name": "h.png", "folder": "/blog/2026/covers/", "content": std}, "/blog/2026/covers/h.png", "image/png", len(testMCPPng)},
	}

	for _, s := range scenarios {
		scenario := tests.ApiScenario{
			Name:    "upload_media " + s.name,
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("upload_media", s.args),
			Headers: map[string]string{"Authorization": "Bearer " + testMCPToken},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"isError":false`, `\"path\":\"` + s.expectedPath + `\"`},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				media := findTestMediaByPath(t, app, s.expectedPath)
				if media.GetInt("size") != s.expectedSize {
					t.Fatalf("Expected size %d, got %d", s.expectedSize, media.GetInt("size"))
				}
				if !strings.HasPrefix(media.GetString("mime"), strings.Split(s.expectedMime, ";")[0]) {
					t.Fatalf("Expected mime %q, got %q", s.expectedMime, media.GetString("mime"))
				}
			},
		}
		scenario.Test(t)
	}

	errScenarios := []struct {
		name     string
		args     map[string]any
		expected string
	}{
		{"truncated base64", map[string]any{"name": "x.png", "content": std[:len(std)-3]}, "looks truncated"},
		{"invalid character", map[string]any{"name": "x.png", "content": "iVBOR*w0KGgo"}, `invalid character '*' at position 5`},
		{"unsupported encoding", map[string]any{"name": "x.png", "content": std, "encoding": "hex"}, "unsupported encoding"},
	}

	for _, s := range errScenarios {
		scenario := tests.ApiScenario{
			Name:    "upload_media " + s.name,
			Method:  http.MethodPost,
			URL:     "/api/mcp",
			Body:    mcpCallBody("upload_media", s.args),
			Headers: map[string]string{"Authorization": "Bearer " + testMCPToken},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createMCPKeyFixture(t, app, testMCPToken, nil)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"isError":true`, s.expected},
			ExpectedEvents:  map[string]int{"*": 0},
		}
		scenario.Test(t)
	}
}
