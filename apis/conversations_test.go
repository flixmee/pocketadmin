package apis_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/types"
)

// seedZaloConversations creates (ordered by the latest message):
//   - "phone:+84901234567" - a pending outbox message queued by phone
//   - "u1" - Alice: received + sent message (+ its outbox duplicate) + pending outbox
//   - "u2" - started by phone 0909123456, resolved by the bridge
//   - "g1" - a group message from Bob
func seedZaloConversations(t testing.TB, app core.App) {
	base := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	at := func(minutes int) types.DateTime {
		d, _ := types.ParseDateTime(base.Add(time.Duration(minutes) * time.Minute))
		return d
	}

	session := core.NewRecord(mustFindCollection(t, app, core.ZaloCollectionSession))
	session.Id = core.ZaloSessionRecordId
	session.Set("status", "logged_in")
	session.Set("account_name", "Shop")
	if err := app.Save(session); err != nil {
		t.Fatal(err)
	}

	messages := mustFindCollection(t, app, core.ZaloCollectionMessages)
	saveMessage := func(id string, data map[string]any) {
		record := core.NewRecord(messages)
		record.Id = id
		record.Load(data)
		// autodate fields keep only the values set with SetRaw
		record.SetRaw("created", data["created"])
		if err := app.Save(record); err != nil {
			t.Fatal(err)
		}
	}
	saveMessage("zmsgxg100000001", map[string]any{
		"msg_id": "g-1", "thread_id": "g1", "thread_type": "group",
		"sender_id": "b1", "sender_name": "Bob", "text": "group hello", "created": at(0),
	})
	saveMessage("zmsgxu100000001", map[string]any{
		"msg_id": "u1-1", "thread_id": "u1", "thread_type": "user",
		"sender_id": "a1", "sender_name": "Alice", "text": "hello shop", "created": at(10),
	})
	saveMessage("zmsgxu100000002", map[string]any{
		"msg_id": "u1-2", "thread_id": "u1", "thread_type": "user",
		"sender_id": "self", "sender_name": "Shop", "is_self": true, "text": "hi Alice", "created": at(11),
	})

	outbox := mustFindCollection(t, app, core.ZaloCollectionOutbox)
	saveOutbox := func(id string, data map[string]any) {
		record := core.NewRecord(outbox)
		record.Id = id
		record.Load(data)
		// autodate fields keep only the values set with SetRaw
		record.SetRaw("created", data["created"])
		if err := app.Save(record); err != nil {
			t.Fatal(err)
		}
	}
	// already saved by the bridge as zalo_messages "u1-2"
	saveOutbox("zoutxu100000001", map[string]any{
		"thread_id": "u1", "thread_type": "user", "text": "hi Alice", "status": "sent",
		"zalo_msg_id": "u1-2", "resolved_thread_id": "u1", "created": at(11),
	})
	saveOutbox("zoutxu100000002", map[string]any{
		"thread_id": "u1", "thread_type": "user", "text": "follow up", "status": "pending",
		"reply_to": "zmsgxu100000001", "created": at(20),
	})
	saveOutbox("zoutxu200000001", map[string]any{
		"phone": "0909123456", "text": "hello u2", "status": "sent",
		"zalo_msg_id": "u2-1", "resolved_thread_id": "u2", "thread_type": "user", "created": at(5),
	})
	saveOutbox("zoutxph00000001", map[string]any{
		"phone": "+84901234567", "text": "hi by phone", "status": "pending", "created": at(30),
	})
}

func mustFindCollection(t testing.TB, app core.App, name string) *core.Collection {
	collection, err := app.FindCollectionByNameOrId(name)
	if err != nil {
		t.Fatal(err)
	}
	return collection
}

func TestConversationsApi(t *testing.T) {
	t.Parallel()

	bridge := &fakeZaloBridge{}
	server := httptest.NewServer(bridge)
	defer server.Close()

	superuser := map[string]string{"Authorization": testSuperuserAuthHeader}
	user := map[string]string{"Authorization": testZaloUserAuthHeader}

	setup := func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
		enableZalo(t, app, server.URL, testZaloSecret)
		seedZaloConversations(t, app)
	}

	scenarios := []tests.ApiScenario{
		{
			Name:            "channels unauthorized",
			Method:          http.MethodGet,
			URL:             "/api/conversations/channels",
			ExpectedStatus:  401,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "channels requires superuser",
			Method:          http.MethodGet,
			URL:             "/api/conversations/channels",
			Headers:         user,
			ExpectedStatus:  403,
			ExpectedContent: []string{`"data":{}`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "channels with disabled Zalo",
			Method:          http.MethodGet,
			URL:             "/api/conversations/channels",
			Headers:         superuser,
			ExpectedStatus:  200,
			ExpectedContent: []string{`"id":"zalo"`, `"active":false`, `"status":"disabled"`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:    "channels with enabled Zalo but no logged in account",
			Method:  http.MethodGet,
			URL:     "/api/conversations/channels",
			Headers: superuser,
			BeforeTestFunc: func(t testing.TB, app *tests.TestApp, e *core.ServeEvent) {
				enableZalo(t, app, server.URL, testZaloSecret)
			},
			ExpectedStatus:  200,
			ExpectedContent: []string{`"active":false`, `"enabled":true`, `"status":"logged_out"`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "channels with logged in Zalo account",
			Method:          http.MethodGet,
			URL:             "/api/conversations/channels",
			Headers:         superuser,
			BeforeTestFunc:  setup,
			ExpectedStatus:  200,
			ExpectedContent: []string{`"active":true`, `"status":"logged_in"`, `"accountName":"Shop"`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:            "list with disabled Zalo",
			Method:          http.MethodGet,
			URL:             "/api/conversations/zalo",
			Headers:         superuser,
			ExpectedStatus:  400,
			ExpectedContent: []string{`"message":"The Zalo integration is not enabled."`},
			ExpectedEvents:  map[string]int{"*": 0},
		},
		{
			Name:           "list conversations",
			Method:         http.MethodGet,
			URL:            "/api/conversations/zalo",
			Headers:        superuser,
			BeforeTestFunc: setup,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"phone:+84901234567"`,
				`"name":"Alice"`,
				`"name":"0909123456"`,
				`"senderName":"Bob"`,
			},
			ExpectedEvents: map[string]int{"*": 0},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				items := []map[string]any{}
				decodeJSONBody(t, res, &items)

				ids := []string{}
				for _, item := range items {
					ids = append(ids, item["id"].(string))
				}
				expected := "phone:+84901234567,u1,u2,g1"
				if strings.Join(ids, ",") != expected {
					t.Fatalf("Expected conversations %s, got %v", expected, ids)
				}

				// u1: the newest is the pending outbox message
				last := items[1]["lastMessage"].(map[string]any)
				if last["text"] != "follow up" || last["status"] != "pending" || last["source"] != "outbox" {
					t.Fatalf("Unexpected u1 last message %v", last)
				}
				if items[3]["threadType"] != "group" || items[3]["name"] != "" {
					t.Fatalf("Expected g1 to be an unnamed group, got %v", items[3])
				}
			},
		},
		{
			Name:           "search conversations",
			Method:         http.MethodGet,
			URL:            "/api/conversations/zalo?search=alice",
			Headers:        superuser,
			BeforeTestFunc: setup,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"id":"u1"`,
			},
			NotExpectedContent: []string{`"id":"u2"`, `"id":"g1"`, `"id":"phone:`},
			ExpectedEvents:     map[string]int{"*": 0},
		},
		{
			Name:           "conversation messages merge the outbox",
			Method:         http.MethodGet,
			URL:            "/api/conversations/zalo/u1/messages",
			Headers:        superuser,
			BeforeTestFunc: setup,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"hasMore":false`,
				`"msg":"hello shop"`,
				`"fromD":"Alice"`,
			},
			NotExpectedContent: []string{`"id":"zoutxu100000001"`}, // the duplicate of u1-2
			ExpectedEvents:     map[string]int{"*": 0},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				body := struct {
					Items []map[string]any `json:"items"`
				}{}
				decodeJSONBody(t, res, &body)

				texts := []string{}
				for _, item := range body.Items {
					texts = append(texts, item["text"].(string))
				}
				expected := "hello shop,hi Alice,follow up"
				if strings.Join(texts, ",") != expected {
					t.Fatalf("Expected messages %s, got %v", expected, texts)
				}
			},
		},
		{
			Name:           "conversation messages paging",
			Method:         http.MethodGet,
			URL:            "/api/conversations/zalo/u1/messages?limit=1&before=2026-10-20+10:20:00.000Z",
			Headers:        superuser,
			BeforeTestFunc: setup,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"hasMore":true`,
				`"text":"hi Alice"`,
			},
			NotExpectedContent: []string{`"text":"follow up"`, `"text":"hello shop"`},
			ExpectedEvents:     map[string]int{"*": 0},
		},
		{
			Name:           "phone conversation messages",
			Method:         http.MethodGet,
			URL:            "/api/conversations/zalo/phone:+84901234567/messages",
			Headers:        superuser,
			BeforeTestFunc: setup,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"text":"hi by phone"`,
				`"threadId":"phone:+84901234567"`,
			},
			NotExpectedContent: []string{`"text":"hello u2"`},
			ExpectedEvents:     map[string]int{"*": 0},
		},
		{
			Name:           "send validation",
			Method:         http.MethodPost,
			URL:            "/api/conversations/zalo/messages",
			Headers:        superuser,
			Body:           strings.NewReader(`{"text":"   ","replyTo":"missing"}`),
			BeforeTestFunc: setup,
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"threadId":{`,
				`"text":{`,
				`"replyTo":{`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:           "send invalid phone",
			Method:         http.MethodPost,
			URL:            "/api/conversations/zalo/messages",
			Headers:        superuser,
			Body:           strings.NewReader(`{"phone":"abc","text":"hi"}`),
			BeforeTestFunc: setup,
			ExpectedStatus: 400,
			ExpectedContent: []string{
				`"phone":{`,
			},
			ExpectedEvents: map[string]int{"*": 0},
		},
		{
			Name:           "send reply",
			Method:         http.MethodPost,
			URL:            "/api/conversations/zalo/messages",
			Headers:        superuser,
			Body:           strings.NewReader(`{"threadId":"u1","threadType":"user","text":" thanks! ","replyTo":"zmsgxu100000001"}`),
			BeforeTestFunc: setup,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"source":"outbox"`,
				`"status":"pending"`,
				`"text":"thanks!"`,
				`"threadId":"u1"`,
				`"msg":"hello shop"`,
			},
			ExpectedEvents: map[string]int{"OnRecordCreate": 1},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				record, err := app.FindFirstRecordByFilter(core.ZaloCollectionOutbox, "text = 'thanks!'")
				if err != nil {
					t.Fatal(err)
				}
				if record.GetString("thread_id") != "u1" || record.GetString("reply_to") != "zmsgxu100000001" {
					t.Fatalf("Unexpected outbox record %v", record.PublicExport())
				}

				// the bridge is notified in the background
				for i := 0; i < 50 && !bridge.calledWith("POST /kick"); i++ {
					time.Sleep(20 * time.Millisecond)
				}
				if !bridge.calledWith("POST /kick") {
					t.Fatal("Expected the bridge /kick to be called")
				}
			},
		},
		{
			Name:           "send to the phone conversation",
			Method:         http.MethodPost,
			URL:            "/api/conversations/zalo/messages",
			Headers:        superuser,
			Body:           strings.NewReader(`{"threadId":"phone:+84901234567","text":"are you there?"}`),
			BeforeTestFunc: setup,
			ExpectedStatus: 200,
			ExpectedContent: []string{
				`"threadId":"phone:+84901234567"`,
			},
			ExpectedEvents: map[string]int{"OnRecordCreate": 1},
			AfterTestFunc: func(t testing.TB, app *tests.TestApp, res *http.Response) {
				total, err := app.CountRecords(core.ZaloCollectionOutbox, dbx.HashExp{"phone": "+84901234567", "thread_id": ""})
				if err != nil {
					t.Fatal(err)
				}
				if total != 2 {
					t.Fatalf("Expected 2 outbox messages for the phone, got %d", total)
				}
			},
		},
	}

	for _, scenario := range scenarios {
		scenario.Test(t)
	}
}

func decodeJSONBody(t testing.TB, res *http.Response, dst any) {
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("Failed to decode %s: %v", raw, err)
	}
}
