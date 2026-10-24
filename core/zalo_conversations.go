package core

import (
	"regexp"
	"sort"
	"strings"

	validation "github.com/pocketbase/ozzo-validation/v4"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/types"
)

// ZaloPhoneThreadPrefix prefixes the conversation id of the messages queued
// by phone number that the bridge didn't resolve to a Zalo thread yet.
const ZaloPhoneThreadPrefix = "phone:"

const (
	zaloConversationsMaxLimit = 100
	zaloMessagesMaxLimit      = 200
	zaloSendTextMaxLength     = 5000
)

var zaloPhonePattern = regexp.MustCompile(`^\+?[0-9][0-9 .-]{5,28}$`)

// ZaloConversation summarizes a Zalo thread (incl. the queued outbox messages).
type ZaloConversation struct {
	Id          string                   `json:"id"`
	ThreadType  string                   `json:"threadType"`
	Name        string                   `json:"name"`
	Phone       string                   `json:"phone,omitempty"`
	LastMessage *ZaloConversationMessage `json:"lastMessage"`
	LastAt      types.DateTime           `json:"lastAt"`
}

// ZaloConversationMessage is a received/sent message (zalo_messages)
// or a message queued for sending (zalo_outbox).
type ZaloConversationMessage struct {
	Id          string         `json:"id"`
	Source      string         `json:"source"` // "message" or "outbox"
	ThreadId    string         `json:"threadId"`
	ThreadType  string         `json:"threadType"`
	MsgId       string         `json:"msgId,omitempty"`
	SenderId    string         `json:"senderId,omitempty"`
	SenderName  string         `json:"senderName,omitempty"`
	IsSelf      bool           `json:"isSelf"`
	MsgType     string         `json:"msgType,omitempty"`
	Text        string         `json:"text"`
	Content     any            `json:"content,omitempty"`
	Quote       any            `json:"quote,omitempty"`
	Recalled    bool           `json:"recalled,omitempty"`
	Status      string         `json:"status,omitempty"` // outbox delivery status
	Error       string         `json:"error,omitempty"`
	Attachments []string       `json:"attachments,omitempty"`
	Created     types.DateTime `json:"created"`
}

// ZaloConversationsQuery defines the ZaloConversations filters.
type ZaloConversationsQuery struct {
	Search string
	Limit  int
	Offset int
}

// zaloOutboxThreadExpr is the conversation id of an outbox record:
// the thread resolved by the bridge, the requested thread or the phone.
const zaloOutboxThreadExpr = "(CASE WHEN [[resolved_thread_id]] != '' THEN [[resolved_thread_id]] " +
	"WHEN [[thread_id]] != '' THEN [[thread_id]] " +
	"ELSE '" + ZaloPhoneThreadPrefix + "' || [[phone]] END)"

// ZaloConversations returns the Zalo conversations ordered by their latest message.
func ZaloConversations(app App, q ZaloConversationsQuery) ([]*ZaloConversation, error) {
	if q.Limit <= 0 || q.Limit > zaloConversationsMaxLimit {
		q.Limit = zaloConversationsMaxLimit
	}
	if q.Offset < 0 {
		q.Offset = 0
	}

	params := dbx.Params{"limit": q.Limit, "offset": q.Offset}

	having := ""
	if search := strings.TrimSpace(q.Search); search != "" {
		params["search"] = "%" + zaloEscapeLike(search) + "%"
		having = "HAVING SUM(CASE WHEN [[thread]] LIKE {:search} ESCAPE '\\' " +
			"OR [[sender_name]] LIKE {:search} ESCAPE '\\' " +
			"OR [[text]] LIKE {:search} ESCAPE '\\' " +
			"OR [[phone]] LIKE {:search} ESCAPE '\\' THEN 1 ELSE 0 END) > 0"
	}

	rows := []struct {
		Thread     string `db:"thread"`
		ThreadType string `db:"thread_type"`
		LastAt     string `db:"last_at"`
	}{}

	err := app.DB().NewQuery(
		"SELECT [[thread]], MAX([[thread_type]]) AS [[thread_type]], MAX([[created]]) AS [[last_at]] FROM (" +
			"SELECT [[thread_id]] AS [[thread]], [[thread_type]], [[sender_name]], [[text]], '' AS [[phone]], [[created]] " +
			"FROM {{" + ZaloCollectionMessages + "}} " +
			"UNION ALL " +
			"SELECT " + zaloOutboxThreadExpr + " AS [[thread]], [[thread_type]], '' AS [[sender_name]], [[text]], [[phone]], [[created]] " +
			"FROM {{" + ZaloCollectionOutbox + "}}" +
			") GROUP BY [[thread]] " + having +
			" ORDER BY [[last_at]] DESC LIMIT {:limit} OFFSET {:offset}",
	).Bind(params).All(&rows)
	if err != nil {
		return nil, err
	}

	result := make([]*ZaloConversation, 0, len(rows))
	for _, row := range rows {
		conversation := &ZaloConversation{
			Id:         row.Thread,
			ThreadType: row.ThreadType,
		}
		conversation.LastAt, _ = types.ParseDateTime(row.LastAt)
		if conversation.ThreadType == "" {
			conversation.ThreadType = "user"
		}

		if phone, ok := strings.CutPrefix(row.Thread, ZaloPhoneThreadPrefix); ok {
			conversation.Phone = phone
			conversation.Name = phone
		} else {
			// the group names aren't part of the messages (the UI shows the group id)
			if conversation.ThreadType != "group" {
				conversation.Name = zaloThreadDisplayName(app, row.Thread)
			}
			conversation.Phone = zaloThreadPhone(app, row.Thread)
			if conversation.Name == "" {
				conversation.Name = conversation.Phone
			}
		}

		messages, _, err := ZaloConversationMessages(app, row.Thread, "", 1)
		if err != nil {
			return nil, err
		}
		if len(messages) > 0 {
			conversation.LastMessage = messages[len(messages)-1]
		}

		result = append(result, conversation)
	}

	return result, nil
}

// ZaloConversationMessages returns up to limit messages of the conversation
// created before the specified date (all when empty), in chronological order.
//
// The outbox messages that the bridge already saved in zalo_messages
// (matched by their Zalo message id) are skipped.
func ZaloConversationMessages(app App, conversationId string, before string, limit int) ([]*ZaloConversationMessage, bool, error) {
	if limit <= 0 || limit > zaloMessagesMaxLimit {
		limit = zaloMessagesMaxLimit
	}

	var beforeExpr dbx.Expression
	if before = strings.TrimSpace(before); before != "" {
		beforeDate, err := types.ParseDateTime(before)
		if err != nil {
			return nil, false, validation.NewError("validation_invalid_before", "Invalid before date.")
		}
		beforeExpr = dbx.NewExp("[[created]] < {:before}", dbx.Params{"before": beforeDate.String()})
	}

	result := []*ZaloConversationMessage{}
	msgIds := map[string]struct{}{}

	phone, isPhoneThread := strings.CutPrefix(conversationId, ZaloPhoneThreadPrefix)

	if !isPhoneThread {
		query := app.RecordQuery(ZaloCollectionMessages).
			AndWhere(dbx.HashExp{"thread_id": conversationId}).
			OrderBy("created DESC", "rowid DESC").
			Limit(int64(limit + 1))
		if beforeExpr != nil {
			query.AndWhere(beforeExpr)
		}

		records := []*Record{}
		if err := query.All(&records); err != nil {
			return nil, false, err
		}

		for _, record := range records {
			item := zaloMessageFromRecord(record)
			if item.MsgId != "" {
				msgIds[item.MsgId] = struct{}{}
			}
			result = append(result, item)
		}
	}

	outboxQuery := app.RecordQuery(ZaloCollectionOutbox).
		OrderBy("created DESC", "rowid DESC").
		Limit(int64(limit + 1))
	if isPhoneThread {
		outboxQuery.AndWhere(dbx.HashExp{"resolved_thread_id": "", "thread_id": "", "phone": phone})
	} else {
		outboxQuery.AndWhere(dbx.Or(
			dbx.HashExp{"resolved_thread_id": conversationId},
			dbx.HashExp{"resolved_thread_id": "", "thread_id": conversationId},
		))
	}
	if beforeExpr != nil {
		outboxQuery.AndWhere(beforeExpr)
	}

	outbox := []*Record{}
	if err := outboxQuery.All(&outbox); err != nil {
		return nil, false, err
	}

	for _, record := range outbox {
		if msgId := record.GetString("zalo_msg_id"); msgId != "" {
			if _, ok := msgIds[msgId]; ok {
				continue
			}
		}
		result = append(result, zaloOutboxMessageFromRecord(app, record, conversationId))
	}

	// newest first, then keep the requested page in chronological order
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Created.After(result[j].Created)
	})

	hasMore := len(result) > limit
	if hasMore {
		result = result[:limit]
	}

	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, hasMore, nil
}

// ZaloSendForm defines the data of a message sent from the admin UI.
type ZaloSendForm struct {
	// ThreadId is a Zalo user/group id or a "phone:<number>" conversation id.
	ThreadId   string `json:"threadId"`
	ThreadType string `json:"threadType"`
	Phone      string `json:"phone"`
	Text       string `json:"text"`
	// ReplyTo is the zalo_messages record id of the quoted message.
	ReplyTo string `json:"replyTo"`
}

// ZaloSendMessage validates the form and queues the message in zalo_outbox
// (the bridge is notified by the outbox after create hook).
func ZaloSendMessage(app App, form ZaloSendForm) (*ZaloConversationMessage, error) {
	if !app.Settings().Credentials.Zalo.Enabled {
		return nil, ErrZaloDisabled
	}

	outbox, err := app.FindCachedCollectionByNameOrId(ZaloCollectionOutbox)
	if err != nil {
		return nil, ErrZaloDisabled
	}

	form.ThreadId = strings.TrimSpace(form.ThreadId)
	form.ThreadType = strings.TrimSpace(form.ThreadType)
	form.Phone = strings.TrimSpace(form.Phone)
	form.Text = strings.TrimSpace(form.Text)
	form.ReplyTo = strings.TrimSpace(form.ReplyTo)

	if phone, ok := strings.CutPrefix(form.ThreadId, ZaloPhoneThreadPrefix); ok {
		form.ThreadId = ""
		form.Phone = phone
	}
	if form.ThreadType == "" {
		form.ThreadType = "user"
	}

	err = validation.ValidateStruct(&form,
		validation.Field(&form.ThreadId,
			validation.When(form.Phone == "", validation.Required.Error("Enter a Zalo user/group id or a phone number.")),
			validation.Length(1, 100),
		),
		validation.Field(&form.ThreadType, validation.In("user", "group")),
		validation.Field(&form.Phone,
			validation.When(form.ThreadId == "" && form.Phone != "",
				validation.Match(zaloPhonePattern).Error("Invalid phone number."),
			),
		),
		validation.Field(&form.Text,
			validation.Required.Error("The message can't be empty."),
			validation.RuneLength(1, zaloSendTextMaxLength),
		),
		validation.Field(&form.ReplyTo, validation.By(func(value any) error {
			id, _ := value.(string)
			if id == "" {
				return nil
			}
			if _, err := app.FindRecordById(ZaloCollectionMessages, id); err != nil {
				return validation.NewError("validation_invalid_reply_to", "The replied message doesn't exist.")
			}
			return nil
		})),
	)
	if err != nil {
		return nil, err
	}

	record := NewRecord(outbox)
	record.Set("status", "pending")
	record.Set("text", form.Text)
	record.Set("reply_to", form.ReplyTo)
	if form.ThreadId != "" {
		record.Set("thread_id", form.ThreadId)
		record.Set("thread_type", form.ThreadType)
	} else {
		record.Set("phone", form.Phone)
	}

	if err := app.Save(record); err != nil {
		return nil, err
	}

	return zaloOutboxMessageFromRecord(app, record, ""), nil
}

func zaloMessageFromRecord(record *Record) *ZaloConversationMessage {
	return &ZaloConversationMessage{
		Id:         record.Id,
		Source:     "message",
		ThreadId:   record.GetString("thread_id"),
		ThreadType: record.GetString("thread_type"),
		MsgId:      record.GetString("msg_id"),
		SenderId:   record.GetString("sender_id"),
		SenderName: record.GetString("sender_name"),
		IsSelf:     record.GetBool("is_self"),
		MsgType:    record.GetString("msg_type"),
		Text:       record.GetString("text"),
		Content:    zaloJSONValue(record.Get("content")),
		Quote:      zaloJSONValue(record.Get("quote")),
		Recalled:   record.GetBool("recalled"),
		Created:    record.GetDateTime("created"),
	}
}

func zaloOutboxMessageFromRecord(app App, record *Record, conversationId string) *ZaloConversationMessage {
	if conversationId == "" {
		conversationId = record.GetString("resolved_thread_id")
		if conversationId == "" {
			conversationId = record.GetString("thread_id")
		}
		if conversationId == "" {
			conversationId = ZaloPhoneThreadPrefix + record.GetString("phone")
		}
	}

	item := &ZaloConversationMessage{
		Id:          record.Id,
		Source:      "outbox",
		ThreadId:    conversationId,
		ThreadType:  record.GetString("thread_type"),
		MsgId:       record.GetString("zalo_msg_id"),
		IsSelf:      true,
		Text:        record.GetString("text"),
		Status:      record.GetString("status"),
		Error:       record.GetString("error"),
		Attachments: record.GetStringSlice("attachments"),
		Created:     record.GetDateTime("created"),
	}

	// same shape as the zca-js quote of the received messages
	if replyTo := record.GetString("reply_to"); replyTo != "" {
		if quoted, err := app.FindRecordById(ZaloCollectionMessages, replyTo); err == nil {
			item.Quote = map[string]any{
				"msg":   quoted.GetString("text"),
				"fromD": quoted.GetString("sender_name"),
			}
		}
	}

	return item
}

// zaloThreadDisplayName returns the latest name of the other party of a direct conversation.
func zaloThreadDisplayName(app App, threadId string) string {
	var name string
	_ = app.DB().Select("sender_name").
		From(ZaloCollectionMessages).
		AndWhere(dbx.HashExp{"thread_id": threadId}).
		AndWhere(dbx.NewExp("[[is_self]] != TRUE AND [[sender_name]] != ''")).
		OrderBy("created DESC").
		Limit(1).
		Row(&name)
	return name
}

// zaloThreadPhone returns the phone number used to start the conversation (if any).
func zaloThreadPhone(app App, threadId string) string {
	var phone string
	_ = app.DB().Select("phone").
		From(ZaloCollectionOutbox).
		AndWhere(dbx.HashExp{"resolved_thread_id": threadId}).
		AndWhere(dbx.NewExp("[[phone]] != ''")).
		OrderBy("created DESC").
		Limit(1).
		Row(&phone)
	return phone
}

func zaloJSONValue(value any) any {
	raw, ok := value.(types.JSONRaw)
	if !ok {
		return value
	}
	if len(raw) == 0 || raw.String() == "null" {
		return nil
	}
	return raw
}

func zaloEscapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}
