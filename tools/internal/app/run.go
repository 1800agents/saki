package app

import (
	"context"
	"fmt"

	"github.com/1800agents/saki/tools/internal/config"
	"github.com/1800agents/saki/tools/internal/logging"
	"github.com/1800agents/saki/tools/internal/tool"
)

func Run(ctx context.Context, args []string) error {
	cfg := config.Load()
	logger := logging.New()
	service := tool.NewService()

	if len(args) > 0 && args[0] == "version" {
		fmt.Println("saki-tools dev")
		return nil
	}

	logger.Printf("starting in %s mode on %s", cfg.Mode, cfg.Addr)
	if err := service.Run(ctx); err != nil && err != context.Canceled {
		return err
	}
	return nil
}
