package minimal

import (
	"net/http"

	"github.com/MikelCalvo/go-metin2-server/internal/ops"
)

// RegisterGamedMigrationQuarantineExportOps registers the loopback-only
// read-only migration preflight surface plus the tip-0015 migration-shaped
// export/quarantine pairs. It is the single owner shared by cmd/gamed so those
// routes cannot drift. File-store backup/restore remains under
// RegisterGamedFileStorePersistenceOps; quest mutation, login-ticket
// issued-before triage, spawn/content-bundle, and world introspection stay at
// the call site.
func RegisterGamedMigrationQuarantineExportOps(mux *http.ServeMux, runtime *gameRuntime) *http.ServeMux {
	if mux == nil {
		mux = ops.NewPprofMux("gamed")
	}
	if runtime == nil {
		return mux
	}

	mux = ops.RegisterLocalMigrationStatusEndpoint(mux, runtime.MigrationStatus)
	mux = ops.RegisterLocalMigrationCatalogEndpoint(mux, runtime.MigrationCatalogSummary)
	mux = ops.RegisterLocalMigrationPlanEndpoint(mux, runtime.MigrationPlanToVersion)
	mux = ops.RegisterLocalMigrationLedgerSnapshotEndpoint(mux, runtime.MigrationLedgerSnapshot)
	mux = ops.RegisterLocalMigrationLedgerSnapshotPlanEndpoint(mux, runtime.MigrationPlanFromLedgerSnapshot)
	mux = ops.RegisterLocalAccountCharacterRosterExportEndpoint(mux, runtime.ExportAccountCharacterRoster)
	mux = ops.RegisterLocalAccountCharacterRosterQuarantineEndpoint(mux)
	mux = ops.RegisterLocalCharacterItemStateExportEndpoint(mux, runtime.ExportCharacterItemState)
	mux = ops.RegisterLocalCharacterItemStateQuarantineEndpoint(mux)
	mux = ops.RegisterLocalCharacterPointStateExportEndpoint(mux, runtime.ExportCharacterPointState)
	mux = ops.RegisterLocalCharacterPointStateQuarantineEndpoint(mux)
	mux = ops.RegisterLocalCharacterMyShopUnitPricesExportEndpoint(mux, runtime.ExportCharacterMyShopUnitPrices)
	mux = ops.RegisterLocalCharacterMyShopUnitPricesQuarantineEndpoint(mux)
	mux = ops.RegisterLocalAuthLoginTicketHandoffExportEndpoint(mux, runtime.ExportAuthLoginTicketHandoff)
	mux = ops.RegisterLocalAuthLoginTicketHandoffQuarantineEndpoint(mux)
	mux = ops.RegisterLocalCharacterQuestStateExportEndpoint(mux, runtime.ExportCharacterQuestState)
	mux = ops.RegisterLocalCharacterQuestStateQuarantineEndpoint(mux)
	mux = ops.RegisterLocalCharacterSafeboxStateExportEndpoint(mux, runtime.ExportCharacterSafeboxState)
	mux = ops.RegisterLocalCharacterSafeboxStateQuarantineEndpoint(mux)
	mux = ops.RegisterLocalItemTemplateStateExportEndpoint(mux, runtime.ExportItemTemplateState)
	mux = ops.RegisterLocalItemTemplateStateQuarantineEndpoint(mux)
	mux = ops.RegisterLocalStaticActorContentStateExportEndpoint(mux, runtime.ExportStaticActorContentState)
	mux = ops.RegisterLocalStaticActorContentStateQuarantineEndpoint(mux)
	mux = ops.RegisterLocalBootstrapGroundItemStateExportEndpoint(mux, runtime.ExportBootstrapGroundItemState)
	mux = ops.RegisterLocalBootstrapGroundItemStateQuarantineEndpoint(mux)
	return mux
}
