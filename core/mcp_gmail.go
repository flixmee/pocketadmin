package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/tools/mcpclient"
	"github.com/pocketbase/pocketbase/tools/security"
	"github.com/pocketbase/pocketbase/tools/types"
)

// GoogleEndpoints lists the Google API endpoints used by the Gmail
// connection (overridable for tests).
var GoogleEndpoints = struct {
	AuthURL     string
	TokenURL    string
	UserInfoURL string
	GmailAPIURL string
}{
	AuthURL:     "https://accounts.google.com/o/oauth2/v2/auth",
	TokenURL:    "https://oauth2.googleapis.com/token",
	UserInfoURL: "https://openidconnect.googleapis.com/v1/userinfo",
	GmailAPIURL: "https://gmail.googleapis.com/gmail/v1",
}

// GmailOAuthScopes are the OAuth scopes requested for a Gmail connection.
var GmailOAuthScopes = []string{
	"openid",
	"email",
	"https://www.googleapis.com/auth/gmail.readonly",
	"https://www.googleapis.com/auth/gmail.send",
}

// GoogleOAuthCallbackPath is the API path of the Google OAuth redirect URI.
const GoogleOAuthCallbackPath = "/api/mcp/oauth/google/callback"

const (
	mcpOAuthStateTTL      = 15 * time.Minute
	mcpOAuthStateKey      = "pbMCPOAuthState_"
	gmailHTTPTimeout      = 30 * time.Second
	gmailMaxSearchResults = 50
	gmailMaxBodyChars     = 20000
	gmailMaxResponseSize  = 25 << 20
)

// GoogleOAuthRedirectURL returns the default redirect URI based on the
// application URL setting (used when the admin UI doesn't provide one).
func GoogleOAuthRedirectURL(app App) string {
	return strings.TrimRight(app.Settings().Meta.AppURL, "/") + GoogleOAuthCallbackPath
}

// NormalizeGoogleOAuthRedirectURL validates a redirect URI provided by the
// admin UI (e.g. the public URL behind a reverse proxy). It must be an
// absolute http(s) URL pointing to the Google OAuth callback route.
func NormalizeGoogleOAuthRedirectURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		!strings.HasSuffix(parsed.Path, GoogleOAuthCallbackPath) {
		return "", errors.New("invalid redirect URL - it must be an absolute http(s) URL ending with " + GoogleOAuthCallbackPath)
	}

	return parsed.String(), nil
}

// GoogleOAuthClient returns the Google OAuth client credentials of the
// connection, falling back to the application Google credentials.
func (m *MCPServer) GoogleOAuthClient(app App) (string, string, error) {
	if m.ClientId != "" && m.ClientSecret != "" {
		return m.ClientId, m.ClientSecret, nil
	}

	google := app.Settings().Credentials.GoogleSheets
	if google.ClientID != "" && google.ClientSecret != "" {
		return google.ClientID, google.ClientSecret, nil
	}

	return "", "", errors.New("missing Google OAuth client ID and secret (set them on the connection or in Settings > Credentials)")
}

// GoogleOAuthAuthURL returns the Google consent screen URL to connect the Gmail account.
func (m *MCPServer) GoogleOAuthAuthURL(app App, state string, redirectURL string) (string, error) {
	clientId, _, err := m.GoogleOAuthClient(app)
	if err != nil {
		return "", err
	}

	query := url.Values{}
	query.Set("client_id", clientId)
	query.Set("redirect_uri", redirectURL)
	query.Set("response_type", "code")
	query.Set("scope", strings.Join(GmailOAuthScopes, " "))
	query.Set("access_type", "offline")
	query.Set("prompt", "consent")
	query.Set("include_granted_scopes", "true")
	query.Set("state", state)

	return GoogleEndpoints.AuthURL + "?" + query.Encode(), nil
}

type mcpOAuthState struct {
	ServerId    string
	RedirectURL string
	Expires     time.Time
}

// NewMCPServerOAuthState creates a one-time OAuth state bound to the
// connection and the redirect URI used for the sign-in (the same URI
// must be sent again when exchanging the code).
func NewMCPServerOAuthState(app App, serverId string, redirectURL string) string {
	state := security.RandomString(40)
	app.Store().Set(mcpOAuthStateKey+state, mcpOAuthState{
		ServerId:    serverId,
		RedirectURL: redirectURL,
		Expires:     time.Now().Add(mcpOAuthStateTTL),
	})
	return state
}

// ConsumeMCPServerOAuthState validates and removes the OAuth state,
// returning the id of the connection and the redirect URI it was created for.
func ConsumeMCPServerOAuthState(app App, state string) (string, string, error) {
	if state == "" {
		return "", "", errors.New("missing OAuth state")
	}

	raw := app.Store().Get(mcpOAuthStateKey + state)
	app.Store().Remove(mcpOAuthStateKey + state)

	data, ok := raw.(mcpOAuthState)
	if !ok || time.Now().After(data.Expires) {
		return "", "", errors.New("invalid or expired OAuth state - please try connecting again")
	}

	return data.ServerId, data.RedirectURL, nil
}

type googleTokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func googleTokenRequest(ctx context.Context, form url.Values) (*googleTokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, GoogleEndpoints.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := (&http.Client{Timeout: gmailHTTPTimeout}).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	token := &googleTokenResponse{}
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	_ = json.Unmarshal(raw, token)

	if res.StatusCode >= 400 || token.AccessToken == "" {
		message := token.ErrorDescription
		if message == "" {
			message = token.Error
		}
		if message == "" {
			message = strings.TrimSpace(string(raw))
		}
		if token.Error == "invalid_grant" {
			return token, errors.New("the Google authorization has expired or was revoked - reconnect the Gmail account")
		}
		return token, fmt.Errorf("Google token request failed (%d): %s", res.StatusCode, message)
	}

	return token, nil
}

// ConnectGoogleAccount exchanges the OAuth code and stores the tokens
// and the account email in the connection.
func (m *MCPServer) ConnectGoogleAccount(ctx context.Context, app App, code string, redirectURL string) error {
	clientId, clientSecret, err := m.GoogleOAuthClient(app)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("code", code)
	form.Set("client_id", clientId)
	form.Set("client_secret", clientSecret)
	form.Set("redirect_uri", redirectURL)
	form.Set("grant_type", "authorization_code")

	token, err := googleTokenRequest(ctx, form)
	if err != nil {
		return err
	}

	refreshToken := token.RefreshToken
	if refreshToken == "" {
		refreshToken = m.OAuth.RefreshToken
	}
	if refreshToken == "" {
		return errors.New("Google didn't return a refresh token - remove the app access from your Google account and connect again")
	}

	email, err := googleAccountEmail(ctx, token.AccessToken)
	if err != nil {
		return err
	}

	m.OAuth = MCPServerOAuth{
		RefreshToken: refreshToken,
		AccessToken:  token.AccessToken,
		Expiry:       types.NowDateTime().Add(time.Duration(token.ExpiresIn) * time.Second),
		Email:        email,
	}

	return SaveMCPServerOAuth(app, m)
}

func googleAccountEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, GoogleEndpoints.UserInfoURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	res, err := (&http.Client{Timeout: gmailHTTPTimeout}).Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	var info struct {
		Email string `json:"email"`
	}
	if res.StatusCode >= 400 {
		return "", fmt.Errorf("failed to load the Google account (%d)", res.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&info); err != nil {
		return "", err
	}

	return info.Email, nil
}

// -------------------------------------------------------------------
// Gmail tools session
// -------------------------------------------------------------------

var gmailTools = []mcpclient.Tool{
	{
		Name:        "gmail_search_emails",
		Title:       "Search emails",
		Description: "Searches the Gmail mailbox and returns the matching messages (id, threadId, from, to, subject, date, snippet). Supports the Gmail search syntax, e.g. `from:alice@example.com is:unread after:2026/01/01 subject:invoice`.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"Gmail search query. Empty returns the latest messages."},"maxResults":{"type":"integer","minimum":1,"maximum":50,"description":"Max number of messages (default 10)."}}}`),
	},
	{
		Name:        "gmail_read_email",
		Title:       "Read email",
		Description: "Returns a single Gmail message with its headers, plain text body and attachment names.",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"The message id."}},"required":["id"]}`),
	},
	{
		Name:        "gmail_read_thread",
		Title:       "Read thread",
		Description: "Returns all messages of a Gmail conversation thread (oldest first).",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"threadId":{"type":"string","description":"The thread id."}},"required":["threadId"]}`),
	},
	{
		Name:  "gmail_send_email",
		Title: "Send email",
		Description: "Sends an email from the connected Gmail account. To reply in an existing conversation set `replyToMessageId` " +
			"(`to` and `subject` then default to the original sender and \"Re: <subject>\").",
		InputSchema: json.RawMessage(`{"type":"object","properties":{` +
			`"to":{"type":"array","items":{"type":"string"},"description":"Recipient addresses."},` +
			`"cc":{"type":"array","items":{"type":"string"}},` +
			`"bcc":{"type":"array","items":{"type":"string"}},` +
			`"subject":{"type":"string"},` +
			`"body":{"type":"string","description":"Plain text body."},` +
			`"html":{"type":"string","description":"Optional HTML version of the body."},` +
			`"replyToMessageId":{"type":"string","description":"Id of the message to reply to."}` +
			`},"required":["body"]}`),
	},
}

type gmailToolSession struct {
	app    App
	server *MCPServer
	client *http.Client
	mu     sync.Mutex
}

func newGmailToolSession(app App, server *MCPServer) *gmailToolSession {
	return &gmailToolSession{app: app, server: server, client: &http.Client{Timeout: gmailHTTPTimeout}}
}

// ListTools returns the Gmail tools (it also verifies that the account is connected).
func (s *gmailToolSession) ListTools(ctx context.Context) ([]mcpclient.Tool, error) {
	if _, err := s.accessToken(ctx); err != nil {
		return nil, err
	}

	return gmailTools, nil
}

func (s *gmailToolSession) Close(ctx context.Context) error {
	return nil
}

// CallTool executes the Gmail tool. API failures are reported as tool errors.
func (s *gmailToolSession) CallTool(ctx context.Context, name string, args map[string]any) (*mcpclient.CallToolResult, error) {
	var result any
	var err error

	switch name {
	case "gmail_search_emails":
		result, err = s.search(ctx, args)
	case "gmail_read_email":
		result, err = s.readMessage(ctx, toString(args["id"]))
	case "gmail_read_thread":
		result, err = s.readThread(ctx, toString(args["threadId"]))
	case "gmail_send_email":
		result, err = s.send(ctx, args)
	default:
		err = fmt.Errorf("unknown tool %q", name)
	}

	if err != nil {
		return &mcpclient.CallToolResult{
			Content: []mcpclient.Content{{Type: "text", Text: err.Error()}},
			IsError: true,
		}, nil
	}

	raw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	return &mcpclient.CallToolResult{Content: []mcpclient.Content{{Type: "text", Text: string(raw)}}}, nil
}

// accessToken returns a valid access token, refreshing it when needed.
func (s *gmailToolSession) accessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.server.OAuth.IsConnected() {
		return "", fmt.Errorf("the Gmail account of %q is not connected", s.server.Name)
	}

	if !mcpServerTokenExpired(s.server.OAuth) {
		return s.server.OAuth.AccessToken, nil
	}

	clientId, clientSecret, err := s.server.GoogleOAuthClient(s.app)
	if err != nil {
		return "", err
	}

	form := url.Values{}
	form.Set("client_id", clientId)
	form.Set("client_secret", clientSecret)
	form.Set("refresh_token", s.server.OAuth.RefreshToken)
	form.Set("grant_type", "refresh_token")

	token, err := googleTokenRequest(ctx, form)
	if err != nil {
		return "", err
	}

	s.server.OAuth.AccessToken = token.AccessToken
	s.server.OAuth.Expiry = types.NowDateTime().Add(time.Duration(token.ExpiresIn) * time.Second)
	if token.RefreshToken != "" {
		s.server.OAuth.RefreshToken = token.RefreshToken
	}

	if err := SaveMCPServerOAuth(s.app, s.server); err != nil {
		s.app.Logger().Warn("Failed to save the refreshed Gmail token", "connection", s.server.Id, "error", err)
	}

	return s.server.OAuth.AccessToken, nil
}

func (s *gmailToolSession) api(ctx context.Context, method string, path string, body any, result any) error {
	token, err := s.accessToken(ctx)
	if err != nil {
		return err
	}

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(GoogleEndpoints.GmailAPIURL, "/")+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("Gmail request failed: %w", err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(res.Body, gmailMaxResponseSize))
	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(raw, &apiErr)
		message := apiErr.Error.Message
		if message == "" {
			message = strings.TrimSpace(string(raw))
		}
		return fmt.Errorf("Gmail API error (%d): %s", res.StatusCode, message)
	}

	if result != nil {
		return json.Unmarshal(raw, result)
	}

	return nil
}

type gmailMessagePart struct {
	MimeType string `json:"mimeType"`
	Filename string `json:"filename"`
	Headers  []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"headers"`
	Body struct {
		Size int    `json:"size"`
		Data string `json:"data"`
	} `json:"body"`
	Parts []gmailMessagePart `json:"parts"`
}

type gmailMessage struct {
	Id       string           `json:"id"`
	ThreadId string           `json:"threadId"`
	LabelIds []string         `json:"labelIds"`
	Snippet  string           `json:"snippet"`
	Payload  gmailMessagePart `json:"payload"`
}

func (p *gmailMessagePart) header(name string) string {
	for _, h := range p.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

func (s *gmailToolSession) search(ctx context.Context, args map[string]any) (any, error) {
	maxResults := int(toFloat(args["maxResults"]))
	if maxResults <= 0 {
		maxResults = 10
	}
	maxResults = min(maxResults, gmailMaxSearchResults)

	query := url.Values{}
	query.Set("maxResults", fmt.Sprint(maxResults))
	if q := strings.TrimSpace(toString(args["query"])); q != "" {
		query.Set("q", q)
	}

	var list struct {
		Messages []struct {
			Id string `json:"id"`
		} `json:"messages"`
		ResultSizeEstimate int `json:"resultSizeEstimate"`
	}
	if err := s.api(ctx, http.MethodGet, "/users/me/messages?"+query.Encode(), nil, &list); err != nil {
		return nil, err
	}

	items := make([]map[string]any, 0, len(list.Messages))
	for _, item := range list.Messages {
		message := &gmailMessage{}
		path := "/users/me/messages/" + url.PathEscape(item.Id) +
			"?format=metadata&metadataHeaders=From&metadataHeaders=To&metadataHeaders=Subject&metadataHeaders=Date"
		if err := s.api(ctx, http.MethodGet, path, nil, message); err != nil {
			return nil, err
		}

		items = append(items, map[string]any{
			"id":       message.Id,
			"threadId": message.ThreadId,
			"from":     message.Payload.header("From"),
			"to":       message.Payload.header("To"),
			"subject":  message.Payload.header("Subject"),
			"date":     message.Payload.header("Date"),
			"snippet":  html.UnescapeString(message.Snippet),
			"unread":   containsString(message.LabelIds, "UNREAD"),
		})
	}

	return map[string]any{"messages": items, "resultSizeEstimate": list.ResultSizeEstimate}, nil
}

func (s *gmailToolSession) readMessage(ctx context.Context, id string) (any, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("missing required argument \"id\"")
	}

	message := &gmailMessage{}
	if err := s.api(ctx, http.MethodGet, "/users/me/messages/"+url.PathEscape(id)+"?format=full", nil, message); err != nil {
		return nil, err
	}

	return formatGmailMessage(message), nil
}

func (s *gmailToolSession) readThread(ctx context.Context, threadId string) (any, error) {
	if strings.TrimSpace(threadId) == "" {
		return nil, errors.New("missing required argument \"threadId\"")
	}

	var thread struct {
		Id       string          `json:"id"`
		Messages []*gmailMessage `json:"messages"`
	}
	if err := s.api(ctx, http.MethodGet, "/users/me/threads/"+url.PathEscape(threadId)+"?format=full", nil, &thread); err != nil {
		return nil, err
	}

	messages := make([]map[string]any, 0, len(thread.Messages))
	for _, message := range thread.Messages {
		messages = append(messages, formatGmailMessage(message))
	}

	return map[string]any{"threadId": thread.Id, "messages": messages}, nil
}

func formatGmailMessage(message *gmailMessage) map[string]any {
	var plain, htmlBody []string
	var attachments []string
	collectGmailParts(&message.Payload, &plain, &htmlBody, &attachments)

	body := strings.TrimSpace(strings.Join(plain, "\n"))
	if body == "" && len(htmlBody) > 0 {
		body = htmlToText(strings.Join(htmlBody, "\n"))
	}
	if len(body) > gmailMaxBodyChars {
		body = body[:gmailMaxBodyChars] + "\n[truncated]"
	}

	return map[string]any{
		"id":          message.Id,
		"threadId":    message.ThreadId,
		"from":        message.Payload.header("From"),
		"to":          message.Payload.header("To"),
		"cc":          message.Payload.header("Cc"),
		"subject":     message.Payload.header("Subject"),
		"date":        message.Payload.header("Date"),
		"labels":      message.LabelIds,
		"body":        body,
		"attachments": attachments,
	}
}

func collectGmailParts(part *gmailMessagePart, plain *[]string, htmlBody *[]string, attachments *[]string) {
	if part.Filename != "" {
		*attachments = append(*attachments, part.Filename)
		return
	}

	if part.Body.Data != "" {
		data := decodeGmailData(part.Body.Data)
		switch {
		case strings.HasPrefix(part.MimeType, "text/plain"):
			*plain = append(*plain, data)
		case strings.HasPrefix(part.MimeType, "text/html"):
			*htmlBody = append(*htmlBody, data)
		}
	}

	for i := range part.Parts {
		collectGmailParts(&part.Parts[i], plain, htmlBody, attachments)
	}
}

func decodeGmailData(data string) string {
	data = strings.TrimRight(data, "=")
	decoded, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(data)
		if err != nil {
			return ""
		}
	}
	return string(decoded)
}

var (
	htmlDropBlocks   = regexp.MustCompile(`(?is)<(script|style|head)[^>]*>.*?</(script|style|head)>`)
	htmlLineBreaks   = regexp.MustCompile(`(?i)<(br|/p|/div|/li|/tr|/h[1-6])[^>]*>`)
	htmlTags         = regexp.MustCompile(`(?s)<[^>]+>`)
	htmlBlankLines   = regexp.MustCompile(`\n\s*\n+`)
	htmlInlineSpaces = regexp.MustCompile(`[ \t]+`)
)

func htmlToText(value string) string {
	value = htmlDropBlocks.ReplaceAllString(value, "")
	value = htmlLineBreaks.ReplaceAllString(value, "\n")
	value = htmlTags.ReplaceAllString(value, "")
	value = html.UnescapeString(value)
	value = htmlInlineSpaces.ReplaceAllString(value, " ")
	value = htmlBlankLines.ReplaceAllString(value, "\n\n")
	return strings.TrimSpace(value)
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func gmailAddressList(value any, field string) ([]string, error) {
	var raw []string
	switch v := value.(type) {
	case nil:
	case string:
		raw = strings.Split(v, ",")
	case []any:
		for _, item := range v {
			raw = append(raw, toString(item))
		}
	case []string:
		raw = v
	default:
		return nil, fmt.Errorf("argument %q must be a list of email addresses", field)
	}

	result := []string{}
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		address, err := mail.ParseAddress(item)
		if err != nil {
			return nil, fmt.Errorf("invalid %s address %q", field, item)
		}
		result = append(result, address.String())
	}

	return result, nil
}

func (s *gmailToolSession) send(ctx context.Context, args map[string]any) (any, error) {
	to, err := gmailAddressList(args["to"], "to")
	if err != nil {
		return nil, err
	}
	cc, err := gmailAddressList(args["cc"], "cc")
	if err != nil {
		return nil, err
	}
	bcc, err := gmailAddressList(args["bcc"], "bcc")
	if err != nil {
		return nil, err
	}

	subject := strings.TrimSpace(toString(args["subject"]))
	body := toString(args["body"])
	htmlBody := toString(args["html"])
	if strings.TrimSpace(body) == "" && strings.TrimSpace(htmlBody) == "" {
		return nil, errors.New("missing required argument \"body\"")
	}

	headers := [][2]string{}
	threadId := ""

	if replyToId := strings.TrimSpace(toString(args["replyToMessageId"])); replyToId != "" {
		original := &gmailMessage{}
		path := "/users/me/messages/" + url.PathEscape(replyToId) +
			"?format=metadata&metadataHeaders=From&metadataHeaders=Reply-To&metadataHeaders=Subject&metadataHeaders=Message-ID&metadataHeaders=References"
		if err := s.api(ctx, http.MethodGet, path, nil, original); err != nil {
			return nil, err
		}

		threadId = original.ThreadId

		if len(to) == 0 {
			replyTo := original.Payload.header("Reply-To")
			if replyTo == "" {
				replyTo = original.Payload.header("From")
			}
			if to, err = gmailAddressList(replyTo, "to"); err != nil {
				return nil, err
			}
		}

		if subject == "" {
			subject = original.Payload.header("Subject")
			if !strings.HasPrefix(strings.ToLower(subject), "re:") {
				subject = "Re: " + subject
			}
		}

		if messageId := original.Payload.header("Message-ID"); messageId != "" {
			references := strings.TrimSpace(original.Payload.header("References") + " " + messageId)
			headers = append(headers, [2]string{"In-Reply-To", messageId}, [2]string{"References", references})
		}
	}

	if len(to) == 0 && len(cc) == 0 && len(bcc) == 0 {
		return nil, errors.New("at least one recipient is required")
	}

	if strings.ContainsAny(subject, "\r\n") {
		return nil, errors.New("the subject must be a single line")
	}

	raw := buildGmailMIME(s.server.OAuth.Email, to, cc, bcc, subject, body, htmlBody, headers)

	payload := map[string]any{"raw": base64.RawURLEncoding.EncodeToString(raw)}
	if threadId != "" {
		payload["threadId"] = threadId
	}

	var sent struct {
		Id       string `json:"id"`
		ThreadId string `json:"threadId"`
	}
	if err := s.api(ctx, http.MethodPost, "/users/me/messages/send", payload, &sent); err != nil {
		return nil, err
	}

	return map[string]any{"sent": true, "id": sent.Id, "threadId": sent.ThreadId, "to": to, "subject": subject}, nil
}

func buildGmailMIME(from string, to, cc, bcc []string, subject, body, htmlBody string, extra [][2]string) []byte {
	var b strings.Builder

	writeHeader := func(name, value string) {
		value = strings.NewReplacer("\r", " ", "\n", " ").Replace(value)
		if value != "" {
			b.WriteString(name + ": " + value + "\r\n")
		}
	}

	writeHeader("From", from)
	writeHeader("To", strings.Join(to, ", "))
	writeHeader("Cc", strings.Join(cc, ", "))
	writeHeader("Bcc", strings.Join(bcc, ", "))
	writeHeader("Subject", mime.BEncoding.Encode("UTF-8", subject))
	for _, h := range extra {
		writeHeader(h[0], h[1])
	}
	writeHeader("MIME-Version", "1.0")

	writePart := func(contentType, content string) {
		b.WriteString("Content-Type: " + contentType + "; charset=\"UTF-8\"\r\n")
		b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		encoded := base64.StdEncoding.EncodeToString([]byte(content))
		for len(encoded) > 76 {
			b.WriteString(encoded[:76] + "\r\n")
			encoded = encoded[76:]
		}
		b.WriteString(encoded + "\r\n")
	}

	switch {
	case htmlBody == "":
		writePart("text/plain", body)
	case strings.TrimSpace(body) == "":
		writePart("text/html", htmlBody)
	default:
		boundary := "pa_" + security.RandomString(24)
		b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n")
		b.WriteString("--" + boundary + "\r\n")
		writePart("text/plain", body)
		b.WriteString("--" + boundary + "\r\n")
		writePart("text/html", htmlBody)
		b.WriteString("--" + boundary + "--\r\n")
	}

	return []byte(b.String())
}
