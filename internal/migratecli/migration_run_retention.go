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
	maxMigrationRunBuildInfoBytes       = 64 * 1024
	defaultMigrationRunOpsBaseURL       = "http://127.0.0.1:6060"
	defaultMigrationRunAuthdOpsBaseURL  = "http://127.0.0.1:6061"
	defaultMigrationRunsBase            = "/var/metin2/migration-runs"
	defaultMigrationRunTargetVersion    = "latest"
	defaultMigrationRunLockFile         = "migration-apply.lock"
	defaultMigrationRunRollbackLockFile = "migration-rollback.lock"
	defaultMigrationRunGamedLogPath     = "/var/log/metin2/gamed.log"
	defaultMigrationRunAuthdLogPath     = "/var/log/metin2/authd.log"
	migrationRunCommitSuffixMaxRunes    = 12
)

var errInvalidMigrationRunRetentionInput = errors.New("invalid migration-run-retention input")

type migrationRunBuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

type migrationRunRetentionPlan struct {
	OpsBaseURL        string
	AuthdOpsBaseURL   string
	MigrationRunsBase string
	GamedLogPath      string
	AuthdLogPath      string
	TargetVersion     string
	LockFile          string
	Commit12          string
	BuildVersion      string
	BuildCommit       string
	BuildDate         string
	AllowRollback     bool
}

func runMigrationRunRetention(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("migration-run-retention", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var buildInfoPath string
	var opsBaseURL string
	var authdOpsBaseURL string
	var migrationRunsBase string
	var targetVersion string
	var lockFile string
	var gamedLogPath string
	var authdLogPath string
	var allowRollback bool
	flags.StringVar(&buildInfoPath, "build-info", "", "path to retained /local/build-info or metin2-migrate version JSON, or - for stdin")
	flags.StringVar(&opsBaseURL, "ops-base-url", defaultMigrationRunOpsBaseURL, "loopback gamed ops base URL used in printed curl commands")
	flags.StringVar(&authdOpsBaseURL, "authd-ops-base-url", defaultMigrationRunAuthdOpsBaseURL, "loopback authd ops base URL used in printed curl commands")
	flags.StringVar(&migrationRunsBase, "migration-runs-base", defaultMigrationRunsBase, "absolute migration-runs root used in printed retention commands")
	flags.StringVar(&targetVersion, "target-version", defaultMigrationRunTargetVersion, "plan/apply target version printed into the retention script")
	flags.StringVar(&lockFile, "lock-file", defaultMigrationRunLockFile, "apply lock file name or absolute path printed into the retention script")
	flags.StringVar(&gamedLogPath, "gamed-log-path", defaultMigrationRunGamedLogPath, "absolute gamed JSON log path optionally copied into the retention tree")
	flags.StringVar(&authdLogPath, "authd-log-path", defaultMigrationRunAuthdLogPath, "absolute authd JSON log path optionally copied into the retention tree")
	flags.BoolVar(&allowRollback, "allow-rollback", false, "print rollback-direction retention commands and require an explicit non-latest target-version")
	flags.Usage = func() { printMigrationRunRetentionUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected migration-run-retention argument %q\n", flags.Arg(0))
		printMigrationRunRetentionUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(buildInfoPath) == "" {
		fmt.Fprintln(stderr, "--build-info is required for migration-run-retention")
		printMigrationRunRetentionUsage(stderr)
		return exitUsage
	}

	lockFileExplicit := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "lock-file" {
			lockFileExplicit = true
		}
	})

	reader, closeReader, err := openMigrationRunBuildInfoReader(buildInfoPath, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "migration-run-retention: %v\n", err)
		return exitError
	}
	if closeReader != nil {
		defer closeReader()
	}

	raw, err := readBoundedMigrationRunBuildInfo(reader)
	if err != nil {
		fmt.Fprintf(stderr, "migration-run-retention: %v\n", err)
		return exitError
	}

	plan, err := buildMigrationRunRetentionPlan(raw, opsBaseURL, authdOpsBaseURL, migrationRunsBase, targetVersion, lockFile, gamedLogPath, authdLogPath, lockFileExplicit, allowRollback)
	if err != nil {
		fmt.Fprintf(stderr, "migration-run-retention: %v\n", err)
		return exitError
	}
	if _, err := io.WriteString(stdout, renderMigrationRunRetentionScript(plan)); err != nil {
		fmt.Fprintf(stderr, "migration-run-retention: write script: %v\n", err)
		return exitError
	}
	return exitOK
}

func openMigrationRunBuildInfoReader(path string, stdin io.Reader) (io.Reader, func(), error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "-" {
		return stdin, nil, nil
	}
	info, err := os.Lstat(trimmed)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: stat build-info: %v", errInvalidMigrationRunRetentionInput, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, nil, fmt.Errorf("%w: build-info must not be a symlink: %s", errInvalidMigrationRunRetentionInput, trimmed)
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("%w: build-info must be a regular file: %s", errInvalidMigrationRunRetentionInput, trimmed)
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: open build-info: %v", errInvalidMigrationRunRetentionInput, err)
	}
	openedInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, fmt.Errorf("%w: stat opened build-info: %v", errInvalidMigrationRunRetentionInput, err)
	}
	if !openedInfo.Mode().IsRegular() {
		_ = file.Close()
		return nil, nil, fmt.Errorf("%w: opened build-info must be a regular file: %s", errInvalidMigrationRunRetentionInput, trimmed)
	}
	return file, func() { _ = file.Close() }, nil
}

func readBoundedMigrationRunBuildInfo(reader io.Reader) ([]byte, error) {
	if reader == nil {
		reader = strings.NewReader("")
	}
	raw, err := io.ReadAll(io.LimitReader(reader, maxMigrationRunBuildInfoBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read build-info: %v", errInvalidMigrationRunRetentionInput, err)
	}
	if len(raw) > maxMigrationRunBuildInfoBytes {
		return nil, fmt.Errorf("%w: build-info exceeds %d bytes", errInvalidMigrationRunRetentionInput, maxMigrationRunBuildInfoBytes)
	}
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("%w: build-info is not valid UTF-8", errInvalidMigrationRunRetentionInput)
	}
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, fmt.Errorf("%w: build-info is empty", errInvalidMigrationRunRetentionInput)
	}
	return raw, nil
}

func buildMigrationRunRetentionPlan(raw []byte, opsBaseURL, authdOpsBaseURL, migrationRunsBase, targetVersion, lockFile, gamedLogPath, authdLogPath string, lockFileExplicit, allowRollback bool) (migrationRunRetentionPlan, error) {
	var snapshot migrationRunBuildInfo
	if err := decodeStrictMigrationRunBuildInfoJSON(raw, &snapshot); err != nil {
		return migrationRunRetentionPlan{}, err
	}

	commit := strings.TrimSpace(snapshot.Commit)
	if commit == "" {
		return migrationRunRetentionPlan{}, fmt.Errorf("%w: commit is required", errInvalidMigrationRunRetentionInput)
	}
	commit12 := commit
	if len(commit12) > migrationRunCommitSuffixMaxRunes {
		commit12 = commit12[:migrationRunCommitSuffixMaxRunes]
	}

	normalizedOps, err := normalizeMigrationRunOpsBaseURL(opsBaseURL)
	if err != nil {
		return migrationRunRetentionPlan{}, err
	}
	normalizedAuthdOps, err := normalizeMigrationRunOpsBaseURLLabeled(authdOpsBaseURL, "authd-ops-base-url")
	if err != nil {
		return migrationRunRetentionPlan{}, err
	}
	normalizedRunsBase, err := normalizeMigrationRunAbsolutePath(migrationRunsBase, "migration-runs-base")
	if err != nil {
		return migrationRunRetentionPlan{}, err
	}
	normalizedGamedLog, err := normalizeMigrationRunAbsolutePath(gamedLogPath, "gamed-log-path")
	if err != nil {
		return migrationRunRetentionPlan{}, err
	}
	normalizedAuthdLog, err := normalizeMigrationRunAbsolutePath(authdLogPath, "authd-log-path")
	if err != nil {
		return migrationRunRetentionPlan{}, err
	}

	trimmedTarget := strings.TrimSpace(targetVersion)
	if trimmedTarget == "" {
		return migrationRunRetentionPlan{}, fmt.Errorf("%w: target-version is required", errInvalidMigrationRunRetentionInput)
	}
	if allowRollback && trimmedTarget == defaultMigrationRunTargetVersion {
		return migrationRunRetentionPlan{}, fmt.Errorf("%w: --allow-rollback requires an explicit non-latest --target-version", errInvalidMigrationRunRetentionInput)
	}

	trimmedLock := strings.TrimSpace(lockFile)
	if trimmedLock == "" {
		return migrationRunRetentionPlan{}, fmt.Errorf("%w: lock-file is required", errInvalidMigrationRunRetentionInput)
	}
	if !lockFileExplicit && allowRollback {
		trimmedLock = defaultMigrationRunRollbackLockFile
	}
	if filepath.IsAbs(trimmedLock) {
		trimmedLock = filepath.Clean(trimmedLock)
	}

	return migrationRunRetentionPlan{
		OpsBaseURL:        normalizedOps,
		AuthdOpsBaseURL:   normalizedAuthdOps,
		MigrationRunsBase: normalizedRunsBase,
		GamedLogPath:      normalizedGamedLog,
		AuthdLogPath:      normalizedAuthdLog,
		TargetVersion:     trimmedTarget,
		LockFile:          trimmedLock,
		Commit12:          commit12,
		BuildVersion:      strings.TrimSpace(snapshot.Version),
		BuildCommit:       commit,
		BuildDate:         strings.TrimSpace(snapshot.BuildDate),
		AllowRollback:     allowRollback,
	}, nil
}

func decodeStrictMigrationRunBuildInfoJSON(raw []byte, dest *migrationRunBuildInfo) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("%w: decode build-info: %v", errInvalidMigrationRunRetentionInput, err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("%w: build-info has trailing JSON", errInvalidMigrationRunRetentionInput)
	}
	return nil
}

func normalizeMigrationRunOpsBaseURL(raw string) (string, error) {
	return normalizeMigrationRunOpsBaseURLLabeled(raw, "ops-base-url")
}

func normalizeMigrationRunOpsBaseURLLabeled(raw string, label string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: %s is required", errInvalidMigrationRunRetentionInput, label)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("%w: %s: %v", errInvalidMigrationRunRetentionInput, label, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%w: %s scheme must be http or https", errInvalidMigrationRunRetentionInput, label)
	}
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("%w: %s must be an absolute http(s) URL with a host and no query/fragment", errInvalidMigrationRunRetentionInput, label)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if parsed.Path == "/" {
		parsed.Path = ""
	}
	return parsed.String(), nil
}

func normalizeMigrationRunAbsolutePath(raw string, label string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: %s is required", errInvalidMigrationRunRetentionInput, label)
	}
	if !filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("%w: %s must be an absolute path", errInvalidMigrationRunRetentionInput, label)
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." {
		return "", fmt.Errorf("%w: %s is invalid", errInvalidMigrationRunRetentionInput, label)
	}
	return cleaned, nil
}

func renderMigrationRunRetentionScript(plan migrationRunRetentionPlan) string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("# read-only printer: does not execute migration apply/rollback\n")
	b.WriteString("# Generated from a retained /local/build-info (or metin2-migrate version) snapshot\n")
	b.WriteString("# for docs/workflow/migration-apply-runbook.md and docs/workflow/lab-deployment-topology.md\n")
	b.WriteString("# require operator-exported DRIVER/DSN; printer never embeds a DSN\n")
	b.WriteString("set -eu\n")
	b.WriteString("\n")
	fmt.Fprintf(&b, "OPS=%s\n", shellSingleQuote(plan.OpsBaseURL))
	fmt.Fprintf(&b, "AUTH_OPS=%s\n", shellSingleQuote(plan.AuthdOpsBaseURL))
	fmt.Fprintf(&b, "RUNS_BASE=%s\n", shellSingleQuote(plan.MigrationRunsBase))
	fmt.Fprintf(&b, "GAMED_LOG=%s\n", shellSingleQuote(plan.GamedLogPath))
	fmt.Fprintf(&b, "AUTHD_LOG=%s\n", shellSingleQuote(plan.AuthdLogPath))
	fmt.Fprintf(&b, "TARGET_VERSION=%s\n", shellSingleQuote(plan.TargetVersion))
	fmt.Fprintf(&b, "LOCK_FILE=%s\n", shellSingleQuote(plan.LockFile))
	fmt.Fprintf(&b, "COMMIT12=%s\n", shellSingleQuote(plan.Commit12))
	fmt.Fprintf(&b, "BUILD_VERSION=%s\n", shellSingleQuote(plan.BuildVersion))
	fmt.Fprintf(&b, "BUILD_COMMIT=%s\n", shellSingleQuote(plan.BuildCommit))
	fmt.Fprintf(&b, "BUILD_DATE=%s\n", shellSingleQuote(plan.BuildDate))
	b.WriteString("\n")
	b.WriteString("TS=$(date -u +%Y%m%dT%H%M%SZ)\n")
	b.WriteString(`RUN="${RUNS_BASE}/${TS}-${COMMIT12}"` + "\n")
	b.WriteString("\n")
	b.WriteString("echo '== prepare migration-runs tree =='\n")
	b.WriteString(`mkdir -p "$RUN"` + "\n")
	b.WriteString("\n")
	b.WriteString("echo '== retain daemon identity / runtime correlation =='\n")
	b.WriteString(`curl -sS "$OPS/local/build-info" > "$RUN/gamed-build-info.json"` + "\n")
	b.WriteString(`curl -sS "$AUTH_OPS/local/build-info" > "$RUN/authd-build-info.json"` + "\n")
	b.WriteString(`curl -sS "$OPS/local/runtime-config" > "$RUN/runtime-config.json"` + "\n")
	b.WriteString(`curl -sS "$OPS/local/persistence/status" > "$RUN/persistence-status-before.json"` + "\n")
	b.WriteString(`# optional when a daemon is configured against the migration target:` + "\n")
	b.WriteString(`curl -sS "$OPS/local/db/migrations/status" > "$RUN/daemon-migrations-status.json"` + "\n")
	b.WriteString("\n")
	b.WriteString("echo '== optional retain daemon JSON logs =='\n")
	b.WriteString(`# Missing files are non-fatal when unit samples have not been renamed yet.` + "\n")
	b.WriteString(`if [ -f "$GAMED_LOG" ]; then cp -p "$GAMED_LOG" "$RUN/gamed.log"; fi` + "\n")
	b.WriteString(`if [ -f "$AUTHD_LOG" ]; then cp -p "$AUTHD_LOG" "$RUN/authd.log"; fi` + "\n")
	b.WriteString("\n")
	b.WriteString("echo '== operator notes stub =='\n")
	b.WriteString(`cat > "$RUN/notes.md" <<'EOF'` + "\n")
	b.WriteString("# Migration window notes\n")
	b.WriteString("\n")
	b.WriteString("- Operator:\n")
	b.WriteString("- Window start (UTC):\n")
	b.WriteString("- Target version:\n")
	b.WriteString("- Reviewed plan/preflight checksums:\n")
	b.WriteString("- Backup evidence path:\n")
	b.WriteString("- Outcome / follow-ups:\n")
	b.WriteString("\n")
	b.WriteString("Do not paste DSNs, passwords, login keys, tickets, or executable SQL here.\n")
	b.WriteString("EOF\n")
	b.WriteString("\n")

	planArtifact := "migration-plan-artifact.json"
	planArtifactStatus := "plan-artifact-status.json"
	preflightArtifact := "apply-preflight.json"
	preflightStatus := "apply-preflight-status.json"
	auditArtifact := "migration-apply-audit.json"
	auditStatus := "apply-audit-status.json"
	postStatus := "post-apply-status.json"
	offlineEcho := "echo '== offline catalog / ledger / plan / preflight =='\n"
	mutateEcho := "echo '== mutating apply (after deployment-specific DB/file-store backups) =='\n"
	postEcho := "echo '== post-apply retention =='\n"
	if plan.AllowRollback {
		planArtifact = "rollback-plan-artifact.json"
		planArtifactStatus = "rollback-plan-artifact-status.json"
		preflightArtifact = "rollback-apply-preflight.json"
		preflightStatus = "rollback-apply-preflight-status.json"
		auditArtifact = "migration-rollback-audit.json"
		auditStatus = "rollback-apply-audit-status.json"
		postStatus = "post-rollback-status.json"
		offlineEcho = "echo '== offline catalog / ledger / rollback plan / preflight =='\n"
		mutateEcho = "echo '== mutating rollback apply (after deployment-specific DB/file-store backups) =='\n"
		postEcho = "echo '== post-rollback retention =='\n"
	}

	b.WriteString(offlineEcho)
	b.WriteString(`metin2-migrate catalog > "$RUN/migration-catalog.json"` + "\n")
	b.WriteString(`: "${DRIVER:?export DRIVER to the database/sql driver name}"` + "\n")
	b.WriteString(`: "${DSN:?export DSN to the operator-managed database/sql DSN}"` + "\n")
	b.WriteString(`metin2-migrate ledger-snapshot \` + "\n")
	b.WriteString(`  --driver "$DRIVER" \` + "\n")
	b.WriteString(`  --dsn "$DSN" \` + "\n")
	b.WriteString(`  > "$RUN/ledger-snapshot.json"` + "\n")
	b.WriteString(`metin2-migrate ledger-snapshot-status \` + "\n")
	b.WriteString(`  --ledger-snapshot "$RUN/ledger-snapshot.json" \` + "\n")
	b.WriteString(`  > "$RUN/ledger-snapshot-status.json"` + "\n")
	b.WriteString(`metin2-migrate plan-artifact \` + "\n")
	b.WriteString(`  --ledger-snapshot "$RUN/ledger-snapshot.json" \` + "\n")
	b.WriteString(`  --target-version "$TARGET_VERSION" \` + "\n")
	fmt.Fprintf(&b, "  > \"$RUN/%s\"\n", planArtifact)
	b.WriteString(`metin2-migrate plan-artifact-status \` + "\n")
	fmt.Fprintf(&b, "  --plan-artifact \"$RUN/%s\" \\\n", planArtifact)
	fmt.Fprintf(&b, "  > \"$RUN/%s\"\n", planArtifactStatus)
	b.WriteString(`metin2-migrate apply-preflight \` + "\n")
	b.WriteString(`  --ledger-snapshot "$RUN/ledger-snapshot.json" \` + "\n")
	b.WriteString(`  --target-version "$TARGET_VERSION" \` + "\n")
	fmt.Fprintf(&b, "  --plan-artifact \"$RUN/%s\" \\\n", planArtifact)
	if plan.AllowRollback {
		b.WriteString(`  --allow-rollback \` + "\n")
	}
	fmt.Fprintf(&b, "  > \"$RUN/%s\"\n", preflightArtifact)
	b.WriteString(`metin2-migrate apply-preflight-status \` + "\n")
	fmt.Fprintf(&b, "  --apply-preflight \"$RUN/%s\" \\\n", preflightArtifact)
	fmt.Fprintf(&b, "  > \"$RUN/%s\"\n", preflightStatus)
	b.WriteString("\n")
	b.WriteString(mutateEcho)
	b.WriteString(`metin2-migrate apply \` + "\n")
	b.WriteString(`  --driver "$DRIVER" \` + "\n")
	b.WriteString(`  --dsn "$DSN" \` + "\n")
	b.WriteString(`  --ledger-snapshot "$RUN/ledger-snapshot.json" \` + "\n")
	b.WriteString(`  --target-version "$TARGET_VERSION" \` + "\n")
	fmt.Fprintf(&b, "  --apply-preflight \"$RUN/%s\" \\\n", preflightArtifact)
	if plan.AllowRollback {
		b.WriteString(`  --allow-rollback \` + "\n")
	}
	b.WriteString(`  --lock-file "$RUN/$LOCK_FILE" \` + "\n")
	fmt.Fprintf(&b, "  --audit-file \"$RUN/%s\"\n", auditArtifact)
	b.WriteString("\n")
	b.WriteString(postEcho)
	b.WriteString(`metin2-migrate apply-audit-status \` + "\n")
	fmt.Fprintf(&b, "  --audit-file \"$RUN/%s\" \\\n", auditArtifact)
	fmt.Fprintf(&b, "  > \"$RUN/%s\"\n", auditStatus)
	b.WriteString(`metin2-migrate status \` + "\n")
	b.WriteString(`  --driver "$DRIVER" \` + "\n")
	b.WriteString(`  --dsn "$DSN" \` + "\n")
	b.WriteString(`  --target-version "$TARGET_VERSION" \` + "\n")
	fmt.Fprintf(&b, "  > \"$RUN/%s\"\n", postStatus)
	b.WriteString(`curl -sS "$OPS/local/persistence/status" > "$RUN/persistence-status-after.json"` + "\n")
	b.WriteString("\n")
	b.WriteString("echo '== optional lab stale-lock triage / aside-rename =='\n")
	b.WriteString(`# Only when apply fails because "$RUN/$LOCK_FILE" already exists.` + "\n")
	b.WriteString(`# Follow docs/workflow/lab-stale-lock-recovery.md; confirmation is required.` + "\n")
	b.WriteString(`metin2-migrate apply-lock-status --lock-file "$RUN/$LOCK_FILE" > "$RUN/apply-lock-status.json"` + "\n")
	b.WriteString(`metin2-migrate apply-lock-aside --lock-file "$RUN/$LOCK_FILE" --i-confirm-lab-aside-rename > "$RUN/apply-lock-aside.json"` + "\n")
	b.WriteString(`# Successful aside leaves "$RUN/$LOCK_FILE.stale-<UTC>" beside the retained JSON.` + "\n")
	return b.String()
}

func printMigrationRunRetentionUsage(w io.Writer) {
	fmt.Fprintln(w, "migration-run-retention usage:")
	fmt.Fprintln(w, "  metin2-migrate migration-run-retention --build-info <path|-> [--ops-base-url http://127.0.0.1:6060] [--authd-ops-base-url http://127.0.0.1:6061] [--migration-runs-base /var/metin2/migration-runs] [--target-version latest] [--lock-file migration-apply.lock|migration-rollback.lock] [--gamed-log-path /var/log/metin2/gamed.log] [--authd-log-path /var/log/metin2/authd.log] [--allow-rollback]")
}
