package tool

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/1800agents/saki/tools/contracts"
	"github.com/1800agents/saki/tools/controlplane"
	"github.com/1800agents/saki/tools/docker"
	"github.com/1800agents/saki/tools/internal/apperrors"
	"github.com/1800agents/saki/tools/internal/logging"
	tooltemplate "github.com/1800agents/saki/tools/internal/template"
)

const (
	controlPlaneURLEnv        = "SAKI_CONTROL_PLANE_URL"
	templateRepoEnv           = "SAKI_TEMPLATE_REPOSITORY"
	templateRefEnv            = "SAKI_TEMPLATE_REF"
	dockerRegistryEnv         = "SAKI_DOCKER_REGISTRY"
	registryOnlyEnv           = "SAKI_REGISTRY_ONLY"
	defaultTemplateRepository = "https://github.com/1800agents/saki-app-template"
	defaultDockerRegistry     = "https://registry.corgi-teeth.ts.net/v2/"
)

type Logger interface {
	Info(msg string, fields map[string]any)
	Error(msg string, fields map[string]any)
}

type controlPlaneClient interface {
	PrepareApp(ctx context.Context, req controlplane.PrepareAppRequest) (controlplane.PrepareAppResponse, error)
	DeployApp(ctx context.Context, req controlplane.DeployAppRequest) (controlplane.DeployAppResponse, error)
}

type dockerClient interface {
	Build(ctx context.Context, workDir, image string) error
	Push(ctx context.Context, image string) error
}

type controlPlaneFactory func(controlPlaneURL string) (controlPlaneClient, error)

// Service owns deploy orchestration and runtime server lifecycle.
type Service struct {
	logger               Logger
	newControlPlane      controlPlaneFactory
	newDockerClient      func(logger Logger) dockerClient
	resolveGitCommit     func(ctx context.Context) (string, error)
	makeTempDir          func() (string, error)
	removeAll            func(path string) error
	cloneFromPrepare     func(ctx context.Context, prepare tooltemplate.PrepareResponse, destinationDir string) error
	writeEnv             func(appDir, name, description string) error
	templateRepoValue    func() string
	templateRefValue     func() string
	dockerRegistryValue  func() string
	registryOnlyValue    func() string
	controlPlaneURLValue func() string
}

func NewService() *Service {
	return &Service{
		logger:          logging.New(),
		newControlPlane: newControlPlaneClient,
		newDockerClient: func(logger Logger) dockerClient {
			return docker.NewAdapter(logger, nil)
		},
		resolveGitCommit: resolveGitCommit,
		makeTempDir: func() (string, error) {
			return os.MkdirTemp("", "saki-template-*")
		},
		removeAll:            os.RemoveAll,
		cloneFromPrepare:     tooltemplate.CloneFromPrepare,
		writeEnv:             tooltemplate.WriteEnv,
		templateRepoValue:    func() string { return os.Getenv(templateRepoEnv) },
		templateRefValue:     func() string { return os.Getenv(templateRefEnv) },
		dockerRegistryValue:  func() string { return os.Getenv(dockerRegistryEnv) },
		registryOnlyValue:    func() string { return os.Getenv(registryOnlyEnv) },
		controlPlaneURLValue: func() string { return os.Getenv(controlPlaneURLEnv) },
	}
}

func (s *Service) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

// DeployApp executes the v1 deploy flow and returns normalized output payload.
func (s *Service) DeployApp(ctx context.Context, in contracts.DeployAppInput) (contracts.DeployAppOutput, error) {
	var zero contracts.DeployAppOutput

	if err := in.Validate(); err != nil {
		return zero, apperrors.Wrap(apperrors.CodeInvalidInput, "validate deploy input", err)
	}

	envControlPlaneURL := ""
	if s.controlPlaneURLValue != nil {
		envControlPlaneURL = s.controlPlaneURLValue()
	}
	controlPlaneURL, err := resolveControlPlaneURL(in.SakiControlPlaneURL, envControlPlaneURL)
	if err != nil {
		return zero, err
	}

	cp, err := s.newControlPlane(controlPlaneURL)
	if err != nil {
		return zero, err
	}

	commit, err := s.resolveGitCommit(ctx)
	if err != nil {
		return zero, err
	}

	prepareRes, err := cp.PrepareApp(ctx, controlplane.PrepareAppRequest{
		Name:      in.Name,
		GitCommit: commit,
	})
	if err != nil {
		return zero, err
	}

	imageRepository := resolveImageRepository(
		prepareRes.Repository,
		resolveDockerRegistry(envValue(s.dockerRegistryValue)),
	)
	image, err := buildImageName(imageRepository, prepareRes.RequiredTag)
	if err != nil {
		return zero, err
	}

	templateRepository := resolveTemplateRepository(prepareRes.TemplateRepository, s.templateRepoValue())
	templateRef := firstNonEmpty(prepareRes.TemplateRef, s.templateRefValue())

	workDir, err := s.makeTempDir()
	if err != nil {
		return zero, apperrors.Wrap(apperrors.CodeInternal, "create temp workdir", err)
	}
	defer func() {
		if rmErr := s.removeAll(workDir); rmErr != nil {
			s.logger.Error("failed to clean temp workdir", map[string]any{
				"error":   rmErr.Error(),
				"workdir": workDir,
			})
		}
	}()

	if err := s.cloneFromPrepare(ctx, tooltemplate.PrepareResponse{
		TemplateRepository: templateRepository,
		TemplateRef:        templateRef,
	}, workDir); err != nil {
		return zero, err
	}

	if err := s.writeEnv(workDir, in.Name, in.Description); err != nil {
		return zero, err
	}

	dockerClient := s.newDockerClient(s.logger)
	if err := dockerClient.Build(ctx, workDir, image); err != nil {
		return zero, err
	}
	if err := dockerClient.Push(ctx, image); err != nil {
		return zero, err
	}

	if envEnabled(envValue(s.registryOnlyValue)) {
		return contracts.DeployAppOutput{
			Image:  image,
			Status: "pushed",
		}, nil
	}

	deployRes, err := cp.DeployApp(ctx, controlplane.DeployAppRequest{
		Name:        in.Name,
		Description: in.Description,
		Image:       image,
	})
	if err != nil {
		return zero, err
	}

	return contracts.DeployAppOutput{
		AppID:        deployRes.AppID,
		DeploymentID: deployRes.DeploymentID,
		Image:        image,
		URL:          deployRes.URL,
		Status:       deployRes.Status,
	}, nil
}

func newControlPlaneClient(controlPlaneURL string) (controlPlaneClient, error) {
	return controlplane.NewClient(controlPlaneURL)
}

func resolveGitCommit(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", apperrors.Wrap(apperrors.CodeConfig, "resolve git commit", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output))))
	}

	commit := strings.TrimSpace(string(output))
	if commit == "" {
		return "", apperrors.New(apperrors.CodeConfig, "resolve git commit", "git commit hash is empty")
	}

	return commit, nil
}

func buildImageName(repository, requiredTag string) (string, error) {
	repo := strings.TrimSpace(repository)
	tag := strings.TrimSpace(requiredTag)

	if repo == "" {
		return "", apperrors.New(apperrors.CodeControlPlane, "prepare app", "repository is empty")
	}
	if tag == "" {
		return "", apperrors.New(apperrors.CodeControlPlane, "prepare app", "required tag is empty")
	}

	return repo + ":" + tag, nil
}

func resolveDockerRegistry(envRegistry string) string {
	return firstNonEmpty(envRegistry, defaultDockerRegistry)
}

func resolveImageRepository(prepareRepository, registry string) string {
	repository := strings.TrimSpace(prepareRepository)
	normalizedRegistry := normalizeRegistryForImage(registry)

	if repository == "" || normalizedRegistry == "" {
		return repository
	}

	hasHost := false
	if strings.Contains(repository, "://") {
		parts := strings.SplitN(repository, "://", 2)
		repository = parts[1]
		hasHost = true
	}

	if !hasHost {
		firstSegment := repository
		if slash := strings.IndexByte(firstSegment, '/'); slash >= 0 {
			firstSegment = firstSegment[:slash]
		}
		hasHost = firstSegment == "localhost" || strings.Contains(firstSegment, ".") || strings.Contains(firstSegment, ":")
	}

	if hasHost {
		if slash := strings.IndexByte(repository, '/'); slash >= 0 {
			return normalizedRegistry + "/" + repository[slash+1:]
		}
		return normalizedRegistry + "/" + repository
	}

	return normalizedRegistry + "/" + repository
}

func normalizeRegistryForImage(registry string) string {
	value := strings.TrimSpace(registry)
	if value == "" {
		return ""
	}

	if strings.Contains(value, "://") {
		parts := strings.SplitN(value, "://", 2)
		value = parts[1]
	}

	value = strings.TrimRight(value, "/")
	value = strings.TrimSuffix(value, "/v2")
	return strings.TrimRight(value, "/")
}

func resolveTemplateRepository(prepareRepository, envRepository string) string {
	return firstNonEmpty(prepareRepository, envRepository, defaultTemplateRepository)
}

func resolveControlPlaneURL(inputURL, envURL string) (string, error) {
	if url := firstNonEmpty(inputURL, envURL); url != "" {
		return url, nil
	}

	return "", apperrors.New(apperrors.CodeInvalidInput, "resolve control plane URL", "saki_control_plane_url is required (or set SAKI_CONTROL_PLANE_URL)")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func envValue(read func() string) string {
	if read == nil {
		return ""
	}
	return read()
}

func envEnabled(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.EqualFold(trimmed, "1") || strings.EqualFold(trimmed, "true")
}
