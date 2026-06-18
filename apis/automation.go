package apis

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/inflector"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/pocketbase/pocketbase/tools/routine"
	"github.com/pocketbase/pocketbase/tools/types"
)

var automationAllowedFields = []string{
	"name",
	"tag",
	"active",
	"notifyOnCompletion",
	"triggerType",
	"collectionRef",
	"cronExpr",
	"webhookMethod",
	"steps",
	"notes",
}

// bindAutomationApi registers the automation api endpoints.
func bindAutomationApi(app core.App, rg *router.RouterGroup[*core.RequestEvent]) {
	bindAutomationWebhookRoutes(rg)
	rg.POST("/automation-resume/{token}", automationResumeByToken).Bind(SkipSuccessActivityLog())

	subGroup := rg.Group("/automations").Bind(RequireSuperuserAuth())
	subGroup.GET("", automationsList)
	subGroup.GET("/schemas", automationSchemas)
	subGroup.GET("/templates", automationTemplatesList)
	subGroup.POST("/templates/import", automationTemplateImport)
	subGroup.POST("/templates/{templateId}/install", automationTemplateInstall)
	subGroup.GET("/workflow-states", automationWorkflowStatesList)
	subGroup.POST("/workflow-states/{id}/resume", automationWorkflowStateResume)
	subGroup.GET("/approvals", automationApprovalsList)
	subGroup.POST("/approvals/{id}/decision", automationApprovalDecision)
	subGroup.POST("", automationCreate)
	subGroup.GET("/{id}", automationView)
	subGroup.PATCH("/{id}", automationUpdate)
	subGroup.DELETE("/{id}", automationDelete)
	subGroup.POST("/{id}/publish", automationPublish)
	subGroup.POST("/{id}/export-template", automationTemplateExport)
	subGroup.POST("/{id}/run", automationRun)
	subGroup.POST("/{id}/dry-run", automationDryRun)
	subGroup.POST("/{id}/runs/{runId}/dry-run", automationRunDryRun)
	subGroup.POST("/{id}/runs/{runId}/rerun", automationRunRerun)
	subGroup.GET("/{id}/runs", automationRunsList)
	subGroup.DELETE("/{id}/runs", automationRunsClear)
}

func bindAutomationWebhookRoutes(rg *router.RouterGroup[*core.RequestEvent]) {
	rg.GET("/automation-webhooks/{id}", automationWebhook).Bind(SkipSuccessActivityLog())
	rg.POST("/automation-webhooks/{id}", automationWebhook).Bind(SkipSuccessActivityLog())
	rg.PUT("/automation-webhooks/{id}", automationWebhook).Bind(SkipSuccessActivityLog())
	rg.PATCH("/automation-webhooks/{id}", automationWebhook).Bind(SkipSuccessActivityLog())
	rg.DELETE("/automation-webhooks/{id}", automationWebhook).Bind(SkipSuccessActivityLog())
}

func automationPublish(e *core.RequestEvent) error {
	automation, err := findAutomationForAPI(e.App, e.Request.PathValue("id"))
	if err != nil {
		return automationAPIError(e, "publish", err)
	}

	body := core.AutomationPublishOptions{}
	if e.Request.Body != nil {
		if err := e.BindBody(&body); err != nil {
			return e.BadRequestError("Failed to load publish options.", err)
		}
	}

	version, err := e.App.PublishAutomationVersion(automation.Id, body)
	if err != nil {
		return e.BadRequestError("Failed to publish automation.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, version)
	})
}

func automationTemplateExport(e *core.RequestEvent) error {
	automation, err := findAutomationForAPI(e.App, e.Request.PathValue("id"))
	if err != nil {
		return automationAPIError(e, "export template", err)
	}

	body := core.WorkflowTemplateExportOptions{}
	if e.Request.Body != nil {
		if err := e.BindBody(&body); err != nil {
			return e.BadRequestError("Failed to load workflow template export options.", err)
		}
	}

	template, err := e.App.ExportAutomationTemplate(automation.Id, body)
	if err != nil {
		return e.BadRequestError("Failed to export workflow template.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, template)
	})
}

func automationWorkflowStatesList(e *core.RequestEvent) error {
	states, err := e.App.FindAllWorkflowStates()
	if err != nil {
		return e.BadRequestError("Failed to load workflow states.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, states)
	})
}

func automationWorkflowStateResume(e *core.RequestEvent) error {
	body := core.AutomationResumeInput{}
	if e.Request.Body != nil {
		if err := e.BindBody(&body); err != nil {
			return e.BadRequestError("Failed to load workflow resume input.", err)
		}
	}

	if err := e.App.ResumeAutomationWorkflowState(e.Request.PathValue("id"), body); err != nil {
		return e.BadRequestError("Failed to resume workflow state.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.NoContent(http.StatusNoContent)
	})
}

func automationResumeByToken(e *core.RequestEvent) error {
	body := map[string]any{}
	if e.Request.Body != nil {
		if err := e.BindBody(&body); err != nil {
			return e.BadRequestError("Failed to load workflow resume input.", err)
		}
	}

	if err := e.App.ResumeAutomationWorkflowByToken(e.Request.PathValue("token"), body); err != nil {
		return e.BadRequestError("Failed to resume workflow.", err)
	}

	return e.NoContent(http.StatusNoContent)
}

func automationApprovalsList(e *core.RequestEvent) error {
	approvals := []*core.Approval{}
	query := e.App.RecordQuery(core.CollectionNameApprovals).OrderBy("created DESC")
	if status := strings.TrimSpace(e.Request.URL.Query().Get("status")); status != "" {
		query.AndWhere(dbx.HashExp{"status": status})
	}

	if err := query.All(&approvals); err != nil {
		return e.BadRequestError("Failed to load approvals.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, approvals)
	})
}

func automationApprovalDecision(e *core.RequestEvent) error {
	body := core.AutomationApprovalDecision{}
	if err := e.BindBody(&body); err != nil {
		return e.BadRequestError("Failed to load approval decision.", err)
	}

	approval, err := e.App.ResolveAutomationApprovalDecision(e.Request.PathValue("id"), body)
	if err != nil {
		return e.BadRequestError("Failed to resolve approval.", err)
	}

	app := e.App
	approvalID := approval.Id
	routine.FireAndForget(func() {
		if err := app.ContinueAutomationApproval(approvalID, body.Input); err != nil {
			app.Logger().Warn(
				"Failed to continue automation approval workflow",
				"approvalId", approvalID,
				"error", err,
			)
		}
	})

	return execAfterSuccessTx(true, e.App, func() error {
		return e.NoContent(http.StatusNoContent)
	})
}

func automationSchemas(e *core.RequestEvent) error {
	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, core.AutomationSchemas())
	})
}

func automationTemplatesList(e *core.RequestEvent) error {
	templates := []*core.WorkflowTemplate{}
	err := e.App.RecordQuery(core.CollectionNameWorkflowTemplates).
		OrderBy("created DESC").
		All(&templates)
	if err != nil {
		return e.BadRequestError("Failed to load workflow templates.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, templates)
	})
}

func automationTemplateImport(e *core.RequestEvent) error {
	body := core.WorkflowTemplatePackage{}
	if err := e.BindBody(&body); err != nil {
		return e.BadRequestError("Failed to load workflow template package.", err)
	}

	template, err := e.App.ImportWorkflowTemplatePackage(body)
	if err != nil {
		return e.BadRequestError("Failed to import workflow template.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, template)
	})
}

func automationTemplateInstall(e *core.RequestEvent) error {
	body := core.WorkflowTemplateInstallOptions{}
	if e.Request.Body != nil {
		if err := e.BindBody(&body); err != nil {
			return e.BadRequestError("Failed to load workflow template install options.", err)
		}
	}

	result, err := e.App.InstallWorkflowTemplate(e.Request.PathValue("templateId"), body)
	if result != nil && err != nil {
		return e.BadRequestError("Workflow template dependencies are missing.", err)
	}
	if err != nil {
		return e.BadRequestError("Failed to install workflow template.", err)
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.JSON(http.StatusOK, result)
	})
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

func automationDryRun(e *core.RequestEvent) error {
	automation, err := findAutomationForAPI(e.App, e.Request.PathValue("id"))
	if err != nil {
		return automationAPIError(e, "dry-run", err)
	}

	body := map[string]any{}
	if e.Request.Body != nil {
		if err := e.BindBody(&body); err != nil {
			return e.BadRequestError("Failed to load dry-run input.", err)
		}
	}

	result, err := e.App.RunAutomationDryRun(automation.Id, body)
	if result != nil {
		return execAfterSuccessTx(true, e.App, func() error {
			return e.JSON(http.StatusOK, result)
		})
	}
	if err != nil {
		return e.BadRequestError("Failed to dry-run automation.", err)
	}

	return e.BadRequestError("Failed to dry-run automation.", nil)
}

func automationRunDryRun(e *core.RequestEvent) error {
	automation, err := findAutomationForAPI(e.App, e.Request.PathValue("id"))
	if err != nil {
		return automationAPIError(e, "dry-run run", err)
	}

	run, err := findAutomationRunForAPI(e.App, e.Request.PathValue("runId"))
	if err != nil {
		return automationRunAPIError(e, "dry-run", err)
	}
	if run.AutomationRef() != automation.Id {
		return e.NotFoundError("Missing or invalid automation run.", sql.ErrNoRows)
	}

	result, err := e.App.RunAutomationDryRunFromRun(run.Id)
	if result != nil {
		return execAfterSuccessTx(true, e.App, func() error {
			return e.JSON(http.StatusOK, result)
		})
	}
	if err != nil {
		return e.BadRequestError("Failed to dry-run automation run.", err)
	}

	return e.BadRequestError("Failed to dry-run automation run.", nil)
}

func automationRunRerun(e *core.RequestEvent) error {
	automation, err := findAutomationForAPI(e.App, e.Request.PathValue("id"))
	if err != nil {
		return automationAPIError(e, "rerun", err)
	}

	run, err := findAutomationRunForAPI(e.App, e.Request.PathValue("runId"))
	if err != nil {
		return automationRunAPIError(e, "rerun", err)
	}
	if run.AutomationRef() != automation.Id {
		return e.NotFoundError("Missing or invalid automation run.", sql.ErrNoRows)
	}

	routine.FireAndForget(func() {
		if err := e.App.RunAutomationFromRun(run.Id); err != nil {
			e.App.Logger().Warn(
				"Failed to rerun automation run",
				"automationId", automation.Id,
				"runId", run.Id,
				"error", err,
			)
		}
	})

	return e.NoContent(http.StatusNoContent)
}

func automationWebhook(e *core.RequestEvent) error {
	automation, err := findAutomationForAPI(e.App, e.Request.PathValue("id"))
	if err != nil || automation == nil || !automation.Active() || automation.TriggerType() != core.AutomationTriggerWebhook {
		return e.NotFoundError("Missing or invalid automation webhook.", err)
	}
	if automation.WebhookMethod() != core.NormalizeAutomationWebhookMethod(e.Request.Method) {
		e.Response.Header().Set("Allow", automation.WebhookMethod())
		return router.NewApiError(http.StatusMethodNotAllowed, "Webhook automation does not allow this HTTP method.", nil)
	}

	request, err := automationWebhookRequest(e)
	if err != nil {
		return e.BadRequestError("Failed to load webhook request.", err)
	}

	response, err := e.App.RunAutomationWebhook(automation.Id, request)
	if err != nil {
		e.App.Logger().Warn(
			"Failed to execute automation webhook run",
			"automationId", automation.Id,
			"error", err,
		)
		return e.BadRequestError("Failed to execute automation webhook.", err)
	}

	return automationWebhookResponse(e, response)
}

func automationWebhookResponse(e *core.RequestEvent, response *core.AutomationWebhookResponse) error {
	if response == nil {
		return e.NoContent(http.StatusNoContent)
	}

	for key, value := range response.Headers {
		e.Response.Header().Set(key, value)
	}

	statusCode := response.StatusCode
	if statusCode == 0 {
		statusCode = http.StatusNoContent
	}
	if response.Body == nil {
		return e.NoContent(statusCode)
	}

	if text, ok := response.Body.(string); ok {
		return e.String(statusCode, text)
	}

	return e.JSON(statusCode, response.Body)
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

func automationRunsClear(e *core.RequestEvent) error {
	automation, err := findAutomationForAPI(e.App, e.Request.PathValue("id"))
	if err != nil {
		return automationAPIError(e, "clear runs", err)
	}

	runs := []*core.AutomationRun{}
	err = e.App.RecordQuery(core.CollectionNameAutomationRuns).
		AndWhere(dbx.HashExp{"automationRef": automation.Id}).
		All(&runs)
	if err != nil {
		return e.BadRequestError("Failed to load automation runs.", err)
	}

	for _, run := range runs {
		if err := e.App.Delete(run); err != nil {
			return e.BadRequestError("Failed to clear automation runs.", err)
		}
	}

	return execAfterSuccessTx(true, e.App, func() error {
		return e.NoContent(http.StatusNoContent)
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

func automationWebhookRequest(e *core.RequestEvent) (*core.AutomationWebhookRequest, error) {
	body, err := automationWebhookBody(e)
	if err != nil {
		return nil, err
	}

	return &core.AutomationWebhookRequest{
		Method:   e.Request.Method,
		Path:     e.Request.URL.Path,
		Query:    automationWebhookQuery(e.Request.URL.Query()),
		Headers:  automationWebhookHeaders(e.Request.Header),
		Body:     body,
		RemoteIP: e.RealIP(),
	}, nil
}

func automationWebhookBody(e *core.RequestEvent) (any, error) {
	if e.Request.Body == nil {
		return nil, nil
	}

	contentType := e.Request.Header.Get("Content-Type")

	switch {
	case strings.HasPrefix(contentType, "application/json"):
		raw, err := io.ReadAll(e.Request.Body)
		if err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			return nil, nil
		}

		var body any
		if err := json.Unmarshal(raw, &body); err != nil {
			return nil, err
		}

		return body, nil
	case strings.HasPrefix(contentType, "application/x-www-form-urlencoded"):
		if err := e.Request.ParseForm(); err != nil {
			return nil, err
		}

		return normalizeAutomationWebhookValues(e.Request.PostForm), nil
	case strings.HasPrefix(contentType, "multipart/form-data"):
		if err := e.Request.ParseMultipartForm(router.DefaultMaxMemory); err != nil {
			return nil, err
		}
		if e.Request.MultipartForm == nil {
			return nil, nil
		}

		return normalizeAutomationWebhookValues(e.Request.MultipartForm.Value), nil
	default:
		raw, err := io.ReadAll(e.Request.Body)
		if err != nil {
			return nil, err
		}
		if len(raw) == 0 {
			return nil, nil
		}

		return string(raw), nil
	}
}

func automationWebhookQuery(values map[string][]string) map[string]string {
	if len(values) == 0 {
		return nil
	}

	result := make(map[string]string, len(values))
	for key, entries := range values {
		if len(entries) > 0 {
			result[key] = entries[0]
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

func automationWebhookHeaders(headers http.Header) map[string]string {
	if len(headers) == 0 {
		return nil
	}

	result := make(map[string]string, len(headers))
	for key, entries := range headers {
		if len(entries) > 0 {
			result[inflector.Snakecase(key)] = entries[0]
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}

func normalizeAutomationWebhookValues(values map[string][]string) map[string]any {
	if len(values) == 0 {
		return nil
	}

	result := make(map[string]any, len(values))
	for key, entries := range values {
		switch len(entries) {
		case 0:
			continue
		case 1:
			result[key] = entries[0]
		default:
			items := make([]any, len(entries))
			for i, entry := range entries {
				items[i] = entry
			}
			result[key] = items
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
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

func findAutomationRunForAPI(app core.App, id string) (*core.AutomationRun, error) {
	result := &core.AutomationRun{}

	err := app.RecordQuery(core.CollectionNameAutomationRuns).
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

func automationRunAPIError(e *core.RequestEvent, action string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return e.NotFoundError("Missing or invalid automation run.", err)
	}

	return e.BadRequestError("Failed to "+action+" automation run.", err)
}
