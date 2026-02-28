package tool

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/1800agents/saki/tools/contracts"
	"github.com/1800agents/saki/tools/controlplane"
	"github.com/1800agents/saki/tools/internal/apperrors"
)

func TestDeployApp_HappyPath(t *testing.T) {
	cp := &stubControlPlane{
		prepareRes: controlplane.PrepareAppResponse{
			Repository:  "registry.internal/owner/my-app",
			RequiredTag: "abc1234",
		},
		deployRes: controlplane.DeployAppResponse{
			AppID:        "app_123",
			DeploymentID: "dep_123",
			URL:          "https://my-app.saki.internal",
			Status:       "deploying",
		},
	}
	dockerStub := &stubDockerClient{}
	appDir := t.TempDir()

	svc := &Service{
		newControlPlane:     func(string) (controlPlaneClient, error) { return cp, nil },
		newDockerClient:     func(Logger) dockerClient { return dockerStub },
		resolveGitCommit:    func(context.Context) (string, error) { return "0123456789abcdef", nil },
		dockerRegistryValue: func() string { return "" },
		logger:              &noopLogger{},
	}

	out, err := svc.DeployApp(context.Background(), contracts.DeployAppInput{
		SakiControlPlaneURL: "https://cp.internal?token=test-token",
		Name:                "my-app",
		Description:         "internal app",
		AppDir:              appDir,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(cp.prepareReqs) != 1 {
		t.Fatalf("expected one prepare request, got %d", len(cp.prepareReqs))
	}
	if cp.prepareReqs[0].Name != "my-app" || cp.prepareReqs[0].GitCommit != "0123456789abcdef" {
		t.Fatalf("unexpected prepare request: %+v", cp.prepareReqs[0])
	}

	if dockerStub.buildDir != appDir || dockerStub.image != "registry.corgi-teeth.ts.net/owner/my-app:abc1234" {
		t.Fatalf("unexpected docker build params: dir=%q image=%q", dockerStub.buildDir, dockerStub.image)
	}
	if dockerStub.pushImage != "registry.corgi-teeth.ts.net/owner/my-app:abc1234" {
		t.Fatalf("unexpected docker push image: %q", dockerStub.pushImage)
	}

	if len(cp.deployReqs) != 1 {
		t.Fatalf("expected one deploy request, got %d", len(cp.deployReqs))
	}
	if cp.deployReqs[0].Image != "registry.corgi-teeth.ts.net/owner/my-app:abc1234" {
		t.Fatalf("unexpected deploy image: %q", cp.deployReqs[0].Image)
	}

	if out.AppID != "app_123" || out.DeploymentID != "dep_123" || out.URL != "https://my-app.saki.internal" || out.Status != "deploying" {
		t.Fatalf("unexpected output payload: %+v", out)
	}
	if out.Image != "registry.corgi-teeth.ts.net/owner/my-app:abc1234" {
		t.Fatalf("expected output image to include required tag, got %q", out.Image)
	}
}

func TestDeployApp_ValidationFailure(t *testing.T) {
	svc := &Service{}
	_, err := svc.DeployApp(context.Background(), contracts.DeployAppInput{
		Name:        "INVALID_NAME",
		Description: "internal app",
		AppDir:      "/tmp/app",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if got := apperrors.CodeOf(err); got != apperrors.CodeInvalidInput {
		t.Fatalf("expected code %q, got %q", apperrors.CodeInvalidInput, got)
	}
}

func TestDeployApp_StopsOnPrepareFailure(t *testing.T) {
	prepareErr := errors.New("prepare failed")
	cp := &stubControlPlane{prepareErr: prepareErr}

	svc := &Service{
		newControlPlane:  func(string) (controlPlaneClient, error) { return cp, nil },
		resolveGitCommit: func(context.Context) (string, error) { return "abc", nil },
	}

	_, err := svc.DeployApp(context.Background(), contracts.DeployAppInput{
		Name:                "my-app",
		Description:         "internal app",
		SakiControlPlaneURL: "https://cp.internal?token=test-token",
		AppDir:              t.TempDir(),
	})
	if !errors.Is(err, prepareErr) {
		t.Fatalf("expected prepare error, got %v", err)
	}
	if len(cp.deployReqs) != 0 {
		t.Fatalf("expected no deploy call after prepare failure, got %d", len(cp.deployReqs))
	}
}

func TestDeployApp_StopsOnDockerFailure(t *testing.T) {
	dockerErr := errors.New("docker build failed")
	cp := &stubControlPlane{
		prepareRes: controlplane.PrepareAppResponse{
			Repository:  "registry.internal/owner/my-app",
			RequiredTag: "abc1234",
		},
	}
	dockerStub := &stubDockerClient{buildErr: dockerErr}

	svc := &Service{
		newControlPlane:      func(string) (controlPlaneClient, error) { return cp, nil },
		newDockerClient:      func(Logger) dockerClient { return dockerStub },
		resolveGitCommit:     func(context.Context) (string, error) { return "abc", nil },
		dockerRegistryValue:  func() string { return "" },
		controlPlaneURLValue: func() string { return "" },
		logger:               &noopLogger{},
	}

	_, err := svc.DeployApp(context.Background(), contracts.DeployAppInput{
		Name:                "my-app",
		Description:         "internal app",
		SakiControlPlaneURL: "https://cp.internal?token=test-token",
		AppDir:              t.TempDir(),
	})
	if !errors.Is(err, dockerErr) {
		t.Fatalf("expected docker error, got %v", err)
	}
	if len(cp.deployReqs) != 0 {
		t.Fatalf("expected no deploy call after docker failure, got %d", len(cp.deployReqs))
	}
}

func TestDeployApp_RegistryOnlySkipsDeploy(t *testing.T) {
	cp := &stubControlPlane{
		prepareRes: controlplane.PrepareAppResponse{
			Repository:  "registry.internal/owner/my-app",
			RequiredTag: "abc1234",
		},
	}
	dockerStub := &stubDockerClient{}

	svc := &Service{
		newControlPlane:      func(string) (controlPlaneClient, error) { return cp, nil },
		newDockerClient:      func(Logger) dockerClient { return dockerStub },
		resolveGitCommit:     func(context.Context) (string, error) { return "abc", nil },
		dockerRegistryValue:  func() string { return "" },
		registryOnlyValue:    func() string { return "true" },
		controlPlaneURLValue: func() string { return "" },
		logger:               &noopLogger{},
	}

	out, err := svc.DeployApp(context.Background(), contracts.DeployAppInput{
		Name:                "my-app",
		Description:         "internal app",
		SakiControlPlaneURL: "https://cp.internal?token=test-token",
		AppDir:              t.TempDir(),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cp.deployReqs) != 0 {
		t.Fatalf("expected deploy to be skipped in registry-only mode, got %d deploy requests", len(cp.deployReqs))
	}
	if out.Status != "pushed" {
		t.Fatalf("expected status pushed, got %q", out.Status)
	}
	if out.Image != "registry.corgi-teeth.ts.net/owner/my-app:abc1234" {
		t.Fatalf("unexpected output image: %q", out.Image)
	}
}

func TestResolveAppDir(t *testing.T) {
	t.Run("accepts existing directory", func(t *testing.T) {
		dir := t.TempDir()
		got, err := resolveAppDir(dir)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if got != dir {
			t.Fatalf("expected %q, got %q", dir, got)
		}
	})

	t.Run("rejects missing directory", func(t *testing.T) {
		_, err := resolveAppDir(filepath.Join(t.TempDir(), "missing"))
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rejects file path", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "app.txt")
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		_, err := resolveAppDir(file)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestResolveDockerRegistry(t *testing.T) {
	t.Run("uses env value when set", func(t *testing.T) {
		got := resolveDockerRegistry("https://registry.env.example/v2/")
		if got != "https://registry.env.example/v2/" {
			t.Fatalf("expected env registry, got %q", got)
		}
	})

	t.Run("falls back to default value when env is empty", func(t *testing.T) {
		got := resolveDockerRegistry(" ")
		if got != defaultDockerRegistry {
			t.Fatalf("expected default registry %q, got %q", defaultDockerRegistry, got)
		}
	})
}

func TestResolveImageRepository(t *testing.T) {
	t.Run("replaces prepare registry host with configured registry", func(t *testing.T) {
		got := resolveImageRepository("registry.internal/owner/my-app", "https://registry.corgi-teeth.ts.net/v2/")
		if got != "registry.corgi-teeth.ts.net/owner/my-app" {
			t.Fatalf("expected repository with configured registry host, got %q", got)
		}
	})

	t.Run("keeps path-only repository and prefixes configured registry", func(t *testing.T) {
		got := resolveImageRepository("owner/my-app", "https://registry.corgi-teeth.ts.net/v2/")
		if got != "registry.corgi-teeth.ts.net/owner/my-app" {
			t.Fatalf("expected prefixed repository, got %q", got)
		}
	})

	t.Run("strips session-like UUID segments from path", func(t *testing.T) {
		got := resolveImageRepository(
			"registry.internal/owner/11111111-1111-4111-8111-111111111111/my-app",
			"https://registry.corgi-teeth.ts.net/v2/",
		)
		if got != "registry.corgi-teeth.ts.net/owner/my-app" {
			t.Fatalf("expected UUID segment to be removed, got %q", got)
		}
	})

	t.Run("strips UUID suffixes from path segment names", func(t *testing.T) {
		got := resolveImageRepository(
			"registry.internal/owner/my-app-11111111111111111111111111111111",
			"https://registry.corgi-teeth.ts.net/v2/",
		)
		if got != "registry.corgi-teeth.ts.net/owner/my-app" {
			t.Fatalf("expected UUID suffix to be removed, got %q", got)
		}
	})
}

func TestFirstNonEmpty(t *testing.T) {
	got := firstNonEmpty(" ", "\n", "value", "later")
	if got != "value" {
		t.Fatalf("expected first non-empty value, got %q", got)
	}

	got = firstNonEmpty(" ", "\n")
	if got != "" {
		t.Fatalf("expected empty string when all values are empty, got %q", got)
	}
}

func TestEnvEnabled(t *testing.T) {
	if !envEnabled("true") || !envEnabled("1") || !envEnabled(" TRUE ") {
		t.Fatal("expected true-like values to be enabled")
	}
	if envEnabled("") || envEnabled("0") || envEnabled("false") {
		t.Fatal("expected false-like values to be disabled")
	}
}

func TestResolveControlPlaneURL(t *testing.T) {
	t.Run("uses call input when provided", func(t *testing.T) {
		got, err := resolveControlPlaneURL("https://from-input.example?token=abc", "https://from-env.example?token=def")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if got != "https://from-input.example?token=abc" {
			t.Fatalf("expected input URL, got %q", got)
		}
	})

	t.Run("falls back to environment value when input missing", func(t *testing.T) {
		got, err := resolveControlPlaneURL("  ", "https://from-env.example?token=def")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if got != "https://from-env.example?token=def" {
			t.Fatalf("expected env URL, got %q", got)
		}
	})

	t.Run("returns clear error when both input and environment are missing", func(t *testing.T) {
		_, err := resolveControlPlaneURL(" ", "\n")
		if err == nil {
			t.Fatal("expected error when no control plane URL is provided")
		}
		if err.Error() != "resolve control plane URL: saki_control_plane_url is required (or set SAKI_CONTROL_PLANE_URL) (invalid_input)" {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

type stubControlPlane struct {
	prepareRes  controlplane.PrepareAppResponse
	prepareErr  error
	prepareReqs []controlplane.PrepareAppRequest

	deployRes  controlplane.DeployAppResponse
	deployErr  error
	deployReqs []controlplane.DeployAppRequest
}

func (s *stubControlPlane) PrepareApp(_ context.Context, req controlplane.PrepareAppRequest) (controlplane.PrepareAppResponse, error) {
	s.prepareReqs = append(s.prepareReqs, req)
	if s.prepareErr != nil {
		return controlplane.PrepareAppResponse{}, s.prepareErr
	}
	return s.prepareRes, nil
}

func (s *stubControlPlane) DeployApp(_ context.Context, req controlplane.DeployAppRequest) (controlplane.DeployAppResponse, error) {
	s.deployReqs = append(s.deployReqs, req)
	if s.deployErr != nil {
		return controlplane.DeployAppResponse{}, s.deployErr
	}
	return s.deployRes, nil
}

type stubDockerClient struct {
	buildDir string
	image    string
	buildErr error

	pushImage string
	pushErr   error
}

func (s *stubDockerClient) Build(_ context.Context, workDir, image string) error {
	s.buildDir = workDir
	s.image = image
	return s.buildErr
}

func (s *stubDockerClient) Push(_ context.Context, image string) error {
	s.pushImage = image
	return s.pushErr
}

type noopLogger struct{}

func (n *noopLogger) Info(string, map[string]any)  {}
func (n *noopLogger) Error(string, map[string]any) {}
