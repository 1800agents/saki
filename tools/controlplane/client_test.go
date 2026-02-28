package controlplane

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/1800agents/saki/tools/internal/apperrors"
)

func TestNewClient_RequiresToken(t *testing.T) {
	_, err := NewClient("https://cp.internal")
	if err == nil {
		t.Fatal("expected missing token error")
	}
	if got := apperrors.CodeOf(err); got != apperrors.CodeInvalidInput {
		t.Fatalf("expected code %q, got %q", apperrors.CodeInvalidInput, got)
	}
}

func TestPrepareApp_ForwardsTokenAndPayload(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/apps/prepare" {
			t.Fatalf("expected /apps/prepare path, got %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("token"); got != "test-token" {
			t.Fatalf("expected token query to be forwarded, got %q", got)
		}

		var req PrepareAppRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Name != "my-app" || req.GitCommit != "abcdef" {
			t.Fatalf("unexpected request payload: %+v", req)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"repository":"registry.internal/o/my-app","push_token":"pt","expires_at":"2026-02-28T12:00:00Z","required_tag":"abcdef0"}`)
	}))
	defer srv.Close()

	client, err := NewClient(srv.URL + "?token=test-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	res, err := client.PrepareApp(context.Background(), PrepareAppRequest{
		Name:      "my-app",
		GitCommit: "abcdef",
	})
	if err != nil {
		t.Fatalf("prepare app: %v", err)
	}
	if res.Repository != "registry.internal/o/my-app" || res.RequiredTag != "abcdef0" || res.PushToken != "pt" {
		t.Fatalf("unexpected prepare response: %+v", res)
	}
	if res.ExpiresAt.IsZero() {
		t.Fatalf("expected expires_at to be parsed, got zero time")
	}
}

func TestDeployApp_ReturnsAPIErrorEnvelope(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"code":"invalid_image","message":"tag not allowed"}}`)
	}))
	defer srv.Close()

	client, err := NewClient(srv.URL + "?token=test-token")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.DeployApp(context.Background(), DeployAppRequest{
		Name:        "my-app",
		Description: "desc",
		Image:       "registry.internal/o/my-app:bad",
	})
	if err == nil {
		t.Fatal("expected API error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest || apiErr.RemoteCode != "invalid_image" || apiErr.Message != "tag not allowed" {
		t.Fatalf("unexpected API error: %+v", apiErr)
	}
	if got := apperrors.CodeOf(err); got != apperrors.CodeControlPlaneAPI {
		t.Fatalf("expected code %q, got %q", apperrors.CodeControlPlaneAPI, got)
	}
}

func TestDeployApp_MapsTransportTimeout(t *testing.T) {
	t.Parallel()

	client, err := NewClient("https://cp.internal?token=test-token",
		WithHTTPClient(timeoutHTTPClient{}),
		WithRequestTimeout(5*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	_, err = client.DeployApp(context.Background(), DeployAppRequest{
		Name:        "my-app",
		Description: "desc",
		Image:       "registry.internal/o/my-app:abc",
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}

	var reqErr *RequestError
	if !errors.As(err, &reqErr) {
		t.Fatalf("expected RequestError, got %T", err)
	}
	if !reqErr.Timeout {
		t.Fatalf("expected timeout request error, got %+v", reqErr)
	}
	if got := apperrors.CodeOf(err); got != apperrors.CodeTimeout {
		t.Fatalf("expected code %q, got %q", apperrors.CodeTimeout, got)
	}
}

type timeoutHTTPClient struct{}

func (timeoutHTTPClient) Do(*http.Request) (*http.Response, error) {
	return nil, timeoutErr{}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return false }

var _ net.Error = timeoutErr{}
