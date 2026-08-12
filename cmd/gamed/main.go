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
	"time"

	"github.com/MikelCalvo/go-metin2-server/internal/buildinfo"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	contentbundle "github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/minimal"
	"github.com/MikelCalvo/go-metin2-server/internal/ops"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
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

	cfg := config.LoadService("gamed", "127.0.0.1:6060", ":13000", "127.0.0.1")
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
	opsHandler = ops.RegisterLocalAccountStoreValidateEndpoint(
		opsHandler,
		func() (any, error) { return gameRuntime.ValidateAccountStore() },
	)
	opsHandler = ops.RegisterLocalAccountStoreCrashTempCleanupEndpoint(
		opsHandler,
		func() (any, error) { return gameRuntime.CleanupAccountStoreCrashTempFiles() },
	)
	opsHandler = ops.RegisterLocalLoginTicketStoreValidateEndpoint(
		opsHandler,
		func() (any, error) { return gameRuntime.ValidateLoginTicketStore() },
	)
	opsHandler = ops.RegisterLocalLoginTicketStoreCrashTempCleanupEndpoint(
		opsHandler,
		func() (any, error) { return gameRuntime.CleanupLoginTicketStoreCrashTempFiles() },
	)
	opsHandler = ops.RegisterLocalLoginTicketStoreIssuedBeforePreviewEndpoint(
		opsHandler,
		func(issuedBefore time.Time) (any, error) {
			return gameRuntime.PreviewLoginTicketStoreIssuedBefore(issuedBefore)
		},
	)
	opsHandler = ops.RegisterLocalLoginTicketStoreIssuedBeforeCleanupEndpoint(
		opsHandler,
		func(issuedBefore time.Time) (any, error) {
			return gameRuntime.CleanupLoginTicketStoreIssuedBefore(issuedBefore)
		},
	)
	opsHandler = ops.RegisterLocalItemTemplateStoreValidateEndpoint(
		opsHandler,
		func() (any, error) { return gameRuntime.ValidateItemTemplateStore() },
	)
	opsHandler = ops.RegisterLocalItemTemplateStoreCrashTempCleanupEndpoint(
		opsHandler,
		func() (any, error) { return gameRuntime.CleanupItemTemplateStoreCrashTempFiles() },
	)
	opsHandler = ops.RegisterLocalStaticActorStoreValidateEndpoint(
		opsHandler,
		func() (any, error) { return gameRuntime.ValidateStaticActorStore() },
	)
	opsHandler = ops.RegisterLocalStaticActorStoreCrashTempCleanupEndpoint(
		opsHandler,
		func() (any, error) { return gameRuntime.CleanupStaticActorStoreCrashTempFiles() },
	)
	opsHandler = ops.RegisterLocalInteractionStoreValidateEndpoint(
		opsHandler,
		func() (any, error) { return gameRuntime.ValidateInteractionStore() },
	)
	opsHandler = ops.RegisterLocalInteractionStoreCrashTempCleanupEndpoint(
		opsHandler,
		func() (any, error) { return gameRuntime.CleanupInteractionStoreCrashTempFiles() },
	)
	opsHandler = ops.RegisterLocalQuestStateStoreValidateEndpoint(
		opsHandler,
		func() (any, error) { return gameRuntime.ValidateQuestStateStore() },
	)
	opsHandler = ops.RegisterLocalQuestStateStoreCrashTempCleanupEndpoint(
		opsHandler,
		func() (any, error) { return gameRuntime.CleanupQuestStateStoreCrashTempFiles() },
	)
	opsHandler = ops.RegisterLocalQuestStateTransitionEndpoint(
		opsHandler,
		func(transition queststate.Transition) (any, error) {
			return gameRuntime.ApplyQuestStateTransition(transition)
		},
	)
	opsHandler = ops.RegisterLocalQuestStateCharacterEndpoint(
		opsHandler,
		func(character string) (any, bool, error) {
			snapshot, ok, err := gameRuntime.QuestState(character)
			if err != nil || !ok {
				return nil, ok, err
			}
			return snapshot, true, nil
		},
	)
	opsHandler = ops.RegisterLocalItemTemplateStoreBackupEndpoint(
		opsHandler,
		func(dstDir string) (any, error) { return gameRuntime.BackupItemTemplateStore(dstDir) },
	)
	opsHandler = ops.RegisterLocalItemTemplateStoreBackupValidateEndpoint(
		opsHandler,
		func(srcDir string) (any, error) { return gameRuntime.ValidateItemTemplateStoreBackup(srcDir) },
	)
	opsHandler = ops.RegisterLocalItemTemplateStoreRestoreEndpoint(
		opsHandler,
		func(srcDir string) (any, error) { return gameRuntime.RestoreItemTemplateStore(srcDir) },
	)
	opsHandler = ops.RegisterLocalAccountStoreBackupEndpoint(
		opsHandler,
		func(dstDir string) (any, error) { return gameRuntime.BackupAccountStore(dstDir) },
	)
	opsHandler = ops.RegisterLocalAccountStoreBackupValidateEndpoint(
		opsHandler,
		func(srcDir string) (any, error) { return gameRuntime.ValidateAccountStoreBackup(srcDir) },
	)
	opsHandler = ops.RegisterLocalAccountStoreRestoreEndpoint(
		opsHandler,
		func(srcDir string) (any, error) { return gameRuntime.RestoreAccountStore(srcDir) },
	)
	opsHandler = ops.RegisterLocalRuntimeConfigEndpoint(
		opsHandler,
		func() any { return gameRuntime.RuntimeConfigSnapshot() },
	)
	opsHandler = ops.RegisterLocalPersistenceStatusEndpoint(
		opsHandler,
		func() any { return gameRuntime.PersistenceStatus() },
	)
	opsHandler = ops.RegisterLocalMigrationStatusEndpoint(
		opsHandler,
		gameRuntime.MigrationStatus,
	)
	opsHandler = ops.RegisterLocalMigrationPlanEndpoint(
		opsHandler,
		gameRuntime.MigrationPlanToVersion,
	)
	opsHandler = ops.RegisterLocalMigrationLedgerSnapshotPlanEndpoint(
		opsHandler,
		gameRuntime.MigrationPlanFromLedgerSnapshot,
	)
	opsHandler = ops.RegisterLocalAccountCharacterRosterExportEndpoint(
		opsHandler,
		gameRuntime.ExportAccountCharacterRoster,
	)
	opsHandler = ops.RegisterLocalCharacterItemStateExportEndpoint(
		opsHandler,
		gameRuntime.ExportCharacterItemState,
	)
	opsHandler = ops.RegisterLocalCharacterQuestStateExportEndpoint(
		opsHandler,
		gameRuntime.ExportCharacterQuestState,
	)
	opsHandler = ops.RegisterLocalItemTemplateStateExportEndpoint(
		opsHandler,
		gameRuntime.ExportItemTemplateState,
	)
	opsHandler = ops.RegisterLocalStaticActorRespawnsEndpoint(
		opsHandler,
		func() any { return gameRuntime.StaticActorRespawns() },
	)
	opsHandler = ops.RegisterLocalStaticActorRespawnEndpoint(
		opsHandler,
		func(entityID uint64) (any, bool) {
			respawn, ok := gameRuntime.StaticActorRespawn(entityID)
			if !ok {
				return nil, false
			}
			return respawn, true
		},
	)
	opsHandler = ops.RegisterLocalSpawnGroupsEndpoint(
		opsHandler,
		func() any { return gameRuntime.SpawnGroups() },
	)
	opsHandler = ops.RegisterLocalSpawnGroupEndpoint(
		opsHandler,
		func(entityID uint64) (any, bool) {
			snapshot, ok := gameRuntime.SpawnGroup(entityID)
			if !ok {
				return nil, false
			}
			return snapshot, true
		},
	)
	opsHandler = ops.RegisterLocalSpawnGroupByRefEndpoint(
		opsHandler,
		func(ref string) (any, bool) {
			snapshot, ok := gameRuntime.SpawnGroupByRef(ref)
			if !ok {
				return nil, false
			}
			return snapshot, true
		},
	)
	opsHandler = ops.RegisterLocalSpawnGroupLeashEndpoint(
		opsHandler,
		func(entityID uint64, radius int32) (any, bool) {
			snapshot, ok := gameRuntime.SpawnGroupLeash(entityID, radius)
			if !ok {
				return nil, false
			}
			return snapshot, true
		},
	)
	opsHandler = ops.RegisterLocalGroundItemsEndpoint(
		opsHandler,
		func() any { return gameRuntime.GroundItems() },
	)
	opsHandler = ops.RegisterLocalGroundItemEndpoint(
		opsHandler,
		func(vid uint32) (any, bool) {
			snapshot, ok := gameRuntime.GroundItem(vid)
			if !ok {
				return nil, false
			}
			return snapshot, true
		},
	)
	opsHandler = ops.RegisterLocalMapOccupancyEndpoint(
		opsHandler,
		func(mapIndex uint32) (any, bool) {
			snapshot, ok := gameRuntime.MapOccupancySnapshot(mapIndex)
			if !ok {
				return nil, false
			}
			return snapshot, true
		},
	)
	opsHandler = ops.RegisterLocalMapStaticActorsEndpoint(
		opsHandler,
		func(mapIndex uint32) (any, bool) {
			actors, ok := gameRuntime.StaticActorsForMap(mapIndex)
			if !ok {
				return nil, false
			}
			return actors, true
		},
	)
	opsHandler = ops.RegisterLocalMapSpawnGroupsEndpoint(
		opsHandler,
		func(mapIndex uint32) (any, bool) {
			groups, ok := gameRuntime.SpawnGroupsForMap(mapIndex)
			if !ok {
				return nil, false
			}
			return groups, true
		},
	)
	opsHandler = ops.RegisterLocalMapStaticActorRespawnsEndpoint(
		opsHandler,
		func(mapIndex uint32) (any, bool) {
			respawns, ok := gameRuntime.StaticActorRespawnsForMap(mapIndex)
			if !ok {
				return nil, false
			}
			return respawns, true
		},
	)
	opsHandler = ops.RegisterLocalMapCombatTargetsEndpoint(
		opsHandler,
		func(mapIndex uint32) (any, bool) {
			targets, ok := gameRuntime.CombatTargetSnapshotsForMap(mapIndex)
			if !ok {
				return nil, false
			}
			return targets, true
		},
	)
	opsHandler = ops.RegisterLocalConnectedCharacterEndpoint(
		opsHandler,
		func(name string) (any, bool) {
			snapshot, ok := gameRuntime.ConnectedCharacterSnapshot(name)
			if !ok {
				return nil, false
			}
			return snapshot, true
		},
	)
	opsHandler = ops.RegisterLocalInventoryEndpoint(
		opsHandler,
		func(name string) (any, bool) {
			snapshot, ok := gameRuntime.InventorySnapshot(name)
			if !ok {
				return nil, false
			}
			return snapshot, true
		},
	)
	opsHandler = ops.RegisterLocalEquipmentEndpoint(
		opsHandler,
		func(name string) (any, bool) {
			snapshot, ok := gameRuntime.EquipmentSnapshot(name)
			if !ok {
				return nil, false
			}
			return snapshot, true
		},
	)
	opsHandler = ops.RegisterLocalQuickslotsEndpoint(
		opsHandler,
		func(name string) (any, bool) {
			snapshot, ok := gameRuntime.QuickslotsSnapshot(name)
			if !ok {
				return nil, false
			}
			return snapshot, true
		},
	)
	opsHandler = ops.RegisterLocalCurrencyEndpoint(
		opsHandler,
		func(name string) (any, bool) {
			snapshot, ok := gameRuntime.CurrencySnapshot(name)
			if !ok {
				return nil, false
			}
			return snapshot, true
		},
	)
	opsHandler = ops.RegisterLocalCombatTargetEndpoint(
		opsHandler,
		func(name string) (any, bool) {
			snapshot, ok := gameRuntime.CombatTargetSnapshot(name)
			if !ok {
				return nil, false
			}
			return snapshot, true
		},
	)
	opsHandler = ops.RegisterLocalCharacterVisibilityEndpoint(
		opsHandler,
		func(name string) (any, bool) {
			snapshot, ok := gameRuntime.CharacterVisibilitySnapshot(name)
			if !ok {
				return nil, false
			}
			return snapshot, true
		},
	)
	opsHandler = ops.RegisterLocalCombatTargetsEndpoint(
		opsHandler,
		func() any { return gameRuntime.CombatTargetSnapshots() },
	)
	opsHandler = ops.RegisterLocalInteractionVisibilityEndpoint(
		opsHandler,
		func() any { return gameRuntime.InteractionVisibility() },
	)
	opsHandler = ops.RegisterLocalCharacterInteractionVisibilityEndpoint(
		opsHandler,
		func(name string) (any, bool) {
			snapshot, ok := gameRuntime.InteractionVisibilitySnapshot(name)
			if !ok {
				return nil, false
			}
			return snapshot, true
		},
	)
	opsHandler = ops.RegisterLocalStaticActorEndpoints(
		opsHandler,
		func() any { return gameRuntime.StaticActors() },
		func(name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string, combatProfile string) (any, bool) {
			actor, ok := gameRuntime.RegisterStaticActorWithInteractionAndCombatProfile(name, mapIndex, x, y, raceNum, interactionKind, interactionRef, combatProfile)
			if !ok {
				return nil, false
			}
			return actor, true
		},
	)
	opsHandler = ops.RegisterLocalStaticActorEndpoint(
		opsHandler,
		func(entityID uint64) (any, bool) {
			actor, ok := gameRuntime.StaticActor(entityID)
			if !ok {
				return nil, false
			}
			return actor, true
		},
	)
	opsHandler = ops.RegisterLocalStaticActorUpdateEndpoint(
		opsHandler,
		func(entityID uint64, name string, mapIndex uint32, x int32, y int32, raceNum uint32, interactionKind string, interactionRef string, combatProfile string) (any, bool) {
			actor, ok := gameRuntime.UpdateStaticActorWithInteractionAndCombatProfile(entityID, name, mapIndex, x, y, raceNum, interactionKind, interactionRef, combatProfile)
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
	opsHandler = ops.RegisterLocalStaticActorCombatProfileEndpoint(opsHandler)
	opsHandler = ops.RegisterLocalInteractionDefinitionEndpoints(
		opsHandler,
		func() any { return gameRuntime.InteractionDefinitions() },
		func(definition interactionstore.Definition) (any, int) {
			definition, err := gameRuntime.CreateInteractionDefinition(definition)
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
	opsHandler = ops.RegisterLocalInteractionDefinitionLookupEndpoint(
		opsHandler,
		func(kind string, ref string) (any, int) {
			definition, ok := gameRuntime.InteractionDefinition(kind, ref)
			if !ok {
				return nil, http.StatusNotFound
			}
			return definition, http.StatusOK
		},
	)
	opsHandler = ops.RegisterLocalInteractionDefinitionUpdateEndpoint(
		opsHandler,
		func(definition interactionstore.Definition) (any, int) {
			definition, err := gameRuntime.UpsertInteractionDefinition(definition)
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
	opsHandler = ops.RegisterLocalContentBundleEndpoint(
		opsHandler,
		func() (any, int) {
			bundle, err := gameRuntime.ExportContentBundle()
			if err != nil {
				return nil, http.StatusInternalServerError
			}
			return bundle, http.StatusOK
		},
		func(bundle contentbundle.Bundle) (any, int) {
			imported, err := gameRuntime.ImportContentBundle(bundle)
			if err == nil {
				return imported, http.StatusOK
			}
			switch {
			case errors.Is(err, contentbundle.ErrInvalidBundle):
				return nil, http.StatusBadRequest
			default:
				return nil, http.StatusInternalServerError
			}
		},
	)
	exportContentBundleSummary := func() (any, int) {
		summary, err := gameRuntime.ExportContentBundleSummary()
		if err != nil {
			return nil, http.StatusInternalServerError
		}
		return summary, http.StatusOK
	}
	opsHandler = ops.RegisterLocalContentBundleSummaryEndpoint(
		opsHandler,
		exportContentBundleSummary,
	)
	opsHandler = ops.RegisterLocalContentBundleMapSummaryEndpoint(
		opsHandler,
		exportContentBundleSummary,
	)
	opsHandler = ops.RegisterLocalContentBundleSpawnGroupEndpoint(
		opsHandler,
		exportContentBundleSummary,
	)
	opsHandler = ops.RegisterLocalContentBundleInteractableStaticActorEndpoint(
		opsHandler,
		exportContentBundleSummary,
	)
	opsHandler = ops.RegisterLocalContentBundleInteractionDefinitionEndpoint(
		opsHandler,
		exportContentBundleSummary,
	)
	opsHandler = ops.RegisterLocalContentBundleItemTemplateEndpoint(
		opsHandler,
		exportContentBundleSummary,
	)
	opsHandler = ops.RegisterLocalContentBundleRewardDropEndpoint(
		opsHandler,
		exportContentBundleSummary,
	)
	opsHandler = ops.RegisterLocalContentBundleQuestStateCharacterEndpoint(
		opsHandler,
		exportContentBundleSummary,
	)
	opsHandler = ops.RegisterLocalContentBundleQuestStateQuestEndpoint(
		opsHandler,
		exportContentBundleSummary,
	)
	opsHandler = ops.RegisterLocalContentBundleQuestStateFlagEndpoint(
		opsHandler,
		exportContentBundleSummary,
	)
	opsHandler = ops.RegisterLocalContentBundleShopCatalogEndpoint(
		opsHandler,
		exportContentBundleSummary,
	)
	opsHandler = ops.RegisterLocalContentBundleShopRouteEndpoint(
		opsHandler,
		exportContentBundleSummary,
	)
	opsHandler = ops.RegisterLocalContentBundleWarpDestinationEndpoint(
		opsHandler,
		exportContentBundleSummary,
	)
	opsHandler = ops.RegisterLocalContentBundleWarpRouteEndpoint(
		opsHandler,
		exportContentBundleSummary,
	)
	opsHandler = ops.RegisterLocalContentBundleImportPreviewEndpoint(
		opsHandler,
		func(bundle contentbundle.Bundle) (any, int) {
			preview, err := gameRuntime.PreviewContentBundleImport(bundle)
			if err != nil {
				return nil, http.StatusInternalServerError
			}
			return preview, http.StatusOK
		},
	)
	opsHandler = ops.RegisterLocalContentBundleValidateEndpoint(opsHandler)
	if err := service.RunWithOpsHandler(ctx, cfg, logger, gameRuntime.SessionFactory(), opsHandler); err != nil {
		logger.Error("service stopped with error", "err", err)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
