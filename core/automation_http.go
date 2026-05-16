package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"strings"
	"syscall"
	"time"
)

const (
	AutomationHTTPMaxTimeout      = 60 * time.Second
	AutomationHTTPInputBodyLimit  = 1024 * 1024
	automationHTTPOutputBodyLimit = 64 * 1024
)

type automationHTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

func getAutomationHTTPDoer(app App) automationHTTPDoer {
	doer, ok := app.Store().Get(StoreKeyAutomationHTTPDoer).(automationHTTPDoer)
	if ok && doer != nil {
		return doer
	}

	return newSafeAutomationHTTPClient()
}

func newSafeAutomationHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}

			ip := net.ParseIP(host)
			if ip == nil ||
				ip.IsLoopback() ||
				ip.IsUnspecified() ||
				ip.IsPrivate() ||
				ip.IsLinkLocalUnicast() ||
				ip.IsLinkLocalMulticast() ||
				ip.IsMulticast() {
				return fmt.Errorf("address %q is invalid or resolves to a disallowed IP", address)
			}

			return nil
		},
	}

	return &http.Client{
		Timeout: 180 * time.Second,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
	}
}

func executeAutomationHTTPStep(ctx *automationExecutionContext, step map[string]any) (map[string]any, error) {
	method := stringsToUpperDefault(toString(step["method"]), http.MethodGet)

	renderedURL, err := renderAutomationTemplateString(toString(step["url"]), ctx.TemplateData)
	if err != nil {
		return nil, err
	}

	url, ok := renderedURL.(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("http step is missing a valid url")
	}

	parsedURL, err := neturl.Parse(url)
	if err != nil {
		return nil, fmt.Errorf("invalid http step url: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("http step url must use http or https")
	}

	body, contentType, err := buildAutomationHTTPBody(ctx, step["body"])
	if err != nil {
		return nil, err
	}

	reqCtx := context.Background()
	timeout := automationStepDuration(step["timeout"])
	if timeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(reqCtx, timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(reqCtx, method, url, body)
	if err != nil {
		return nil, err
	}

	if headers, ok := step["headers"].(map[string]any); ok {
		renderedHeaders, err := renderAutomationTemplateValue(headers, ctx.TemplateData)
		if err != nil {
			return nil, err
		}

		for key, value := range renderedHeaders.(map[string]any) {
			req.Header.Set(key, fmt.Sprint(value))
		}
	}

	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}

	res, err := getAutomationHTTPDoer(ctx.App).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	responseBody, _ := io.ReadAll(io.LimitReader(res.Body, automationHTTPOutputBodyLimit))
	if res.StatusCode < 200 || res.StatusCode > 399 {
		if len(responseBody) > 0 {
			return nil, fmt.Errorf("http step request failed with status %d: %s", res.StatusCode, string(responseBody))
		}

		return nil, fmt.Errorf("http step request failed with status %d", res.StatusCode)
	}

	return map[string]any{
		"status":     res.Status,
		"statusCode": res.StatusCode,
		"headers":    automationHTTPHeaderData(res.Header),
		"body":       automationHTTPResponseBodyData(responseBody),
		"bodyText":   string(responseBody),
	}, nil
}

func buildAutomationHTTPBody(ctx *automationExecutionContext, raw any) (io.Reader, string, error) {
	if raw == nil {
		return nil, "", nil
	}

	rendered, err := renderAutomationTemplateValue(raw, ctx.TemplateData)
	if err != nil {
		return nil, "", err
	}

	switch v := rendered.(type) {
	case nil:
		return nil, "", nil
	case string:
		if len(v) > AutomationHTTPInputBodyLimit {
			return nil, "", fmt.Errorf("http step body exceeds %d bytes", AutomationHTTPInputBodyLimit)
		}
		return bytes.NewBufferString(v), "text/plain; charset=utf-8", nil
	default:
		encoded, err := toJSONRaw(v)
		if err != nil {
			return nil, "", err
		}
		if len(encoded.String()) > AutomationHTTPInputBodyLimit {
			return nil, "", fmt.Errorf("http step body exceeds %d bytes", AutomationHTTPInputBodyLimit)
		}

		return bytes.NewReader([]byte(encoded.String())), "application/json", nil
	}
}

func automationStepDuration(value any) time.Duration {
	switch v := value.(type) {
	case float64:
		if v > 0 {
			return time.Duration(v * float64(time.Second))
		}
	case float32:
		if v > 0 {
			return time.Duration(float64(v) * float64(time.Second))
		}
	case int:
		if v > 0 {
			return time.Duration(v) * time.Second
		}
	case int64:
		if v > 0 {
			return time.Duration(v) * time.Second
		}
	}

	return 0
}

func stringsToUpperDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}

	return strings.ToUpper(value)
}

func automationHTTPHeaderData(headers http.Header) map[string]any {
	if len(headers) == 0 {
		return nil
	}

	result := make(map[string]any, len(headers))
	for key, values := range headers {
		if len(values) == 1 {
			result[key] = values[0]
		} else {
			items := make([]any, len(values))
			for i, value := range values {
				items[i] = value
			}
			result[key] = items
		}
	}

	return result
}

func automationHTTPResponseBodyData(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err == nil {
		return decoded
	}

	return string(raw)
}
