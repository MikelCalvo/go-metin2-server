package migratecli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

const (
	statusStatusFormat   = "go-metin2-migration-status-status-v1"
	maxStatusStatusBytes = 64 * 1024
)

// ErrStatusStatus reports a fail-closed retained Plan inspection failure.
var ErrStatusStatus = errors.New("status-status failed")

type statusStatus struct {
	Format                string             `json:"format"`
	Present               bool               `json:"present"`
	StatusSHA256          string             `json:"status_sha256,omitempty"`
	MatchesEmbeddedLatest *bool              `json:"matches_embedded_latest,omitempty"`
	Plan                  *dbmigrations.Plan `json:"plan,omitempty"`
}

func runStatusStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("status-status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var statusPath string
	var requireUpToDate bool
	var requireMatchesEmbeddedLatest bool
	flags.StringVar(&statusPath, "status", "", "path to a retained metin2-migrate status Plan JSON file")
	flags.BoolVar(&requireUpToDate, "require-up-to-date", false, "fail closed unless the retained Plan is up_to_date")
	flags.BoolVar(&requireMatchesEmbeddedLatest, "require-matches-embedded-latest", false, "fail closed unless plan.latest_version matches the inspecting catalog")
	flags.Usage = func() { printStatusStatusUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected status-status argument %q\n", flags.Arg(0))
		printStatusStatusUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(statusPath) == "" {
		fmt.Fprintln(stderr, "--status is required for status-status")
		printStatusStatusUsage(stderr)
		return exitUsage
	}

	raw, present, err := readOptionalStatusStatusFile(statusPath)
	if err != nil {
		fmt.Fprintf(stderr, "status-status: %v\n", err)
		return exitError
	}
	if !present {
		if err := enforceStatusStatusRequireGates(false, false, false, requireUpToDate, requireMatchesEmbeddedLatest); err != nil {
			fmt.Fprintf(stderr, "status-status: %v\n", err)
			return exitError
		}
		return writeJSON(stdout, stderr, statusStatus{
			Format:  statusStatusFormat,
			Present: false,
		})
	}

	plan, err := decodeRetainedStatusPlan(raw)
	if err != nil {
		fmt.Fprintf(stderr, "status-status: %v\n", err)
		return exitError
	}
	if _, err := validateMigrationPlanShape(plan, ErrStatusStatus, "status"); err != nil {
		fmt.Fprintf(stderr, "status-status: %v\n", err)
		return exitError
	}
	matchesEmbeddedLatest, err := statusPlanMatchesEmbeddedLatest(plan)
	if err != nil {
		fmt.Fprintf(stderr, "status-status: %v\n", err)
		return exitError
	}
	if err := enforceStatusStatusRequireGates(true, plan.UpToDate, matchesEmbeddedLatest, requireUpToDate, requireMatchesEmbeddedLatest); err != nil {
		fmt.Fprintf(stderr, "status-status: %v\n", err)
		return exitError
	}
	return writeJSON(stdout, stderr, statusStatus{
		Format:                statusStatusFormat,
		Present:               true,
		StatusSHA256:          sha256Hex(raw),
		MatchesEmbeddedLatest: boolPtr(matchesEmbeddedLatest),
		Plan:                  &plan,
	})
}

func enforceStatusStatusRequireGates(present, upToDate, matchesEmbeddedLatest, requireUpToDate, requireMatchesEmbeddedLatest bool) error {
	if requireUpToDate {
		if !present {
			return fmt.Errorf("--require-up-to-date failed: status is absent")
		}
		if !upToDate {
			return fmt.Errorf("--require-up-to-date failed: up_to_date=false")
		}
	}
	if requireMatchesEmbeddedLatest {
		if !present {
			return fmt.Errorf("--require-matches-embedded-latest failed: status is absent")
		}
		if !matchesEmbeddedLatest {
			return fmt.Errorf("--require-matches-embedded-latest failed: matches_embedded_latest=false")
		}
	}
	return nil
}

func readOptionalStatusStatusFile(path string) ([]byte, bool, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, false, fmt.Errorf("%w: status path is required", ErrStatusStatus)
	}
	info, err := os.Lstat(trimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("%w: stat status: %v", ErrStatusStatus, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, fmt.Errorf("%w: status must not be a symlink: %s", ErrStatusStatus, trimmed)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%w: status must be a regular file: %s", ErrStatusStatus, trimmed)
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return nil, false, fmt.Errorf("%w: open status: %v", ErrStatusStatus, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("%w: stat opened status: %v", ErrStatusStatus, err)
	}
	if !openedInfo.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%w: opened status must be a regular file: %s", ErrStatusStatus, trimmed)
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxStatusStatusBytes+1))
	if err != nil {
		return nil, false, fmt.Errorf("%w: read status: %v", ErrStatusStatus, err)
	}
	if len(raw) > maxStatusStatusBytes {
		return nil, false, fmt.Errorf("%w: status exceeds %d bytes", ErrStatusStatus, maxStatusStatusBytes)
	}
	if !utf8.Valid(raw) {
		return nil, false, fmt.Errorf("%w: status is not valid UTF-8", ErrStatusStatus)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, false, fmt.Errorf("%w: status is empty", ErrStatusStatus)
	}
	return raw, true, nil
}

func decodeRetainedStatusPlan(raw []byte) (dbmigrations.Plan, error) {
	var formatEnvelope struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(raw, &formatEnvelope); err != nil {
		return dbmigrations.Plan{}, fmt.Errorf("%w: decode status: %v", ErrStatusStatus, err)
	}
	if formatEnvelope.Format != "" {
		return dbmigrations.Plan{}, fmt.Errorf("%w: unexpected format %q", ErrStatusStatus, formatEnvelope.Format)
	}
	var plan dbmigrations.Plan
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return dbmigrations.Plan{}, fmt.Errorf("%w: decode status: %v", ErrStatusStatus, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return dbmigrations.Plan{}, fmt.Errorf("%w: status has trailing JSON", ErrStatusStatus)
	}
	return plan, nil
}

func statusPlanMatchesEmbeddedLatest(plan dbmigrations.Plan) (bool, error) {
	catalog, err := dbmigrations.Catalog()
	if err != nil {
		return false, fmt.Errorf("%w: load embedded catalog: %v", ErrStatusStatus, err)
	}
	return plan.LatestVersion == len(catalog), nil
}

func printStatusStatusUsage(w io.Writer) {
	fmt.Fprintln(w, "status-status usage:")
	fmt.Fprintln(w, "  metin2-migrate status-status --status <path> [--require-up-to-date] [--require-matches-embedded-latest]")
}
