package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/types"
)

// Zalo bridge collections (same names as the standalone zalo-bridge expects).
const (
	ZaloCollectionSession  = "zalo_session"
	ZaloCollectionMessages = "zalo_messages"
	ZaloCollectionOutbox   = "zalo_outbox"

	// ZaloSessionRecordId is the id of the single zalo_session record.
	ZaloSessionRecordId = "zalomainsession"

	zaloBridgeTimeout = 5 * time.Second

	// default access rule of the Zalo collections (any authenticated user)
	zaloStaffRule = `@request.auth.id != ""`
)

// ZaloCollectionNames lists the collections used by the Zalo bridge.
var ZaloCollectionNames = []string{ZaloCollectionSession, ZaloCollectionMessages, ZaloCollectionOutbox}

// EnsureZaloCollections creates the Zalo bridge collections that don't exist yet.
//
// Existing collections (e.g. created by the old JS migration) are left untouched.
func EnsureZaloCollections(app App) error {
	return app.RunInTransaction(func(txApp App) error {
		if _, err := txApp.FindCollectionByNameOrId(ZaloCollectionSession); err != nil {
			if err := txApp.Save(newZaloSessionCollection()); err != nil {
				return fmt.Errorf("failed to create %s: %w", ZaloCollectionSession, err)
			}
		}

		messages, err := txApp.FindCollectionByNameOrId(ZaloCollectionMessages)
		if err != nil {
			messages = newZaloMessagesCollection()
			if err := txApp.Save(messages); err != nil {
				return fmt.Errorf("failed to create %s: %w", ZaloCollectionMessages, err)
			}
		}

		if _, err := txApp.FindCollectionByNameOrId(ZaloCollectionOutbox); err != nil {
			if err := txApp.Save(newZaloOutboxCollection(messages.Id)); err != nil {
				return fmt.Errorf("failed to create %s: %w", ZaloCollectionOutbox, err)
			}
		}

		return nil
	})
}

// ZaloCollectionsStatus reports which Zalo bridge collections exist.
func ZaloCollectionsStatus(app App) map[string]bool {
	result := make(map[string]bool, len(ZaloCollectionNames))
	for _, name := range ZaloCollectionNames {
		_, err := app.FindCachedCollectionByNameOrId(name)
		result[name] = err == nil
	}
	return result
}

func zaloTimestampFields(c *Collection) {
	c.Fields.Add(&AutodateField{Name: "created", OnCreate: true})
	c.Fields.Add(&AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})
}

func newZaloSessionCollection() *Collection {
	c := NewBaseCollection(ZaloCollectionSession)
	c.ListRule = types.Pointer(zaloStaffRule)
	c.ViewRule = types.Pointer(zaloStaffRule)

	c.Fields.Add(&SelectField{
		Name:      "status",
		MaxSelect: 1,
		Values:    []string{"starting", "logged_out", "connecting", "qr_pending", "qr_scanned", "logged_in", "disconnected", "error"},
	})
	c.Fields.Add(&TextField{Name: "message", Max: 2000})
	c.Fields.Add(&TextField{Name: "qr_image", Max: 500000}) // PNG data URL
	c.Fields.Add(&TextField{Name: "account_id", Max: 100})
	c.Fields.Add(&TextField{Name: "account_name", Max: 200})
	// the Zalo login cookies - only superusers can see them
	c.Fields.Add(&JSONField{Name: "credentials", Hidden: true, MaxSize: 2000000})
	zaloTimestampFields(c)

	return c
}

func newZaloMessagesCollection() *Collection {
	c := NewBaseCollection(ZaloCollectionMessages)
	c.ListRule = types.Pointer(zaloStaffRule)
	c.ViewRule = types.Pointer(zaloStaffRule)

	c.Fields.Add(&TextField{Name: "msg_id", Required: true, Max: 100})
	c.Fields.Add(&TextField{Name: "cli_msg_id", Max: 100})
	c.Fields.Add(&TextField{Name: "thread_id", Required: true, Max: 100})
	c.Fields.Add(&SelectField{Name: "thread_type", MaxSelect: 1, Values: []string{"user", "group"}})
	c.Fields.Add(&TextField{Name: "sender_id", Max: 100})
	c.Fields.Add(&TextField{Name: "sender_name", Max: 300})
	c.Fields.Add(&TextField{Name: "msg_type", Max: 100})
	c.Fields.Add(&TextField{Name: "text", Max: 50000})
	c.Fields.Add(&JSONField{Name: "content", MaxSize: 2000000})
	c.Fields.Add(&JSONField{Name: "quote", MaxSize: 2000000})
	c.Fields.Add(&BoolField{Name: "is_self"})
	c.Fields.Add(&BoolField{Name: "recalled"})
	c.Fields.Add(&NumberField{Name: "zalo_ts"})
	c.Fields.Add(&JSONField{Name: "raw", MaxSize: 2000000})
	zaloTimestampFields(c)

	c.AddIndex("idx_zalo_messages_msg_id", true, "msg_id", "")
	c.AddIndex("idx_zalo_messages_thread", false, "thread_id, zalo_ts", "")

	return c
}

func newZaloOutboxCollection(messagesId string) *Collection {
	c := NewBaseCollection(ZaloCollectionOutbox)
	c.ListRule = types.Pointer(zaloStaffRule)
	c.ViewRule = types.Pointer(zaloStaffRule)
	c.CreateRule = types.Pointer(zaloStaffRule)

	c.Fields.Add(&TextField{Name: "thread_id", Max: 100})
	c.Fields.Add(&SelectField{Name: "thread_type", MaxSelect: 1, Values: []string{"user", "group"}})
	c.Fields.Add(&TextField{Name: "phone", Max: 30}) // used when the thread id is unknown
	c.Fields.Add(&TextField{Name: "text", Max: 5000})
	c.Fields.Add(&RelationField{Name: "reply_to", CollectionId: messagesId, MaxSelect: 1})
	c.Fields.Add(&FileField{Name: "attachments", MaxSelect: 10, MaxSize: 25 << 20, Protected: true})
	c.Fields.Add(&SelectField{Name: "status", MaxSelect: 1, Values: []string{"pending", "sending", "sent", "failed"}})
	c.Fields.Add(&TextField{Name: "error", Max: 2000})
	c.Fields.Add(&TextField{Name: "zalo_msg_id", Max: 100})
	c.Fields.Add(&TextField{Name: "resolved_thread_id", Max: 100})
	c.Fields.Add(&DateField{Name: "sent_at"})
	zaloTimestampFields(c)

	c.AddIndex("idx_zalo_outbox_status", false, "status, created", "")

	return c
}

// -------------------------------------------------------------------
// Bridge client
// -------------------------------------------------------------------

// ErrZaloDisabled is returned when the Zalo integration is not enabled.
var ErrZaloDisabled = errors.New("the Zalo integration is not enabled")

// ZaloBridgeResponse is the response of a bridge call.
type ZaloBridgeResponse struct {
	StatusCode int
	Body       map[string]any
}

// CallZaloBridge sends a request to the configured zalo-bridge.
//
// It returns an error when the integration is disabled or the bridge is unreachable.
func CallZaloBridge(ctx context.Context, app App, method string, path string) (*ZaloBridgeResponse, error) {
	config := app.Settings().Credentials.Zalo
	if !config.Enabled {
		return nil, ErrZaloDisabled
	}

	ctx, cancel := context.WithTimeout(ctx, zaloBridgeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, config.BridgeBaseURL()+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Bridge-Secret", config.Secret)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	result := &ZaloBridgeResponse{StatusCode: res.StatusCode, Body: map[string]any{}}

	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &result.Body)
	}

	return result, nil
}

// KickZaloBridge asks the bridge to process the pending outbox messages
// (best effort, errors are only logged).
func KickZaloBridge(app App) {
	res, err := CallZaloBridge(context.Background(), app, http.MethodPost, "/kick")
	if err != nil {
		if !errors.Is(err, ErrZaloDisabled) {
			// the bridge is offline - the message stays "pending" and is sent when it is back
			app.Logger().Warn("zalo-bridge /kick failed", "error", err.Error())
		}
		return
	}
	if res.StatusCode >= 400 {
		app.Logger().Warn("zalo-bridge /kick failed", "status", res.StatusCode, "body", res.Body)
	}
}

// -------------------------------------------------------------------
// Hooks
// -------------------------------------------------------------------

func (app *BaseApp) registerZaloHooks() {
	// create the collections on startup when the integration is enabled
	app.OnBootstrap().Bind(&hook.Handler[*BootstrapEvent]{
		Func: func(e *BootstrapEvent) error {
			if err := e.Next(); err != nil {
				return err
			}

			if e.App.Settings().Credentials.Zalo.Enabled {
				if err := EnsureZaloCollections(e.App); err != nil {
					e.App.Logger().Error("Failed to create the Zalo collections", "error", err)
				}
			}

			return nil
		},
	})

	// normalize the records created by the app clients
	app.OnRecordCreateRequest(ZaloCollectionOutbox).Bind(&hook.Handler[*RecordRequestEvent]{
		Func: func(e *RecordRequestEvent) error {
			// clients can't set the delivery status/result
			e.Record.Set("status", "pending")
			e.Record.Set("error", "")
			e.Record.Set("zalo_msg_id", "")
			e.Record.Set("resolved_thread_id", "")
			e.Record.Set("sent_at", "")

			threadId := strings.TrimSpace(e.Record.GetString("thread_id"))
			phone := strings.TrimSpace(e.Record.GetString("phone"))
			if threadId == "" && phone == "" {
				return e.BadRequestError("Either thread_id or phone is required.", nil)
			}
			if threadId != "" && e.Record.GetString("thread_type") == "" {
				e.Record.Set("thread_type", "user")
			}

			hasAttachments := len(e.Record.GetStringSlice("attachments")) > 0 || len(e.Record.GetUnsavedFiles("attachments")) > 0
			if strings.TrimSpace(e.Record.GetString("text")) == "" && !hasAttachments {
				return e.BadRequestError("The message requires text or attachments.", nil)
			}

			return e.Next()
		},
	})

	// run the "On Zalo message" automations for the messages saved by the bridge
	app.OnRecordAfterCreateSuccess(ZaloCollectionMessages).Bind(&hook.Handler[*RecordEvent]{
		Func: func(e *RecordEvent) error {
			if err := e.Next(); err != nil {
				return err
			}

			if _, err := QueueAutomationZaloMessage(e.App, e.Record); err != nil {
				e.App.Logger().Warn("Failed to queue the Zalo message automations", "error", err)
			}

			return nil
		},
	})

	// notify the bridge to send the new message right away
	app.OnRecordAfterCreateSuccess(ZaloCollectionOutbox).Bind(&hook.Handler[*RecordEvent]{
		Func: func(e *RecordEvent) error {
			if err := e.Next(); err != nil {
				return err
			}

			if e.App.Settings().Credentials.Zalo.Enabled {
				app := e.App
				go KickZaloBridge(app)
			}

			return nil
		},
	})
}
