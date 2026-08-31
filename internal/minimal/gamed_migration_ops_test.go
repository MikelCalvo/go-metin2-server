package minimal

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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

func TestRegisterGamedMigrationQuarantineExportOpsServesCatalogExportAndQuarantine(t *testing.T) {
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
		Login:  "mig-helper",
		Empire: 1,
		Characters: []loginticket.Character{
			peerVisibilityCharacter("MigHero", 0x01030901, 0x02040901, 1100, 2100, 0, 101, 201),
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

	mux := RegisterGamedMigrationQuarantineExportOps(ops.NewPprofMux("gamed"), runtime)

	catalogReq := httptest.NewRequest(http.MethodGet, "/local/db/migrations/catalog", nil)
	catalogReq.RemoteAddr = "127.0.0.1:4242"
	catalogRec := httptest.NewRecorder()
	mux.ServeHTTP(catalogRec, catalogReq)
	if catalogRec.Code != http.StatusOK {
		t.Fatalf("expected migration catalog 200, got %d body=%s", catalogRec.Code, catalogRec.Body.String())
	}
	catalogBody := catalogRec.Body.String()
	for _, want := range []string{
		`"format":"go-metin2-migration-catalog-summary-v1"`,
		`"latest_version":29`,
		`"name":"character_item_instance_attributes"`,
		`"up_path":"0027_character_item_instance_attributes.up.sql"`,
		`"name":"character_safebox_item_instance_attributes"`,
		`"up_path":"0028_character_safebox_item_instance_attributes.up.sql"`,
		`"name":"bootstrap_ground_item_instance_attributes"`,
		`"up_path":"0029_bootstrap_ground_item_instance_attributes.up.sql"`,
	} {
		if !strings.Contains(catalogBody, want) {
			t.Fatalf("expected catalog body to contain %s, got %s", want, catalogBody)
		}
	}
	for _, forbidden := range []string{"CREATE TABLE", "DROP TABLE", "UpSQL", "DownSQL", "postgres://"} {
		if strings.Contains(catalogBody, forbidden) {
			t.Fatalf("migration catalog must not expose SQL/DSN marker %q, got %s", forbidden, catalogBody)
		}
	}

	exportReq := httptest.NewRequest(http.MethodGet, "/local/account-store/exports/account-character-roster", nil)
	exportReq.RemoteAddr = "127.0.0.1:4242"
	exportRec := httptest.NewRecorder()
	mux.ServeHTTP(exportRec, exportReq)
	if exportRec.Code != http.StatusOK {
		t.Fatalf("expected roster export 200, got %d body=%s", exportRec.Code, exportRec.Body.String())
	}
	exportBody := exportRec.Body.String()
	for _, want := range []string{
		`"migration_version":2`,
		`"migration_name":"account_character_roster"`,
		`"login":"mig-helper"`,
		`"name":"MigHero"`,
	} {
		if !strings.Contains(exportBody, want) {
			t.Fatalf("expected roster export body to contain %s, got %s", want, exportBody)
		}
	}

	quarantinePayload, err := json.Marshal(accountstore.AccountCharacterRosterExport{
		MigrationVersion: accountstore.AccountCharacterRosterMigrationVersion,
		MigrationName:    accountstore.AccountCharacterRosterMigrationName,
		Accounts: []accountstore.AccountCharacterRosterAccountRow{
			{ID: 100, Login: "Alpha", LoginNormalized: "alpha", Empire: 1},
		},
		Characters: []accountstore.AccountCharacterRosterCharacterRow{
			{ID: 11, AccountID: 100, Slot: 0, Name: "AlphaWar", NameNormalized: "alphawar", Level: 5, MapIndex: 1, Gold: 1234},
		},
	})
	if err != nil {
		t.Fatalf("marshal quarantine payload: %v", err)
	}
	quarantineReq := httptest.NewRequest(http.MethodPost, "/local/account-store/exports/account-character-roster/quarantine", bytes.NewReader(quarantinePayload))
	quarantineReq.RemoteAddr = "127.0.0.1:4242"
	quarantineRec := httptest.NewRecorder()
	mux.ServeHTTP(quarantineRec, quarantineReq)
	if quarantineRec.Code != http.StatusOK {
		t.Fatalf("expected roster quarantine 200, got %d body=%s", quarantineRec.Code, quarantineRec.Body.String())
	}
	quarantineBody := quarantineRec.Body.String()
	for _, want := range []string{`"account_count":1`, `"character_count":1`, `"login":"Alpha"`, `"name":"AlphaWar"`} {
		if !strings.Contains(quarantineBody, want) {
			t.Fatalf("expected quarantine body to contain %s, got %s", want, quarantineBody)
		}
	}

	remoteReq := httptest.NewRequest(http.MethodGet, "/local/db/migrations/catalog", nil)
	remoteReq.RemoteAddr = "8.8.8.8:4242"
	remoteRec := httptest.NewRecorder()
	mux.ServeHTTP(remoteRec, remoteReq)
	if remoteRec.Code != http.StatusForbidden {
		t.Fatalf("expected non-loopback catalog 403, got %d", remoteRec.Code)
	}

	// Helper must stay migration/quarantine-scoped: file-store backup and quest
	// mutation remain unregistered here.
	backupReq := httptest.NewRequest(http.MethodPost, "/local/account-store/backup", strings.NewReader(`{"dst_dir":"/tmp"}`))
	backupReq.RemoteAddr = "127.0.0.1:4242"
	backupRec := httptest.NewRecorder()
	mux.ServeHTTP(backupRec, backupReq)
	if backupRec.Code != http.StatusNotFound {
		t.Fatalf("expected helper to omit account-store backup, got %d", backupRec.Code)
	}

	questReq := httptest.NewRequest(http.MethodPost, "/local/quest-state/transition", strings.NewReader(`{}`))
	questReq.RemoteAddr = "127.0.0.1:4242"
	questRec := httptest.NewRecorder()
	mux.ServeHTTP(questRec, questReq)
	if questRec.Code != http.StatusNotFound {
		t.Fatalf("expected helper to omit quest-state transition, got %d", questRec.Code)
	}
}
