package core

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"strings"
	"syscall"
	"time"
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

func executeAutomationHTTPStep(ctx *automationExecutionContext, step map[string]any) error {
	method := stringsToUpperDefault(toString(step["method"]), http.MethodGet)

	renderedURL, err := renderAutomationTemplateString(toString(step["url"]), ctx.TemplateData)
	if err != nil {
		return err
	}

	url, ok := renderedURL.(string)
	if !ok || url == "" {
		return fmt.Errorf("http step is missing a valid url")
	}

	parsedURL, err := neturl.Parse(url)
	if err != nil {
		return fmt.Errorf("invalid http step url: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("http step url must use http or https")
	}

	body, contentType, err := buildAutomationHTTPBody(ctx, step["body"])
	if err != nil {
		return err
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
		return err
	}

	if headers, ok := step["headers"].(map[string]any); ok {
		renderedHeaders, err := renderAutomationTemplateValue(headers, ctx.TemplateData)
		if err != nil {
			return err
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
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 399 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		if len(body) > 0 {
			return fmt.Errorf("http step request failed with status %d: %s", res.StatusCode, string(body))
		}

		return fmt.Errorf("http step request failed with status %d", res.StatusCode)
	}

	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))

	return nil
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
		return bytes.NewBufferString(v), "text/plain; charset=utf-8", nil
	default:
		encoded, err := toJSONRaw(v)
		if err != nil {
			return nil, "", err
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
