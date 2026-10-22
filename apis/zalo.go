package apis

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/router"
)

// bindZaloApi registers the routes that control the external zalo-bridge.
//
// The status/login/logout/kick routes are compatible with the former
// demo pb_hooks (zalo.pb.js), so existing app clients keep working.
func bindZaloApi(app core.App, rg *router.RouterGroup[*core.RequestEvent]) {
	sub := rg.Group("/zalo")
	sub.GET("/overview", zaloOverview).Bind(RequireSuperuserAuth())
	sub.POST("/settings", zaloSaveSettings).Bind(RequireSuperuserAuth())

	// The routes compatible with the former demo pb_hooks are registered as
	// late as possible (after the JS hooks routes) and skipped when the old
	// zalo.pb.js hooks are still installed, instead of crashing the app with
	// duplicated route patterns.
	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Id:       "pbZaloCompatRoutes",
		Priority: 9999,
		Func: func(se *core.ServeEvent) error {
			routes := []struct {
				method string
				path   string
				action func(*core.RequestEvent) error
				auth   *hook.Handler[*core.RequestEvent]
			}{
				{http.MethodGet, "/api/zalo/status", zaloStatus, RequireAuth()},
				{http.MethodPost, "/api/zalo/{action}", zaloAction, RequireSuperuserAuth()},
			}

			skipped := []string{}
			for _, r := range routes {
				if se.Router.HasRoute(r.method, r.path) {
					skipped = append(skipped, r.method+" "+r.path)
					continue
				}
				se.Router.Route(r.method, r.path, r.action).Bind(r.auth)
			}

			if len(skipped) > 0 {
				se.App.Logger().Warn(
					"The Zalo routes are already registered (most likely by the old pb_hooks/zalo.pb.js). "+
						"Delete pb_hooks/zalo.pb.js and pb_hooks/zalo/bridge.js to use the built-in Zalo integration.",
					"routes", skipped,
				)
			}

			return se.Next()
		},
	})
}

func zaloDisabledResponse(e *core.RequestEvent) error {
	return e.JSON(http.StatusServiceUnavailable, map[string]any{
		"connected": false,
		"status":    "disabled",
		"message":   "The Zalo integration is not enabled.",
	})
}

// zaloStatus returns the bridge status (any authenticated user).
func zaloStatus(e *core.RequestEvent) error {
	res, err := core.CallZaloBridge(e.Request.Context(), e.App, http.MethodGet, "/status")
	if errors.Is(err, core.ErrZaloDisabled) {
		return zaloDisabledResponse(e)
	}
	if err != nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]any{
			"connected": false,
			"status":    "bridge_offline",
			"message":   "Failed to connect to zalo-bridge.",
		})
	}

	return e.JSON(res.StatusCode, res.Body)
}

// zaloAction forwards the login/logout/kick actions to the bridge (superusers only).
func zaloAction(e *core.RequestEvent) error {
	action := e.Request.PathValue("action")
	if action != "login" && action != "logout" && action != "kick" {
		return e.NotFoundError("Unknown action.", nil)
	}

	res, err := core.CallZaloBridge(e.Request.Context(), e.App, http.MethodPost, "/"+action)
	if errors.Is(err, core.ErrZaloDisabled) {
		return zaloDisabledResponse(e)
	}
	if err != nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]any{"error": "Failed to connect to zalo-bridge."})
	}

	return e.JSON(res.StatusCode, res.Body)
}

// zaloOverview returns everything the admin UI needs: the settings, the
// bridge availability and the Zalo session state (incl. the QR code).
func zaloOverview(e *core.RequestEvent) error {
	config := e.App.Settings().Credentials.Zalo

	bridge := map[string]any{"online": false}
	if config.Enabled {
		started := time.Now()
		res, err := core.CallZaloBridge(e.Request.Context(), e.App, http.MethodGet, "/status")
		bridge["latencyMs"] = time.Since(started).Milliseconds()

		switch {
		case err != nil:
			bridge["error"] = zaloBridgeErrorMessage(err)
		case res.StatusCode == http.StatusUnauthorized:
			bridge["error"] = "The bridge rejected the secret - make sure BRIDGE_SECRET matches the secret below."
		case res.StatusCode >= 400:
			bridge["error"] = "The bridge returned an error (" + http.StatusText(res.StatusCode) + ")."
		default:
			bridge["online"] = true
			bridge["state"] = res.Body
		}
	}

	var session any // null when the bridge never reported a session
	if record, err := e.App.FindRecordById(core.ZaloCollectionSession, core.ZaloSessionRecordId); err == nil {
		session = map[string]any{
			"status":       record.GetString("status"),
			"message":      record.GetString("message"),
			"qr_image":     record.GetString("qr_image"),
			"account_id":   record.GetString("account_id"),
			"account_name": record.GetString("account_name"),
			"updated":      record.GetDateTime("updated"),
		}
	}

	return e.JSON(http.StatusOK, map[string]any{
		"settings": map[string]any{
			"enabled":   config.Enabled,
			"bridgeURL": config.BridgeBaseURL(),
			"hasSecret": config.Secret != "",
		},
		"collections": core.ZaloCollectionsStatus(e.App),
		"bridge":      bridge,
		"session":     session,
	})
}

func zaloBridgeErrorMessage(err error) string {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return "The bridge didn't respond in time."
		}
		message := urlErr.Err.Error()
		if strings.Contains(message, "connection refused") {
			return "The bridge is not running (connection refused)."
		}
		return "Failed to connect to the bridge: " + message
	}

	return "Failed to connect to the bridge: " + err.Error()
}

type zaloSettingsForm struct {
	Enabled   *bool   `json:"enabled"`
	BridgeURL *string `json:"bridgeURL"`
	// Secret is left unchanged when nil or empty.
	Secret *string `json:"secret"`
}

// zaloSaveSettings saves the Zalo settings and creates the collections when enabled.
func zaloSaveSettings(e *core.RequestEvent) error {
	form := &zaloSettingsForm{}
	if err := e.BindBody(form); err != nil {
		return e.BadRequestError("Failed to load the submitted data due to invalid formatting.", err)
	}

	settings, err := e.App.Settings().Clone()
	if err != nil {
		return e.InternalServerError("Failed to clone app settings.", err)
	}

	config := &settings.Credentials.Zalo
	if form.Enabled != nil {
		config.Enabled = *form.Enabled
	}
	if form.BridgeURL != nil {
		config.BridgeURL = strings.TrimSpace(*form.BridgeURL)
	}
	if form.Secret != nil && strings.TrimSpace(*form.Secret) != "" {
		config.Secret = strings.TrimSpace(*form.Secret)
	}

	if err := config.Validate(); err != nil {
		return e.BadRequestError("Failed to save the Zalo settings.", err)
	}

	if config.Enabled {
		if err := core.EnsureZaloCollections(e.App); err != nil {
			return e.BadRequestError("Failed to create the Zalo collections.", validation.NewError("zalo_collections", err.Error()))
		}
	}

	if err := e.App.Save(settings); err != nil {
		return e.BadRequestError("Failed to save the Zalo settings.", err)
	}

	return zaloOverview(e)
}
