package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/tools/mcpclient"
)

const (
	AutomationAIDefaultMaxToolCalls = 10
	AutomationAIMaxToolCalls        = 50

	automationAIToolListTimeout  = 30 * time.Second
	automationAIToolCallTimeout  = 60 * time.Second
	automationAIToolOutputLimit  = 50000
	automationAIToolLogLimit     = 2000
	automationAIToolMaxTokens    = 4096
	automationAIToolNameMaxChars = 64
)

// AutomationAITool describes a tool that the AI model can call.
type AutomationAITool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

// AutomationAIToolCaller executes a tool call requested by the AI model
// and returns its text result and whether the call failed.
type AutomationAIToolCaller func(name string, args map[string]any) (string, bool)

// automationAIMCPServerRef is a single entry of the AI step "mcpServers" option.
type automationAIMCPServerRef struct {
	Server string
	Tools  []string
}

func automationAIMCPServerRefs(step map[string]any) ([]automationAIMCPServerRef, error) {
	raw, ok := step["mcpServers"]
	if !ok || raw == nil {
		return nil, nil
	}

	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("AI step mcpServers must be a JSON array")
	}

	refs := make([]automationAIMCPServerRef, 0, len(list))
	for i, item := range list {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("AI step mcpServers[%d] must be an object", i)
		}

		ref := automationAIMCPServerRef{Server: strings.TrimSpace(toString(obj["server"]))}
		if ref.Server == "" {
			return nil, fmt.Errorf("AI step mcpServers[%d] requires a server", i)
		}

		if obj["tools"] != nil {
			tools, err := automationAIStringListField(obj, "tools")
			if err != nil {
				return nil, fmt.Errorf("AI step mcpServers[%d].tools must be a JSON array", i)
			}
			ref.Tools = tools
		}

		refs = append(refs, ref)
	}

	return refs, nil
}

func automationAIMaxToolCalls(step map[string]any) int {
	value := int(toFloat(step["maxToolCalls"]))
	if value <= 0 {
		return AutomationAIDefaultMaxToolCalls
	}

	return min(value, AutomationAIMaxToolCalls)
}

func toFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	case string:
		var f float64
		_, _ = fmt.Sscan(strings.TrimSpace(v), &f)
		return f
	default:
		return 0
	}
}

// automationAIMCPToolset holds the MCP tools available to a single AI step execution.
type automationAIMCPToolset struct {
	Tools []AutomationAITool
	Calls []map[string]any

	targets map[string]automationAIMCPToolTarget
	clients []MCPToolSession
}

type automationAIMCPToolTarget struct {
	server *MCPServer
	client MCPToolSession
	tool   string
}

var automationAIToolNameInvalidChars = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func automationAIToolName(name string) string {
	name = automationAIToolNameInvalidChars.ReplaceAllString(name, "_")
	if len(name) > automationAIToolNameMaxChars {
		name = name[:automationAIToolNameMaxChars]
	}
	if name == "" {
		name = "tool"
	}
	return name
}

// loadAutomationAIMCPToolset connects to the referenced MCP servers and
// collects the selected tools.
func loadAutomationAIMCPToolset(app App, refs []automationAIMCPServerRef) (*automationAIMCPToolset, error) {
	toolset := &automationAIMCPToolset{targets: map[string]automationAIMCPToolTarget{}}

	for _, ref := range refs {
		server, err := FindActiveMCPServerById(app, ref.Server)
		if err != nil {
			toolset.Close()
			return nil, err
		}

		client := server.NewToolSession(app)
		toolset.clients = append(toolset.clients, client)

		ctx, cancel := context.WithTimeout(context.Background(), automationAIToolListTimeout)
		tools, err := client.ListTools(ctx)
		cancel()
		if err != nil {
			toolset.Close()
			return nil, fmt.Errorf("MCP connection %q: %w", server.Name, err)
		}

		available := make(map[string]mcpclient.Tool, len(tools))
		for _, tool := range tools {
			available[tool.Name] = tool
		}

		selected := tools
		if len(ref.Tools) > 0 {
			selected = make([]mcpclient.Tool, 0, len(ref.Tools))
			for _, name := range ref.Tools {
				tool, ok := available[name]
				if !ok {
					toolset.Close()
					return nil, fmt.Errorf("MCP connection %q has no tool %q", server.Name, name)
				}
				selected = append(selected, tool)
			}
		}

		for _, tool := range selected {
			// prefix with the server name on conflict and add a numeric suffix as last resort
			name := automationAIToolName(tool.Name)
			if _, exists := toolset.targets[name]; exists {
				name = automationAIToolName(server.Name + "_" + tool.Name)
			}
			base := name
			for i := 2; toolset.targets[name].client != nil; i++ {
				suffix := fmt.Sprintf("_%d", i)
				name = base[:min(len(base), automationAIToolNameMaxChars-len(suffix))] + suffix
			}

			schema := map[string]any{}
			if len(tool.InputSchema) > 0 {
				_ = json.Unmarshal(tool.InputSchema, &schema)
			}
			if schema["type"] != "object" {
				schema = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			delete(schema, "$schema")

			description := strings.TrimSpace(tool.Description)
			if description == "" {
				description = tool.Title
			}

			toolset.targets[name] = automationAIMCPToolTarget{server: server, client: client, tool: tool.Name}
			toolset.Tools = append(toolset.Tools, AutomationAITool{
				Name:        name,
				Description: strings.TrimSpace("[" + server.Name + "] " + description),
				InputSchema: schema,
			})
		}
	}

	return toolset, nil
}

// Call executes the tool call and records it in the toolset call log.
func (s *automationAIMCPToolset) Call(name string, args map[string]any) (string, bool) {
	target, ok := s.targets[name]
	if !ok {
		return fmt.Sprintf("Unknown tool %q.", name), true
	}

	ctx, cancel := context.WithTimeout(context.Background(), automationAIToolCallTimeout)
	defer cancel()

	var text string
	var isError bool

	result, err := target.client.CallTool(ctx, target.tool, args)
	if err != nil {
		text = "Tool call failed: " + err.Error()
		isError = true
	} else {
		text = result.Text()
		isError = result.IsError
	}

	if len(text) > automationAIToolOutputLimit {
		text = text[:automationAIToolOutputLimit] + "\n[output truncated]"
	}

	logged := text
	if len(logged) > automationAIToolLogLimit {
		logged = logged[:automationAIToolLogLimit] + "..."
	}

	s.Calls = append(s.Calls, map[string]any{
		"server":    target.server.Name,
		"tool":      target.tool,
		"arguments": args,
		"isError":   isError,
		"output":    logged,
	})

	return text, isError
}

// Close terminates the opened MCP sessions.
func (s *automationAIMCPToolset) Close() {
	for _, client := range s.clients {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = client.Close(ctx)
		cancel()
	}
	s.clients = nil
}

// -------------------------------------------------------------------
// Provider tool loops
// -------------------------------------------------------------------

func callAutomationAIProviderWithTools(provider, baseURL, apiKey, model, prompt string, req AutomationAIRequest) (string, map[string]int, error) {
	maxCalls := req.MaxToolCalls
	if maxCalls <= 0 {
		maxCalls = AutomationAIDefaultMaxToolCalls
	}

	switch provider {
	case AIProviderGemini:
		return callAutomationAIGeminiWithTools(baseURL, apiKey, model, prompt, req.SystemPrompt, req.History, req.Tools, req.CallTool, maxCalls)
	case AIProviderAnthropic:
		return callAutomationAIAnthropicWithTools(baseURL, apiKey, model, prompt, req.SystemPrompt, req.History, req.Tools, req.CallTool, maxCalls)
	default:
		return callAutomationAIOpenAIWithTools(baseURL, apiKey, model, prompt, req.SystemPrompt, req.History, req.Tools, req.CallTool, maxCalls)
	}
}

const automationAIToolLimitMessage = "Tool call limit reached. Answer with the information gathered so far."

func addAutomationAIUsage(total map[string]int, input, output int) {
	total["input"] += input
	total["output"] += output
	total["total"] = total["input"] + total["output"]
}

func callAutomationAIOpenAIWithTools(baseURL, apiKey, model, prompt string, system string, history []AutomationAIMessage, tools []AutomationAITool, callTool AutomationAIToolCaller, maxCalls int) (string, map[string]int, error) {
	endpoint := strings.TrimRight(baseURL, "/") + "/chat/completions"

	definitions := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		definitions = append(definitions, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name,
				"description": tool.Description,
				"parameters":  tool.InputSchema,
			},
		})
	}

	messages := automationAIOpenAIMessages(system, history, prompt)
	usage := map[string]int{}
	calls := 0

	for round := 0; round <= maxCalls+1; round++ {
		body := map[string]any{"model": model, "messages": messages, "tools": definitions}
		if calls >= maxCalls {
			body["tool_choice"] = "none"
		}

		var response struct {
			Choices []struct {
				Message json.RawMessage `json:"message"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}

		if err := automationAIJSONRequest(http.MethodPost, endpoint, apiKey, "", body, &response); err != nil {
			return "", nil, err
		}
		if len(response.Choices) == 0 {
			return "", nil, fmt.Errorf("AI provider returned no choices")
		}
		addAutomationAIUsage(usage, response.Usage.PromptTokens, response.Usage.CompletionTokens)

		var message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		}
		if err := json.Unmarshal(response.Choices[0].Message, &message); err != nil {
			return "", nil, fmt.Errorf("failed to decode AI provider response: %w", err)
		}

		if len(message.ToolCalls) == 0 {
			return message.Content, usage, nil
		}

		messages = append(messages, response.Choices[0].Message)

		for _, toolCall := range message.ToolCalls {
			var text string
			if calls >= maxCalls {
				text = automationAIToolLimitMessage
			} else {
				calls++
				text, _ = callTool(toolCall.Function.Name, parseAutomationAIToolArgs(toolCall.Function.Arguments))
			}

			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": toolCall.ID,
				"content":      text,
			})
		}
	}

	return "", nil, fmt.Errorf("AI provider kept requesting tools after the limit of %d tool calls", maxCalls)
}

func callAutomationAIAnthropicWithTools(baseURL, apiKey, model, prompt string, system string, history []AutomationAIMessage, tools []AutomationAITool, callTool AutomationAIToolCaller, maxCalls int) (string, map[string]int, error) {
	endpoint := strings.TrimRight(baseURL, "/") + "/messages"

	definitions := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		definitions = append(definitions, map[string]any{
			"name":         tool.Name,
			"description":  tool.Description,
			"input_schema": tool.InputSchema,
		})
	}

	messages := automationAIChatMessages(history, prompt)
	usage := map[string]int{}
	calls := 0

	for round := 0; round <= maxCalls+1; round++ {
		body := map[string]any{
			"model":      model,
			"max_tokens": automationAIToolMaxTokens,
			"messages":   messages,
			"tools":      definitions,
		}
		applyAutomationAISystem(AIProviderAnthropic, body, system)
		if calls >= maxCalls {
			body["tool_choice"] = map[string]any{"type": "none"}
		}

		var response struct {
			Content []json.RawMessage `json:"content"`
			Usage   struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}

		if err := automationAIJSONRequest(http.MethodPost, endpoint, "", apiKey, body, &response); err != nil {
			return "", nil, err
		}
		addAutomationAIUsage(usage, response.Usage.InputTokens, response.Usage.OutputTokens)

		var texts []string
		var results []map[string]any

		for _, raw := range response.Content {
			var block struct {
				Type  string         `json:"type"`
				Text  string         `json:"text"`
				ID    string         `json:"id"`
				Name  string         `json:"name"`
				Input map[string]any `json:"input"`
			}
			if err := json.Unmarshal(raw, &block); err != nil {
				continue
			}

			switch block.Type {
			case "text":
				texts = append(texts, block.Text)
			case "tool_use":
				var text string
				var isError bool
				if calls >= maxCalls {
					text = automationAIToolLimitMessage
					isError = true
				} else {
					calls++
					text, isError = callTool(block.Name, block.Input)
				}

				results = append(results, map[string]any{
					"type":        "tool_result",
					"tool_use_id": block.ID,
					"content":     text,
					"is_error":    isError,
				})
			}
		}

		if len(results) == 0 {
			if len(texts) == 0 {
				return "", nil, fmt.Errorf("AI provider returned no text content")
			}
			return strings.Join(texts, "\n"), usage, nil
		}

		messages = append(messages,
			map[string]any{"role": "assistant", "content": response.Content},
			map[string]any{"role": "user", "content": results},
		)
	}

	return "", nil, fmt.Errorf("AI provider kept requesting tools after the limit of %d tool calls", maxCalls)
}

func callAutomationAIGeminiWithTools(baseURL, apiKey, model, prompt string, system string, history []AutomationAIMessage, tools []AutomationAITool, callTool AutomationAIToolCaller, maxCalls int) (string, map[string]int, error) {
	normalizedModel := strings.TrimPrefix(strings.TrimSpace(model), "models/")
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/") + "/models/" + url.PathEscape(normalizedModel) + ":generateContent")
	if err != nil {
		return "", nil, err
	}
	q := parsed.Query()
	q.Set("key", apiKey)
	parsed.RawQuery = q.Encode()

	declarations := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		declarations = append(declarations, map[string]any{
			"name":                 tool.Name,
			"description":          tool.Description,
			"parametersJsonSchema": tool.InputSchema,
		})
	}

	contents := automationAIGeminiContents(history, prompt)
	usage := map[string]int{}
	calls := 0

	for round := 0; round <= maxCalls+1; round++ {
		body := map[string]any{
			"contents": contents,
			"tools":    []map[string]any{{"functionDeclarations": declarations}},
		}
		applyAutomationAISystem(AIProviderGemini, body, system)
		if calls >= maxCalls {
			body["toolConfig"] = map[string]any{"functionCallingConfig": map[string]any{"mode": "NONE"}}
		}

		var response struct {
			Candidates []struct {
				Content json.RawMessage `json:"content"`
			} `json:"candidates"`
			UsageMetadata struct {
				PromptTokenCount     int `json:"promptTokenCount"`
				CandidatesTokenCount int `json:"candidatesTokenCount"`
			} `json:"usageMetadata"`
		}

		if err := automationAIJSONRequest(http.MethodPost, parsed.String(), "", "", body, &response); err != nil {
			return "", nil, err
		}
		if len(response.Candidates) == 0 {
			return "", nil, fmt.Errorf("AI provider returned no candidates")
		}
		addAutomationAIUsage(usage, response.UsageMetadata.PromptTokenCount, response.UsageMetadata.CandidatesTokenCount)

		var content struct {
			Parts []struct {
				Text         string `json:"text"`
				Thought      bool   `json:"thought"`
				FunctionCall *struct {
					ID   string         `json:"id"`
					Name string         `json:"name"`
					Args map[string]any `json:"args"`
				} `json:"functionCall"`
			} `json:"parts"`
		}
		if err := json.Unmarshal(response.Candidates[0].Content, &content); err != nil {
			return "", nil, fmt.Errorf("failed to decode AI provider response: %w", err)
		}

		var texts []string
		var responses []map[string]any

		for _, part := range content.Parts {
			if part.FunctionCall == nil {
				if part.Text != "" && !part.Thought {
					texts = append(texts, part.Text)
				}
				continue
			}

			var text string
			if calls >= maxCalls {
				text = automationAIToolLimitMessage
			} else {
				calls++
				text, _ = callTool(part.FunctionCall.Name, part.FunctionCall.Args)
			}

			functionResponse := map[string]any{
				"name":     part.FunctionCall.Name,
				"response": map[string]any{"result": text},
			}
			if part.FunctionCall.ID != "" {
				functionResponse["id"] = part.FunctionCall.ID
			}
			responses = append(responses, map[string]any{"functionResponse": functionResponse})
		}

		if len(responses) == 0 {
			if len(texts) == 0 {
				return "", nil, fmt.Errorf("AI provider returned no candidates")
			}
			return strings.Join(texts, ""), usage, nil
		}

		contents = append(contents,
			response.Candidates[0].Content,
			map[string]any{"role": "user", "parts": responses},
		)
	}

	return "", nil, fmt.Errorf("AI provider kept requesting tools after the limit of %d tool calls", maxCalls)
}

func parseAutomationAIToolArgs(raw string) map[string]any {
	args := map[string]any{}

	raw = strings.TrimSpace(raw)
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &args)
	}

	return args
}
