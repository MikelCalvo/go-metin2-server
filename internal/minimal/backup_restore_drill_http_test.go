package minimal

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestBackupRestoreDrillHTTPExecutesAgainstDrainedGamedOps(t *testing.T) {
	defer worldruntime.DisableDurableGroundItemSyncForTest()()

	originalCommit := buildinfo.Commit
	originalVersion := buildinfo.Version
	originalBuildDate := buildinfo.BuildDate
	t.Cleanup(func() {
		buildinfo.Commit = originalCommit
		buildinfo.Version = originalVersion
		buildinfo.BuildDate = originalBuildDate
	})
	buildinfo.Version = "v0.1.0-drill"
	buildinfo.Commit = "drillproof0123456789abcdef"
	buildinfo.BuildDate = "2026-08-24T14:00:00Z"

	root := t.TempDir()
	accountDir := filepath.Join(root, "accounts")
	loginTicketDir := filepath.Join(root, "login-tickets")
	staticActorPath := filepath.Join(root, "static-actors", "static-actors.json")
	interactionPath := filepath.Join(root, "interactions", "interaction-definitions.json")
	itemTemplatePath := filepath.Join(root, "item-templates", "item-templates.json")
	questStatePath := filepath.Join(root, "quest-state", "quest-state.json")
	groundItemPath := filepath.Join(root, "ground-items", "ground-items.json")
	safeboxPath := filepath.Join(root, "safebox", "safebox.json")
	backupBase := filepath.Join(root, "backups")
	mustMkdirAll(t, accountDir)
	mustMkdirAll(t, loginTicketDir)
	mustMkdirAll(t, filepath.Dir(staticActorPath))
	mustMkdirAll(t, filepath.Dir(interactionPath))
	mustMkdirAll(t, filepath.Dir(itemTemplatePath))
	mustMkdirAll(t, filepath.Dir(questStatePath))
	mustMkdirAll(t, filepath.Dir(groundItemPath))
	mustMkdirAll(t, filepath.Dir(safeboxPath))
	mustMkdirAll(t, backupBase)

	accounts := accountstore.NewFileStore(accountDir)
	if err := accounts.Save(accountstore.Account{
		Login:  "drill-owner",
		Empire: 1,
		Characters: []loginticket.Character{
			peerVisibilityCharacter("DrillHero", 0x01030901, 0x02040901, 1100, 2100, 0, 101, 201),
		},
	}); err != nil {
		t.Fatalf("seed account store: %v", err)
	}
	if err := safeboxstore.NewFileStore(safeboxPath).Save(sampleRuntimeDurableSafebox()); err != nil {
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

	statusBefore := runtime.PersistenceStatus()
	if statusBefore.LiveSelectedCharacterCount != 0 {
		t.Fatalf("expected drained runtime before drill, got live_selected_character_count=%d", statusBefore.LiveSelectedCharacterCount)
	}
	if !statusBefore.AccountStore.Valid || statusBefore.AccountStore.Summary.AccountCount != 1 {
		t.Fatalf("expected seeded account store before drill: %#v", statusBefore.AccountStore)
	}
	if !statusBefore.SafeboxStore.Valid || statusBefore.SafeboxStore.Summary.CharacterCount != 1 {
		t.Fatalf("expected seeded safebox store before drill: %#v", statusBefore.SafeboxStore)
	}

	gamedMux := newDrainedBackupRestoreOpsMux(runtime)
	gamedServer := httptest.NewUnstartedServer(gamedMux)
	gamedServer.Listener = mustListenLoopback(t)
	gamedServer.Start()
	t.Cleanup(gamedServer.Close)

	authdMux := ops.NewPprofMux("authd")
	authdServer := httptest.NewUnstartedServer(authdMux)
	authdServer.Listener = mustListenLoopback(t)
	authdServer.Start()
	t.Cleanup(authdServer.Close)

	runtimeConfigPath := filepath.Join(root, "runtime-config.json")
	runtimeConfigRaw, err := json.Marshal(runtime.RuntimeConfigSnapshot())
	if err != nil {
		t.Fatalf("marshal runtime config: %v", err)
	}
	mustWriteFile(t, runtimeConfigPath, runtimeConfigRaw)

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
			"backup-restore-drill",
			"--runtime-config", runtimeConfigPath,
			"--build-info", buildInfoPath,
			"--ops-base-url", gamedServer.URL,
			"--authd-ops-base-url", authdServer.URL,
			"--backup-base", backupBase,
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
		t.Fatal("expected non-empty printed drill script")
	}
	for _, banned := range []string{"SELECT ", "INSERT ", "DSN=", "postgres://", "mysql://"} {
		if strings.Contains(strings.ToUpper(script), strings.ToUpper(banned)) {
			t.Fatalf("printed drill script must not embed SQL/DSN markers; found %q in %s", banned, script)
		}
	}

	stdout, stderr, exitCode := runPrintedShellScript(t, script)
	if exitCode != 0 {
		t.Fatalf("expected printed drill script exit 0, got %d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}

	retentionTree := mustFindSingleRetentionTree(t, backupBase, "drillproof01")
	for _, subdir := range []string{
		"accounts",
		"login-tickets",
		"item-templates",
		"interaction-store",
		"static-actors",
		"quest-state",
		"ground-items",
		"safebox",
	} {
		assertDirExists(t, filepath.Join(retentionTree, subdir))
	}
	for _, name := range []string{
		"gamed-build-info.json",
		"authd-build-info.json",
		"runtime-config.json",
		"persistence-status-before.json",
		"persistence-status-after.json",
		"notes.md",
	} {
		assertRegularFileExists(t, filepath.Join(retentionTree, name))
	}
	assertRegularFileExists(t, filepath.Join(retentionTree, "accounts", accountstore.BackupManifestFilename))
	assertRegularFileExists(t, filepath.Join(retentionTree, "safebox", safeboxstore.BackupManifestFilename))

	assertDirExists(t, accountDir+".aside-"+retentionTreeTimestamp(t, retentionTree))
	assertDirExists(t, filepath.Dir(safeboxPath)+".aside-"+retentionTreeTimestamp(t, retentionTree))

	restoredAccount, err := accountstore.NewFileStore(accountDir).Load("drill-owner")
	if err != nil {
		t.Fatalf("load restored account: %v", err)
	}
	if restoredAccount.Empire != 1 || len(restoredAccount.Characters) == 0 || restoredAccount.Characters[0].Name != "DrillHero" {
		t.Fatalf("unexpected restored account: %#v", restoredAccount)
	}
	restoredSafebox, err := safeboxstore.NewFileStore(safeboxPath).Load()
	if err != nil {
		t.Fatalf("load restored safebox: %v", err)
	}
	wantSafebox := sampleRuntimeDurableSafebox()
	if len(restoredSafebox.Characters) != 1 || restoredSafebox.Characters[0].Login != wantSafebox.Characters[0].Login {
		t.Fatalf("unexpected restored safebox: %#v", restoredSafebox)
	}
	if len(restoredSafebox.Characters[0].Cells) != len(wantSafebox.Characters[0].Cells) {
		t.Fatalf("unexpected restored safebox cells: %#v", restoredSafebox.Characters[0].Cells)
	}

	statusResp, err := http.Get(gamedServer.URL + "/local/persistence/status")
	if err != nil {
		t.Fatalf("GET persistence status after drill: %v", err)
	}
	defer statusResp.Body.Close()
	if statusResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(statusResp.Body)
		t.Fatalf("expected persistence status 200, got %d body=%s", statusResp.StatusCode, body)
	}
	var statusAfter PersistenceStatusSnapshot
	if err := json.NewDecoder(statusResp.Body).Decode(&statusAfter); err != nil {
		t.Fatalf("decode persistence status: %v", err)
	}
	if !statusAfter.OK || statusAfter.LiveSelectedCharacterCount != 0 {
		t.Fatalf("expected healthy drained persistence status after drill: %#v", statusAfter)
	}
	if !statusAfter.AccountStore.Valid || statusAfter.AccountStore.Summary.AccountCount != 1 {
		t.Fatalf("expected restored account store in status: %#v", statusAfter.AccountStore)
	}
	if !statusAfter.SafeboxStore.Valid || statusAfter.SafeboxStore.Summary.CharacterCount != 1 {
		t.Fatalf("expected restored safebox store in status: %#v", statusAfter.SafeboxStore)
	}
}

func newDrainedBackupRestoreOpsMux(runtime *gameRuntime) http.Handler {
	return RegisterGamedFileStorePersistenceOps(ops.NewPprofMux("gamed"), runtime)
}

func mustListenLoopback(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen loopback: %v", err)
	}
	return listener
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runPrintedShellScript(t *testing.T, script string) (stdout string, stderr string, code int) {
	t.Helper()
	cmd := exec.Command("/bin/sh", "-s")
	cmd.Stdin = strings.NewReader(script)
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

func mustFindSingleRetentionTree(t *testing.T, backupBase, commit12 string) string {
	t.Helper()
	entries, err := os.ReadDir(backupBase)
	if err != nil {
		t.Fatalf("read backup base: %v", err)
	}
	var matches []string
	suffix := "-" + commit12
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, suffix) {
			matches = append(matches, filepath.Join(backupBase, name))
		}
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one retention tree ending with %q under %s, got %#v", suffix, backupBase, matches)
	}
	return matches[0]
}

func retentionTreeTimestamp(t *testing.T, retentionTree string) string {
	t.Helper()
	base := filepath.Base(retentionTree)
	parts := strings.SplitN(base, "-", 2)
	if len(parts) != 2 || parts[0] == "" {
		t.Fatalf("unexpected retention tree name %q", base)
	}
	if _, err := time.Parse("20060102T150405Z", parts[0]); err != nil {
		t.Fatalf("parse retention timestamp from %q: %v", base, err)
	}
	return parts[0]
}

func assertDirExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected directory %s to exist: %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", path)
	}
}

func assertRegularFileExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected regular file %s to exist: %v", path, err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("expected %s to be a regular file", path)
	}
}
