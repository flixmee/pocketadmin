package apis

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"

	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/pocketbase/pocketbase/tools/types"
)

// mcpProtocolVersions lists the supported MCP protocol versions (newest first).
var mcpProtocolVersions = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

const mcpServerInstructions = "This server exposes the PocketAdmin (PocketBase) collections and records. " +
	"Call list_collections first to discover the available collections and their fields, " +
	"then use the record tools to query and modify data. " +
	"Media fields reference files from the shared media library: upload files with upload_media (or find existing ones with list_media) " +
	"and set the returned file url as the field value. " +
	"Filters and sorting use the PocketBase syntax, e.g. filter `status = 'active' && created > '2024-01-01'` and sort `-created,title`."

const (
	mcpErrParse          = -32700
	mcpErrInvalidRequest = -32600
	mcpErrMethodNotFound = -32601
	mcpErrInvalidParams  = -32602
)

func bindMCPApi(app core.App, rg *router.RouterGroup[*core.RequestEvent]) {
	rg.POST("/mcp", mcpHandler)
	rg.GET("/mcp", mcpMethodNotAllowed)
	rg.DELETE("/mcp", mcpMethodNotAllowed)

	keys := rg.Group("/mcp/keys").Bind(RequireSuperuserAuth())
	keys.GET("", mcpKeysList)
	keys.POST("", mcpKeyCreate)
	keys.PATCH("/{id}", mcpKeyUpdate)
	keys.DELETE("/{id}", mcpKeyDelete)
	keys.POST("/{id}/regenerate", mcpKeyRegenerate)
}

// -------------------------------------------------------------------
// Keys management
// -------------------------------------------------------------------

type mcpKeyForm struct {
	Name        *string   `json:"name"`
	Permissions *[]string `json:"permissions"`
	Collections *[]string `json:"collections"`
	Active      *bool     `json:"active"`
	ExpiresAt   *string   `json:"expiresAt"`
}

func (form *mcpKeyForm) apply(app core.App, key *core.MCPKey) error {
	if form.Name != nil {
		key.Name = *form.Name
	}
	if form.Permissions != nil {
		key.Permissions = *form.Permissions
	}
	if form.Collections != nil {
		key.Collections = *form.Collections
	}
	if form.Active != nil {
		key.Active = *form.Active
	}

	errs := validation.Errors{}

	if form.ExpiresAt != nil {
		if strings.TrimSpace(*form.ExpiresAt) == "" {
			key.ExpiresAt = types.DateTime{}
		} else if expiresAt, err := types.ParseDateTime(*form.ExpiresAt); err != nil || expiresAt.IsZero() {
			errs["expiresAt"] = validation.NewError("validation_invalid_date", "Must be a valid date.")
		} else {
			key.ExpiresAt = expiresAt
		}
	}

	for _, id := range key.Collections {
		collection, err := app.FindCachedCollectionByNameOrId(id)
		if err != nil || collection.System || collection.Id != id {
			errs["collections"] = validation.NewError("validation_invalid_collection", "Unknown or system collection "+id+".")
			break
		}
	}

	if len(errs) > 0 {
		return errs
	}

	return key.Validate()
}

func mcpKeysList(e *core.RequestEvent) error {
	keys, err := core.FindAllMCPKeys(e.App)
	if err != nil {
		return e.InternalServerError("Failed to load MCP keys.", err)
	}

	return e.JSON(http.StatusOK, map[string]any{
		"items":       keys,
		"permissions": core.MCPPermissions,
	})
}

func mcpKeyCreate(e *core.RequestEvent) error {
	form := &mcpKeyForm{}
	if err := e.BindBody(form); err != nil {
		return e.BadRequestError("Failed to load the submitted data due to invalid formatting.", err)
	}

	key, token := core.NewMCPKey()

	if err := form.apply(e.App, key); err != nil {
		return e.BadRequestError("Failed to create MCP key.", err)
	}

	if err := core.SaveMCPKey(e.App, key); err != nil {
		return e.BadRequestError("Failed to create MCP key.", err)
	}

	return e.JSON(http.StatusOK, map[string]any{
		"key":   key,
		"token": token,
	})
}

func mcpKeyUpdate(e *core.RequestEvent) error {
	key, err := core.FindMCPKeyById(e.App, e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("", err)
	}

	form := &mcpKeyForm{}
	if err := e.BindBody(form); err != nil {
		return e.BadRequestError("Failed to load the submitted data due to invalid formatting.", err)
	}

	if err := form.apply(e.App, key); err != nil {
		return e.BadRequestError("Failed to update MCP key.", err)
	}

	if err := core.SaveMCPKey(e.App, key); err != nil {
		return e.BadRequestError("Failed to update MCP key.", err)
	}

	return e.JSON(http.StatusOK, key)
}

func mcpKeyRegenerate(e *core.RequestEvent) error {
	key, err := core.FindMCPKeyById(e.App, e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("", err)
	}

	token := key.RefreshToken()

	if err := core.SaveMCPKey(e.App, key); err != nil {
		return e.BadRequestError("Failed to regenerate MCP key.", err)
	}

	return e.JSON(http.StatusOK, map[string]any{
		"key":   key,
		"token": token,
	})
}

func mcpKeyDelete(e *core.RequestEvent) error {
	key, err := core.FindMCPKeyById(e.App, e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("", err)
	}

	if err := core.DeleteMCPKey(e.App, key.Id); err != nil {
		return e.BadRequestError("Failed to delete MCP key.", err)
	}

	return e.NoContent(http.StatusNoContent)
}

// -------------------------------------------------------------------
// MCP Streamable HTTP transport (stateless, JSON responses only)
// -------------------------------------------------------------------

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpResponse struct {
	JSONRPC string
	ID      json.RawMessage
	Result  any
	Error   *mcpError
}

// MarshalJSON ensures that exactly one of "result" or "error" is always
// serialized (even when the result is an empty object).
func (r mcpResponse) MarshalJSON() ([]byte, error) {
	if r.Error != nil {
		return json.Marshal(struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Error   *mcpError       `json:"error"`
		}{r.JSONRPC, r.ID, r.Error})
	}

	return json.Marshal(struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Result  any             `json:"result"`
	}{r.JSONRPC, r.ID, r.Result})
}

func mcpMethodNotAllowed(e *core.RequestEvent) error {
	e.Response.Header().Set("Allow", http.MethodPost)
	return e.Error(http.StatusMethodNotAllowed, "The MCP endpoint supports only POST requests.", nil)
}

func mcpTokenFromRequest(e *core.RequestEvent) string {
	if token := strings.TrimSpace(e.Request.Header.Get("X-API-Key")); token != "" {
		return token
	}

	return strings.TrimSpace(getAuthTokenFromRequest(e))
}

func mcpHandler(e *core.RequestEvent) error {
	key, err := core.FindActiveMCPKeyByToken(e.App, mcpTokenFromRequest(e))
	if err != nil {
		e.Response.Header().Set("WWW-Authenticate", `Bearer realm="mcp"`)
		return e.UnauthorizedError("Missing or invalid MCP API key.", err)
	}

	raw, err := io.ReadAll(e.Request.Body)
	if err != nil {
		return e.BadRequestError("Failed to read the request body.", err)
	}

	raw = bytes.TrimSpace(raw)

	server := &mcpServer{app: e.App, key: key}

	// batch request (supported by older protocol revisions)
	if len(raw) > 0 && raw[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(raw, &batch); err != nil || len(batch) == 0 {
			return e.JSON(http.StatusBadRequest, mcpErrorResponse(nil, mcpErrParse, "Invalid JSON-RPC batch."))
		}

		responses := make([]*mcpResponse, 0, len(batch))
		for _, item := range batch {
			if res := server.handleRaw(item); res != nil {
				responses = append(responses, res)
			}
		}

		if len(responses) == 0 {
			return e.NoContent(http.StatusAccepted)
		}

		return e.JSON(http.StatusOK, responses)
	}

	res := server.handleRaw(raw)
	if res == nil {
		return e.NoContent(http.StatusAccepted)
	}

	if res.Error != nil && (res.Error.Code == mcpErrParse || res.Error.Code == mcpErrInvalidRequest) {
		return e.JSON(http.StatusBadRequest, res)
	}

	return e.JSON(http.StatusOK, res)
}

func mcpErrorResponse(id json.RawMessage, code int, message string) *mcpResponse {
	if len(id) == 0 {
		id = json.RawMessage("null")
	}

	return &mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &mcpError{Code: code, Message: message},
	}
}

type mcpServer struct {
	app core.App
	key *core.MCPKey
}

// handleRaw processes a single JSON-RPC message and returns its response
// (or nil for notifications and client responses).
func (s *mcpServer) handleRaw(raw []byte) *mcpResponse {
	req := &mcpRequest{}
	if err := json.Unmarshal(raw, req); err != nil {
		return mcpErrorResponse(nil, mcpErrParse, "Invalid JSON-RPC message.")
	}

	if req.JSONRPC != "2.0" {
		return mcpErrorResponse(req.ID, mcpErrInvalidRequest, "Unsupported JSON-RPC version.")
	}

	isNotification := len(req.ID) == 0

	// response from the client (eg. to a server request) - nothing to do
	if req.Method == "" {
		if isNotification {
			return mcpErrorResponse(nil, mcpErrInvalidRequest, "Missing JSON-RPC method.")
		}
		return nil
	}

	result, rpcErr := s.dispatch(req)

	if isNotification {
		return nil
	}

	if rpcErr != nil {
		return mcpErrorResponse(req.ID, rpcErr.Code, rpcErr.Message)
	}

	if result == nil {
		result = map[string]any{}
	}

	return &mcpResponse{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (s *mcpServer) dispatch(req *mcpRequest) (any, *mcpError) {
	switch req.Method {
	case "initialize":
		params := struct {
			ProtocolVersion string `json:"protocolVersion"`
		}{}
		_ = json.Unmarshal(req.Params, &params)

		version := mcpProtocolVersions[0]
		if slices.Contains(mcpProtocolVersions, params.ProtocolVersion) {
			version = params.ProtocolVersion
		}

		return map[string]any{
			"protocolVersion": version,
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"serverInfo": map[string]any{
				"name":    "pocketadmin",
				"title":   "PocketAdmin",
				"version": "1.0.0",
			},
			"instructions": mcpServerInstructions,
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		tools := []map[string]any{}
		for _, tool := range mcpTools {
			if s.key.HasPermission(tool.Permission) {
				tools = append(tools, tool.definition())
			}
		}
		return map[string]any{"tools": tools}, nil
	case "tools/call":
		params := struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}{}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, &mcpError{Code: mcpErrInvalidParams, Message: "Invalid tools/call params."}
		}

		tool := findMCPTool(params.Name)
		if tool == nil || !s.key.HasPermission(tool.Permission) {
			return nil, &mcpError{Code: mcpErrInvalidParams, Message: "Unknown tool: " + params.Name}
		}

		if params.Arguments == nil {
			params.Arguments = map[string]any{}
		}

		return s.callTool(tool, params.Arguments), nil
	default:
		if strings.HasPrefix(req.Method, "notifications/") {
			return nil, nil
		}

		return nil, &mcpError{Code: mcpErrMethodNotFound, Message: "Method not found: " + req.Method}
	}
}

func (s *mcpServer) callTool(tool *mcpTool, args map[string]any) map[string]any {
	result, err := tool.Handler(s, mcpArgs(args))
	if err != nil {
		return mcpToolErrorResult(err)
	}

	text, err := json.Marshal(result)
	if err != nil {
		return mcpToolErrorResult(err)
	}

	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(text)}},
		"isError": false,
	}
}

func mcpToolErrorResult(err error) map[string]any {
	message := err.Error()

	var validationErrs validation.Errors
	if errors.As(err, &validationErrs) {
		if raw, marshalErr := json.Marshal(validationErrs); marshalErr == nil {
			message = "Validation failed: " + string(raw)
		}
	}

	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": message}},
		"isError": true,
	}
}
