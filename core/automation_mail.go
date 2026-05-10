package core

import (
	"bytes"
	"fmt"
	"io"
	"net/mail"
	"strings"

	"github.com/pocketbase/pocketbase/tools/mailer"
)

func executeAutomationMailStep(ctx *automationExecutionContext, step map[string]any) (map[string]any, error) {
	to, err := renderAutomationMailAddresses(ctx, step["to"], "to")
	if err != nil {
		return nil, err
	}
	if len(to) == 0 {
		return nil, fmt.Errorf("mail step requires at least one recipient")
	}

	cc, err := renderAutomationMailAddresses(ctx, step["cc"], "cc")
	if err != nil {
		return nil, err
	}

	bcc, err := renderAutomationMailAddresses(ctx, step["bcc"], "bcc")
	if err != nil {
		return nil, err
	}

	subject, err := renderAutomationMailString(ctx, step["subject"])
	if err != nil {
		return nil, err
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return nil, fmt.Errorf("mail step requires a subject")
	}

	text, err := renderAutomationMailString(ctx, step["text"])
	if err != nil {
		return nil, err
	}

	html, err := renderAutomationMailString(ctx, step["html"])
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(text) == "" && strings.TrimSpace(html) == "" {
		return nil, fmt.Errorf("mail step requires text or html content")
	}

	attachments, err := resolveAutomationMailAttachments(ctx, step["attachments"])
	if err != nil {
		return nil, err
	}

	message := &mailer.Message{
		From: mail.Address{
			Name:    strings.TrimSpace(ctx.App.Settings().Meta.SenderName),
			Address: strings.TrimSpace(ctx.App.Settings().Meta.SenderAddress),
		},
		To:          to,
		Cc:          cc,
		Bcc:         bcc,
		Subject:     subject,
		Text:        text,
		HTML:        html,
		Attachments: attachments,
	}

	if err := ctx.App.NewMailClient().Send(message); err != nil {
		return nil, err
	}

	return map[string]any{
		"sent":            true,
		"to":              automationMailAddressStrings(to),
		"cc":              automationMailAddressStrings(cc),
		"bcc":             automationMailAddressStrings(bcc),
		"subject":         subject,
		"attachmentCount": len(attachments),
	}, nil
}

func renderAutomationMailAddresses(ctx *automationExecutionContext, raw any, fieldName string) ([]mail.Address, error) {
	if raw == nil {
		return nil, nil
	}

	rendered, err := renderAutomationTemplateValue(raw, ctx.TemplateData)
	if err != nil {
		return nil, err
	}

	values, err := normalizeAutomationStringValues(rendered)
	if err != nil {
		return nil, fmt.Errorf("mail step %s must contain only strings", fieldName)
	}

	result := make([]mail.Address, 0, len(values))
	for _, value := range values {
		addr, err := mail.ParseAddress(value)
		if err != nil {
			return nil, fmt.Errorf("invalid %s address %q", fieldName, value)
		}
		result = append(result, *addr)
	}

	return result, nil
}

func renderAutomationMailString(ctx *automationExecutionContext, raw any) (string, error) {
	if raw == nil {
		return "", nil
	}

	rendered, err := renderAutomationTemplateValue(raw, ctx.TemplateData)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(fmt.Sprint(rendered)), nil
}

func resolveAutomationMailAttachments(ctx *automationExecutionContext, raw any) (map[string]io.Reader, error) {
	fields, err := normalizeAutomationAttachmentFields(raw)
	if err != nil || len(fields) == 0 {
		return nil, err
	}

	record, err := automationMailAttachmentRecord(ctx)
	if err != nil {
		return nil, err
	}

	fsys, err := ctx.App.NewFilesystem()
	if err != nil {
		return nil, err
	}
	defer fsys.Close()

	attachments := map[string]io.Reader{}

	for _, fieldName := range fields {
		fileField, ok := record.Collection().Fields.GetByName(fieldName).(*FileField)
		if !ok || fileField == nil {
			return nil, fmt.Errorf("mail attachment field %q is not a file field", fieldName)
		}

		for _, filename := range record.GetStringSlice(fieldName) {
			if strings.TrimSpace(filename) == "" {
				continue
			}

			fileKey := record.BaseFilesPath() + "/" + filename
			reuploadable, err := fsys.GetReuploadableFile(fileKey, true)
			if err != nil {
				return nil, fmt.Errorf("failed to load attachment %q from %s: %w", filename, fieldName, err)
			}

			reader, err := reuploadable.Reader.Open()
			if err != nil {
				return nil, fmt.Errorf("failed to open attachment %q from %s: %w", filename, fieldName, err)
			}

			content, err := io.ReadAll(reader)
			reader.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to read attachment %q from %s: %w", filename, fieldName, err)
			}

			name := reuploadable.OriginalName
			if strings.TrimSpace(name) == "" {
				name = filename
			}

			attachments[uniqueAutomationAttachmentName(attachments, name)] = bytes.NewReader(content)
		}
	}

	if len(attachments) == 0 {
		return nil, nil
	}

	return attachments, nil
}

func automationMailAttachmentRecord(ctx *automationExecutionContext) (*Record, error) {
	if ctx.TriggerRecord != nil {
		return ctx.TriggerRecord, nil
	}

	recordID := strings.TrimSpace(toString(ctx.Payload.Record["id"]))
	if recordID == "" {
		return nil, fmt.Errorf("mail step attachments require a trigger record")
	}

	collectionRef := strings.TrimSpace(ctx.Payload.CollectionId)
	if collectionRef == "" {
		collectionRef = strings.TrimSpace(ctx.Payload.CollectionName)
	}
	if collectionRef == "" {
		return nil, fmt.Errorf("mail step attachments require a trigger collection")
	}

	return ctx.App.FindRecordById(collectionRef, recordID)
}

func normalizeAutomationStringValues(raw any) ([]string, error) {
	switch value := raw.(type) {
	case nil:
		return nil, nil
	case string:
		return splitAutomationStringValues(value), nil
	case []string:
		result := make([]string, 0, len(value))
		for _, item := range value {
			result = append(result, splitAutomationStringValues(item)...)
		}
		return result, nil
	case []any:
		result := make([]string, 0, len(value))
		for _, item := range value {
			switch v := item.(type) {
			case string:
				result = append(result, splitAutomationStringValues(v)...)
			default:
				return nil, fmt.Errorf("unsupported list item type %T", item)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported list type %T", raw)
	}
}

func splitAutomationStringValues(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ','
	})

	result := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			result = append(result, field)
		}
	}

	return result
}

func automationMailAddressStrings(addresses []mail.Address) []any {
	if len(addresses) == 0 {
		return nil
	}

	result := make([]any, len(addresses))
	for i, address := range addresses {
		result[i] = address.String()
	}

	return result
}

func normalizeAutomationAttachmentFields(raw any) ([]string, error) {
	values, err := normalizeAutomationStringValues(raw)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}

	return result, nil
}

func uniqueAutomationAttachmentName(existing map[string]io.Reader, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "attachment"
	}

	if _, ok := existing[name]; !ok {
		return name
	}

	base := name
	ext := ""
	if idx := strings.LastIndex(name, "."); idx > 0 {
		base = name[:idx]
		ext = name[idx:]
	}

	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s_%d%s", base, i, ext)
		if _, ok := existing[candidate]; !ok {
			return candidate
		}
	}
}
