package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/1800agents/saki/tools/internal/logging"
	"github.com/1800agents/saki/tools/internal/mcp"
	"github.com/1800agents/saki/tools/internal/tool"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger := logging.New()
	logger.Info("starting saki-tools MCP stdio server", map[string]any{
		"debug": os.Getenv("SAKI_TOOLS_MCP_DEBUG"),
	})
	service := tool.NewService()
	server := mcp.NewServer(service, logger)

	if err := server.Serve(ctx); err != nil {
		logger.Error("mcp server stopped with error", map[string]any{"error": err.Error()})
		os.Exit(1)
	}
}
