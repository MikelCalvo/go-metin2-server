package minimal

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/config"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	itemcatalog "github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/ops"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestRegisterGamedFileStorePersistenceOpsServesStatusAndAccountBackup(t *testing.T) {
	defer worldruntime.DisableDurableGroundItemSyncForTest()()

	root := t.TempDir()
	accountDir := filepath.Join(root, "accounts")
	loginTicketDir := filepath.Join(root, "login-tickets")
	staticActorPath := filepath.Join(root, "static-actors", "static-actors.json")
	interactionPath := filepath.Join(root, "interactions", "interaction-definitions.json")
	itemTemplatePath := filepath.Join(root, "item-templates", "item-templates.json")
	questStatePath := filepath.Join(root, "quest-state", "quest-state.json")
	groundItemPath := filepath.Join(root, "ground-items", "ground-items.json")
	safeboxPath := filepath.Join(root, "safebox", "safebox.json")
	mustMkdirAll(t, accountDir)
	mustMkdirAll(t, loginTicketDir)
	mustMkdirAll(t, filepath.Dir(staticActorPath))
	mustMkdirAll(t, filepath.Dir(interactionPath))
	mustMkdirAll(t, filepath.Dir(itemTemplatePath))
	mustMkdirAll(t, filepath.Dir(questStatePath))
	mustMkdirAll(t, filepath.Dir(groundItemPath))
	mustMkdirAll(t, filepath.Dir(safeboxPath))

	accounts := accountstore.NewFileStore(accountDir)
	if err := accounts.Save(accountstore.Account{
		Login:  "ops-helper",
		Empire: 1,
		Characters: []loginticket.Character{
			peerVisibilityCharacter("OpsHero", 0x01030901, 0x02040901, 1100, 2100, 0, 101, 201),
		},
	}); err != nil {
		t.Fatalf("seed account store: %v", err)
	}

	runtime, err := newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore(
		config.Service{
			LegacyAddr:            ":13000",
			PublicAddr:            "127.0.0.1",
			LoginTicketStoreDir:   loginTicketDir,
			AccountStoreDir:       accountDir,
			StaticActorStorePath:  staticActorPath,
			InteractionStorePath:  interactionPath,
			ItemTemplateStorePath: itemTemplatePath,
			QuestStateStorePath:   questStatePath,
			GroundItemStorePath:   groundItemPath,
			SafeboxStorePath:      safeboxPath,
		},
		loginticket.NewFileStore(loginTicketDir),
		accounts,
		staticstore.NewFileStore(staticActorPath),
		interactionstore.NewFileStore(interactionPath),
		itemcatalog.NewFileStore(itemTemplatePath),
		queststate.NewFileStore(questStatePath),
		nil,
	)
	if err != nil {
		t.Fatalf("new game runtime: %v", err)
	}

	mux := RegisterGamedFileStorePersistenceOps(ops.NewPprofMux("gamed"), runtime)

	statusReq := httptest.NewRequest(http.MethodGet, "/local/persistence/status", nil)
	statusReq.RemoteAddr = "127.0.0.1:4242"
	statusRec := httptest.NewRecorder()
	mux.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected persistence status 200, got %d body=%s", statusRec.Code, statusRec.Body.String())
	}
	var status PersistenceStatusSnapshot
	if err := json.Unmarshal(statusRec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode persistence status: %v", err)
	}
	if !status.OK || status.LiveSelectedCharacterCount != 0 {
		t.Fatalf("expected healthy drained status, got %#v", status)
	}
	if !status.AccountStore.Valid || status.AccountStore.Summary.AccountCount != 1 {
		t.Fatalf("expected seeded account store in status: %#v", status.AccountStore)
	}

	runtimeReq := httptest.NewRequest(http.MethodGet, "/local/runtime-config", nil)
	runtimeReq.RemoteAddr = "127.0.0.1:4242"
	runtimeRec := httptest.NewRecorder()
	mux.ServeHTTP(runtimeRec, runtimeReq)
	if runtimeRec.Code != http.StatusOK {
		t.Fatalf("expected runtime-config 200, got %d body=%s", runtimeRec.Code, runtimeRec.Body.String())
	}

	backupDir := filepath.Join(root, "account-backup")
	mustMkdirAll(t, backupDir)
	backupBody, err := json.Marshal(map[string]string{"dst_dir": backupDir})
	if err != nil {
		t.Fatalf("marshal backup body: %v", err)
	}
	backupReq := httptest.NewRequest(http.MethodPost, "/local/account-store/backup", bytes.NewReader(backupBody))
	backupReq.RemoteAddr = "127.0.0.1:4242"
	backupReq.Header.Set("Content-Type", "application/json")
	backupRec := httptest.NewRecorder()
	mux.ServeHTTP(backupRec, backupReq)
	if backupRec.Code != http.StatusOK {
		t.Fatalf("expected account-store backup 200, got %d body=%s", backupRec.Code, backupRec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(backupDir, "account-backup-manifest.json")); err != nil {
		t.Fatalf("expected account backup manifest after helper backup: %v", err)
	}

	remoteReq := httptest.NewRequest(http.MethodGet, "/local/persistence/status", nil)
	remoteReq.RemoteAddr = "8.8.8.8:4242"
	remoteRec := httptest.NewRecorder()
	mux.ServeHTTP(remoteRec, remoteReq)
	if remoteRec.Code != http.StatusForbidden {
		t.Fatalf("expected non-loopback persistence status 403, got %d", remoteRec.Code)
	}

	// Helper must stay drill-scoped: migration status remains unregistered here.
	migrationReq := httptest.NewRequest(http.MethodGet, "/local/db/migrations/status", nil)
	migrationReq.RemoteAddr = "127.0.0.1:4242"
	migrationRec := httptest.NewRecorder()
	mux.ServeHTTP(migrationRec, migrationReq)
	if migrationRec.Code != http.StatusNotFound {
		t.Fatalf("expected helper to omit migration status, got %d", migrationRec.Code)
	}
}
