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
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	toolNameSakiDeployApp        = "saki_deploy_app"
	toolDescriptionSakiDeployApp = "Deploy a Saki app from conversation-provided app details. If any field is missing, ask follow-up questions in plain language instead of asking for JSON."
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
	debug := envEnabled("SAKI_TOOLS_MCP_DEBUG")
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
			"has_url":     strings.TrimSpace(in.SakiControlPlaneURL) != "",
		})

		if missing := missingDeployFields(in); len(missing) > 0 {
			missingMessage := missingFieldsMessage(missing)
			logger.Info("deploy input incomplete", map[string]any{
				"missing_fields": missing,
			})
			return nil, contracts.DeployAppOutput{}, fmt.Errorf("%s", missingMessage)
		}

		output, err := service.DeployApp(ctx, in)
		if err != nil {
			logger.Error("deploy failed", map[string]any{"error": err.Error()})
			return nil, contracts.DeployAppOutput{}, err
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
			},
			"additionalProperties": false,
		},
	}
}

func normalizeDeployInput(in contracts.DeployAppInput) contracts.DeployAppInput {
	in.SakiControlPlaneURL = strings.TrimSpace(in.SakiControlPlaneURL)
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	return in
}

func missingDeployFields(in contracts.DeployAppInput) []string {
	missing := make([]string, 0, 3)
	if in.SakiControlPlaneURL == "" {
		missing = append(missing, "saki_control_plane_url")
	}
	if in.Name == "" {
		missing = append(missing, "name")
	}
	if in.Description == "" {
		missing = append(missing, "description")
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
