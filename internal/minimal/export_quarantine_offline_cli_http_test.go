package minimal

import (
	"bytes"
	"encoding/json"
	"io"
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
	"github.com/MikelCalvo/go-metin2-server/internal/migratecli"
	"github.com/MikelCalvo/go-metin2-server/internal/ops"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

func TestExportQuarantineOfflineCLIHTTPAcceptsRetainedLoopbackExports(t *testing.T) {
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
		Login:  "export-owner",
		Empire: 1,
		Characters: []loginticket.Character{
			peerVisibilityCharacter("ExportHero", 0x01030911, 0x02040911, 1200, 2200, 0, 101, 201),
		},
	}); err != nil {
		t.Fatalf("seed account store: %v", err)
	}

	seededSafebox := sampleRuntimeDurableSafebox()
	seededSafebox.Characters[0].Login = "export-owner"
	seededSafebox.Characters[0].CharacterID = 0x01030911
	seededSafebox.Characters[0].Money = 1750
	if err := safeboxstore.NewFileStore(safeboxPath).Save(seededSafebox); err != nil {
		t.Fatalf("seed safebox store: %v", err)
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

	gamedMux := RegisterGamedMigrationQuarantineExportOps(ops.NewPprofMux("gamed"), runtime)
	gamedServer := httptest.NewUnstartedServer(gamedMux)
	gamedServer.Listener = mustListenLoopback(t)
	gamedServer.Start()
	t.Cleanup(gamedServer.Close)

	rosterExportPath := filepath.Join(root, "account-character-roster.export.json")
	rosterExportBody := mustGETLoopbackExport(t, gamedServer.URL+"/local/account-store/exports/account-character-roster")
	mustWriteFile(t, rosterExportPath, rosterExportBody)
	assertOfflineQuarantineExport(t, "account-character-roster", rosterExportPath, []string{
		`"account_count": 1`,
		`"character_count": 1`,
		`"login": "export-owner"`,
		`"name": "ExportHero"`,
		`"migration_version": 2`,
		`"migration_name": "account_character_roster"`,
	})

	safeboxExportPath := filepath.Join(root, "character-safebox-state.export.json")
	safeboxExportBody := mustGETLoopbackExport(t, gamedServer.URL+"/local/safebox-store/exports/character-safebox-state")
	mustWriteFile(t, safeboxExportPath, safeboxExportBody)
	assertOfflineQuarantineExport(t, "character-safebox-state", safeboxExportPath, []string{
		`"password_count": 1`,
		`"item_count": 2`,
		`"login": "export-owner"`,
		`"migration_version": 15`,
		`"migration_name": "character_safebox_money"`,
		`"money": 1750`,
	})

	// Parity: the same retained roster bytes must also satisfy the loopback
	// POST quarantine handler that operators may use before offline triage.
	quarantineReq, err := http.NewRequest(http.MethodPost, gamedServer.URL+"/local/account-store/exports/account-character-roster/quarantine", bytes.NewReader(rosterExportBody))
	if err != nil {
		t.Fatalf("new roster quarantine request: %v", err)
	}
	quarantineResp, err := http.DefaultClient.Do(quarantineReq)
	if err != nil {
		t.Fatalf("POST roster quarantine: %v", err)
	}
	defer quarantineResp.Body.Close()
	quarantineBody, err := io.ReadAll(quarantineResp.Body)
	if err != nil {
		t.Fatalf("read roster quarantine body: %v", err)
	}
	if quarantineResp.StatusCode != http.StatusOK {
		t.Fatalf("expected loopback roster quarantine 200, got %d body=%s", quarantineResp.StatusCode, quarantineBody)
	}
	compactBody := compactJSONForAssert(string(quarantineBody))
	for _, want := range []string{`"account_count":1`, `"character_count":1`, `"login":"export-owner"`, `"name":"ExportHero"`} {
		if !strings.Contains(compactBody, want) {
			t.Fatalf("expected loopback quarantine body to contain %s, got %s", want, quarantineBody)
		}
	}
}

func compactJSONForAssert(s string) string {
	replacer := strings.NewReplacer(" ", "", "\n", "", "	", "", "\r", "")
	return replacer.Replace(s)
}

func mustGETLoopbackExport(t *testing.T, url string) []byte {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s body: %v", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected GET %s 200, got %d body=%s", url, resp.StatusCode, body)
	}
	if !json.Valid(body) {
		t.Fatalf("expected JSON export body from %s, got %s", url, body)
	}
	return body
}

func assertOfflineQuarantineExport(t *testing.T, kind, exportPath string, wantSubstrings []string) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := migratecli.Run(
		[]string{"quarantine-export", "--kind", kind, "--export", exportPath},
		nil,
		&stdout,
		&stderr,
	)
	if code != 0 {
		t.Fatalf("expected quarantine-export %s exit 0, got %d stderr=%q", kind, code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no quarantine-export stderr for %s, got %q", kind, stderr.String())
	}
	body := stdout.String()
	if body == "" {
		t.Fatalf("expected non-empty quarantine-export stdout for %s", kind)
	}
	compactBody := compactJSONForAssert(body)
	for _, want := range wantSubstrings {
		if !strings.Contains(body, want) && !strings.Contains(compactBody, compactJSONForAssert(want)) {
			t.Fatalf("expected quarantine-export %s stdout to contain %s, got %s", kind, want, body)
		}
	}
	for _, banned := range []string{"CREATE TABLE", "DROP TABLE", "INSERT ", "SELECT ", "postgres://", "mysql://", "DSN="} {
		if strings.Contains(strings.ToUpper(body), strings.ToUpper(banned)) {
			t.Fatalf("quarantine-export %s stdout must not expose SQL/DSN marker %q, got %s", kind, banned, body)
		}
	}
}
