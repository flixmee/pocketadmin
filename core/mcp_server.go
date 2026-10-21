package core

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/tools/mcpclient"
	"github.com/pocketbase/pocketbase/tools/security"
	"github.com/pocketbase/pocketbase/tools/types"
)

const MCPServersTableName = "_mcpServers"

// MCP connection kinds.
const (
	// MCPServerKindHTTP is an external MCP server (Streamable HTTP).
	MCPServerKindHTTP = "http"

	// MCPServerKindGmail is the built-in Gmail tools connection (Google OAuth).
	MCPServerKindGmail = "gmail"
)

var _ Model = (*MCPServer)(nil)

// MCPServer defines a connection whose tools can be used by the AI
// automation steps - either an external MCP server or a built-in
// integration such as Gmail.
type MCPServer struct {
	BaseModel

	Name         string                `db:"name" json:"name"`
	Kind         string                `db:"kind" json:"kind"`
	URL          string                `db:"url" json:"url"`
	AuthToken    string                `db:"authToken" json:"-"`
	Headers      types.JSONMap[string] `db:"headers" json:"headers"`
	ClientId     string                `db:"clientId" json:"clientId"`
	ClientSecret string                `db:"clientSecret" json:"-"`
	OAuth        MCPServerOAuth        `db:"oauth" json:"-"`
	Active       bool                  `db:"active" json:"active"`
	Created      types.DateTime        `db:"created" json:"created"`
	Updated      types.DateTime        `db:"updated" json:"updated"`
}

// MCPServerOAuth holds the OAuth tokens of a connection (never exported).
type MCPServerOAuth struct {
	RefreshToken string         `json:"refreshToken,omitempty"`
	AccessToken  string         `json:"accessToken,omitempty"`
	Expiry       types.DateTime `json:"expiry"`
	Email        string         `json:"email,omitempty"`
}

// Value implements [driver.Valuer].
func (o MCPServerOAuth) Value() (driver.Value, error) {
	raw, err := json.Marshal(o)
	return string(raw), err
}

// Scan implements [sql.Scanner].
func (o *MCPServerOAuth) Scan(value any) error {
	*o = MCPServerOAuth{}

	var raw []byte
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		raw = []byte(v)
	case []byte:
		raw = v
	default:
		return errors.New("unsupported MCP server oauth value")
	}

	if len(raw) == 0 {
		return nil
	}

	return json.Unmarshal(raw, o)
}

// IsConnected reports whether the OAuth account was connected.
func (o MCPServerOAuth) IsConnected() bool {
	return o.RefreshToken != ""
}

func (m *MCPServer) TableName() string {
	return MCPServersTableName
}

// MarshalJSON implements [json.Marshaler] and exports whether the
// secrets are set without exposing them.
func (m MCPServer) MarshalJSON() ([]byte, error) {
	type alias MCPServer

	return json.Marshal(struct {
		alias
		HasAuthToken    bool   `json:"hasAuthToken"`
		HasClientSecret bool   `json:"hasClientSecret"`
		Connected       bool   `json:"connected"`
		Account         string `json:"account"`
	}{
		alias:           alias(m),
		HasAuthToken:    m.AuthToken != "",
		HasClientSecret: m.ClientSecret != "",
		Connected:       m.OAuth.IsConnected(),
		Account:         m.OAuth.Email,
	})
}

// Validate validates the user configurable server fields.
func (m *MCPServer) Validate() error {
	isHTTP := m.Kind == MCPServerKindHTTP

	return validation.ValidateStruct(m,
		validation.Field(&m.Name, validation.Required, validation.Length(1, 100)),
		validation.Field(&m.Kind, validation.Required, validation.In(MCPServerKindHTTP, MCPServerKindGmail)),
		validation.Field(
			&m.URL,
			validation.When(isHTTP, validation.Required, validation.Length(1, 2000), validation.By(validateMCPServerURL)),
		),
		validation.Field(&m.Headers, validation.By(validateMCPServerHeaders)),
		validation.Field(&m.ClientId, validation.Length(0, 500)),
	)
}

func validateMCPServerURL(value any) error {
	raw, _ := value.(string)

	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return validation.NewError("validation_invalid_url", "Must be a valid http(s) URL.")
	}

	return nil
}

func validateMCPServerHeaders(value any) error {
	headers, _ := value.(types.JSONMap[string])

	for name := range headers {
		if name == "" || strings.ContainsAny(name, " :\r\n\t") || http.CanonicalHeaderKey(name) == "" {
			return validation.NewError("validation_invalid_header", "Invalid header name "+name+".")
		}
	}

	return nil
}

// MCPToolSession is a session that exposes the tools of an MCP connection.
//
// [mcpclient.Client] implements it for external servers and the
// built-in integrations (e.g. Gmail) implement it in-process.
type MCPToolSession interface {
	ListTools(ctx context.Context) ([]mcpclient.Tool, error)
	CallTool(ctx context.Context, name string, args map[string]any) (*mcpclient.CallToolResult, error)
	Close(ctx context.Context) error
}

// NewToolSession creates a new tools session for the connection.
func (m *MCPServer) NewToolSession(app App) MCPToolSession {
	if m.Kind == MCPServerKindGmail {
		return newGmailToolSession(app, m)
	}

	return m.NewClient()
}

// NewClient creates a new MCP client for an external (http) server.
//
// The auth token (if any) is sent as "Authorization: Bearer <token>".
func (m *MCPServer) NewClient() *mcpclient.Client {
	headers := make(map[string]string, len(m.Headers)+1)
	for k, v := range m.Headers {
		headers[k] = v
	}

	if m.AuthToken != "" {
		headers["Authorization"] = "Bearer " + m.AuthToken
	}

	return mcpclient.New(m.URL, headers)
}

// NewMCPServer initializes a new active MCP server connection model.
func NewMCPServer() *MCPServer {
	m := &MCPServer{
		Kind:    MCPServerKindHTTP,
		Active:  true,
		Headers: types.JSONMap[string]{},
	}
	m.Id = security.RandomString(15)

	return m
}

// SaveMCPServer validates and persists the provided MCP server connection (insert or update).
func SaveMCPServer(app App, m *MCPServer) error {
	m.Name = strings.TrimSpace(m.Name)
	m.URL = strings.TrimSpace(m.URL)
	m.AuthToken = strings.TrimSpace(m.AuthToken)
	m.ClientId = strings.TrimSpace(m.ClientId)
	m.ClientSecret = strings.TrimSpace(m.ClientSecret)
	if m.Kind == "" {
		m.Kind = MCPServerKindHTTP
	}

	headers := types.JSONMap[string]{}
	for k, v := range m.Headers {
		if k = strings.TrimSpace(k); k != "" {
			headers[k] = strings.TrimSpace(v)
		}
	}
	m.Headers = headers

	if err := m.Validate(); err != nil {
		return err
	}

	now := types.NowDateTime()
	m.Updated = now

	params := dbx.Params{
		"name":         m.Name,
		"kind":         m.Kind,
		"url":          m.URL,
		"authToken":    m.AuthToken,
		"headers":      m.Headers,
		"clientId":     m.ClientId,
		"clientSecret": m.ClientSecret,
		"oauth":        m.OAuth,
		"active":       m.Active,
		"updated":      m.Updated.String(),
	}

	if m.IsNew() {
		if m.Id == "" {
			m.Id = security.RandomString(15)
		}
		m.Created = now
		params["id"] = m.Id
		params["created"] = m.Created.String()

		if _, err := app.NonconcurrentDB().Insert(MCPServersTableName, params).Execute(); err != nil {
			return err
		}

		m.MarkAsNotNew()

		return nil
	}

	_, err := app.NonconcurrentDB().Update(MCPServersTableName, params, dbx.HashExp{"id": m.Id}).Execute()

	return err
}

// DeleteMCPServer deletes the MCP server connection with the specified id.
func DeleteMCPServer(app App, id string) error {
	_, err := app.NonconcurrentDB().Delete(MCPServersTableName, dbx.HashExp{"id": id}).Execute()
	return err
}

// FindMCPServerById returns a single MCP server connection by its id.
func FindMCPServerById(app App, id string) (*MCPServer, error) {
	m := &MCPServer{}

	err := app.ModelQuery(m).AndWhere(dbx.HashExp{"id": id}).Limit(1).One(m)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// FindActiveMCPServerById returns a single enabled MCP server connection by its id.
func FindActiveMCPServerById(app App, id string) (*MCPServer, error) {
	m, err := FindMCPServerById(app, id)
	if err != nil {
		return nil, errors.New("MCP connection " + id + " doesn't exist")
	}

	if !m.Active {
		return nil, errors.New("MCP connection \"" + m.Name + "\" is disabled")
	}

	return m, nil
}

// FindAllMCPServers returns all MCP server connections ordered by name.
func FindAllMCPServers(app App) ([]*MCPServer, error) {
	servers := []*MCPServer{}

	err := app.ModelQuery(&MCPServer{}).OrderBy("name ASC", "id ASC").All(&servers)
	if err != nil {
		return nil, err
	}

	return servers, nil
}

// SaveMCPServerOAuth persists only the OAuth tokens of the connection.
func SaveMCPServerOAuth(app App, m *MCPServer) error {
	_, err := app.NonconcurrentDB().Update(
		MCPServersTableName,
		dbx.Params{"oauth": m.OAuth},
		dbx.HashExp{"id": m.Id},
	).Execute()

	return err
}

// mcpServerTokenExpired reports whether the access token is missing or about to expire.
func mcpServerTokenExpired(o MCPServerOAuth) bool {
	return o.AccessToken == "" || o.Expiry.IsZero() || time.Until(o.Expiry.Time()) < time.Minute
}
