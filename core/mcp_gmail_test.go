package core_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/types"
)

type fakeGoogle struct {
	mu        sync.Mutex
	refreshes int
	exchanges int
	sentRaw   string
	sentBody  map[string]any
}

func b64url(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

func (f *fakeGoogle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")

	if r.URL.Path == "/token" {
		_ = r.ParseForm()
		if r.Form.Get("client_id") != "cid" || r.Form.Get("client_secret") != "csecret" {
			w.WriteHeader(401)
			fmt.Fprint(w, `{"error":"invalid_client"}`)
			return
		}
		switch r.Form.Get("grant_type") {
		case "refresh_token":
			if r.Form.Get("refresh_token") == "revoked" {
				w.WriteHeader(400)
				fmt.Fprint(w, `{"error":"invalid_grant","error_description":"Token has been expired or revoked."}`)
				return
			}
			f.refreshes++
			fmt.Fprint(w, `{"access_token":"fresh_token","expires_in":3600}`)
		case "authorization_code":
			if r.Form.Get("code") != "good_code" || !strings.HasSuffix(r.Form.Get("redirect_uri"), core.GoogleOAuthCallbackPath) {
				w.WriteHeader(400)
				fmt.Fprint(w, `{"error":"invalid_grant"}`)
				return
			}
			f.exchanges++
			fmt.Fprint(w, `{"access_token":"fresh_token","refresh_token":"new_refresh","expires_in":3600}`)
		}
		return
	}

	if r.Header.Get("Authorization") != "Bearer fresh_token" {
		w.WriteHeader(401)
		fmt.Fprint(w, `{"error":{"message":"Invalid Credentials"}}`)
		return
	}

	switch {
	case r.URL.Path == "/userinfo":
		fmt.Fprint(w, `{"email":"me@example.com"}`)
	case r.URL.Path == "/gmail/v1/users/me/messages":
		if r.URL.Query().Get("q") != "is:unread" || r.URL.Query().Get("maxResults") != "5" {
			w.WriteHeader(400)
			fmt.Fprintf(w, `{"error":{"message":"unexpected query %s"}}`, r.URL.RawQuery)
			return
		}
		fmt.Fprint(w, `{"messages":[{"id":"m1","threadId":"t1"},{"id":"m2","threadId":"t2"}],"resultSizeEstimate":2}`)
	case r.URL.Path == "/gmail/v1/users/me/messages/m1":
		if r.URL.Query().Get("format") == "metadata" {
			fmt.Fprint(w, `{"id":"m1","threadId":"t1","labelIds":["UNREAD","INBOX"],"snippet":"Hello &amp; welcome","payload":{"headers":[`+
				`{"name":"From","value":"Alice <alice@example.com>"},{"name":"Reply-To","value":"support@example.com"},`+
				`{"name":"Subject","value":"Invoice 42"},{"name":"Message-ID","value":"<abc@mail>"},{"name":"Date","value":"Mon, 20 Oct 2026"}]}}`)
			return
		}
		fmt.Fprintf(w, `{"id":"m1","threadId":"t1","labelIds":["INBOX"],"payload":{"mimeType":"multipart/mixed","headers":[{"name":"Subject","value":"Invoice 42"},{"name":"From","value":"Alice <alice@example.com>"}],"parts":[`+
			`{"mimeType":"multipart/alternative","parts":[{"mimeType":"text/plain","body":{"data":%q}},{"mimeType":"text/html","body":{"data":%q}}]},`+
			`{"mimeType":"application/pdf","filename":"invoice.pdf","body":{"attachmentId":"x","size":10}}]}}`,
			b64url("Hi,\nplease pay the invoice."), b64url("<p>ignored</p>"))
	case r.URL.Path == "/gmail/v1/users/me/messages/m2":
		fmt.Fprintf(w, `{"id":"m2","threadId":"t2","payload":{"mimeType":"text/html","headers":[{"name":"Subject","value":"News"}],"body":{"data":%q}}}`,
			b64url("<html><style>.x{}</style><body><h1>Big &amp; news</h1><p>Line one<br>Line two</p></body></html>"))
	case r.URL.Path == "/gmail/v1/users/me/threads/t1":
		fmt.Fprintf(w, `{"id":"t1","messages":[{"id":"m1","threadId":"t1","payload":{"mimeType":"text/plain","headers":[{"name":"Subject","value":"Invoice 42"}],"body":{"data":%q}}}]}`, b64url("first"))
	case r.URL.Path == "/gmail/v1/users/me/messages/send" && r.Method == http.MethodPost:
		body := map[string]any{}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		decoded, _ := base64.RawURLEncoding.DecodeString(fmt.Sprint(body["raw"]))
		f.sentRaw = string(decoded)
		f.sentBody = body
		fmt.Fprint(w, `{"id":"sent1","threadId":"t1"}`)
	default:
		w.WriteHeader(404)
		fmt.Fprintf(w, `{"error":{"message":"not found %s"}}`, r.URL.Path)
	}
}

func setupFakeGoogle(t *testing.T) *fakeGoogle {
	fake := &fakeGoogle{}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)

	original := core.GoogleEndpoints
	core.GoogleEndpoints.AuthURL = server.URL + "/auth"
	core.GoogleEndpoints.TokenURL = server.URL + "/token"
	core.GoogleEndpoints.UserInfoURL = server.URL + "/userinfo"
	core.GoogleEndpoints.GmailAPIURL = server.URL + "/gmail/v1"
	t.Cleanup(func() { core.GoogleEndpoints = original })

	return fake
}

func createGmailConnection(t *testing.T, app core.App, refreshToken string) *core.MCPServer {
	server := core.NewMCPServer()
	server.Name = "Support inbox"
	server.Kind = core.MCPServerKindGmail
	server.ClientId = "cid"
	server.ClientSecret = "csecret"
	if refreshToken != "" {
		server.OAuth = core.MCPServerOAuth{
			RefreshToken: refreshToken,
			AccessToken:  "expired_token",
			Expiry:       types.NowDateTime().Add(-time.Hour),
			Email:        "me@example.com",
		}
	}
	if err := core.SaveMCPServer(app, server); err != nil {
		t.Fatal(err)
	}
	return server
}

func callGmailTool(t *testing.T, session core.MCPToolSession, name string, args map[string]any) (map[string]any, string, bool) {
	t.Helper()

	result, err := session.CallTool(context.Background(), name, args)
	if err != nil {
		t.Fatal(err)
	}

	text := result.Text()
	parsed := map[string]any{}
	_ = json.Unmarshal([]byte(text), &parsed)

	return parsed, text, result.IsError
}

// note: not parallel because the tests override the global Google endpoints
func TestGmailToolSession(t *testing.T) {
	fake := setupFakeGoogle(t)

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	server := createGmailConnection(t, app, "refresh_1")
	session := server.NewToolSession(app)

	tools, err := session.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	if strings.Join(names, ",") != "gmail_search_emails,gmail_read_email,gmail_read_thread,gmail_send_email" {
		t.Fatalf("Unexpected tools %v", names)
	}

	// the expired access token was refreshed and persisted
	stored, _ := core.FindMCPServerById(app, server.Id)
	if stored.OAuth.AccessToken != "fresh_token" || stored.OAuth.RefreshToken != "refresh_1" || fake.refreshes != 1 {
		t.Fatalf("Expected the refreshed token to be stored, got %+v (refreshes %d)", stored.OAuth, fake.refreshes)
	}

	t.Run("search", func(t *testing.T) {
		result, text, isError := callGmailTool(t, session, "gmail_search_emails", map[string]any{"query": "is:unread", "maxResults": 5})
		if isError {
			t.Fatal(text)
		}
		messages := result["messages"].([]any)
		first := messages[0].(map[string]any)
		if len(messages) != 2 || first["subject"] != "Invoice 42" || first["unread"] != true || first["snippet"] != "Hello & welcome" {
			t.Fatalf("Unexpected search result %s", text)
		}
	})

	t.Run("read multipart", func(t *testing.T) {
		result, text, isError := callGmailTool(t, session, "gmail_read_email", map[string]any{"id": "m1"})
		if isError {
			t.Fatal(text)
		}
		if result["body"] != "Hi,\nplease pay the invoice." || fmt.Sprint(result["attachments"]) != "[invoice.pdf]" {
			t.Fatalf("Unexpected message %s", text)
		}
	})

	t.Run("read html only", func(t *testing.T) {
		result, text, _ := callGmailTool(t, session, "gmail_read_email", map[string]any{"id": "m2"})
		if result["body"] != "Big & news\nLine one\nLine two" {
			t.Fatalf("Unexpected html body %q (%s)", result["body"], text)
		}
	})

	t.Run("read thread", func(t *testing.T) {
		result, text, _ := callGmailTool(t, session, "gmail_read_thread", map[string]any{"threadId": "t1"})
		messages, _ := result["messages"].([]any)
		if result["threadId"] != "t1" || len(messages) != 1 {
			t.Fatalf("Unexpected thread %s", text)
		}
	})

	t.Run("reply", func(t *testing.T) {
		result, text, isError := callGmailTool(t, session, "gmail_send_email", map[string]any{
			"replyToMessageId": "m1",
			"body":             "Paid, thanks!",
			"html":             "<p>Paid, thanks!</p>",
		})
		if isError || result["sent"] != true {
			t.Fatalf("Expected sent result, got %s", text)
		}

		fake.mu.Lock()
		raw, body := fake.sentRaw, fake.sentBody
		fake.mu.Unlock()

		if body["threadId"] != "t1" {
			t.Fatalf("Expected the reply to be sent in thread t1, got %v", body["threadId"])
		}

		msg, err := mail.ReadMessage(strings.NewReader(raw))
		if err != nil {
			t.Fatalf("Invalid MIME message: %v\n%s", err, raw)
		}
		subject, _ := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
		if msg.Header.Get("To") != "<support@example.com>" || subject != "Re: Invoice 42" ||
			msg.Header.Get("In-Reply-To") != "<abc@mail>" || msg.Header.Get("From") != "me@example.com" {
			t.Fatalf("Unexpected headers %v (subject %q)", msg.Header, subject)
		}
		if !strings.Contains(msg.Header.Get("Content-Type"), "multipart/alternative") ||
			!strings.Contains(raw, base64.StdEncoding.EncodeToString([]byte("Paid, thanks!"))) {
			t.Fatalf("Unexpected body\n%s", raw)
		}
	})

	t.Run("send validation", func(t *testing.T) {
		_, text, isError := callGmailTool(t, session, "gmail_send_email", map[string]any{"to": []any{"not an email"}, "body": "x"})
		if !isError || !strings.Contains(text, "invalid to address") {
			t.Fatalf("Expected invalid address error, got %s", text)
		}

		_, text, isError = callGmailTool(t, session, "gmail_send_email", map[string]any{"body": "x"})
		if !isError || !strings.Contains(text, "at least one recipient") {
			t.Fatalf("Expected missing recipient error, got %s", text)
		}

		_, text, isError = callGmailTool(t, session, "gmail_send_email", map[string]any{"to": "bob@example.com", "subject": "a\r\nBcc: evil@example.com", "body": "x"})
		if !isError || !strings.Contains(text, "single line") {
			t.Fatalf("Expected header injection to be rejected, got %s", text)
		}
	})

	t.Run("api error", func(t *testing.T) {
		_, text, isError := callGmailTool(t, session, "gmail_read_email", map[string]any{"id": "missing"})
		if !isError || !strings.Contains(text, "Gmail API error (404)") {
			t.Fatalf("Expected API error, got %s", text)
		}
	})

	t.Run("not connected", func(t *testing.T) {
		notConnected := createGmailConnection(t, app, "")
		_, err := notConnected.NewToolSession(app).ListTools(context.Background())
		if err == nil || !strings.Contains(err.Error(), "is not connected") {
			t.Fatalf("Expected not connected error, got %v", err)
		}
	})

	t.Run("revoked", func(t *testing.T) {
		revoked := createGmailConnection(t, app, "revoked")
		_, err := revoked.NewToolSession(app).ListTools(context.Background())
		if err == nil || !strings.Contains(err.Error(), "reconnect the Gmail account") {
			t.Fatalf("Expected revoked error, got %v", err)
		}
	})

	t.Run("connect account", func(t *testing.T) {
		connection := createGmailConnection(t, app, "")

		state := core.NewMCPServerOAuthState(app, connection.Id)
		authURL, err := connection.GoogleOAuthAuthURL(app, state)
		if err != nil {
			t.Fatal(err)
		}
		parsed, _ := url.Parse(authURL)
		q := parsed.Query()
		if q.Get("client_id") != "cid" || q.Get("access_type") != "offline" || q.Get("state") != state ||
			!strings.Contains(q.Get("scope"), "gmail.send") || q.Get("redirect_uri") != core.GoogleOAuthRedirectURL(app) {
			t.Fatalf("Unexpected auth URL %s", authURL)
		}

		serverId, err := core.ConsumeMCPServerOAuthState(app, state)
		if err != nil || serverId != connection.Id {
			t.Fatalf("Expected the state to resolve to %s, got %s (%v)", connection.Id, serverId, err)
		}
		if _, err := core.ConsumeMCPServerOAuthState(app, state); err == nil {
			t.Fatal("Expected the state to be usable only once")
		}

		if err := connection.ConnectGoogleAccount(context.Background(), app, "good_code"); err != nil {
			t.Fatal(err)
		}
		stored, _ := core.FindMCPServerById(app, connection.Id)
		if !stored.OAuth.IsConnected() || stored.OAuth.Email != "me@example.com" || stored.OAuth.RefreshToken != "new_refresh" {
			t.Fatalf("Unexpected stored oauth %+v", stored.OAuth)
		}

		raw, _ := json.Marshal(stored)
		if strings.Contains(string(raw), "new_refresh") || strings.Contains(string(raw), "fresh_token") || strings.Contains(string(raw), "csecret") ||
			!strings.Contains(string(raw), `"connected":true`) || !strings.Contains(string(raw), `"account":"me@example.com"`) {
			t.Fatalf("Unexpected exported connection %s", raw)
		}
	})

	t.Run("fallback client credentials", func(t *testing.T) {
		connection := createGmailConnection(t, app, "")
		connection.ClientId = ""
		connection.ClientSecret = ""

		if _, _, err := connection.GoogleOAuthClient(app); err == nil {
			t.Fatal("Expected missing client error")
		}

		app.Settings().Credentials.GoogleSheets.ClientID = "app_cid"
		app.Settings().Credentials.GoogleSheets.ClientSecret = "app_secret"
		id, secret, err := connection.GoogleOAuthClient(app)
		if err != nil || id != "app_cid" || secret != "app_secret" {
			t.Fatalf("Expected the app credentials fallback, got %q %q %v", id, secret, err)
		}
	})
}
