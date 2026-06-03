package apis

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
)

func TestTestTelegramCredentials(t *testing.T) {
	t.Parallel()

	var gotGetMe bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			gotGetMe = true
			w.Write([]byte(`{"ok":true,"result":{"id":123}}`))
		default:
			t.Fatalf("Unexpected Telegram test request: %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	err := testTelegramCredentials(context.Background(), server.Client(), core.TelegramCredentialsConfig{
		Enabled:     true,
		BaseURL:     server.URL,
		AccessToken: "123:abc",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !gotGetMe {
		t.Fatal("Expected the Telegram getMe request to run")
	}
}

func TestTestGoogleSheetsCredentials(t *testing.T) {
	t.Parallel()

	var gotToken bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			t.Fatalf("Unexpected Google Sheets test request: %s %s", r.Method, r.URL.String())
		}

		gotToken = true
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("grant_type") != "authorization_code" {
			t.Fatalf("Unexpected grant_type %q", r.Form.Get("grant_type"))
		}
		if r.Form.Get("redirect_uri") != "https://example.com/oauth2/callback" {
			t.Fatalf("Unexpected redirect_uri %q", r.Form.Get("redirect_uri"))
		}
		if r.Form.Get("client_id") != "client_id" {
			t.Fatalf("Unexpected client_id %q", r.Form.Get("client_id"))
		}
		if r.Form.Get("client_secret") != "client_secret" {
			t.Fatalf("Unexpected client_secret %q", r.Form.Get("client_secret"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer server.Close()

	err := testGoogleSheetsCredentials(context.Background(), server.Client(), server.URL+"/token", core.GoogleSheetsCredentialsConfig{
		Enabled:          true,
		OAuthRedirectURL: "https://example.com/oauth2/callback",
		ClientID:         "client_id",
		ClientSecret:     "client_secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !gotToken {
		t.Fatal("Expected the Google OAuth token request to run")
	}
}
