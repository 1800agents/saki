package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/1800agents/saki/tools/contracts"
	"github.com/1800agents/saki/tools/docker"
	"github.com/1800agents/saki/tools/internal/apperrors"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	toolNameSakiDeployApp        = "saki_deploy_app"
	toolDescriptionSakiDeployApp = "Build and deploy a prepared local app directory. The calling agent must clone/customize the app first, then call this tool for prepare, docker build/push, and control-plane deploy. If any required field is missing, ask follow-up questions in plain language instead of asking for JSON."
	resourceURIWorkflow          = "saki://deploy-workflow"
	resourceNameWorkflow         = "saki_deploy_workflow"
	resourceDescriptionWorkflow  = "Authoritative workflow for saki_deploy_app with clear agent/tool boundaries: agent prepares app source; tool performs build/push/deploy."
)

type Logger interface {
	Info(msg string, fields map[string]any)
	Error(msg string, fields map[string]any)
}

type deployService interface {
	DeployApp(ctx context.Context, in contracts.DeployAppInput) (contracts.DeployAppOutput, error)
}

type Server struct {
	service   deployService
	logger    Logger
	sdkServer *sdkmcp.Server
	transport sdkmcp.Transport
	debug     bool
	rawLog    bool
}

func NewServer(service deployService, logger Logger) *Server {
	debug := envEnabledOrDefault("SAKI_TOOLS_MCP_DEBUG", true)
	rawLog := envEnabled("SAKI_TOOLS_MCP_RAW_LOG")

	sdkServer := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "saki-tools",
		Version: "dev",
	}, nil)

	sdkmcp.AddTool(sdkServer, deployToolDefinition(), func(ctx context.Context, _ *sdkmcp.CallToolRequest, in contracts.DeployAppInput) (*sdkmcp.CallToolResult, contracts.DeployAppOutput, error) {
		in = normalizeDeployInput(in)
		logger.Info("tool call requested", map[string]any{
			"tool": toolNameSakiDeployApp,
		})
		logger.Info("deploy input parsed", map[string]any{
			"name":        in.Name,
			"description": in.Description,
			"app_dir":     in.AppDir,
			"has_url":     strings.TrimSpace(in.SakiControlPlaneURL) != "",
		})

		if missing := missingDeployFields(in, strings.TrimSpace(os.Getenv("SAKI_CONTROL_PLANE_URL")) != ""); len(missing) > 0 {
			missingMessage := missingFieldsMessage(missing)
			logger.Info("deploy input incomplete", map[string]any{
				"missing_fields": missing,
			})
			return nil, contracts.DeployAppOutput{}, fmt.Errorf("%s", missingMessage)
		}

		output, err := service.DeployApp(ctx, in)
		if err != nil {
			logger.Error("deploy failed", deployErrorFields(in, err))
			return nil, contracts.DeployAppOutput{}, formatDeployErrorForMCP(in, err)
		}

		logger.Info("deploy completed", map[string]any{
			"app_id":        output.AppID,
			"deployment_id": output.DeploymentID,
			"status":        output.Status,
			"url":           output.URL,
		})

		payload, err := json.Marshal(output)
		if err != nil {
			logger.Error("failed to marshal deploy output", map[string]any{"error": err.Error()})
			return nil, contracts.DeployAppOutput{}, err
		}

		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: string(payload)}},
		}, output, nil
	})
	sdkServer.AddResource(deployWorkflowResourceDefinition(), deployWorkflowResourceHandler)

	var transport sdkmcp.Transport = &sdkmcp.StdioTransport{}
	if rawLog {
		transport = &sdkmcp.LoggingTransport{Transport: transport, Writer: os.Stderr}
	}

	return &Server{
		service:   service,
		logger:    logger,
		sdkServer: sdkServer,
		transport: transport,
		debug:     debug,
		rawLog:    rawLog,
	}
}

func (s *Server) Serve(ctx context.Context) error {
	s.logger.Info("mcp server started", map[string]any{
		"debug":   s.debug,
		"raw_log": s.rawLog,
	})

	err := s.sdkServer.Run(ctx, s.transport)
	if err == nil {
		s.logger.Info("mcp server stopped", nil)
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) || errors.Is(err, sdkmcp.ErrConnectionClosed) || strings.Contains(err.Error(), "EOF") {
		s.logger.Info("mcp server input closed", map[string]any{"error": err.Error()})
		return nil
	}

	return err
}

func deployToolDefinition() *sdkmcp.Tool {
	return &sdkmcp.Tool{
		Name:        toolNameSakiDeployApp,
		Description: toolDescriptionSakiDeployApp,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"saki_control_plane_url": map[string]any{
					"type":        "string",
					"description": "Tokenized Saki control plane URL. Example: https://saki.internal/api?token=<uuid>.",
					"minLength":   1,
				},
				"name": map[string]any{
					"type":        "string",
					"description": "DNS-safe app name (lowercase letters, numbers, hyphens; max 63 chars). Example: team-dashboard.",
					"pattern":     "^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$",
					"maxLength":   63,
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Short human-readable app purpose (max 300 chars). Example: Internal ops dashboard for on-call rotation.",
					"minLength":   1,
					"maxLength":   300,
				},
				"app_dir": map[string]any{
					"type":        "string",
					"description": "Local directory containing the app source to build (prepared by the calling agent). Example: /workspace/my-app.",
					"minLength":   1,
				},
			},
			"required":             []string{"name", "description", "app_dir"},
			"additionalProperties": false,
		},
	}
}

func normalizeDeployInput(in contracts.DeployAppInput) contracts.DeployAppInput {
	in.SakiControlPlaneURL = strings.TrimSpace(in.SakiControlPlaneURL)
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.AppDir = strings.TrimSpace(in.AppDir)
	return in
}

func missingDeployFields(in contracts.DeployAppInput, hasControlPlaneEnv bool) []string {
	missing := make([]string, 0, 4)
	if in.SakiControlPlaneURL == "" && !hasControlPlaneEnv {
		missing = append(missing, "saki_control_plane_url")
	}
	if in.Name == "" {
		missing = append(missing, "name")
	}
	if in.Description == "" {
		missing = append(missing, "description")
	}
	if in.AppDir == "" {
		missing = append(missing, "app_dir")
	}
	return missing
}

func missingFieldsMessage(fields []string) string {
	fields = append([]string(nil), fields...)
	slices.Sort(fields)
	return fmt.Sprintf(
		"missing required deployment fields: %s. Ask the user for the missing values in plain language and retry saki_deploy_app.",
		strings.Join(fields, ", "),
	)
}

func envEnabled(key string) bool {
	v := strings.TrimSpace(os.Getenv(key))
	return strings.EqualFold(v, "1") || strings.EqualFold(v, "true")
}

func envEnabledOrDefault(key string, defaultValue bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return defaultValue
	}
	return strings.EqualFold(v, "1") || strings.EqualFold(v, "true")
}

func deployWorkflowResourceDefinition() *sdkmcp.Resource {
	return &sdkmcp.Resource{
		URI:         resourceURIWorkflow,
		Name:        resourceNameWorkflow,
		Title:       "Saki Deploy Workflow",
		Description: resourceDescriptionWorkflow,
		MIMEType:    "text/markdown",
	}
}

func deployWorkflowResourceHandler(_ context.Context, req *sdkmcp.ReadResourceRequest) (*sdkmcp.ReadResourceResult, error) {
	if req == nil || req.Params == nil || req.Params.URI != resourceURIWorkflow {
		uri := ""
		if req != nil && req.Params != nil {
			uri = req.Params.URI
		}
		return nil, sdkmcp.ResourceNotFoundError(uri)
	}

	return &sdkmcp.ReadResourceResult{
		Contents: []*sdkmcp.ResourceContents{
			{
				URI:      resourceURIWorkflow,
				MIMEType: "text/markdown",
				Text:     deployWorkflowDocument(),
			},
		},
	}, nil
}

func deployWorkflowDocument() string {
	lines := []string{
		"# Saki Deploy Workflow (for agents calling MCP)",
		"",
		"Use this workflow when handling app deployment requests with saki_deploy_app.",
		"",
		"## Required inputs",
		"- name: DNS-safe app name (lowercase letters, numbers, hyphens; max 63 chars).",
		"- description: short purpose text (max 300 chars).",
		"- app_dir: local app directory that was prepared by the calling agent.",
		"- saki_control_plane_url: tokenized URL; may be omitted only if SAKI_CONTROL_PLANE_URL is set in the tool environment.",
		"",
		"If any required field is missing, ask follow-up questions in plain language and then retry the tool call.",
		"",
		"## Agent-side preparation steps (before tool call)",
		"1. Clone the template repository URL: https://github.com/1800agents/saki-app-template.",
		"2. Customize the app with the user (files, dependencies, behavior).",
		"3. Choose the local directory to build, then call saki_deploy_app with app_dir set to that path.",
		"",
		"## Tool-side execution steps (inside saki_deploy_app)",
		"1. Validate inputs.",
		"2. Resolve current git commit (git rev-parse HEAD).",
		"3. Call control plane prepare endpoint (POST /apps/prepare) with app name and git commit.",
		"4. Build image name from prepare repository + required_tag (with SAKI_DOCKER_REGISTRY override support).",
		"5. Run docker build -t <repository>:<required_tag> . in app_dir.",
		"6. Run docker push <repository>:<required_tag>.",
		"7. Create/update deployment via control plane (POST /apps with {name, description, image}), unless registry-only mode is enabled.",
		"8. Return deployment output (app_id, deployment_id, image, url, status).",
		"",
		"## Responsibility boundary",
		"- Agent responsibility: clone template and prepare app source.",
		"- Tool responsibility: build, push, and deploy from app_dir.",
		"",
		"## Debugging notes",
		"- On docker failures, MCP error responses include app_dir, image, command, exit code, and stderr when available.",
	}

	return strings.Join(lines, "\n")
}

func deployErrorFields(in contracts.DeployAppInput, err error) map[string]any {
	fields := map[string]any{
		"error":   err.Error(),
		"code":    apperrors.CodeOf(err),
		"app_dir": in.AppDir,
		"name":    in.Name,
	}

	var dockerErr *docker.CommandError
	if errors.As(err, &dockerErr) {
		fields["docker_op"] = dockerErr.Op
		fields["command"] = dockerErr.Command
		fields["exit_code"] = dockerErr.ExitCode
		fields["stderr"] = dockerErr.Stderr
	}

	return fields
}

func formatDeployErrorForMCP(in contracts.DeployAppInput, err error) error {
	var dockerErr *docker.CommandError
	if errors.As(err, &dockerErr) {
		return fmt.Errorf(
			"docker %s failed for app_dir=%q (app=%q). command=%q exit_code=%d stderr=%q. fix the app source/build context and retry saki_deploy_app: %w",
			dockerErr.Op,
			in.AppDir,
			in.Name,
			dockerErr.Command,
			dockerErr.ExitCode,
			dockerErr.Stderr,
			err,
		)
	}
	return err
}
