package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/1800agents/saki/tools/contracts"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	toolNameSakiDeployApp        = "saki_deploy_app"
	toolDescriptionSakiDeployApp = "Build and deploy a Saki app from template to the internal control plane."
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
		logger.Info("tool call requested", map[string]any{
			"tool": toolNameSakiDeployApp,
		})
		logger.Info("deploy input parsed", map[string]any{
			"name":        in.Name,
			"description": in.Description,
			"has_url":     strings.TrimSpace(in.SakiControlPlaneURL) != "",
		})

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
				"saki_control_plane_url": map[string]any{"type": "string"},
				"name":                   map[string]any{"type": "string"},
				"description":            map[string]any{"type": "string"},
			},
			"required":             []string{"saki_control_plane_url", "name", "description"},
			"additionalProperties": false,
		},
	}
}

func envEnabled(key string) bool {
	v := strings.TrimSpace(os.Getenv(key))
	return strings.EqualFold(v, "1") || strings.EqualFold(v, "true")
}
