package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultRequestTimeout = 15 * time.Second

// HTTPClient abstracts http.Client for easier testing.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client calls the Saki control plane API.
type Client struct {
	baseURL        *url.URL
	token          string
	httpClient     HTTPClient
	requestTimeout time.Duration
}

// PrepareAppRequest is the payload for POST /apps/prepare.
type PrepareAppRequest struct {
	Name      string `json:"name"`
	GitCommit string `json:"git_commit"`
}

// PrepareAppResponse is the response body from POST /apps/prepare.
type PrepareAppResponse struct {
	Repository  string    `json:"repository"`
	PushToken   string    `json:"push_token"`
	ExpiresAt   time.Time `json:"expires_at"`
	RequiredTag string    `json:"required_tag"`
}

// DeployAppRequest is the payload for POST /apps.
type DeployAppRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Image       string `json:"image"`
}

// DeployAppResponse is the response body from POST /apps.
type DeployAppResponse struct {
	AppID        string `json:"app_id"`
	DeploymentID string `json:"deployment_id"`
	URL          string `json:"url"`
	Status       string `json:"status"`
}

// APIError describes a structured error returned by the control plane.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Details    json.RawMessage
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return fmt.Sprintf("control plane request failed with status %d: %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("control plane error (%s): %s", e.Code, e.Message)
}

// RequestError represents transport-level failures, including timeouts.
type RequestError struct {
	Err       error
	Timeout   bool
	Operation string
}

func (e *RequestError) Error() string {
	if e == nil {
		return ""
	}
	if e.Timeout {
		return fmt.Sprintf("control plane request timed out during %s: %v", e.Operation, e.Err)
	}
	return fmt.Sprintf("control plane request failed during %s: %v", e.Operation, e.Err)
}

func (e *RequestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Option configures the control plane client.
type Option func(*Client)

// WithHTTPClient sets a custom HTTP client implementation.
func WithHTTPClient(client HTTPClient) Option {
	return func(c *Client) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// WithRequestTimeout sets the per-request timeout.
func WithRequestTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if timeout > 0 {
			c.requestTimeout = timeout
		}
	}
}

// NewClient creates a control plane client from a tokenized base URL.
func NewClient(controlPlaneURL string, opts ...Option) (*Client, error) {
	parsedURL, err := url.Parse(controlPlaneURL)
	if err != nil {
		return nil, fmt.Errorf("parse control plane URL: %w", err)
	}

	token := strings.TrimSpace(parsedURL.Query().Get("token"))
	if token == "" {
		return nil, fmt.Errorf("missing token in control plane URL")
	}

	cleanURL := *parsedURL
	query := cleanURL.Query()
	query.Del("token")
	cleanURL.RawQuery = query.Encode()
	cleanURL.Fragment = ""

	client := &Client{
		baseURL:        &cleanURL,
		token:          token,
		httpClient:     &http.Client{},
		requestTimeout: defaultRequestTimeout,
	}

	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// PrepareApp calls POST /apps/prepare with token forwarding.
func (c *Client) PrepareApp(ctx context.Context, req PrepareAppRequest) (PrepareAppResponse, error) {
	return doJSON[PrepareAppRequest, PrepareAppResponse](ctx, c, http.MethodPost, "/apps/prepare", req, "prepare app")
}

// DeployApp calls POST /apps with token forwarding.
func (c *Client) DeployApp(ctx context.Context, req DeployAppRequest) (DeployAppResponse, error) {
	return doJSON[DeployAppRequest, DeployAppResponse](ctx, c, http.MethodPost, "/apps", req, "deploy app")
}

func doJSON[TReq any, TResp any](ctx context.Context, c *Client, method, path string, payload TReq, operation string) (TResp, error) {
	var zero TResp

	requestBody, err := json.Marshal(payload)
	if err != nil {
		return zero, fmt.Errorf("marshal %s payload: %w", operation, err)
	}

	endpoint := c.endpointURL(path)
	q := endpoint.Query()
	q.Set("token", c.token)
	endpoint.RawQuery = q.Encode()

	ctxWithTimeout, cancel := withTimeout(ctx, c.requestTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctxWithTimeout, method, endpoint.String(), bytes.NewReader(requestBody))
	if err != nil {
		return zero, fmt.Errorf("build %s request: %w", operation, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return zero, &RequestError{Err: err, Timeout: isTimeoutError(err), Operation: operation}
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := decodeAPIError(resp)
		if apiErr != nil {
			return zero, apiErr
		}
		return zero, fmt.Errorf("%s failed with status %d", operation, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, fmt.Errorf("read %s response: %w", operation, err)
	}

	if len(bytes.TrimSpace(body)) == 0 {
		return zero, nil
	}

	var out TResp
	if err := json.Unmarshal(body, &out); err != nil {
		return zero, fmt.Errorf("decode %s response: %w", operation, err)
	}

	return out, nil
}

func withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func decodeAPIError(resp *http.Response) *APIError {
	body, _ := io.ReadAll(resp.Body)

	type errorEnvelope struct {
		Error struct {
			Code    string          `json:"code"`
			Message string          `json:"message"`
			Details json.RawMessage `json:"details"`
		} `json:"error"`
	}

	var envelope errorEnvelope
	if err := json.Unmarshal(body, &envelope); err == nil && (envelope.Error.Code != "" || envelope.Error.Message != "") {
		return &APIError{
			StatusCode: resp.StatusCode,
			Code:       envelope.Error.Code,
			Message:    envelope.Error.Message,
			Details:    envelope.Error.Details,
		}
	}

	message := strings.TrimSpace(string(body))
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}

	return &APIError{
		StatusCode: resp.StatusCode,
		Message:    message,
	}
}

func (c *Client) endpointURL(path string) *url.URL {
	endpoint := *c.baseURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/" + strings.TrimLeft(path, "/")
	return &endpoint
}
