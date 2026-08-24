package minimal

import (
	"net/http"

	"github.com/MikelCalvo/go-metin2-server/internal/ops"
)

// RegisterGamedFileStorePersistenceOps registers the loopback-only eight-store
// validate / crash-temp cleanup / backup / backup-validate / restore surface
// plus runtime-config and persistence-status. It is the single owner shared by
// cmd/gamed and the hermetic backup-restore drill proof so those routes cannot
// drift. Broader gamed ops (quest mutation, migration, content-bundle, spawn
// groups, login-ticket issued-before triage) stay at the call site.
func RegisterGamedFileStorePersistenceOps(mux *http.ServeMux, runtime *gameRuntime) *http.ServeMux {
	if mux == nil {
		mux = ops.NewPprofMux("gamed")
	}
	if runtime == nil {
		return mux
	}

	mux = ops.RegisterLocalAccountStoreValidateEndpoint(mux, func() (any, error) {
		return runtime.ValidateAccountStore()
	})
	mux = ops.RegisterLocalAccountStoreCrashTempCleanupEndpoint(mux, func() (any, error) {
		return runtime.CleanupAccountStoreCrashTempFiles()
	})
	mux = ops.RegisterLocalLoginTicketStoreValidateEndpoint(mux, func() (any, error) {
		return runtime.ValidateLoginTicketStore()
	})
	mux = ops.RegisterLocalLoginTicketStoreCrashTempCleanupEndpoint(mux, func() (any, error) {
		return runtime.CleanupLoginTicketStoreCrashTempFiles()
	})
	mux = ops.RegisterLocalLoginTicketStoreBackupEndpoint(mux, func(dstDir string) (any, error) {
		return runtime.BackupLoginTicketStore(dstDir)
	})
	mux = ops.RegisterLocalLoginTicketStoreBackupValidateEndpoint(mux, func(srcDir string) (any, error) {
		return runtime.ValidateLoginTicketStoreBackup(srcDir)
	})
	mux = ops.RegisterLocalLoginTicketStoreRestoreEndpoint(mux, func(srcDir string) (any, error) {
		return runtime.RestoreLoginTicketStore(srcDir)
	})
	mux = ops.RegisterLocalItemTemplateStoreValidateEndpoint(mux, func() (any, error) {
		return runtime.ValidateItemTemplateStore()
	})
	mux = ops.RegisterLocalItemTemplateStoreCrashTempCleanupEndpoint(mux, func() (any, error) {
		return runtime.CleanupItemTemplateStoreCrashTempFiles()
	})
	mux = ops.RegisterLocalStaticActorStoreValidateEndpoint(mux, func() (any, error) {
		return runtime.ValidateStaticActorStore()
	})
	mux = ops.RegisterLocalStaticActorStoreCrashTempCleanupEndpoint(mux, func() (any, error) {
		return runtime.CleanupStaticActorStoreCrashTempFiles()
	})
	mux = ops.RegisterLocalStaticActorStoreBackupEndpoint(mux, func(dstDir string) (any, error) {
		return runtime.BackupStaticActorStore(dstDir)
	})
	mux = ops.RegisterLocalStaticActorStoreBackupValidateEndpoint(mux, func(srcDir string) (any, error) {
		return runtime.ValidateStaticActorStoreBackup(srcDir)
	})
	mux = ops.RegisterLocalStaticActorStoreRestoreEndpoint(mux, func(srcDir string) (any, error) {
		return runtime.RestoreStaticActorStore(srcDir)
	})
	mux = ops.RegisterLocalInteractionStoreValidateEndpoint(mux, func() (any, error) {
		return runtime.ValidateInteractionStore()
	})
	mux = ops.RegisterLocalInteractionStoreCrashTempCleanupEndpoint(mux, func() (any, error) {
		return runtime.CleanupInteractionStoreCrashTempFiles()
	})
	mux = ops.RegisterLocalInteractionStoreBackupEndpoint(mux, func(dstDir string) (any, error) {
		return runtime.BackupInteractionStore(dstDir)
	})
	mux = ops.RegisterLocalInteractionStoreBackupValidateEndpoint(mux, func(srcDir string) (any, error) {
		return runtime.ValidateInteractionStoreBackup(srcDir)
	})
	mux = ops.RegisterLocalInteractionStoreRestoreEndpoint(mux, func(srcDir string) (any, error) {
		return runtime.RestoreInteractionStore(srcDir)
	})
	mux = ops.RegisterLocalQuestStateStoreValidateEndpoint(mux, func() (any, error) {
		return runtime.ValidateQuestStateStore()
	})
	mux = ops.RegisterLocalQuestStateStoreCrashTempCleanupEndpoint(mux, func() (any, error) {
		return runtime.CleanupQuestStateStoreCrashTempFiles()
	})
	mux = ops.RegisterLocalGroundItemStoreValidateEndpoint(mux, func() (any, error) {
		return runtime.ValidateGroundItemStore()
	})
	mux = ops.RegisterLocalGroundItemStoreCrashTempCleanupEndpoint(mux, func() (any, error) {
		return runtime.CleanupGroundItemStoreCrashTempFiles()
	})
	mux = ops.RegisterLocalGroundItemStoreBackupEndpoint(mux, func(dstDir string) (any, error) {
		return runtime.BackupGroundItemStore(dstDir)
	})
	mux = ops.RegisterLocalGroundItemStoreBackupValidateEndpoint(mux, func(srcDir string) (any, error) {
		return runtime.ValidateGroundItemStoreBackup(srcDir)
	})
	mux = ops.RegisterLocalGroundItemStoreRestoreEndpoint(mux, func(srcDir string) (any, error) {
		return runtime.RestoreGroundItemStore(srcDir)
	})
	mux = ops.RegisterLocalSafeboxStoreValidateEndpoint(mux, func() (any, error) {
		return runtime.ValidateSafeboxStore()
	})
	mux = ops.RegisterLocalSafeboxStoreCrashTempCleanupEndpoint(mux, func() (any, error) {
		return runtime.CleanupSafeboxStoreCrashTempFiles()
	})
	mux = ops.RegisterLocalSafeboxStoreBackupEndpoint(mux, func(dstDir string) (any, error) {
		return runtime.BackupSafeboxStore(dstDir)
	})
	mux = ops.RegisterLocalSafeboxStoreBackupValidateEndpoint(mux, func(srcDir string) (any, error) {
		return runtime.ValidateSafeboxStoreBackup(srcDir)
	})
	mux = ops.RegisterLocalSafeboxStoreRestoreEndpoint(mux, func(srcDir string) (any, error) {
		return runtime.RestoreSafeboxStore(srcDir)
	})
	mux = ops.RegisterLocalQuestStateStoreBackupEndpoint(mux, func(dstDir string) (any, error) {
		return runtime.BackupQuestStateStore(dstDir)
	})
	mux = ops.RegisterLocalQuestStateStoreBackupValidateEndpoint(mux, func(srcDir string) (any, error) {
		return runtime.ValidateQuestStateStoreBackup(srcDir)
	})
	mux = ops.RegisterLocalQuestStateStoreRestoreEndpoint(mux, func(srcDir string) (any, error) {
		return runtime.RestoreQuestStateStore(srcDir)
	})
	mux = ops.RegisterLocalItemTemplateStoreBackupEndpoint(mux, func(dstDir string) (any, error) {
		return runtime.BackupItemTemplateStore(dstDir)
	})
	mux = ops.RegisterLocalItemTemplateStoreBackupValidateEndpoint(mux, func(srcDir string) (any, error) {
		return runtime.ValidateItemTemplateStoreBackup(srcDir)
	})
	mux = ops.RegisterLocalItemTemplateStoreRestoreEndpoint(mux, func(srcDir string) (any, error) {
		return runtime.RestoreItemTemplateStore(srcDir)
	})
	mux = ops.RegisterLocalAccountStoreBackupEndpoint(mux, func(dstDir string) (any, error) {
		return runtime.BackupAccountStore(dstDir)
	})
	mux = ops.RegisterLocalAccountStoreBackupValidateEndpoint(mux, func(srcDir string) (any, error) {
		return runtime.ValidateAccountStoreBackup(srcDir)
	})
	mux = ops.RegisterLocalAccountStoreRestoreEndpoint(mux, func(srcDir string) (any, error) {
		return runtime.RestoreAccountStore(srcDir)
	})
	mux = ops.RegisterLocalRuntimeConfigEndpoint(mux, func() any {
		return runtime.RuntimeConfigSnapshot()
	})
	mux = ops.RegisterLocalPersistenceStatusEndpoint(mux, func() any {
		return runtime.PersistenceStatus()
	})
	return mux
}
