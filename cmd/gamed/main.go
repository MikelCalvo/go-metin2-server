package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/MikelCalvo/go-metin2-server/internal/buildinfo"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
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
		func(name string, mapIndex uint32, x int32, y int32) (any, bool) {
			result, ok := gameRuntime.TransferCharacter(name, mapIndex, x, y)
			if !ok {
				return nil, false
			}
			return result, true
		},
		func() any { return gameRuntime.ConnectedCharacters() },
		func() any { return gameRuntime.CharacterVisibility() },
		func() any { return gameRuntime.MapOccupancy() },
	)
	opsHandler = ops.RegisterLocalRuntimeConfigEndpoint(
		opsHandler,
		func() any { return gameRuntime.RuntimeConfigSnapshot() },
	)
	opsHandler = ops.RegisterLocalInteractionVisibilityEndpoint(
		opsHandler,
		func() any { return gameRuntime.InteractionVisibility() },
	)
	opsHandler = ops.RegisterLocalStaticActorEndpoints(
		opsHandler,
		func() any { return gameRuntime.StaticActors() },
		func(name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string) (any, bool) {
			actor, ok := gameRuntime.RegisterStaticActorWithInteraction(name, mapIndex, x, y, raceNum, interactionKind, interactionRef)
			if !ok {
				return nil, false
			}
			return actor, true
		},
	)
	opsHandler = ops.RegisterLocalStaticActorUpdateEndpoint(
		opsHandler,
		func(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string) (any, bool) {
			actor, ok := gameRuntime.UpdateStaticActorWithInteraction(entityID, name, mapIndex, x, y, raceNum, interactionKind, interactionRef)
			if !ok {
				return nil, false
			}
			return actor, true
		},
	)
	opsHandler = ops.RegisterLocalStaticActorDeleteEndpoint(
		opsHandler,
		func(entityID uint64) (any, bool) {
			actor, ok := gameRuntime.RemoveStaticActor(entityID)
			if !ok {
				return nil, false
			}
			return actor, true
		},
	)
	opsHandler = ops.RegisterLocalInteractionDefinitionEndpoints(
		opsHandler,
		func() any { return gameRuntime.InteractionDefinitions() },
		func(kind string, ref string, text string) (any, int) {
			definition, err := gameRuntime.CreateInteractionDefinition(kind, ref, text)
			if err == nil {
				return definition, http.StatusOK
			}
			switch {
			case errors.Is(err, minimal.ErrInteractionDefinitionExists):
				return nil, http.StatusConflict
			case errors.Is(err, interactionstore.ErrInvalidSnapshot):
				return nil, http.StatusBadRequest
			default:
				return nil, http.StatusInternalServerError
			}
		},
	)
	opsHandler = ops.RegisterLocalInteractionDefinitionUpdateEndpoint(
		opsHandler,
		func(kind string, ref string, text string) (any, int) {
			definition, err := gameRuntime.UpsertInteractionDefinition(kind, ref, text)
			if err == nil {
				return definition, http.StatusOK
			}
			switch {
			case errors.Is(err, interactionstore.ErrInvalidSnapshot):
				return nil, http.StatusBadRequest
			default:
				return nil, http.StatusInternalServerError
			}
		},
	)
	opsHandler = ops.RegisterLocalInteractionDefinitionDeleteEndpoint(
		opsHandler,
		func(kind string, ref string) (any, int) {
			definition, err := gameRuntime.RemoveInteractionDefinition(kind, ref)
			if err == nil {
				return definition, http.StatusOK
			}
			switch {
			case errors.Is(err, minimal.ErrInteractionDefinitionNotFound):
				return nil, http.StatusNotFound
			case errors.Is(err, minimal.ErrInteractionDefinitionReferenced):
				return nil, http.StatusConflict
			case errors.Is(err, interactionstore.ErrInvalidSnapshot):
				return nil, http.StatusBadRequest
			default:
				return nil, http.StatusInternalServerError
			}
		},
	)
	if err := service.RunWithOpsHandler(ctx, cfg, logger, gameRuntime.SessionFactory(), opsHandler); err != nil {
		logger.Error("service stopped with error", "err", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
