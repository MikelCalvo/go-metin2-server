package migratecli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	maxRuntimeConfigBytes          = 64 * 1024
	defaultBackupRestoreOpsBaseURL = "http://127.0.0.1:6060"
	defaultBackupRestoreBackupBase = "/var/metin2/backups/drill"
)

var errInvalidBackupRestoreDrillInput = errors.New("invalid backup-restore-drill input")

type backupRestoreRuntimeConfig struct {
	LocalChannelID       uint8                          `json:"local_channel_id"`
	VisibilityMode       string                         `json:"visibility_mode"`
	VisibilityRadius     int32                          `json:"visibility_radius"`
	VisibilitySectorSize int32                          `json:"visibility_sector_size"`
	Persistence          backupRestorePersistenceConfig `json:"persistence"`
	Database             backupRestoreDatabaseConfig    `json:"database"`
}

type backupRestorePersistenceConfig struct {
	LoginTicketStoreDir   string `json:"login_ticket_store_dir"`
	AccountStoreDir       string `json:"account_store_dir"`
	StaticActorStorePath  string `json:"static_actor_store_path"`
	InteractionStorePath  string `json:"interaction_store_path"`
	ItemTemplateStorePath string `json:"item_template_store_path"`
	QuestStateStorePath   string `json:"quest_state_store_path"`
}

type backupRestoreDatabaseConfig struct {
	Configured    bool   `json:"configured"`
	Driver        string `json:"driver,omitempty"`
	DSNConfigured bool   `json:"dsn_configured"`
}

type backupRestoreDrillPlan struct {
	OpsBaseURL            string
	BackupBase            string
	AccountStoreDir       string
	LoginTicketStoreDir   string
	ItemTemplateStorePath string
	InteractionStorePath  string
	StaticActorStorePath  string
	QuestStateStorePath   string
}

func runBackupRestoreDrill(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("backup-restore-drill", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var runtimeConfigPath string
	var opsBaseURL string
	var backupBase string
	flags.StringVar(&runtimeConfigPath, "runtime-config", "", "path to retained /local/runtime-config JSON, or - for stdin")
	flags.StringVar(&opsBaseURL, "ops-base-url", defaultBackupRestoreOpsBaseURL, "loopback ops base URL used in printed curl commands")
	flags.StringVar(&backupBase, "backup-base", defaultBackupRestoreBackupBase, "absolute backup root used in printed drill commands")
	flags.Usage = func() { printBackupRestoreDrillUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected backup-restore-drill argument %q\n", flags.Arg(0))
		printBackupRestoreDrillUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(runtimeConfigPath) == "" {
		fmt.Fprintln(stderr, "--runtime-config is required for backup-restore-drill")
		printBackupRestoreDrillUsage(stderr)
		return exitUsage
	}

	reader, closeReader, err := openBackupRestoreRuntimeConfigReader(runtimeConfigPath, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "backup-restore-drill: %v\n", err)
		return exitError
	}
	if closeReader != nil {
		defer closeReader()
	}

	raw, err := readBoundedRuntimeConfig(reader)
	if err != nil {
		fmt.Fprintf(stderr, "backup-restore-drill: %v\n", err)
		return exitError
	}

	plan, err := buildBackupRestoreDrillPlan(raw, opsBaseURL, backupBase)
	if err != nil {
		fmt.Fprintf(stderr, "backup-restore-drill: %v\n", err)
		return exitError
	}
	if _, err := io.WriteString(stdout, renderBackupRestoreDrillScript(plan)); err != nil {
		fmt.Fprintf(stderr, "backup-restore-drill: write script: %v\n", err)
		return exitError
	}
	return exitOK
}

func openBackupRestoreRuntimeConfigReader(path string, stdin io.Reader) (io.Reader, func(), error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "-" {
		return stdin, nil, nil
	}
	info, err := os.Lstat(trimmed)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: stat runtime-config: %v", errInvalidBackupRestoreDrillInput, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, nil, fmt.Errorf("%w: runtime-config must not be a symlink: %s", errInvalidBackupRestoreDrillInput, trimmed)
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("%w: runtime-config must be a regular file: %s", errInvalidBackupRestoreDrillInput, trimmed)
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: open runtime-config: %v", errInvalidBackupRestoreDrillInput, err)
	}
	openedInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, fmt.Errorf("%w: stat opened runtime-config: %v", errInvalidBackupRestoreDrillInput, err)
	}
	if !openedInfo.Mode().IsRegular() {
		_ = file.Close()
		return nil, nil, fmt.Errorf("%w: opened runtime-config must be a regular file: %s", errInvalidBackupRestoreDrillInput, trimmed)
	}
	return file, func() { _ = file.Close() }, nil
}

func readBoundedRuntimeConfig(reader io.Reader) ([]byte, error) {
	if reader == nil {
		reader = strings.NewReader("")
	}
	raw, err := io.ReadAll(io.LimitReader(reader, maxRuntimeConfigBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read runtime-config: %v", errInvalidBackupRestoreDrillInput, err)
	}
	if len(raw) > maxRuntimeConfigBytes {
		return nil, fmt.Errorf("%w: runtime-config exceeds %d bytes", errInvalidBackupRestoreDrillInput, maxRuntimeConfigBytes)
	}
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("%w: runtime-config is not valid UTF-8", errInvalidBackupRestoreDrillInput)
	}
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, fmt.Errorf("%w: runtime-config is empty", errInvalidBackupRestoreDrillInput)
	}
	return raw, nil
}

func buildBackupRestoreDrillPlan(raw []byte, opsBaseURL string, backupBase string) (backupRestoreDrillPlan, error) {
	var snapshot backupRestoreRuntimeConfig
	if err := decodeStrictRuntimeConfigJSON(raw, &snapshot); err != nil {
		return backupRestoreDrillPlan{}, err
	}

	normalizedOps, err := normalizeBackupRestoreOpsBaseURL(opsBaseURL)
	if err != nil {
		return backupRestoreDrillPlan{}, err
	}
	normalizedBackupBase, err := normalizeAbsoluteCleanPath(backupBase, "backup-base")
	if err != nil {
		return backupRestoreDrillPlan{}, err
	}

	accountDir, err := normalizeAbsoluteCleanPath(snapshot.Persistence.AccountStoreDir, "persistence.account_store_dir")
	if err != nil {
		return backupRestoreDrillPlan{}, err
	}
	loginTicketDir, err := normalizeAbsoluteCleanPath(snapshot.Persistence.LoginTicketStoreDir, "persistence.login_ticket_store_dir")
	if err != nil {
		return backupRestoreDrillPlan{}, err
	}
	itemTemplatePath, err := normalizeAbsoluteCleanPath(snapshot.Persistence.ItemTemplateStorePath, "persistence.item_template_store_path")
	if err != nil {
		return backupRestoreDrillPlan{}, err
	}
	interactionPath, err := normalizeAbsoluteCleanPath(snapshot.Persistence.InteractionStorePath, "persistence.interaction_store_path")
	if err != nil {
		return backupRestoreDrillPlan{}, err
	}
	staticActorPath, err := normalizeAbsoluteCleanPath(snapshot.Persistence.StaticActorStorePath, "persistence.static_actor_store_path")
	if err != nil {
		return backupRestoreDrillPlan{}, err
	}
	questStatePath, err := normalizeAbsoluteCleanPath(snapshot.Persistence.QuestStateStorePath, "persistence.quest_state_store_path")
	if err != nil {
		return backupRestoreDrillPlan{}, err
	}

	if cleanedPathEqualsOrNests(accountDir, loginTicketDir) {
		return backupRestoreDrillPlan{}, fmt.Errorf("%w: account_store_dir and login_ticket_store_dir must not equal or nest under each other", errInvalidBackupRestoreDrillInput)
	}

	fileParents := map[string]string{
		"item_template_store_path": filepath.Dir(itemTemplatePath),
		"interaction_store_path":   filepath.Dir(interactionPath),
		"static_actor_store_path":  filepath.Dir(staticActorPath),
		"quest_state_store_path":   filepath.Dir(questStatePath),
	}
	seenParents := make(map[string]string, len(fileParents))
	for label, parent := range fileParents {
		if other, ok := seenParents[parent]; ok {
			return backupRestoreDrillPlan{}, fmt.Errorf("%w: file-path stores share parent directory %s (%s and %s); restore empties filepath.Dir(snapshotPath)", errInvalidBackupRestoreDrillInput, parent, other, label)
		}
		seenParents[parent] = label
	}

	return backupRestoreDrillPlan{
		OpsBaseURL:            normalizedOps,
		BackupBase:            normalizedBackupBase,
		AccountStoreDir:       accountDir,
		LoginTicketStoreDir:   loginTicketDir,
		ItemTemplateStorePath: itemTemplatePath,
		InteractionStorePath:  interactionPath,
		StaticActorStorePath:  staticActorPath,
		QuestStateStorePath:   questStatePath,
	}, nil
}

func decodeStrictRuntimeConfigJSON(raw []byte, dest any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("%w: decode runtime-config: %v", errInvalidBackupRestoreDrillInput, err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("%w: runtime-config has trailing JSON", errInvalidBackupRestoreDrillInput)
	}
	return nil
}

func normalizeBackupRestoreOpsBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: ops-base-url is required", errInvalidBackupRestoreDrillInput)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("%w: ops-base-url: %v", errInvalidBackupRestoreDrillInput, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%w: ops-base-url scheme must be http or https", errInvalidBackupRestoreDrillInput)
	}
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("%w: ops-base-url must be an absolute http(s) URL with a host and no query/fragment", errInvalidBackupRestoreDrillInput)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if parsed.Path == "/" {
		parsed.Path = ""
	}
	return parsed.String(), nil
}

func normalizeAbsoluteCleanPath(raw string, label string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: %s is required", errInvalidBackupRestoreDrillInput, label)
	}
	if !filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("%w: %s must be an absolute path", errInvalidBackupRestoreDrillInput, label)
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." || cleaned == string(filepath.Separator) && trimmed != string(filepath.Separator) {
		return "", fmt.Errorf("%w: %s is invalid", errInvalidBackupRestoreDrillInput, label)
	}
	return cleaned, nil
}

func cleanedPathEqualsOrNests(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if a == b {
		return true
	}
	aPrefix := a + string(filepath.Separator)
	bPrefix := b + string(filepath.Separator)
	return strings.HasPrefix(b, aPrefix) || strings.HasPrefix(a, bPrefix)
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func renderBackupRestoreDrillScript(plan backupRestoreDrillPlan) string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("# read-only printer: does not execute backup/restore\n")
	b.WriteString("# crash-temps/cleanup mutates only hidden crash-temp residue after validate\n")
	b.WriteString("# Generated from a retained /local/runtime-config snapshot for docs/workflow/file-store-backup-restore-drill.md\n")
	b.WriteString("set -eu\n")
	b.WriteString("\n")
	fmt.Fprintf(&b, "OPS=%s\n", shellSingleQuote(plan.OpsBaseURL))
	fmt.Fprintf(&b, "BASE=%s\n", shellSingleQuote(plan.BackupBase))
	fmt.Fprintf(&b, "ACCOUNT_STORE_DIR=%s\n", shellSingleQuote(plan.AccountStoreDir))
	fmt.Fprintf(&b, "LOGIN_TICKET_STORE_DIR=%s\n", shellSingleQuote(plan.LoginTicketStoreDir))
	fmt.Fprintf(&b, "ITEM_TEMPLATE_STORE_PATH=%s\n", shellSingleQuote(plan.ItemTemplateStorePath))
	fmt.Fprintf(&b, "INTERACTION_STORE_PATH=%s\n", shellSingleQuote(plan.InteractionStorePath))
	fmt.Fprintf(&b, "STATIC_ACTOR_STORE_PATH=%s\n", shellSingleQuote(plan.StaticActorStorePath))
	fmt.Fprintf(&b, "QUEST_STATE_STORE_PATH=%s\n", shellSingleQuote(plan.QuestStateStorePath))
	b.WriteString("\n")
	b.WriteString("TS=$(date +%Y%m%dT%H%M%S)\n")
	b.WriteString("BASE=\"${BASE}-${TS}\"\n")
	b.WriteString("\n")
	b.WriteString("echo '== preflight =='\n")
	b.WriteString("curl -sS \"$OPS/healthz\"\n")
	b.WriteString("curl -sS \"$OPS/local/runtime-config\"\n")
	b.WriteString("curl -sS \"$OPS/local/persistence/status\"\n")
	b.WriteString("\n")
	b.WriteString("echo '== store validate / crash-temp triage =='\n")
	b.WriteString("# Optional runbook triage before backup. validate is read-only; crash-temps/cleanup\n")
	b.WriteString("# removes only hidden crash-temp residue after validating committed snapshots.\n")
	b.WriteString("# Do not treat cleanup as enough preparation for restore: committed snapshots and\n")
	b.WriteString("# active *-backup-manifest.json still make destinations non-empty.\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/account-store/validate\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/account-store/crash-temps/cleanup\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/login-tickets/validate\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/login-tickets/crash-temps/cleanup\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/item-templates/validate\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/item-templates/crash-temps/cleanup\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/interaction-store/validate\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/interaction-store/crash-temps/cleanup\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/static-actor-store/validate\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/static-actor-store/crash-temps/cleanup\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/quest-state/validate\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/quest-state/crash-temps/cleanup\"\n")
	b.WriteString("curl -sS \"$OPS/local/persistence/status\"\n")
	b.WriteString("\n")
	b.WriteString("echo '== prepare backup destinations =='\n")
	b.WriteString("mkdir -p \"$BASE\"/account \"$BASE\"/login-tickets \"$BASE\"/item-templates \"$BASE\"/interactions \"$BASE\"/static-actors \"$BASE\"/quest-state\n")
	b.WriteString("\n")
	b.WriteString("echo '== backup =='\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/account-store/backup\" -H 'Content-Type: application/json' -d \"{\\\"dst_dir\\\":\\\"$BASE/account\\\"}\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/login-tickets/backup\" -H 'Content-Type: application/json' -d \"{\\\"dst_dir\\\":\\\"$BASE/login-tickets\\\"}\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/item-templates/backup\" -H 'Content-Type: application/json' -d \"{\\\"dst_dir\\\":\\\"$BASE/item-templates\\\"}\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/interaction-store/backup\" -H 'Content-Type: application/json' -d \"{\\\"dst_dir\\\":\\\"$BASE/interactions\\\"}\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/static-actors/backup\" -H 'Content-Type: application/json' -d \"{\\\"dst_dir\\\":\\\"$BASE/static-actors\\\"}\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/quest-state/backup\" -H 'Content-Type: application/json' -d \"{\\\"dst_dir\\\":\\\"$BASE/quest-state\\\"}\"\n")
	b.WriteString("\n")
	b.WriteString("echo '== backup validate =='\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/account-store/backup/validate\" -H 'Content-Type: application/json' -d \"{\\\"src_dir\\\":\\\"$BASE/account\\\"}\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/login-tickets/backup/validate\" -H 'Content-Type: application/json' -d \"{\\\"src_dir\\\":\\\"$BASE/login-tickets\\\"}\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/item-templates/backup/validate\" -H 'Content-Type: application/json' -d \"{\\\"src_dir\\\":\\\"$BASE/item-templates\\\"}\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/interaction-store/backup/validate\" -H 'Content-Type: application/json' -d \"{\\\"src_dir\\\":\\\"$BASE/interactions\\\"}\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/static-actors/backup/validate\" -H 'Content-Type: application/json' -d \"{\\\"src_dir\\\":\\\"$BASE/static-actors\\\"}\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/quest-state/backup/validate\" -H 'Content-Type: application/json' -d \"{\\\"src_dir\\\":\\\"$BASE/quest-state\\\"}\"\n")
	b.WriteString("\n")
	b.WriteString("echo '== empty active destinations (requires drained selected-character sessions) =='\n")
	b.WriteString("mv \"$ACCOUNT_STORE_DIR\" \"${ACCOUNT_STORE_DIR}.aside-${TS}\"\n")
	b.WriteString("mkdir -p \"$ACCOUNT_STORE_DIR\"\n")
	b.WriteString("mv \"$LOGIN_TICKET_STORE_DIR\" \"${LOGIN_TICKET_STORE_DIR}.aside-${TS}\"\n")
	b.WriteString("mkdir -p \"$LOGIN_TICKET_STORE_DIR\"\n")
	b.WriteString("mv \"$(dirname \"$ITEM_TEMPLATE_STORE_PATH\")\" \"$(dirname \"$ITEM_TEMPLATE_STORE_PATH\").aside-${TS}\"\n")
	b.WriteString("mkdir -p \"$(dirname \"$ITEM_TEMPLATE_STORE_PATH\")\"\n")
	b.WriteString("mv \"$(dirname \"$INTERACTION_STORE_PATH\")\" \"$(dirname \"$INTERACTION_STORE_PATH\").aside-${TS}\"\n")
	b.WriteString("mkdir -p \"$(dirname \"$INTERACTION_STORE_PATH\")\"\n")
	b.WriteString("mv \"$(dirname \"$STATIC_ACTOR_STORE_PATH\")\" \"$(dirname \"$STATIC_ACTOR_STORE_PATH\").aside-${TS}\"\n")
	b.WriteString("mkdir -p \"$(dirname \"$STATIC_ACTOR_STORE_PATH\")\"\n")
	b.WriteString("mv \"$(dirname \"$QUEST_STATE_STORE_PATH\")\" \"$(dirname \"$QUEST_STATE_STORE_PATH\").aside-${TS}\"\n")
	b.WriteString("mkdir -p \"$(dirname \"$QUEST_STATE_STORE_PATH\")\"\n")
	b.WriteString("curl -sS \"$OPS/local/persistence/status\"\n")
	b.WriteString("\n")
	b.WriteString("echo '== restore =='\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/item-templates/restore\" -H 'Content-Type: application/json' -d \"{\\\"src_dir\\\":\\\"$BASE/item-templates\\\"}\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/interaction-store/restore\" -H 'Content-Type: application/json' -d \"{\\\"src_dir\\\":\\\"$BASE/interactions\\\"}\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/static-actors/restore\" -H 'Content-Type: application/json' -d \"{\\\"src_dir\\\":\\\"$BASE/static-actors\\\"}\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/quest-state/restore\" -H 'Content-Type: application/json' -d \"{\\\"src_dir\\\":\\\"$BASE/quest-state\\\"}\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/account-store/restore\" -H 'Content-Type: application/json' -d \"{\\\"src_dir\\\":\\\"$BASE/account\\\"}\"\n")
	b.WriteString("curl -sS -X POST \"$OPS/local/login-tickets/restore\" -H 'Content-Type: application/json' -d \"{\\\"src_dir\\\":\\\"$BASE/login-tickets\\\"}\"\n")
	b.WriteString("\n")
	b.WriteString("echo '== post-restore =='\n")
	b.WriteString("curl -sS \"$OPS/local/persistence/status\"\n")
	return b.String()
}

func printBackupRestoreDrillUsage(w io.Writer) {
	fmt.Fprintln(w, "backup-restore-drill usage:")
	fmt.Fprintln(w, "  metin2-migrate backup-restore-drill --runtime-config <path|-> [--ops-base-url http://127.0.0.1:6060] [--backup-base /var/metin2/backups/drill]")
}
