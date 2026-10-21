package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/pocketbase/dbx"
	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/tools/mcpclient"
	"github.com/pocketbase/pocketbase/tools/security"
	"github.com/pocketbase/pocketbase/tools/types"
)

const MCPServersTableName = "_mcpServers"

var _ Model = (*MCPServer)(nil)

// MCPServer defines a connection to an external MCP server
// whose tools can be used by the AI automation steps.
type MCPServer struct {
	BaseModel

	Name      string                `db:"name" json:"name"`
	URL       string                `db:"url" json:"url"`
	AuthToken string                `db:"authToken" json:"-"`
	Headers   types.JSONMap[string] `db:"headers" json:"headers"`
	Active    bool                  `db:"active" json:"active"`
	Created   types.DateTime        `db:"created" json:"created"`
	Updated   types.DateTime        `db:"updated" json:"updated"`
}

func (m *MCPServer) TableName() string {
	return MCPServersTableName
}

// MarshalJSON implements [json.Marshaler] and exports whether an
// auth token is set without exposing the token itself.
func (m MCPServer) MarshalJSON() ([]byte, error) {
	type alias MCPServer

	return json.Marshal(struct {
		alias
		HasAuthToken bool `json:"hasAuthToken"`
	}{
		alias:        alias(m),
		HasAuthToken: m.AuthToken != "",
	})
}

// Validate validates the user configurable server fields.
func (m *MCPServer) Validate() error {
	return validation.ValidateStruct(m,
		validation.Field(&m.Name, validation.Required, validation.Length(1, 100)),
		validation.Field(&m.URL, validation.Required, validation.Length(1, 2000), validation.By(validateMCPServerURL)),
		validation.Field(&m.Headers, validation.By(validateMCPServerHeaders)),
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

// NewClient creates a new MCP client for the server.
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
		"name":      m.Name,
		"url":       m.URL,
		"authToken": m.AuthToken,
		"headers":   m.Headers,
		"active":    m.Active,
		"updated":   m.Updated.String(),
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
