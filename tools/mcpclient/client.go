// Package mcpclient implements a minimal Model Context Protocol (MCP)
// client for the Streamable HTTP transport.
//
// It supports the subset needed to discover and call tools of a remote
// MCP server (initialize, tools/list, tools/call), with both JSON and
// Server-Sent Events (text/event-stream) responses.
package mcpclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ProtocolVersion is the MCP protocol version requested by the client.
const ProtocolVersion = "2025-06-18"

const (
	defaultTimeout   = 60 * time.Second
	maxResponseSize  = 10 << 20
	maxToolListPages = 20
)

// Tool describes a single tool exposed by an MCP server.
type Tool struct {
	Name        string          `json:"name"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// Content is a single content item of a tool call result.
type Content struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	MimeType string          `json:"mimeType,omitempty"`
	Data     string          `json:"data,omitempty"`
	Resource json.RawMessage `json:"resource,omitempty"`
	URI      string          `json:"uri,omitempty"`
	Name     string          `json:"name,omitempty"`
}

// CallToolResult is the result of a tools/call request.
type CallToolResult struct {
	Content           []Content `json:"content"`
	StructuredContent any       `json:"structuredContent,omitempty"`
	IsError           bool      `json:"isError,omitempty"`
}

// Text returns a plain text representation of the result content,
// suitable to be passed back to a language model.
func (r *CallToolResult) Text() string {
	parts := make([]string, 0, len(r.Content))

	for _, c := range r.Content {
		switch c.Type {
		case "text":
			parts = append(parts, c.Text)
		case "image", "audio":
			parts = append(parts, fmt.Sprintf("[%s content (%s) omitted]", c.Type, c.MimeType))
		case "resource":
			var res struct {
				URI  string `json:"uri"`
				Text string `json:"text"`
			}
			_ = json.Unmarshal(c.Resource, &res)
			if res.Text != "" {
				parts = append(parts, res.Text)
			} else {
				parts = append(parts, "[resource "+res.URI+"]")
			}
		case "resource_link":
			parts = append(parts, "[resource link "+c.URI+"]")
		}
	}

	if len(parts) == 0 && r.StructuredContent != nil {
		if raw, err := json.Marshal(r.StructuredContent); err == nil {
			parts = append(parts, string(raw))
		}
	}

	return strings.Join(parts, "\n")
}

// RPCError is a JSON-RPC error returned by the server.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message)
}

// Client is a Streamable HTTP MCP client.
//
// A Client is safe for concurrent use, but most callers create one
// client per logical session and call [Client.Close] when done.
type Client struct {
	// URL is the MCP server endpoint.
	URL string

	// Headers are extra headers sent with every request (e.g. Authorization).
	Headers map[string]string

	// HTTPClient is the underlying HTTP client (defaults to a client with 60s timeout).
	HTTPClient *http.Client

	// ClientName and ClientVersion are reported to the server on initialize.
	ClientName    string
	ClientVersion string

	mu              sync.Mutex
	nextID          int64
	initialized     bool
	sessionID       string
	protocolVersion string
	serverInfo      map[string]any
}

// New creates a new client for the specified endpoint.
func New(url string, headers map[string]string) *Client {
	return &Client{URL: url, Headers: headers}
}

// ServerInfo returns the server info reported on initialize (if any).
func (c *Client) ServerInfo() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.serverInfo
}

// Initialize performs the MCP initialization handshake.
//
// It is called automatically by [Client.ListTools] and [Client.CallTool].
func (c *Client) Initialize(ctx context.Context) error {
	c.mu.Lock()
	if c.initialized {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	name := c.ClientName
	if name == "" {
		name = "pocketadmin"
	}
	version := c.ClientVersion
	if version == "" {
		version = "1.0.0"
	}

	var result struct {
		ProtocolVersion string         `json:"protocolVersion"`
		ServerInfo      map[string]any `json:"serverInfo"`
	}

	err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": name, "version": version},
	}, &result)
	if err != nil {
		return fmt.Errorf("failed to initialize MCP session: %w", err)
	}

	c.mu.Lock()
	c.initialized = true
	c.protocolVersion = result.ProtocolVersion
	c.serverInfo = result.ServerInfo
	c.mu.Unlock()

	// the notification is best effort - some stateless servers ignore it
	_ = c.notify(ctx, "notifications/initialized", nil)

	return nil
}

// ListTools returns all tools exposed by the server (following pagination).
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}

	tools := []Tool{}
	cursor := ""

	for page := 0; page < maxToolListPages; page++ {
		var params map[string]any
		if cursor != "" {
			params = map[string]any{"cursor": cursor}
		}

		var result struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}

		if err := c.call(ctx, "tools/list", params, &result); err != nil {
			return nil, fmt.Errorf("failed to list MCP tools: %w", err)
		}

		tools = append(tools, result.Tools...)

		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}

	return tools, nil
}

// CallTool calls the named tool with the provided arguments.
//
// Tool execution errors are reported with [CallToolResult.IsError],
// while the returned error is reserved for protocol/transport failures.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (*CallToolResult, error) {
	if err := c.Initialize(ctx); err != nil {
		return nil, err
	}

	if args == nil {
		args = map[string]any{}
	}

	result := &CallToolResult{}
	if err := c.call(ctx, "tools/call", map[string]any{"name": name, "arguments": args}, result); err != nil {
		return nil, err
	}

	return result, nil
}

// Close terminates the server session (if the server assigned one).
func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	sessionID := c.sessionID
	c.sessionID = ""
	c.initialized = false
	c.mu.Unlock()

	if sessionID == "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.URL, nil)
	if err != nil {
		return err
	}
	c.setHeaders(req, sessionID)

	res, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()

	return nil
}

// -------------------------------------------------------------------

type rpcResponse struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Result json.RawMessage `json:"result"`
	Error  *RPCError       `json:"error"`
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}

	return &http.Client{Timeout: defaultTimeout}
}

func (c *Client) setHeaders(req *http.Request, sessionID string) {
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}

	req.Header.Set("Accept", "application/json, text/event-stream")

	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}

	c.mu.Lock()
	version := c.protocolVersion
	c.mu.Unlock()
	if version != "" {
		req.Header.Set("MCP-Protocol-Version", version)
	}
}

func (c *Client) notify(ctx context.Context, method string, params any) error {
	body := map[string]any{"jsonrpc": "2.0", "method": method}
	if params != nil {
		body["params"] = params
	}

	res, err := c.post(ctx, body)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, maxResponseSize))

	if res.StatusCode >= 400 {
		return fmt.Errorf("MCP server returned %d for %s", res.StatusCode, method)
	}

	return nil
}

func (c *Client) call(ctx context.Context, method string, params any, result any) error {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.mu.Unlock()

	body := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		body["params"] = params
	}

	res, err := c.post(ctx, body)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if method == "initialize" {
		if sessionID := res.Header.Get("Mcp-Session-Id"); sessionID != "" {
			c.mu.Lock()
			c.sessionID = sessionID
			c.mu.Unlock()
		}
	}

	if res.StatusCode >= 400 {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		message := strings.TrimSpace(string(raw))
		if message == "" {
			message = http.StatusText(res.StatusCode)
		}

		if res.StatusCode == http.StatusNotFound && method != "initialize" {
			// the session has expired - require a new initialization
			c.mu.Lock()
			c.initialized = false
			c.sessionID = ""
			c.mu.Unlock()
		}

		return fmt.Errorf("MCP server returned %d: %s", res.StatusCode, message)
	}

	response, err := readResponse(res, id)
	if err != nil {
		return err
	}

	if response.Error != nil {
		return response.Error
	}

	if result != nil && len(response.Result) > 0 {
		if err := json.Unmarshal(response.Result, result); err != nil {
			return fmt.Errorf("failed to decode the MCP %s result: %w", method, err)
		}
	}

	return nil
}

func (c *Client) post(ctx context.Context, body any) (*http.Response, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	sessionID := c.sessionID
	c.mu.Unlock()

	c.setHeaders(req, sessionID)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("MCP request failed: %w", err)
	}

	return res, nil
}

// readResponse reads the JSON-RPC response with the specified id either
// from a JSON body or from a text/event-stream body.
func readResponse(res *http.Response, id int64) (*rpcResponse, error) {
	mediaType, _, _ := mime.ParseMediaType(res.Header.Get("Content-Type"))
	body := io.LimitReader(res.Body, maxResponseSize)

	if mediaType == "text/event-stream" {
		return readSSEResponse(body, id)
	}

	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, errors.New("empty MCP response")
	}

	// batch response
	if raw[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(raw, &batch); err != nil {
			return nil, fmt.Errorf("invalid MCP response: %w", err)
		}
		for _, item := range batch {
			if resp, ok := matchResponse(item, id); ok {
				return resp, nil
			}
		}
		return nil, errors.New("missing MCP response")
	}

	resp, ok := matchResponse(raw, id)
	if !ok {
		return nil, fmt.Errorf("invalid MCP response: %s", truncate(string(raw), 200))
	}

	return resp, nil
}

func readSSEResponse(body io.Reader, id int64) (*rpcResponse, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 64*1024), maxResponseSize)

	var data strings.Builder

	flush := func() (*rpcResponse, bool) {
		defer data.Reset()
		if data.Len() == 0 {
			return nil, false
		}
		return matchResponse([]byte(data.String()), id)
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")

		if line == "" {
			if resp, ok := flush(); ok {
				return resp, nil
			}
			continue
		}

		if value, ok := strings.CutPrefix(line, "data:"); ok {
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(value, " "))
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read the MCP event stream: %w", err)
	}

	if resp, ok := flush(); ok {
		return resp, nil
	}

	return nil, errors.New("the MCP event stream ended without a response")
}

// matchResponse decodes raw as JSON-RPC response and reports whether
// it is the response for the request with the specified id
// (server requests and notifications are ignored).
func matchResponse(raw []byte, id int64) (*rpcResponse, bool) {
	resp := &rpcResponse{}
	if err := json.Unmarshal(raw, resp); err != nil {
		return nil, false
	}

	if resp.Method != "" || (resp.Result == nil && resp.Error == nil) {
		return nil, false
	}

	var respID int64
	if err := json.Unmarshal(resp.ID, &respID); err != nil {
		// some servers echo numeric ids as strings
		var strID string
		if err := json.Unmarshal(resp.ID, &strID); err != nil || strID != fmt.Sprint(id) {
			return nil, false
		}
		return resp, true
	}

	return resp, respID == id
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
