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
	maxExportQuarantineDrillBuildInfoBytes      = 64 * 1024
	defaultExportQuarantineDrillOpsBaseURL      = "http://127.0.0.1:6060"
	defaultExportQuarantineDrillAuthdOpsBaseURL = "http://127.0.0.1:6061"
	defaultExportQuarantineDrillExportBase      = "/var/metin2/exports"
	defaultExportQuarantineDrillGamedLogPath    = "/var/log/metin2/gamed.log"
	defaultExportQuarantineDrillAuthdLogPath    = "/var/log/metin2/authd.log"
	exportQuarantineDrillCommitSuffixMax        = 12
)

var errInvalidExportQuarantineDrillInput = errors.New("invalid export-quarantine-drill input")

type exportQuarantineDrillBuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

type exportQuarantineDrillKind struct {
	Kind       string
	ExportPath string
}

type exportQuarantineDrillPlan struct {
	OpsBaseURL      string
	AuthdOpsBaseURL string
	ExportBase      string
	GamedLogPath    string
	AuthdLogPath    string
	Commit12        string
	BuildVersion    string
	BuildCommit     string
	BuildDate       string
	Kinds           []exportQuarantineDrillKind
}

func exportQuarantineDrillKinds() []exportQuarantineDrillKind {
	return []exportQuarantineDrillKind{
		{Kind: "account-character-roster", ExportPath: "/local/account-store/exports/account-character-roster"},
		{Kind: "character-item-state", ExportPath: "/local/account-store/exports/character-item-state"},
		{Kind: "character-point-state", ExportPath: "/local/account-store/exports/character-point-state"},
		{Kind: "character-myshop-unit-prices", ExportPath: "/local/account-store/exports/character-myshop-unit-prices"},
		{Kind: "auth-login-ticket-handoff", ExportPath: "/local/login-tickets/exports/auth-login-ticket-handoff"},
		{Kind: "character-quest-state", ExportPath: "/local/quest-state/exports/character-quest-state"},
		{Kind: "character-safebox-state", ExportPath: "/local/safebox-store/exports/character-safebox-state"},
		{Kind: "item-template-state", ExportPath: "/local/item-templates/exports/item-template-state"},
		{Kind: "static-actor-content-state", ExportPath: "/local/static-actors/exports/static-actor-content-state"},
		{Kind: "bootstrap-ground-item-state", ExportPath: "/local/ground-items/exports/bootstrap-ground-item-state"},
	}
}

func runExportQuarantineDrill(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("export-quarantine-drill", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var buildInfoPath string
	var opsBaseURL string
	var authdOpsBaseURL string
	var exportBase string
	var gamedLogPath string
	var authdLogPath string
	flags.StringVar(&buildInfoPath, "build-info", "", "path to retained /local/build-info or metin2-migrate version JSON, or - for stdin")
	flags.StringVar(&opsBaseURL, "ops-base-url", defaultExportQuarantineDrillOpsBaseURL, "loopback gamed ops base URL used in printed curl commands")
	flags.StringVar(&authdOpsBaseURL, "authd-ops-base-url", defaultExportQuarantineDrillAuthdOpsBaseURL, "loopback authd ops base URL used in printed curl commands")
	flags.StringVar(&exportBase, "export-base", defaultExportQuarantineDrillExportBase, "absolute export retention root used in printed drill commands")
	flags.StringVar(&gamedLogPath, "gamed-log-path", defaultExportQuarantineDrillGamedLogPath, "absolute gamed JSON log path optionally copied into the retention tree")
	flags.StringVar(&authdLogPath, "authd-log-path", defaultExportQuarantineDrillAuthdLogPath, "absolute authd JSON log path optionally copied into the retention tree")
	flags.Usage = func() { printExportQuarantineDrillUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected export-quarantine-drill argument %q\n", flags.Arg(0))
		printExportQuarantineDrillUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(buildInfoPath) == "" {
		fmt.Fprintln(stderr, "--build-info is required for export-quarantine-drill")
		printExportQuarantineDrillUsage(stderr)
		return exitUsage
	}

	reader, closeReader, err := openExportQuarantineDrillBuildInfoReader(buildInfoPath, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "export-quarantine-drill: %v\n", err)
		return exitError
	}
	if closeReader != nil {
		defer closeReader()
	}

	raw, err := readBoundedExportQuarantineDrillBuildInfo(reader)
	if err != nil {
		fmt.Fprintf(stderr, "export-quarantine-drill: %v\n", err)
		return exitError
	}

	plan, err := buildExportQuarantineDrillPlan(raw, opsBaseURL, authdOpsBaseURL, exportBase, gamedLogPath, authdLogPath)
	if err != nil {
		fmt.Fprintf(stderr, "export-quarantine-drill: %v\n", err)
		return exitError
	}
	if _, err := io.WriteString(stdout, renderExportQuarantineDrillScript(plan)); err != nil {
		fmt.Fprintf(stderr, "export-quarantine-drill: write script: %v\n", err)
		return exitError
	}
	return exitOK
}

func openExportQuarantineDrillBuildInfoReader(path string, stdin io.Reader) (io.Reader, func(), error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "-" {
		return stdin, nil, nil
	}
	info, err := os.Lstat(trimmed)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: stat build-info: %v", errInvalidExportQuarantineDrillInput, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, nil, fmt.Errorf("%w: build-info must not be a symlink: %s", errInvalidExportQuarantineDrillInput, trimmed)
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("%w: build-info must be a regular file: %s", errInvalidExportQuarantineDrillInput, trimmed)
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: open build-info: %v", errInvalidExportQuarantineDrillInput, err)
	}
	openedInfo, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, fmt.Errorf("%w: stat opened build-info: %v", errInvalidExportQuarantineDrillInput, err)
	}
	if !openedInfo.Mode().IsRegular() {
		_ = file.Close()
		return nil, nil, fmt.Errorf("%w: opened build-info must be a regular file: %s", errInvalidExportQuarantineDrillInput, trimmed)
	}
	return file, func() { _ = file.Close() }, nil
}

func readBoundedExportQuarantineDrillBuildInfo(reader io.Reader) ([]byte, error) {
	if reader == nil {
		reader = strings.NewReader("")
	}
	raw, err := io.ReadAll(io.LimitReader(reader, maxExportQuarantineDrillBuildInfoBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read build-info: %v", errInvalidExportQuarantineDrillInput, err)
	}
	if len(raw) > maxExportQuarantineDrillBuildInfoBytes {
		return nil, fmt.Errorf("%w: build-info exceeds %d bytes", errInvalidExportQuarantineDrillInput, maxExportQuarantineDrillBuildInfoBytes)
	}
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("%w: build-info is not valid UTF-8", errInvalidExportQuarantineDrillInput)
	}
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, fmt.Errorf("%w: build-info is empty", errInvalidExportQuarantineDrillInput)
	}
	return raw, nil
}

func buildExportQuarantineDrillPlan(buildInfoRaw []byte, opsBaseURL, authdOpsBaseURL, exportBase, gamedLogPath, authdLogPath string) (exportQuarantineDrillPlan, error) {
	var buildInfo exportQuarantineDrillBuildInfo
	if err := decodeStrictExportQuarantineDrillBuildInfoJSON(buildInfoRaw, &buildInfo); err != nil {
		return exportQuarantineDrillPlan{}, err
	}

	commit := strings.TrimSpace(buildInfo.Commit)
	if commit == "" {
		return exportQuarantineDrillPlan{}, fmt.Errorf("%w: commit is required", errInvalidExportQuarantineDrillInput)
	}
	commit12 := commit
	if len(commit12) > exportQuarantineDrillCommitSuffixMax {
		commit12 = commit12[:exportQuarantineDrillCommitSuffixMax]
	}

	normalizedOps, err := normalizeExportQuarantineDrillOpsBaseURLLabeled(opsBaseURL, "ops-base-url")
	if err != nil {
		return exportQuarantineDrillPlan{}, err
	}
	normalizedAuthdOps, err := normalizeExportQuarantineDrillOpsBaseURLLabeled(authdOpsBaseURL, "authd-ops-base-url")
	if err != nil {
		return exportQuarantineDrillPlan{}, err
	}
	normalizedExportBase, err := normalizeExportQuarantineDrillAbsolutePath(exportBase, "export-base")
	if err != nil {
		return exportQuarantineDrillPlan{}, err
	}
	normalizedGamedLog, err := normalizeExportQuarantineDrillAbsolutePath(gamedLogPath, "gamed-log-path")
	if err != nil {
		return exportQuarantineDrillPlan{}, err
	}
	normalizedAuthdLog, err := normalizeExportQuarantineDrillAbsolutePath(authdLogPath, "authd-log-path")
	if err != nil {
		return exportQuarantineDrillPlan{}, err
	}

	return exportQuarantineDrillPlan{
		OpsBaseURL:      normalizedOps,
		AuthdOpsBaseURL: normalizedAuthdOps,
		ExportBase:      normalizedExportBase,
		GamedLogPath:    normalizedGamedLog,
		AuthdLogPath:    normalizedAuthdLog,
		Commit12:        commit12,
		BuildVersion:    strings.TrimSpace(buildInfo.Version),
		BuildCommit:     commit,
		BuildDate:       strings.TrimSpace(buildInfo.BuildDate),
		Kinds:           exportQuarantineDrillKinds(),
	}, nil
}

func decodeStrictExportQuarantineDrillBuildInfoJSON(raw []byte, dest *exportQuarantineDrillBuildInfo) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("%w: decode build-info: %v", errInvalidExportQuarantineDrillInput, err)
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("%w: build-info has trailing JSON", errInvalidExportQuarantineDrillInput)
	}
	return nil
}

func normalizeExportQuarantineDrillOpsBaseURLLabeled(raw string, label string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: %s is required", errInvalidExportQuarantineDrillInput, label)
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("%w: %s: %v", errInvalidExportQuarantineDrillInput, label, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%w: %s scheme must be http or https", errInvalidExportQuarantineDrillInput, label)
	}
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("%w: %s must be an absolute http(s) URL with a host and no query/fragment", errInvalidExportQuarantineDrillInput, label)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if parsed.Path == "/" {
		parsed.Path = ""
	}
	return parsed.String(), nil
}

func normalizeExportQuarantineDrillAbsolutePath(raw string, label string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("%w: %s is required", errInvalidExportQuarantineDrillInput, label)
	}
	if !filepath.IsAbs(trimmed) {
		return "", fmt.Errorf("%w: %s must be an absolute path", errInvalidExportQuarantineDrillInput, label)
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." {
		return "", fmt.Errorf("%w: %s is invalid", errInvalidExportQuarantineDrillInput, label)
	}
	return cleaned, nil
}

func renderExportQuarantineDrillScript(plan exportQuarantineDrillPlan) string {
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString("# read-only printer: does not execute export/quarantine\n")
	b.WriteString("# Generated from a retained /local/build-info (or metin2-migrate version) snapshot\n")
	b.WriteString("# for docs/workflow/lab-deployment-topology.md and docs/plans/2026-08-25-hermetic-export-quarantine-offline-cli-proof.md\n")
	b.WriteString("set -eu\n")
	b.WriteString("\n")
	fmt.Fprintf(&b, "OPS=%s\n", shellSingleQuote(plan.OpsBaseURL))
	fmt.Fprintf(&b, "AUTH_OPS=%s\n", shellSingleQuote(plan.AuthdOpsBaseURL))
	fmt.Fprintf(&b, "EXPORTS_BASE=%s\n", shellSingleQuote(plan.ExportBase))
	fmt.Fprintf(&b, "GAMED_LOG=%s\n", shellSingleQuote(plan.GamedLogPath))
	fmt.Fprintf(&b, "AUTHD_LOG=%s\n", shellSingleQuote(plan.AuthdLogPath))
	fmt.Fprintf(&b, "COMMIT12=%s\n", shellSingleQuote(plan.Commit12))
	fmt.Fprintf(&b, "BUILD_VERSION=%s\n", shellSingleQuote(plan.BuildVersion))
	fmt.Fprintf(&b, "BUILD_COMMIT=%s\n", shellSingleQuote(plan.BuildCommit))
	fmt.Fprintf(&b, "BUILD_DATE=%s\n", shellSingleQuote(plan.BuildDate))
	b.WriteString("\n")
	b.WriteString("TS=$(date -u +%Y%m%dT%H%M%SZ)\n")
	b.WriteString(`BASE="${EXPORTS_BASE}/${TS}-${COMMIT12}"` + "\n")
	b.WriteString("\n")
	b.WriteString("echo '== prepare lab export/quarantine retention tree =='\n")
	b.WriteString(`mkdir -p "$BASE"/account-character-roster "$BASE"/character-item-state "$BASE"/character-point-state "$BASE"/character-myshop-unit-prices "$BASE"/auth-login-ticket-handoff "$BASE"/character-quest-state "$BASE"/character-safebox-state "$BASE"/item-template-state "$BASE"/static-actor-content-state "$BASE"/bootstrap-ground-item-state` + "\n")
	b.WriteString("\n")
	b.WriteString("echo '== retain daemon identity / runtime correlation =='\n")
	b.WriteString(`curl -sS "$OPS/local/build-info" > "$BASE/gamed-build-info.json"` + "\n")
	b.WriteString(`curl -sS "$AUTH_OPS/local/build-info" > "$BASE/authd-build-info.json"` + "\n")
	b.WriteString(`curl -sS "$OPS/local/runtime-config" > "$BASE/runtime-config.json"` + "\n")
	b.WriteString(`curl -sS "$OPS/healthz"` + "\n")
	b.WriteString(`curl -sS "$OPS/local/db/migrations/catalog" > "$BASE/migration-catalog.json"` + "\n")
	b.WriteString("\n")
	b.WriteString("echo '== optional retain daemon JSON logs =='\n")
	b.WriteString(`# Missing files are non-fatal when unit samples have not been renamed yet.` + "\n")
	b.WriteString(`if [ -f "$GAMED_LOG" ]; then cp -p "$GAMED_LOG" "$BASE/gamed.log"; fi` + "\n")
	b.WriteString(`if [ -f "$AUTHD_LOG" ]; then cp -p "$AUTHD_LOG" "$BASE/authd.log"; fi` + "\n")
	b.WriteString("\n")
	b.WriteString("echo '== operator notes stub =='\n")
	b.WriteString(`cat > "$BASE/notes.md" <<'EOF'` + "\n")
	b.WriteString("# Export/quarantine drill notes\n")
	b.WriteString("\n")
	b.WriteString("- Operator:\n")
	b.WriteString("- Window start (UTC):\n")
	b.WriteString("- Drained selected-character sessions:\n")
	b.WriteString("- Export tree path:\n")
	b.WriteString("- Outcome / follow-ups:\n")
	b.WriteString("\n")
	b.WriteString("Do not paste DSNs, passwords, login keys, tickets, or executable SQL here.\n")
	b.WriteString("EOF\n")
	b.WriteString("\n")
	b.WriteString("echo '== loopback export retain + offline quarantine-export =='\n")
	b.WriteString("# Read-only GET exports plus offline quarantine-export. Does not open a database,\n")
	b.WriteString("# mutate live stores, or import SQL from quarantined artifacts.\n")
	for _, kind := range plan.Kinds {
		fmt.Fprintf(&b, "curl -sS \"$OPS%s\" > \"$BASE/%s/export.json\"\n", kind.ExportPath, kind.Kind)
		fmt.Fprintf(&b, "metin2-migrate quarantine-export --kind %s --export \"$BASE/%s/export.json\" > \"$BASE/%s/quarantine.json\"\n", kind.Kind, kind.Kind, kind.Kind)
	}
	return b.String()
}

func printExportQuarantineDrillUsage(w io.Writer) {
	fmt.Fprintln(w, "export-quarantine-drill usage:")
	fmt.Fprintln(w, "  metin2-migrate export-quarantine-drill --build-info <path|-> [--ops-base-url http://127.0.0.1:6060] [--authd-ops-base-url http://127.0.0.1:6061] [--export-base /var/metin2/exports] [--gamed-log-path /var/log/metin2/gamed.log] [--authd-log-path /var/log/metin2/authd.log]")
}
