package apis

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/pocketbase/pocketbase/tools/routine"
	"github.com/pocketbase/pocketbase/tools/types"
)

var automationAllowedFields = []string{
	"name",
	"active",
	"triggerType",
	"collectionRef",
	"cronExpr",
	"steps",
	"notes",
}

// bindAutomationApi registers the automation api endpoints.
func bindAutomationApi(app core.App, rg *router.RouterGroup[*core.RequestEvent]) {
	subGroup := rg.Group("/automations").Bind(RequireSuperuserAuth())
	subGroup.GET("", automationsList)
	subGroup.POST("", automationCreate)
	subGroup.GET("/{id}", automationView)
	subGroup.PATCH("/{id}", automationUpdate)
	subGroup.DELETE("/{id}", automationDelete)
	subGroup.POST("/{id}/run", automationRun)
	subGroup.GET("/{id}/runs", automationRunsList)
}

func automationsList(e *core.RequestEvent) error {
	automations := []*core.Automation{}

	err := e.App.RecordQuery(core.CollectionNameAutomations).
		OrderBy("created ASC").
		All(&automations)
	if err != nil {
		return e.BadRequestError("Failed to load automations.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, automations)
	})
}

func automationView(e *core.RequestEvent) error {
	automation, err := findAutomationForAPI(e.App, e.Request.PathValue("id"))
	if err != nil {
		return automationAPIError(e, "view", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, automation)
	})
}

func automationCreate(e *core.RequestEvent) error {
	body := map[string]any{}
	if err := e.BindBody(&body); err != nil {
		return e.BadRequestError("Failed to load the submitted data due to invalid formatting.", err)
	}

	automation := core.NewAutomation(e.App)
	if err := applyAutomationBody(automation, body); err != nil {
		return e.BadRequestError("Failed to load the submitted data due to invalid formatting.", err)
	}

	if err := e.App.Save(automation); err != nil {
		return automationSaveError(e, "create", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, automation)
	})
}

func automationUpdate(e *core.RequestEvent) error {
	automation, err := findAutomationForAPI(e.App, e.Request.PathValue("id"))
	if err != nil {
		return automationAPIError(e, "update", err)
	}

	body := map[string]any{}
	if err := e.BindBody(&body); err != nil {
		return e.BadRequestError("Failed to load the submitted data due to invalid formatting.", err)
	}

	if err := applyAutomationBody(automation, body); err != nil {
		return e.BadRequestError("Failed to load the submitted data due to invalid formatting.", err)
	}

	if err := e.App.Save(automation); err != nil {
		return automationSaveError(e, "update", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, automation)
	})
}

func automationDelete(e *core.RequestEvent) error {
	automation, err := findAutomationForAPI(e.App, e.Request.PathValue("id"))
	if err != nil {
		return automationAPIError(e, "delete", err)
	}

	if err := e.App.Delete(automation); err != nil {
		return e.BadRequestError("Failed to delete automation.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.NoContent(http.StatusNoContent)
	})
}

func automationRun(e *core.RequestEvent) error {
	automation, err := findAutomationForAPI(e.App, e.Request.PathValue("id"))
	if err != nil {
		return automationAPIError(e, "run", err)
	}

	routine.FireAndForget(func() {
		if err := e.App.RunAutomationManually(automation.Id); err != nil {
			e.App.Logger().Warn(
				"Failed to execute automation manual run",
				"automationId", automation.Id,
				"error", err,
			)
		}
	})

	return e.NoContent(http.StatusNoContent)
}

func automationRunsList(e *core.RequestEvent) error {
	automation, err := findAutomationForAPI(e.App, e.Request.PathValue("id"))
	if err != nil {
		return automationAPIError(e, "list runs", err)
	}

	limit, offset, err := automationRunsQueryParams(e)
	if err != nil {
		return e.BadRequestError("Invalid automation runs query params.", err)
	}

	runs := []*core.AutomationRun{}
	query := e.App.RecordQuery(core.CollectionNameAutomationRuns).
		AndWhere(dbx.HashExp{"automationRef": automation.Id}).
		OrderBy("started DESC")

	if limit > 0 {
		query.Limit(int64(limit))
	}
	if offset > 0 {
		query.Offset(int64(offset))
	}

	err = query.All(&runs)
	if err != nil {
		return e.BadRequestError("Failed to load automation runs.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, runs)
	})
}

func automationRunsQueryParams(e *core.RequestEvent) (int, int, error) {
	query := e.Request.URL.Query()

	limit := 0
	offset := 0

	if raw := query.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return 0, 0, errors.New("limit must be a non-negative integer")
		}
		limit = parsed
	}

	if raw := query.Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return 0, 0, errors.New("offset must be a non-negative integer")
		}
		offset = parsed
	}

	return limit, offset, nil
}

func findAutomationForAPI(app core.App, id string) (*core.Automation, error) {
	result := &core.Automation{}

	err := app.RecordQuery(core.CollectionNameAutomations).
		AndWhere(dbx.HashExp{"id": id}).
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func applyAutomationBody(automation *core.Automation, body map[string]any) error {
	for _, field := range automationAllowedFields {
		value, ok := body[field]
		if !ok {
			continue
		}

		if field == "steps" {
			raw, err := types.ParseJSONRaw(value)
			if err != nil {
				return err
			}
			automation.SetSteps(raw)
			continue
		}

		automation.Set(field, value)
	}

	return nil
}

func automationAPIError(e *core.RequestEvent, action string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return e.NotFoundError("Missing or invalid automation.", err)
	}

	return e.BadRequestError("Failed to "+action+" automation.", err)
}

func automationSaveError(e *core.RequestEvent, action string, err error) error {
	var validationErrors validation.Errors
	if errors.As(err, &validationErrors) {
		return e.BadRequestError("Failed to "+action+" automation.", validationErrors)
	}

	return e.BadRequestError("Failed to "+action+" automation. Raw error: \n"+err.Error(), nil)
}
