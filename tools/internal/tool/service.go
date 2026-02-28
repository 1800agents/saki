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
	templateRepoEnv = "SAKI_TEMPLATE_REPOSITORY"
	templateRefEnv  = "SAKI_TEMPLATE_REF"
	tokenUser       = "token"
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
	Login(ctx context.Context, registry, username, password string) error
	Build(ctx context.Context, workDir, image string) error
	Push(ctx context.Context, image string) error
}

type controlPlaneFactory func(controlPlaneURL string) (controlPlaneClient, error)

// Service owns deploy orchestration and runtime server lifecycle.
type Service struct {
	logger            Logger
	newControlPlane   controlPlaneFactory
	newDockerClient   func(logger Logger) dockerClient
	resolveGitCommit  func(ctx context.Context) (string, error)
	makeTempDir       func() (string, error)
	removeAll         func(path string) error
	cloneFromPrepare  func(ctx context.Context, prepare tooltemplate.PrepareResponse, destinationDir string) error
	writeEnv          func(appDir, name, description string) error
	templateRepoValue func() string
	templateRefValue  func() string
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
		removeAll:         os.RemoveAll,
		cloneFromPrepare:  tooltemplate.CloneFromPrepare,
		writeEnv:          tooltemplate.WriteEnv,
		templateRepoValue: func() string { return os.Getenv(templateRepoEnv) },
		templateRefValue:  func() string { return os.Getenv(templateRefEnv) },
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

	cp, err := s.newControlPlane(in.SakiControlPlaneURL)
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

	image, err := buildImageName(prepareRes.Repository, prepareRes.RequiredTag)
	if err != nil {
		return zero, err
	}

	templateRepository := strings.TrimSpace(prepareRes.TemplateRepository)
	if templateRepository == "" {
		templateRepository = strings.TrimSpace(s.templateRepoValue())
	}
	templateRef := strings.TrimSpace(prepareRes.TemplateRef)
	if templateRef == "" {
		templateRef = strings.TrimSpace(s.templateRefValue())
	}

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
	if err := dockerClient.Login(ctx, registryHost(prepareRes.Repository), tokenUser, prepareRes.PushToken); err != nil {
		return zero, err
	}
	if err := dockerClient.Build(ctx, workDir, image); err != nil {
		return zero, err
	}
	if err := dockerClient.Push(ctx, image); err != nil {
		return zero, err
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

func registryHost(repository string) string {
	repo := strings.TrimSpace(repository)
	if repo == "" {
		return ""
	}
	if strings.Contains(repo, "://") {
		parts := strings.SplitN(repo, "://", 2)
		repo = parts[1]
	}

	if slash := strings.IndexByte(repo, '/'); slash >= 0 {
		return repo[:slash]
	}
	return repo
}
