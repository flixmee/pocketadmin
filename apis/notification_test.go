package apis_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestNotificationsList(t *testing.T) {
	t.Parallel()

	scenarios := []tests.ApiScenario{
		{
			Name:            "unauthorized",
			Method:          http.MethodGet,
			URL:             "/api/notifications",
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:   "authorized",
			Method: http.MethodGet,
			URL:    "/api/notifications",
			Headers: map[string]string{
				"Authorization": testSuperuserAuthHeader,
			},
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				createNotificationFixture(t, app, "notifapi0000001", "First notification", false, false)
				createNotificationFixture(t, app, "notifapi0000002", "Second notification", true, false)
				createNotificationFixture(t, app, "notifapi0000003", "Archived notification", false, true)
			},
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"notifapi0000001"`,
				`"title":"First notification"`,
				`"id":"notifapi0000002"`,
				`"title":"Second notification"`,
			},
			NotExpectedContent: []string{
				`"Archived notification"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func TestNotificationsUnreadCount(t *testing.T) {
	t.Parallel()

	scenario := tests.ApiScenario{
		Name:   "authorized",
		Method: http.MethodGet,
		URL:    "/api/notifications/unread-count",
		Headers: map[string]string{
			"Authorization": testSuperuserAuthHeader,
		},
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			createNotificationFixture(t, app, "notifapi0000011", "Unread notification", false, false)
			createNotificationFixture(t, app, "notifapi0000012", "Read notification", true, false)
			createNotificationFixture(t, app, "notifapi0000013", "Archived notification", false, true)
		},
		ExpectedStatus:  200,
		ExpectedContent: []string{`"count":1`},
		ExpectedEvents:  map[string]int{"*": 0},
	}

	scenario.Test(t)
}

func TestNotificationsMarkRead(t *testing.T) {
	t.Parallel()

	scenario := tests.ApiScenario{
		Name:   "authorized",
		Method: http.MethodPost,
		URL:    "/api/notifications/notifapi0000021/read",
		Headers: map[string]string{
			"Authorization": testSuperuserAuthHeader,
		},
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			createNotificationFixture(t, app, "notifapi0000021", "Unread notification", false, false)
		},
		ExpectedStatus: 200,
		ExpectedContent: []string{
			`"id":"notifapi0000021"`,
			`"read":true`,
			`"readAt":"`,
		},
	}

	scenario.Test(t)
}

func TestNotificationsMarkAllRead(t *testing.T) {
	t.Parallel()

	scenario := tests.ApiScenario{
		Name:   "authorized",
		Method: http.MethodPost,
		URL:    "/api/notifications/read",
		Headers: map[string]string{
			"Authorization": testSuperuserAuthHeader,
		},
		BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
			createNotificationFixture(t, app, "notifapi0000031", "Unread notification 1", false, false)
			createNotificationFixture(t, app, "notifapi0000032", "Unread notification 2", false, false)
			createNotificationFixture(t, app, "notifapi0000033", "Read notification", true, false)
		},
		ExpectedStatus:  200,
		ExpectedContent: []string{`"count":2`},
	}

	scenario.Test(t)
}

func createNotificationFixture(t testing.TB, app *tests.TestApp, id string, title string, read bool, archived bool) {
	t.Helper()

	superuser, err := app.FindRecordById(core.CollectionNameSuperusers, "sywbhecnh46rhm0")
	if err != nil {
		t.Fatal(err)
	}

	notification := core.NewNotification(app)
	notification.SetRaw("id", id)
	notification.SetRecipientCollection(superuser.Collection().Id)
	notification.SetRecipientRef(superuser.Id)
	notification.SetTitle(title)
	notification.SetMessage(strings.ToLower(title))
	notification.SetSeverity(core.NotificationSeverityInfo)
	notification.SetRead(read)
	notification.SetArchived(archived)

	if err := app.Save(notification); err != nil {
		t.Fatal(err)
	}
}
