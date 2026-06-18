package core

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/types"
)

const WorkflowTemplatePackageVersion = "pocketadmin.workflowTemplate.v1"

type WorkflowTemplatePackage struct {
	PackageVersion       string        `json:"packageVersion"`
	Name                 string        `json:"name"`
	Tag                  string        `json:"tag,omitempty"`
	Description          string        `json:"description,omitempty"`
	TriggerType          string        `json:"triggerType"`
	CollectionRef        string        `json:"collectionRef,omitempty"`
	CronExpr             string        `json:"cronExpr,omitempty"`
	WebhookMethod        string        `json:"webhookMethod,omitempty"`
	Steps                types.JSONRaw `json:"steps"`
	RequiredCapabilities []string      `json:"requiredCapabilities,omitempty"`
	RequiredConnectors   []string      `json:"requiredConnectors,omitempty"`
	RequiredCollections  []string      `json:"requiredCollections,omitempty"`
	ConfigPrompts        []string      `json:"configPrompts,omitempty"`
}

type WorkflowTemplateExportOptions struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Active      *bool  `json:"active,omitempty"`
}

type WorkflowTemplateInstallOptions struct {
	Name   string         `json:"name,omitempty"`
	Active bool           `json:"active,omitempty"`
	Config map[string]any `json:"config,omitempty"`
}

type WorkflowTemplateDependencyReport struct {
	MissingCapabilities []string `json:"missingCapabilities,omitempty"`
	MissingConnectors   []string `json:"missingConnectors,omitempty"`
	MissingCollections  []string `json:"missingCollections,omitempty"`
}

type WorkflowTemplateInstallResult struct {
	Automation *Automation                      `json:"automation,omitempty"`
	Missing    WorkflowTemplateDependencyReport `json:"missing"`
	Package    *WorkflowTemplatePackage         `json:"package,omitempty"`
}

func (app *BaseApp) FindWorkflowTemplateById(id string) (*WorkflowTemplate, error) {
	result := &WorkflowTemplate{}
	err := app.RecordQuery(CollectionNameWorkflowTemplates).
		AndWhere(dbx.HashExp{"id": id}).
		Limit(1).
		One(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (app *BaseApp) ExportAutomationTemplate(automationID string, options WorkflowTemplateExportOptions) (*WorkflowTemplate, error) {
	automation, err := app.FindAutomationById(automationID)
	if err != nil {
		return nil, err
	}

	pkg, err := automationTemplatePackageFromAutomation(automation, options)
	if err != nil {
		return nil, err
	}

	pkgRaw, err := toJSONRaw(pkg)
	if err != nil {
		return nil, err
	}
	active := true
	if options.Active != nil {
		active = *options.Active
	}

	template := NewWorkflowTemplate(app)
	template.SetName(pkg.Name)
	template.SetDescription(pkg.Description)
	template.SetPackage(pkgRaw)
	template.SetRequiredCapabilities(mustJSONRaw(pkg.RequiredCapabilities))
	template.SetRequiredConnectors(mustJSONRaw(pkg.RequiredConnectors))
	template.SetRequiredCollections(mustJSONRaw(pkg.RequiredCollections))
	template.SetActive(active)
	if err := app.Save(template); err != nil {
		return nil, err
	}

	return template, nil
}

func (app *BaseApp) ImportWorkflowTemplatePackage(pkg WorkflowTemplatePackage) (*WorkflowTemplate, error) {
	if err := validateWorkflowTemplatePackage(&pkg); err != nil {
		return nil, err
	}

	pkg.RequiredCapabilities = uniqueNonEmptyStrings(pkg.RequiredCapabilities)
	pkg.RequiredConnectors = uniqueNonEmptyStrings(pkg.RequiredConnectors)
	pkg.RequiredCollections = uniqueNonEmptyStrings(pkg.RequiredCollections)
	pkgRaw, err := toJSONRaw(pkg)
	if err != nil {
		return nil, err
	}

	template := NewWorkflowTemplate(app)
	template.SetName(pkg.Name)
	template.SetDescription(pkg.Description)
	template.SetPackage(pkgRaw)
	template.SetRequiredCapabilities(mustJSONRaw(pkg.RequiredCapabilities))
	template.SetRequiredConnectors(mustJSONRaw(pkg.RequiredConnectors))
	template.SetRequiredCollections(mustJSONRaw(pkg.RequiredCollections))
	template.SetActive(true)
	if err := app.Save(template); err != nil {
		return nil, err
	}

	return template, nil
}

func (app *BaseApp) InstallWorkflowTemplate(templateID string, options WorkflowTemplateInstallOptions) (*WorkflowTemplateInstallResult, error) {
	template, err := app.FindWorkflowTemplateById(templateID)
	if err != nil {
		return nil, err
	}

	pkg, err := decodeWorkflowTemplatePackage(template.Package())
	if err != nil {
		return nil, err
	}

	missing := app.CheckWorkflowTemplateDependencies(pkg)
	result := &WorkflowTemplateInstallResult{Missing: missing, Package: pkg}
	if hasMissingWorkflowTemplateDependencies(missing) {
		return result, fmt.Errorf("workflow template has missing dependencies")
	}

	name := strings.TrimSpace(options.Name)
	if name == "" {
		name = pkg.Name
	}

	automation := NewAutomation(app)
	automation.SetName(name)
	automation.SetTag(pkg.Tag)
	automation.SetActive(options.Active)
	automation.SetTriggerType(pkg.TriggerType)
	automation.SetCollectionRef(pkg.CollectionRef)
	automation.SetCronExpr(pkg.CronExpr)
	if automation.TriggerType() == AutomationTriggerWebhook || pkg.WebhookMethod != "" {
		automation.SetWebhookMethod(pkg.WebhookMethod)
	}
	automation.SetSteps(pkg.Steps)
	automation.SetNotes(pkg.Description)
	if err := app.Save(automation); err != nil {
		return nil, err
	}

	result.Automation = automation
	return result, nil
}

func (app *BaseApp) CheckWorkflowTemplateDependencies(pkg *WorkflowTemplatePackage) WorkflowTemplateDependencyReport {
	report := WorkflowTemplateDependencyReport{}
	if pkg == nil {
		return report
	}

	for _, key := range uniqueNonEmptyStrings(pkg.RequiredCapabilities) {
		if _, ok := builtInAutomationCapabilities[key]; ok {
			continue
		}
		if _, err := app.FindCapabilityByKey(key); err != nil {
			report.MissingCapabilities = append(report.MissingCapabilities, key)
		}
	}
	for _, id := range uniqueNonEmptyStrings(pkg.RequiredConnectors) {
		if _, err := app.FindConnectorById(id); err != nil {
			report.MissingConnectors = append(report.MissingConnectors, id)
		}
	}
	for _, nameOrId := range uniqueNonEmptyStrings(pkg.RequiredCollections) {
		if _, err := app.FindCachedCollectionByNameOrId(nameOrId); err != nil {
			report.MissingCollections = append(report.MissingCollections, nameOrId)
		}
	}

	return report
}

func automationTemplatePackageFromAutomation(automation *Automation, options WorkflowTemplateExportOptions) (*WorkflowTemplatePackage, error) {
	name := strings.TrimSpace(options.Name)
	if name == "" {
		name = automation.Name()
	}

	pkg := &WorkflowTemplatePackage{
		PackageVersion: WorkflowTemplatePackageVersion,
		Name:           name,
		Tag:            automation.Tag(),
		Description:    strings.TrimSpace(options.Description),
		TriggerType:    automation.TriggerType(),
		CollectionRef:  automation.CollectionRef(),
		CronExpr:       automation.CronExpr(),
		WebhookMethod:  automationWebhookMethodForExport(automation),
		Steps:          automation.Steps(),
	}
	pkg.RequiredCapabilities = workflowTemplateStepCapabilities(automation.Steps())
	pkg.RequiredConnectors = workflowTemplateStepConnectors(automation.Steps())
	pkg.RequiredCollections = workflowTemplateStepCollections(automation)
	if pkg.CollectionRef != "" {
		pkg.RequiredCollections = append(pkg.RequiredCollections, pkg.CollectionRef)
	}
	pkg.RequiredCapabilities = uniqueNonEmptyStrings(pkg.RequiredCapabilities)
	pkg.RequiredConnectors = uniqueNonEmptyStrings(pkg.RequiredConnectors)
	pkg.RequiredCollections = uniqueNonEmptyStrings(pkg.RequiredCollections)

	return pkg, validateWorkflowTemplatePackage(pkg)
}

func decodeWorkflowTemplatePackage(raw types.JSONRaw) (*WorkflowTemplatePackage, error) {
	pkg := &WorkflowTemplatePackage{}
	if err := json.Unmarshal([]byte(raw.String()), pkg); err != nil {
		return nil, err
	}
	if err := validateWorkflowTemplatePackage(pkg); err != nil {
		return nil, err
	}
	return pkg, nil
}

func validateWorkflowTemplatePackage(pkg *WorkflowTemplatePackage) error {
	if pkg == nil {
		return fmt.Errorf("missing workflow template package")
	}
	if strings.TrimSpace(pkg.PackageVersion) == "" {
		pkg.PackageVersion = WorkflowTemplatePackageVersion
	}
	if pkg.PackageVersion != WorkflowTemplatePackageVersion {
		return fmt.Errorf("unsupported workflow template package version %q", pkg.PackageVersion)
	}
	if strings.TrimSpace(pkg.Name) == "" {
		return fmt.Errorf("workflow template package requires a name")
	}
	if strings.TrimSpace(pkg.TriggerType) == "" {
		return fmt.Errorf("workflow template package requires a triggerType")
	}
	if strings.TrimSpace(pkg.Steps.String()) == "" {
		return fmt.Errorf("workflow template package requires steps")
	}
	return nil
}

func workflowTemplateStepCapabilities(raw types.JSONRaw) []string {
	steps := []map[string]any{}
	_ = json.Unmarshal([]byte(raw.String()), &steps)
	result := []string{}
	for _, step := range steps {
		if strings.TrimSpace(toString(step["type"])) == AutomationStepCapability {
			result = append(result, automationCapabilityKey(step))
		}
	}
	return result
}

func workflowTemplateStepConnectors(raw types.JSONRaw) []string {
	steps := []map[string]any{}
	_ = json.Unmarshal([]byte(raw.String()), &steps)
	result := []string{}
	for _, step := range steps {
		if connectorRef := strings.TrimSpace(toString(step["connectorRef"])); connectorRef != "" {
			result = append(result, connectorRef)
		}
	}
	return result
}

func workflowTemplateStepCollections(automation *Automation) []string {
	steps := []map[string]any{}
	_ = json.Unmarshal([]byte(automation.Steps().String()), &steps)
	result := []string{}
	for _, step := range steps {
		switch strings.TrimSpace(toString(step["type"])) {
		case AutomationStepRecordCreate, AutomationStepRecordUpdate, AutomationStepRecordDelete:
			if collection := strings.TrimSpace(toString(step["collection"])); collection != "" && !strings.Contains(collection, "{{") {
				result = append(result, collection)
			}
		case AutomationStepCapability:
			input, _ := automationCapabilityInput(step)
			if collection := strings.TrimSpace(toString(input["collection"])); collection != "" && !strings.Contains(collection, "{{") {
				result = append(result, collection)
			}
		}
	}
	return result
}

func uniqueNonEmptyStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func hasMissingWorkflowTemplateDependencies(report WorkflowTemplateDependencyReport) bool {
	return len(report.MissingCapabilities) > 0 || len(report.MissingConnectors) > 0 || len(report.MissingCollections) > 0
}

func mustJSONRaw(value any) types.JSONRaw {
	raw, err := toJSONRaw(value)
	if err != nil {
		return types.JSONRaw("[]")
	}
	return raw
}
