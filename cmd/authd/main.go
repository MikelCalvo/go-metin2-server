package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/minimal"
	"github.com/MikelCalvo/go-metin2-server/internal/observability"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
)

func main() {
	logger := observability.NewServiceLogger("authd", os.Stdout)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.LoadService("authd", "127.0.0.1:6061", ":11002", "127.0.0.1")
	authFactory, err := minimal.NewAuthSessionFactoryWithValidatedConfig(cfg)
	if err != nil {
		logger.Error("invalid auth runtime configuration", "err", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := service.Run(ctx, cfg, logger, authFactory); err != nil {
		logger.Error("service stopped with error", "err", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
