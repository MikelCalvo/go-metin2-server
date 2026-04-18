package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/MikelCalvo/go-metin2-server/internal/buildinfo"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/minimal"
	"github.com/MikelCalvo/go-metin2-server/internal/ops"
	"github.com/MikelCalvo/go-metin2-server/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With(
		"service", "gamed",
		"version", buildinfo.Version,
		"commit", buildinfo.Commit,
		"build_date", buildinfo.BuildDate,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.LoadService("gamed", ":6060", ":13000", "127.0.0.1")
	gameRuntime, err := minimal.NewGameRuntime(cfg)
	if err != nil {
		logger.Error("invalid game runtime configuration", "err", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	opsHandler := ops.NewPprofMuxWithLocalRuntimeIntrospection(
		"gamed",
		gameRuntime.BroadcastNotice,
		gameRuntime.RelocateCharacter,
		func(name string, mapIndex uint32, x int32, y int32) (any, bool) {
			preview, ok := gameRuntime.PreviewRelocation(name, mapIndex, x, y)
			if !ok {
				return nil, false
			}
			return preview, true
		},
		func() any { return gameRuntime.ConnectedCharacters() },
		func() any { return gameRuntime.CharacterVisibility() },
		func() any { return gameRuntime.MapOccupancy() },
	)
	if err := service.RunWithOpsHandler(ctx, cfg, logger, gameRuntime.SessionFactory(), opsHandler); err != nil {
		logger.Error("service stopped with error", "err", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
