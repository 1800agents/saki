package app

import (
	"context"
	"fmt"

	"github.com/1800agents/saki/tools/internal/apperrors"
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

	logger.Info("tool starting", map[string]any{
		"mode": cfg.Mode,
		"addr": cfg.Addr,
	})
	if err := service.Run(ctx); err != nil && err != context.Canceled {
		wrapped := apperrors.Wrap(apperrors.CodeInternal, "run service", err)
		logger.Error("tool stopped with error", map[string]any{
			"code":  apperrors.CodeOf(wrapped),
			"error": wrapped.Error(),
		})
		return wrapped
	}
	return nil
}
