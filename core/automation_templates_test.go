package core

import "testing"

func TestRenderAutomationTemplateValueLargeNumbers(t *testing.T) {
	t.Parallel()

	ctx := map[string]any{
		"telegram": map[string]any{
			// JSON decoded numbers are float64
			"chat": map[string]any{"id": float64(-1001234567890)},
		},
		"price": 0.5,
	}

	scenarios := []struct {
		template string
		expected string
	}{
		{"{{telegram.chat.id}}", "-1001234567890"},
		{"chat: {{telegram.chat.id}}", "chat: -1001234567890"},
		{"{{price}}", "0.5"},
	}

	for _, s := range scenarios {
		t.Run(s.template, func(t *testing.T) {
			rendered, err := renderAutomationTemplateValue(s.template, ctx)
			if err != nil {
				t.Fatal(err)
			}

			if result := toString(rendered); result != s.expected {
				t.Fatalf("Expected %q, got %q", s.expected, result)
			}
		})
	}
}
