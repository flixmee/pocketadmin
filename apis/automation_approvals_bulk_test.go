package apis_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// createBulkApprovalFixtures creates 3 pending approvals and 1 already rejected one.
func createBulkApprovalFixtures(t testing.TB, app *tests.TestApp) {
	t.Helper()

	first := createApprovalFixture(t, app) // approvalapi0001

	extra := []struct {
		id     string
		status string
	}{
		{"approvalapi0002", core.ApprovalStatusPending},
		{"approvalapi0003", core.ApprovalStatusPending},
		{"approvalapi0004", core.ApprovalStatusRejected},
	}

	for _, item := range extra {
		approval := core.NewApproval(app)
		approval.SetRaw("id", item.id)
		approval.SetWorkflowStateRef(first.WorkflowStateRef())
		approval.SetAutomationRef(first.AutomationRef())
		approval.SetRunRef(first.RunRef())
		approval.SetStepIndex(0)
		approval.SetAssignee("ops@example.com")
		approval.SetStatus(item.status)
		if err := app.Save(approval); err != nil {
			t.Fatalf("Failed to create approval fixture: %v", err)
		}
	}
}

func TestAutomationApprovalBulkDecision(t *testing.T) {
	t.Parallel()

	superuser := map[string]string{"Authorization": testSuperuserAuthHeader}

	scenarios := []tests.ApiScenario{
		{
			Name:            "unauthorized",
			Method:          http.MethodPost,
			URL:             "/api/automations/approvals/decisions",
			Body:            strings.NewReader(`{"ids":["approvalapi0001"],"decision":"approved"}`),
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "regular user",
			Method:          http.MethodPost,
			URL:             "/api/automations/approvals/decisions",
			Body:            strings.NewReader(`{"ids":["approvalapi0001"],"decision":"approved"}`),
			Headers:         map[string]string{"Authorization": testRegularAuthHeader},
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "empty selection",
			Method:          http.MethodPost,
			URL:             "/api/automations/approvals/decisions",
			Body:            strings.NewReader(`{"ids":[" "],"decision":"approved"}`),
			Headers:         superuser,
			ExpectedStatus:  400,
			ExpectedContent: []string{`Select at least one approval`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:           "bulk approve with partial failures",
			Method:         http.MethodPost,
			URL:            "/api/automations/approvals/decisions",
			Body:           strings.NewReader(`{"ids":["approvalapi0002","approvalapi0003","approvalapi0003","approvalapi0004","missing"],"decision":"approved","comment":"Batch OK"}`),
			Headers:        superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) { createBulkApprovalFixtures(t, app) },
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"resolved":["approvalapi0002","approvalapi0003"]`,
				`{"id":"approvalapi0004","error":"approval has already been resolved"}`,
				`"id":"missing"`,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				for _, id := range []string{"approvalapi0002", "approvalapi0003"} {
					approval, err := app.FindApprovalById(id)
					if err != nil || approval.Status() != core.ApprovalStatusApproved || approval.Comment() != "Batch OK" {
						t.Fatalf("Expected %s to be approved with the comment, got %v (%v)", id, approval, err)
					}
				}

				untouched, _ := app.FindApprovalById("approvalapi0001")
				if untouched.Status() != core.ApprovalStatusPending {
					t.Fatalf("Expected approvalapi0001 to stay pending, got %q", untouched.Status())
				}

				rejected, _ := app.FindApprovalById("approvalapi0004")
				if rejected.Status() != core.ApprovalStatusRejected {
					t.Fatalf("Expected approvalapi0004 to stay rejected, got %q", rejected.Status())
				}
			},
		},
		{
			Name:           "bulk reject",
			Method:         http.MethodPost,
			URL:            "/api/automations/approvals/decisions",
			Body:           strings.NewReader(`{"ids":["approvalapi0001","approvalapi0002"],"decision":"reject"}`),
			Headers:        superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) { createBulkApprovalFixtures(t, app) },
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"resolved":["approvalapi0001","approvalapi0002"]`,
				`"failed":[]`,
			},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				for _, id := range []string{"approvalapi0001", "approvalapi0002"} {
					approval, _ := app.FindApprovalById(id)
					if approval.Status() != core.ApprovalStatusRejected {
						t.Fatalf("Expected %s to be rejected, got %q", id, approval.Status())
					}
				}
			},
		},
		{
			Name:           "invalid decision",
			Method:         http.MethodPost,
			URL:            "/api/automations/approvals/decisions",
			Body:           strings.NewReader(`{"ids":["approvalapi0001"],"decision":"maybe"}`),
			Headers:        superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) { createBulkApprovalFixtures(t, app) },
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"resolved":[]`,
				`must be approved or rejected`,
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestAutomationApprovalsListFilters(t *testing.T) {
	t.Parallel()

	superuser := map[string]string{"Authorization": testSuperuserAuthHeader}

	scenarios := []tests.ApiScenario{
		{
			Name:           "pending with automation names",
			Method:         http.MethodGet,
			URL:            "/api/automations/approvals?status=pending",
			Headers:        superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) { createBulkApprovalFixtures(t, app) },
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"approvalapi0001"`,
				`"id":"approvalapi0003"`,
				`"automationName":"Approval API automation"`,
			},
			NotExpectedContent: []string{`"id":"approvalapi0004"`},
			ExpectedEvents:     map[string]int{"*": 0},
		},
		{
			Name:            "resolved history",
			Method:          http.MethodGet,
			URL:             "/api/automations/approvals?status=resolved",
			Headers:         superuser,
			BeforeTestFunc:  func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) { createBulkApprovalFixtures(t, app) },
			ExpectedStatus:  200,
			ExpectedContent: []string{`"id":"approvalapi0004"`, `"status":"rejected"`},
			NotExpectedContent: []string{
				`"id":"approvalapi0001"`,
				`"status":"pending"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:           "limit",
			Method:         http.MethodGet,
			URL:            "/api/automations/approvals?status=pending&limit=1",
			Headers:        superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) { createBulkApprovalFixtures(t, app) },
			ExpectedStatus: 200,
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				raw, _ := io.ReadAll(res.Body)
				if count := strings.Count(string(raw), `"id":"approvalapi`); count != 1 {
					t.Fatalf("Expected 1 approval, got %d: %s", count, raw)
				}
			},
			ExpectedContent: []string{`"automationName"`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}
