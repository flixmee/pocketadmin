package apis

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/mcpclient"
	"github.com/pocketbase/pocketbase/tools/router"
)

const mcpServerToolsTimeout = 30 * time.Second

func bindMCPServersApi(app core.App, rg *router.RouterGroup[*core.RequestEvent]) {
	servers := rg.Group("/mcp/servers").Bind(RequireSuperuserAuth())
	servers.GET("", mcpServersList)
	servers.POST("", mcpServerCreate)
	servers.PATCH("/{id}", mcpServerUpdate)
	servers.DELETE("/{id}", mcpServerDelete)
	servers.GET("/{id}/tools", mcpServerTools)
	servers.POST("/{id}/oauth/start", mcpServerOAuthStart)
	servers.POST("/{id}/oauth/disconnect", mcpServerOAuthDisconnect)

	// public - validated with the one-time OAuth state
	rg.GET("/mcp/oauth/google/callback", mcpServerGoogleOAuthCallback)
}

type mcpServerForm struct {
	Name *string `json:"name"`
	Kind *string `json:"kind"`
	URL  *string `json:"url"`
	// AuthToken is left unchanged when nil and removed when empty.
	AuthToken *string            `json:"authToken"`
	Headers   *map[string]string `json:"headers"`
	Active    *bool              `json:"active"`
	ClientId  *string            `json:"clientId"`
	// ClientSecret is left unchanged when nil and removed when empty.
	ClientSecret *string `json:"clientSecret"`
}

func (form *mcpServerForm) apply(server *core.MCPServer) {
	if form.Name != nil {
		server.Name = *form.Name
	}
	if form.URL != nil {
		server.URL = *form.URL
	}
	if form.AuthToken != nil {
		server.AuthToken = *form.AuthToken
	}
	if form.Headers != nil {
		server.Headers = *form.Headers
	}
	if form.Active != nil {
		server.Active = *form.Active
	}
	if form.Kind != nil {
		server.Kind = *form.Kind
	}
	if form.ClientId != nil {
		server.ClientId = *form.ClientId
	}
	if form.ClientSecret != nil {
		server.ClientSecret = *form.ClientSecret
	}
}

func mcpServersList(e *core.RequestEvent) error {
	servers, err := core.FindAllMCPServers(e.App)
	if err != nil {
		return e.InternalServerError("Failed to load MCP connections.", err)
	}

	return e.JSON(http.StatusOK, map[string]any{"items": servers})
}

func mcpServerCreate(e *core.RequestEvent) error {
	form := &mcpServerForm{}
	if err := e.BindBody(form); err != nil {
		return e.BadRequestError("Failed to load the submitted data due to invalid formatting.", err)
	}

	server := core.NewMCPServer()
	form.apply(server)

	if err := core.SaveMCPServer(e.App, server); err != nil {
		return e.BadRequestError("Failed to create MCP connection.", err)
	}

	return e.JSON(http.StatusOK, server)
}

func mcpServerUpdate(e *core.RequestEvent) error {
	server, err := core.FindMCPServerById(e.App, e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("", err)
	}

	form := &mcpServerForm{}
	if err := e.BindBody(form); err != nil {
		return e.BadRequestError("Failed to load the submitted data due to invalid formatting.", err)
	}

	form.apply(server)

	if err := core.SaveMCPServer(e.App, server); err != nil {
		return e.BadRequestError("Failed to update MCP connection.", err)
	}

	return e.JSON(http.StatusOK, server)
}

func mcpServerDelete(e *core.RequestEvent) error {
	server, err := core.FindMCPServerById(e.App, e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("", err)
	}

	if err := core.DeleteMCPServer(e.App, server.Id); err != nil {
		return e.BadRequestError("Failed to delete MCP connection.", err)
	}

	return e.NoContent(http.StatusNoContent)
}

// mcpServerTools connects to the server and returns its tools.
// It is also used to test the connection settings.
func mcpServerTools(e *core.RequestEvent) error {
	server, err := core.FindMCPServerById(e.App, e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("", err)
	}

	ctx, cancel := context.WithTimeout(e.Request.Context(), mcpServerToolsTimeout)
	defer cancel()

	client := server.NewToolSession(e.App)
	defer client.Close(context.Background())

	tools, err := client.ListTools(ctx)
	if err != nil {
		return e.BadRequestError("Failed to connect to the MCP server: "+err.Error(), nil)
	}

	type toolItem struct {
		Name        string `json:"name"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}

	items := make([]toolItem, 0, len(tools))
	for _, tool := range tools {
		items = append(items, toolItem{Name: tool.Name, Title: tool.Title, Description: tool.Description})
	}

	var serverInfo map[string]any
	if httpClient, ok := client.(*mcpclient.Client); ok {
		serverInfo = httpClient.ServerInfo()
	} else {
		serverInfo = map[string]any{"name": server.Kind, "account": server.OAuth.Email}
	}

	return e.JSON(http.StatusOK, map[string]any{
		"server": serverInfo,
		"tools":  items,
	})
}

// mcpServerOAuthStart returns the Google consent URL to connect the
// account of a Gmail connection.
func mcpServerOAuthStart(e *core.RequestEvent) error {
	server, err := core.FindMCPServerById(e.App, e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("", err)
	}

	if server.Kind != core.MCPServerKindGmail {
		return e.BadRequestError("Only Gmail connections can be connected with OAuth.", nil)
	}

	// the admin UI sends the callback URL as seen by the browser so that it
	// works behind reverse proxies without relying on the application URL setting
	form := struct {
		RedirectURL string `json:"redirectURL"`
	}{}
	if e.Request.ContentLength != 0 {
		if err := e.BindBody(&form); err != nil {
			return e.BadRequestError("Failed to load the submitted data due to invalid formatting.", err)
		}
	}

	redirectURL := core.GoogleOAuthRedirectURL(e.App)
	if form.RedirectURL != "" {
		redirectURL, err = core.NormalizeGoogleOAuthRedirectURL(form.RedirectURL)
		if err != nil {
			return e.BadRequestError(err.Error(), nil)
		}
	}

	authURL, err := server.GoogleOAuthAuthURL(e.App, core.NewMCPServerOAuthState(e.App, server.Id, redirectURL), redirectURL)
	if err != nil {
		return e.BadRequestError(err.Error(), nil)
	}

	return e.JSON(http.StatusOK, map[string]any{
		"authURL":     authURL,
		"redirectURL": redirectURL,
	})
}

// mcpServerOAuthDisconnect removes the stored OAuth tokens of the connection.
func mcpServerOAuthDisconnect(e *core.RequestEvent) error {
	server, err := core.FindMCPServerById(e.App, e.Request.PathValue("id"))
	if err != nil {
		return e.NotFoundError("", err)
	}

	server.OAuth = core.MCPServerOAuth{}
	if err := core.SaveMCPServerOAuth(e.App, server); err != nil {
		return e.BadRequestError("Failed to disconnect the account.", err)
	}

	return e.JSON(http.StatusOK, server)
}

// mcpServerGoogleOAuthCallback completes the Google OAuth flow and
// renders a small page that notifies the admin UI window.
func mcpServerGoogleOAuthCallback(e *core.RequestEvent) error {
	query := e.Request.URL.Query()

	serverId, redirectURL, err := core.ConsumeMCPServerOAuthState(e.App, query.Get("state"))
	if err != nil {
		return mcpOAuthResultPage(e, http.StatusBadRequest, false, err.Error())
	}

	if errCode := query.Get("error"); errCode != "" {
		return mcpOAuthResultPage(e, http.StatusBadRequest, false, "Google sign-in was not completed: "+errCode)
	}

	server, err := core.FindMCPServerById(e.App, serverId)
	if err != nil {
		return mcpOAuthResultPage(e, http.StatusNotFound, false, "The connection doesn't exist anymore.")
	}

	ctx, cancel := context.WithTimeout(e.Request.Context(), mcpServerToolsTimeout)
	defer cancel()

	if err := server.ConnectGoogleAccount(ctx, e.App, query.Get("code"), redirectURL); err != nil {
		return mcpOAuthResultPage(e, http.StatusBadRequest, false, err.Error())
	}

	return mcpOAuthResultPage(e, http.StatusOK, true, "Connected "+server.OAuth.Email+". You can close this window.")
}

func mcpOAuthResultPage(e *core.RequestEvent, status int, success bool, message string) error {
	payload, _ := json.Marshal(map[string]any{
		"type":    "pocketadmin:mcp-oauth",
		"success": success,
		"message": message,
	})

	title := "Connection failed"
	if success {
		title = "Account connected"
	}

	page := `<!doctype html><html><head><meta charset="utf-8"><title>` + html.EscapeString(title) + `</title>` +
		`<meta name="viewport" content="width=device-width, initial-scale=1">` +
		`<style>body{font-family:system-ui,sans-serif;display:flex;align-items:center;justify-content:center;min-height:90vh;margin:0 16px;color:#1a1a1a;background:#fafafa}` +
		`@media (prefers-color-scheme:dark){body{color:#eee;background:#1d1d1d}}div{max-width:480px;text-align:center}</style></head>` +
		`<body><div><h2>` + html.EscapeString(title) + `</h2><p>` + html.EscapeString(message) + `</p></div>` +
		`<script>try{window.opener&&window.opener.postMessage(` + string(payload) + `,"*")}catch(e){}` +
		func() string {
			if success {
				return `setTimeout(function(){window.close()},1500);`
			}
			return ""
		}() +
		`</script></body></html>`

	return e.HTML(status, page)
}
