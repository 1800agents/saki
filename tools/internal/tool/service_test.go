package tool

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/1800agents/saki/tools/contracts"
	"github.com/1800agents/saki/tools/controlplane"
	"github.com/1800agents/saki/tools/internal/apperrors"
	tooltemplate "github.com/1800agents/saki/tools/internal/template"
)

func TestDeployApp_HappyPath(t *testing.T) {
	cp := &stubControlPlane{
		prepareRes: controlplane.PrepareAppResponse{
			Repository:         "registry.internal/owner/my-app",
			PushToken:          "push-token",
			RequiredTag:        "abc1234",
			TemplateRepository: "https://example.com/template.git",
			TemplateRef:        "main",
		},
		deployRes: controlplane.DeployAppResponse{
			AppID:        "app_123",
			DeploymentID: "dep_123",
			URL:          "https://my-app.saki.internal",
			Status:       "deploying",
		},
	}
	dockerStub := &stubDockerClient{}
	tempDir := filepath.Join(t.TempDir(), "work")

	var cloned tooltemplate.PrepareResponse
	var cloneDir string
	var wroteEnv struct {
		dir         string
		name        string
		description string
	}
	var removedPath string

	svc := &Service{
		newControlPlane:  func(string) (controlPlaneClient, error) { return cp, nil },
		newDockerClient:  func(Logger) dockerClient { return dockerStub },
		resolveGitCommit: func(context.Context) (string, error) { return "0123456789abcdef", nil },
		makeTempDir:      func() (string, error) { return tempDir, nil },
		removeAll: func(path string) error {
			removedPath = path
			return nil
		},
		cloneFromPrepare: func(_ context.Context, prepare tooltemplate.PrepareResponse, destinationDir string) error {
			cloned = prepare
			cloneDir = destinationDir
			return nil
		},
		writeEnv: func(appDir, name, description string) error {
			wroteEnv.dir = appDir
			wroteEnv.name = name
			wroteEnv.description = description
			return nil
		},
		templateRepoValue: func() string { return "https://env.example/template.git" },
		templateRefValue:  func() string { return "env-ref" },
		logger:            &noopLogger{},
	}

	out, err := svc.DeployApp(context.Background(), contracts.DeployAppInput{
		SakiControlPlaneURL: "https://cp.internal?token=test-token",
		Name:                "my-app",
		Description:         "internal app",
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

	if cloneDir != tempDir {
		t.Fatalf("expected clone destination %q, got %q", tempDir, cloneDir)
	}
	if cloned.TemplateRepository != "https://example.com/template.git" || cloned.TemplateRef != "main" {
		t.Fatalf("unexpected clone source: %+v", cloned)
	}

	if wroteEnv.dir != tempDir || wroteEnv.name != "my-app" || wroteEnv.description != "internal app" {
		t.Fatalf("unexpected .env write params: %+v", wroteEnv)
	}

	if dockerStub.loginRegistry != "registry.internal" || dockerStub.loginUser != tokenUser || dockerStub.loginPassword != "push-token" {
		t.Fatalf("unexpected docker login params: registry=%q user=%q", dockerStub.loginRegistry, dockerStub.loginUser)
	}
	if dockerStub.buildDir != tempDir || dockerStub.image != "registry.internal/owner/my-app:abc1234" {
		t.Fatalf("unexpected docker build params: dir=%q image=%q", dockerStub.buildDir, dockerStub.image)
	}
	if dockerStub.pushImage != "registry.internal/owner/my-app:abc1234" {
		t.Fatalf("unexpected docker push image: %q", dockerStub.pushImage)
	}

	if len(cp.deployReqs) != 1 {
		t.Fatalf("expected one deploy request, got %d", len(cp.deployReqs))
	}
	if cp.deployReqs[0].Image != "registry.internal/owner/my-app:abc1234" {
		t.Fatalf("unexpected deploy image: %q", cp.deployReqs[0].Image)
	}

	if removedPath != tempDir {
		t.Fatalf("expected temp dir cleanup for %q, got %q", tempDir, removedPath)
	}

	if out.AppID != "app_123" || out.DeploymentID != "dep_123" || out.URL != "https://my-app.saki.internal" || out.Status != "deploying" {
		t.Fatalf("unexpected output payload: %+v", out)
	}
	if out.Image != "registry.internal/owner/my-app:abc1234" {
		t.Fatalf("expected output image to include required tag, got %q", out.Image)
	}
}

func TestDeployApp_ValidationFailure(t *testing.T) {
	svc := &Service{}
	_, err := svc.DeployApp(context.Background(), contracts.DeployAppInput{
		Name:        "INVALID_NAME",
		Description: "internal app",
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
		makeTempDir:      func() (string, error) { t.Fatal("makeTempDir must not be called"); return "", nil },
	}

	_, err := svc.DeployApp(context.Background(), contracts.DeployAppInput{
		Name:                "my-app",
		Description:         "internal app",
		SakiControlPlaneURL: "https://cp.internal?token=test-token",
	})
	if !errors.Is(err, prepareErr) {
		t.Fatalf("expected prepare error, got %v", err)
	}
	if len(cp.deployReqs) != 0 {
		t.Fatalf("expected no deploy call after prepare failure, got %d", len(cp.deployReqs))
	}
}

func TestDeployApp_StopsOnDockerFailure(t *testing.T) {
	dockerErr := errors.New("docker login failed")
	cp := &stubControlPlane{
		prepareRes: controlplane.PrepareAppResponse{
			Repository:  "registry.internal/owner/my-app",
			PushToken:   "push-token",
			RequiredTag: "abc1234",
		},
	}
	dockerStub := &stubDockerClient{loginErr: dockerErr}

	svc := &Service{
		newControlPlane:   func(string) (controlPlaneClient, error) { return cp, nil },
		newDockerClient:   func(Logger) dockerClient { return dockerStub },
		resolveGitCommit:  func(context.Context) (string, error) { return "abc", nil },
		makeTempDir:       func() (string, error) { return t.TempDir(), nil },
		removeAll:         func(string) error { return nil },
		cloneFromPrepare:  func(context.Context, tooltemplate.PrepareResponse, string) error { return nil },
		writeEnv:          func(string, string, string) error { return nil },
		templateRepoValue: func() string { return "" },
		templateRefValue:  func() string { return "" },
		logger:            &noopLogger{},
	}

	_, err := svc.DeployApp(context.Background(), contracts.DeployAppInput{
		Name:                "my-app",
		Description:         "internal app",
		SakiControlPlaneURL: "https://cp.internal?token=test-token",
	})
	if !errors.Is(err, dockerErr) {
		t.Fatalf("expected docker error, got %v", err)
	}
	if len(cp.deployReqs) != 0 {
		t.Fatalf("expected no deploy call after docker login failure, got %d", len(cp.deployReqs))
	}
}

func TestResolveTemplateRepository(t *testing.T) {
	t.Run("uses prepare repository when provided", func(t *testing.T) {
		got := resolveTemplateRepository("https://example.com/prepare.git", "https://example.com/env.git")
		if got != "https://example.com/prepare.git" {
			t.Fatalf("expected prepare repository, got %q", got)
		}
	})

	t.Run("falls back to env repository when prepare repository is empty", func(t *testing.T) {
		got := resolveTemplateRepository(" ", "https://example.com/env.git")
		if got != "https://example.com/env.git" {
			t.Fatalf("expected env repository, got %q", got)
		}
	})

	t.Run("falls back to default repository when neither prepare nor env repository is set", func(t *testing.T) {
		got := resolveTemplateRepository(" ", " ")
		if got != defaultTemplateRepository {
			t.Fatalf("expected default repository %q, got %q", defaultTemplateRepository, got)
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
	loginRegistry string
	loginUser     string
	loginPassword string
	loginErr      error

	buildDir string
	image    string
	buildErr error

	pushImage string
	pushErr   error
}

func (s *stubDockerClient) Login(_ context.Context, registry, username, password string) error {
	s.loginRegistry = registry
	s.loginUser = username
	s.loginPassword = password
	return s.loginErr
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
