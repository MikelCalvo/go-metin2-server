package migratecli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	dbmigrations "github.com/MikelCalvo/go-metin2-server/db/migrations"
)

const (
	catalogStatusFormat   = "go-metin2-migration-catalog-status-v1"
	maxCatalogStatusBytes = 64 * 1024
)

var (
	ErrCatalogStatus     = errors.New("catalog-status failed")
	catalogSummaryNameRe = regexp.MustCompile(`^[a-z0-9_]+$`)
)

type catalogStatus struct {
	Format          string                              `json:"format"`
	Present         bool                                `json:"present"`
	CatalogSHA256   string                              `json:"catalog_sha256,omitempty"`
	MatchesEmbedded *bool                               `json:"matches_embedded,omitempty"`
	Catalog         *dbmigrations.CatalogSummaryPayload `json:"catalog,omitempty"`
}

func runCatalogStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("catalog-status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var catalogPath string
	var requireMatchesEmbedded bool
	flags.StringVar(&catalogPath, "catalog", "", "path to a retained migration catalog summary JSON file")
	flags.BoolVar(&requireMatchesEmbedded, "require-matches-embedded", false, "fail closed unless the retained catalog matches the inspecting binary")
	flags.Usage = func() { printCatalogStatusUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected catalog-status argument %q\n", flags.Arg(0))
		printCatalogStatusUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(catalogPath) == "" {
		fmt.Fprintln(stderr, "--catalog is required for catalog-status")
		printCatalogStatusUsage(stderr)
		return exitUsage
	}

	raw, present, err := readOptionalCatalogStatusFile(catalogPath)
	if err != nil {
		fmt.Fprintf(stderr, "catalog-status: %v\n", err)
		return exitError
	}
	if !present {
		if requireMatchesEmbedded {
			fmt.Fprintln(stderr, "catalog-status: --require-matches-embedded failed: catalog is absent")
			return exitError
		}
		return writeJSON(stdout, stderr, catalogStatus{
			Format:  catalogStatusFormat,
			Present: false,
		})
	}

	summary, err := decodeCatalogSummary(raw)
	if err != nil {
		fmt.Fprintf(stderr, "catalog-status: %v\n", err)
		return exitError
	}
	if err := validateRetainedCatalogSummary(summary); err != nil {
		fmt.Fprintf(stderr, "catalog-status: %v\n", err)
		return exitError
	}
	matches, err := catalogSummaryMatchesEmbedded(summary)
	if err != nil {
		fmt.Fprintf(stderr, "catalog-status: %v\n", err)
		return exitError
	}
	if requireMatchesEmbedded && !matches {
		fmt.Fprintln(stderr, "catalog-status: --require-matches-embedded failed: matches_embedded is false")
		return exitError
	}
	return writeJSON(stdout, stderr, catalogStatus{
		Format:          catalogStatusFormat,
		Present:         true,
		CatalogSHA256:   sha256Hex(raw),
		MatchesEmbedded: boolPtr(matches),
		Catalog:         &summary,
	})
}

func readOptionalCatalogStatusFile(path string) ([]byte, bool, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, false, fmt.Errorf("%w: catalog path is required", ErrCatalogStatus)
	}
	info, err := os.Lstat(trimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("%w: stat catalog: %v", ErrCatalogStatus, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, fmt.Errorf("%w: catalog must not be a symlink: %s", ErrCatalogStatus, trimmed)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%w: catalog must be a regular file: %s", ErrCatalogStatus, trimmed)
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return nil, false, fmt.Errorf("%w: open catalog: %v", ErrCatalogStatus, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("%w: stat opened catalog: %v", ErrCatalogStatus, err)
	}
	if !openedInfo.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%w: opened catalog must be a regular file: %s", ErrCatalogStatus, trimmed)
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxCatalogStatusBytes+1))
	if err != nil {
		return nil, false, fmt.Errorf("%w: read catalog: %v", ErrCatalogStatus, err)
	}
	if len(raw) > maxCatalogStatusBytes {
		return nil, false, fmt.Errorf("%w: catalog exceeds %d bytes", ErrCatalogStatus, maxCatalogStatusBytes)
	}
	if !utf8.Valid(raw) {
		return nil, false, fmt.Errorf("%w: catalog is not valid UTF-8", ErrCatalogStatus)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, false, fmt.Errorf("%w: catalog is empty", ErrCatalogStatus)
	}
	return raw, true, nil
}

func decodeCatalogSummary(raw []byte) (dbmigrations.CatalogSummaryPayload, error) {
	var formatEnvelope struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(raw, &formatEnvelope); err != nil {
		return dbmigrations.CatalogSummaryPayload{}, fmt.Errorf("%w: decode catalog: %v", ErrCatalogStatus, err)
	}
	if formatEnvelope.Format != dbmigrations.CatalogSummaryFormat {
		return dbmigrations.CatalogSummaryPayload{}, fmt.Errorf("%w: unexpected format %q", ErrCatalogStatus, formatEnvelope.Format)
	}
	var summary dbmigrations.CatalogSummaryPayload
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&summary); err != nil {
		return dbmigrations.CatalogSummaryPayload{}, fmt.Errorf("%w: decode catalog: %v", ErrCatalogStatus, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return dbmigrations.CatalogSummaryPayload{}, fmt.Errorf("%w: catalog has trailing JSON", ErrCatalogStatus)
	}
	return summary, nil
}

func validateRetainedCatalogSummary(summary dbmigrations.CatalogSummaryPayload) error {
	if summary.Format != dbmigrations.CatalogSummaryFormat {
		return fmt.Errorf("%w: unexpected format %q", ErrCatalogStatus, summary.Format)
	}
	if summary.LatestVersion < 1 {
		return fmt.Errorf("%w: latest_version must be >= 1, got %d", ErrCatalogStatus, summary.LatestVersion)
	}
	if summary.LatestVersion != len(summary.Migrations) {
		return fmt.Errorf("%w: latest_version %d does not match %d migrations", ErrCatalogStatus, summary.LatestVersion, len(summary.Migrations))
	}
	for i, entry := range summary.Migrations {
		wantVersion := i + 1
		if entry.Version != wantVersion {
			return fmt.Errorf("%w: migration versions must be contiguous from 1, got version %d at position %d", ErrCatalogStatus, entry.Version, wantVersion)
		}
		if !catalogSummaryNameRe.MatchString(entry.Name) {
			return fmt.Errorf("%w: migration %d has malformed name %q", ErrCatalogStatus, entry.Version, entry.Name)
		}
		wantUpPath := fmt.Sprintf("%04d_%s.up.sql", entry.Version, entry.Name)
		wantDownPath := fmt.Sprintf("%04d_%s.down.sql", entry.Version, entry.Name)
		if entry.UpPath != wantUpPath {
			return fmt.Errorf("%w: migration %d has unexpected up_path %q", ErrCatalogStatus, entry.Version, entry.UpPath)
		}
		if entry.DownPath != wantDownPath {
			return fmt.Errorf("%w: migration %d has unexpected down_path %q", ErrCatalogStatus, entry.Version, entry.DownPath)
		}
		if !validCatalogSHA256Hex(entry.UpSHA256) {
			return fmt.Errorf("%w: migration %d has invalid up_sha256", ErrCatalogStatus, entry.Version)
		}
		if !validCatalogSHA256Hex(entry.DownSHA256) {
			return fmt.Errorf("%w: migration %d has invalid down_sha256", ErrCatalogStatus, entry.Version)
		}
	}
	return nil
}

func catalogSummaryMatchesEmbedded(summary dbmigrations.CatalogSummaryPayload) (bool, error) {
	embedded, err := dbmigrations.BuiltInCatalogSummary()
	if err != nil {
		return false, fmt.Errorf("%w: load embedded catalog: %v", ErrCatalogStatus, err)
	}
	if summary.Format != embedded.Format || summary.LatestVersion != embedded.LatestVersion || len(summary.Migrations) != len(embedded.Migrations) {
		return false, nil
	}
	for i, entry := range summary.Migrations {
		want := embedded.Migrations[i]
		if entry.Version != want.Version ||
			entry.Name != want.Name ||
			entry.UpPath != want.UpPath ||
			entry.DownPath != want.DownPath ||
			entry.UpSHA256 != want.UpSHA256 ||
			entry.DownSHA256 != want.DownSHA256 {
			return false, nil
		}
	}
	return true, nil
}

func validCatalogSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return false
	}
	return true
}

func boolPtr(value bool) *bool {
	return &value
}

func printCatalogStatusUsage(w io.Writer) {
	fmt.Fprintln(w, "catalog-status usage:")
	fmt.Fprintln(w, "  metin2-migrate catalog-status --catalog <path> [--require-matches-embedded]")
}
