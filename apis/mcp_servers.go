package apis

import (
	"context"
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/core"
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
}

type mcpServerForm struct {
	Name *string `json:"name"`
	URL  *string `json:"url"`
	// AuthToken is left unchanged when nil and removed when empty.
	AuthToken *string            `json:"authToken"`
	Headers   *map[string]string `json:"headers"`
	Active    *bool              `json:"active"`
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

	client := server.NewClient()
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

	return e.JSON(http.StatusOK, map[string]any{
		"server": client.ServerInfo(),
		"tools":  items,
	})
}
