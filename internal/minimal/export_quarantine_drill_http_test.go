package minimal

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/buildinfo"
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

func TestExportQuarantineDrillHTTPExecutesAgainstDrainedGamedOps(t *testing.T) {
	defer worldruntime.DisableDurableGroundItemSyncForTest()()

	originalCommit := buildinfo.Commit
	originalVersion := buildinfo.Version
	originalBuildDate := buildinfo.BuildDate
	t.Cleanup(func() {
		buildinfo.Commit = originalCommit
		buildinfo.Version = originalVersion
		buildinfo.BuildDate = originalBuildDate
	})
	buildinfo.Version = "v0.1.0-export-drill"
	buildinfo.Commit = "exportdrill0123456789abcdef"
	buildinfo.BuildDate = "2026-08-25T18:00:00Z"

	root := t.TempDir()
	accountDir := filepath.Join(root, "accounts")
	loginTicketDir := filepath.Join(root, "login-tickets")
	staticActorPath := filepath.Join(root, "static-actors", "static-actors.json")
	interactionPath := filepath.Join(root, "interactions", "interaction-definitions.json")
	itemTemplatePath := filepath.Join(root, "item-templates", "item-templates.json")
	questStatePath := filepath.Join(root, "quest-state", "quest-state.json")
	groundItemPath := filepath.Join(root, "ground-items", "ground-items.json")
	safeboxPath := filepath.Join(root, "safebox", "safebox.json")
	exportBase := filepath.Join(root, "exports")
	binDir := filepath.Join(root, "bin")
	mustMkdirAll(t, accountDir)
	mustMkdirAll(t, loginTicketDir)
	mustMkdirAll(t, filepath.Dir(staticActorPath))
	mustMkdirAll(t, filepath.Dir(interactionPath))
	mustMkdirAll(t, filepath.Dir(itemTemplatePath))
	mustMkdirAll(t, filepath.Dir(questStatePath))
	mustMkdirAll(t, filepath.Dir(groundItemPath))
	mustMkdirAll(t, filepath.Dir(safeboxPath))
	mustMkdirAll(t, exportBase)
	mustMkdirAll(t, binDir)

	accounts := accountstore.NewFileStore(accountDir)
	if err := accounts.Save(accountstore.Account{
		Login:  "export-drill-owner",
		Empire: 1,
		Characters: []loginticket.Character{
			peerVisibilityCharacter("ExportDrillHero", 0x01030921, 0x02040921, 1300, 2300, 0, 101, 201),
		},
	}); err != nil {
		t.Fatalf("seed account store: %v", err)
	}

	seededSafebox := sampleRuntimeDurableSafebox()
	seededSafebox.Characters[0].Login = "export-drill-owner"
	seededSafebox.Characters[0].CharacterID = 0x01030921
	seededSafebox.Characters[0].Money = 1850
	if err := safeboxstore.NewFileStore(safeboxPath).Save(seededSafebox); err != nil {
		t.Fatalf("seed safebox store: %v", err)
	}

	gameRT, err := newGameRuntimeWithStoresAndTransferTriggersAndItemAndQuestStore(
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

	statusBefore := gameRT.PersistenceStatus()
	if statusBefore.LiveSelectedCharacterCount != 0 {
		t.Fatalf("expected drained runtime before drill, got live_selected_character_count=%d", statusBefore.LiveSelectedCharacterCount)
	}

	gamedMux := newDrainedExportQuarantineOpsMux(gameRT)
	gamedServer := httptest.NewUnstartedServer(gamedMux)
	gamedServer.Listener = mustListenLoopback(t)
	gamedServer.Start()
	t.Cleanup(gamedServer.Close)

	authdMux := ops.NewPprofMux("authd")
	authdServer := httptest.NewUnstartedServer(authdMux)
	authdServer.Listener = mustListenLoopback(t)
	authdServer.Start()
	t.Cleanup(authdServer.Close)

	migrateBin := mustBuildMetin2Migrate(t, binDir)

	buildInfoPath := filepath.Join(root, "build-info.json")
	buildInfoRaw, err := json.Marshal(buildinfo.Current())
	if err != nil {
		t.Fatalf("marshal build info: %v", err)
	}
	mustWriteFile(t, buildInfoPath, buildInfoRaw)

	var printerOut bytes.Buffer
	var printerErr bytes.Buffer
	code := migratecli.Run(
		[]string{
			"export-quarantine-drill",
			"--build-info", buildInfoPath,
			"--ops-base-url", gamedServer.URL,
			"--authd-ops-base-url", authdServer.URL,
			"--export-base", exportBase,
			"--gamed-log-path", filepath.Join(root, "missing-gamed.log"),
			"--authd-log-path", filepath.Join(root, "missing-authd.log"),
		},
		nil,
		&printerOut,
		&printerErr,
	)
	if code != 0 {
		t.Fatalf("expected printer exit 0, got %d stderr=%q", code, printerErr.String())
	}
	if printerErr.Len() != 0 {
		t.Fatalf("expected no printer stderr, got %q", printerErr.String())
	}
	script := printerOut.String()
	if script == "" {
		t.Fatal("expected non-empty printed export-quarantine drill script")
	}
	for _, banned := range []string{"SELECT ", "INSERT ", "DSN=", "postgres://", "mysql://", "CREATE TABLE"} {
		if strings.Contains(strings.ToUpper(script), strings.ToUpper(banned)) {
			t.Fatalf("printed drill script must not embed SQL/DSN markers; found %q in %s", banned, script)
		}
	}

	pathEnv := filepath.Dir(migrateBin) + string(os.PathListSeparator) + "/usr/local/bin:/usr/bin:/bin"
	stdout, stderr, exitCode := runPrintedShellScriptWithEnv(t, script, []string{
		"PATH=" + pathEnv,
		"HOME=" + root,
		"TMPDIR=" + root,
	})
	if exitCode != 0 {
		t.Fatalf("expected printed drill script exit 0, got %d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}

	retentionTree := mustFindSingleRetentionTree(t, exportBase, "exportdrill0")
	for _, name := range []string{
		"gamed-build-info.json",
		"authd-build-info.json",
		"runtime-config.json",
		"migration-catalog.json",
		"notes.md",
	} {
		assertRegularFileExists(t, filepath.Join(retentionTree, name))
	}

	kinds := []string{
		"account-character-roster",
		"character-item-state",
		"character-point-state",
		"auth-login-ticket-handoff",
		"character-quest-state",
		"character-safebox-state",
		"item-template-state",
		"static-actor-content-state",
		"bootstrap-ground-item-state",
	}
	for _, kind := range kinds {
		assertDirExists(t, filepath.Join(retentionTree, kind))
		assertRegularFileExists(t, filepath.Join(retentionTree, kind, "export.json"))
		assertRegularFileExists(t, filepath.Join(retentionTree, kind, "quarantine.json"))
		quarantineBody := mustReadFile(t, filepath.Join(retentionTree, kind, "quarantine.json"))
		for _, banned := range []string{"CREATE TABLE", "DROP TABLE", "INSERT ", "SELECT ", "postgres://", "mysql://", "DSN="} {
			if strings.Contains(strings.ToUpper(quarantineBody), strings.ToUpper(banned)) {
				t.Fatalf("quarantine.json for %s must not expose SQL/DSN marker %q, got %s", kind, banned, quarantineBody)
			}
		}
	}

	rosterQuarantine := mustReadFile(t, filepath.Join(retentionTree, "account-character-roster", "quarantine.json"))
	for _, want := range []string{
		`"account_count": 1`,
		`"character_count": 1`,
		`"login": "export-drill-owner"`,
		`"name": "ExportDrillHero"`,
		`"migration_version": 2`,
		`"migration_name": "account_character_roster"`,
	} {
		assertContainsLooseJSON(t, rosterQuarantine, want)
	}

	safeboxQuarantine := mustReadFile(t, filepath.Join(retentionTree, "character-safebox-state", "quarantine.json"))
	for _, want := range []string{
		`"password_count": 1`,
		`"item_count": 2`,
		`"login": "export-drill-owner"`,
		`"migration_version": 15`,
		`"migration_name": "character_safebox_money"`,
		`"money": 1850`,
	} {
		assertContainsLooseJSON(t, safeboxQuarantine, want)
	}
}

func newDrainedExportQuarantineOpsMux(runtime *gameRuntime) http.Handler {
	mux := RegisterGamedFileStorePersistenceOps(ops.NewPprofMux("gamed"), runtime)
	return RegisterGamedMigrationQuarantineExportOps(mux, runtime)
}

func mustBuildMetin2Migrate(t *testing.T, binDir string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test caller path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	out := filepath.Join(binDir, "metin2-migrate")
	cmd := exec.Command("go", "build", "-o", out, "./cmd/metin2-migrate")
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build ./cmd/metin2-migrate: %v stderr=%s", err, stderr.String())
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatalf("stat built metin2-migrate: %v", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		t.Fatalf("expected executable metin2-migrate at %s mode=%v", out, info.Mode())
	}
	return out
}

func runPrintedShellScriptWithEnv(t *testing.T, script string, env []string) (stdout string, stderr string, code int) {
	t.Helper()
	cmd := exec.Command("/bin/sh", "-s")
	cmd.Stdin = strings.NewReader(script)
	cmd.Env = env
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err == nil {
		return outBuf.String(), errBuf.String(), 0
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError from printed script, got %v", err)
	}
	return outBuf.String(), errBuf.String(), exitErr.ExitCode()
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func assertContainsLooseJSON(t *testing.T, body, want string) {
	t.Helper()
	compactBody := compactJSONForAssert(body)
	compactWant := compactJSONForAssert(want)
	if !strings.Contains(body, want) && !strings.Contains(compactBody, compactWant) {
		t.Fatalf("expected body to contain %s, got %s", want, body)
	}
}
