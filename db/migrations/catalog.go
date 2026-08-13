package migrations

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	manifestFilename = "migrations.manifest.json"
	manifestFormat   = "go-metin2-migration-manifest-v1"

	CatalogSummaryFormat = "go-metin2-migration-catalog-summary-v1"
)

//go:embed *.sql migrations.manifest.json
var embeddedMigrations embed.FS

var ErrInvalidCatalog = errors.New("invalid migration catalog")

// Migration is one validated project-owned SQL migration pair.
//
// The catalog deliberately models only static migration metadata and SQL text.
// Applying migrations to a database is a later production-ops slice.
type Migration struct {
	Version    int
	Name       string
	UpPath     string
	DownPath   string
	UpSQL      string
	DownSQL    string
	UpSHA256   string
	DownSHA256 string
}

// CatalogSummaryPayload is the metadata-only shape safe for operator preflight
// endpoints and runbooks. It pins migration paths and checksums without exposing
// executable SQL text.
type CatalogSummaryPayload struct {
	Format        string                `json:"format"`
	LatestVersion int                   `json:"latest_version"`
	Migrations    []CatalogSummaryEntry `json:"migrations"`
}

// CatalogSummaryEntry is one migration row in a metadata-only catalog summary.
type CatalogSummaryEntry struct {
	Version    int    `json:"version"`
	Name       string `json:"name"`
	UpPath     string `json:"up_path"`
	DownPath   string `json:"down_path"`
	UpSHA256   string `json:"up_sha256"`
	DownSHA256 string `json:"down_sha256"`
}

// Catalog returns the embedded project-owned migration catalog after validating
// naming, pairing, version ordering, per-file headers, and manifest checksums.
func Catalog() ([]Migration, error) {
	return LoadCatalog(embeddedMigrations)
}

// BuiltInCatalogSummary returns a metadata-only summary of the embedded
// project-owned migration catalog. It validates the catalog first and never
// includes executable SQL text.
func BuiltInCatalogSummary() (CatalogSummaryPayload, error) {
	catalog, err := Catalog()
	if err != nil {
		return CatalogSummaryPayload{}, err
	}
	return CatalogSummary(catalog)
}

// CatalogSummary validates a catalog and returns a deterministic metadata-only
// summary suitable for local ops/status endpoints and offline runbooks.
func CatalogSummary(catalog []Migration) (CatalogSummaryPayload, error) {
	if err := validatePlanCatalog(catalog); err != nil {
		return CatalogSummaryPayload{}, err
	}
	entries := make([]CatalogSummaryEntry, 0, len(catalog))
	for _, migration := range catalog {
		entries = append(entries, CatalogSummaryEntry{
			Version:    migration.Version,
			Name:       migration.Name,
			UpPath:     migration.UpPath,
			DownPath:   migration.DownPath,
			UpSHA256:   migration.UpSHA256,
			DownSHA256: migration.DownSHA256,
		})
	}
	return CatalogSummaryPayload{
		Format:        CatalogSummaryFormat,
		LatestVersion: catalog[len(catalog)-1].Version,
		Migrations:    entries,
	}, nil
}

// LoadCatalog validates and returns a deterministic migration catalog from fsys.
//
// Migration files must be flat SQL files named:
//
//	NNNN_name.up.sql
//	NNNN_name.down.sql
//
// Versions must be contiguous starting at 0001, every version must have exactly
// one up/down pair with the same name, and every SQL body must begin with a
// matching project-owned header comment. The mandatory manifest must pin every
// SQL path to its SHA-256 checksum so historical migration drift fails closed:
//
//	-- go-metin2 migration: NNNN name up
//	-- go-metin2 migration: NNNN name down
func LoadCatalog(fsys fs.FS) ([]Migration, error) {
	manifest, err := loadManifest(fsys)
	if err != nil {
		return nil, err
	}

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migration catalog: %w", err)
	}

	byVersion := make(map[int]*migrationPair)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := entry.Name()
		if filename == manifestFilename {
			continue
		}
		if !strings.HasSuffix(filename, ".sql") {
			continue
		}

		parsed, err := parseMigrationFilename(filename)
		if err != nil {
			return nil, err
		}
		raw, err := fs.ReadFile(fsys, filename)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", filename, err)
		}
		body := string(raw)
		if !utf8.ValidString(body) {
			return nil, fmt.Errorf("%w: migration %q is not valid UTF-8", ErrInvalidCatalog, filename)
		}
		if strings.TrimSpace(body) == "" {
			return nil, fmt.Errorf("%w: migration %q has empty SQL body", ErrInvalidCatalog, filename)
		}
		if err := validateMigrationHeader(parsed, body); err != nil {
			return nil, err
		}
		if _, err := splitMigrationSQLStatements(body); err != nil {
			return nil, fmt.Errorf("%w: migration %q has invalid SQL statement boundaries: %v", ErrInvalidCatalog, filename, err)
		}

		sum := sha256Hex(raw)
		if err := manifest.validateSQLFile(parsed, filename, sum); err != nil {
			return nil, err
		}

		pair := byVersion[parsed.version]
		if pair == nil {
			pair = &migrationPair{version: parsed.version, name: parsed.name}
			byVersion[parsed.version] = pair
		}
		if pair.name != parsed.name {
			return nil, fmt.Errorf("%w: migration version %04d has mismatched names %q and %q", ErrInvalidCatalog, parsed.version, pair.name, parsed.name)
		}
		switch parsed.direction {
		case "up":
			if pair.upPath != "" {
				return nil, fmt.Errorf("%w: duplicate up migration for version %04d", ErrInvalidCatalog, parsed.version)
			}
			pair.upPath = filename
			pair.upSQL = body
			pair.upSHA256 = sum
		case "down":
			if pair.downPath != "" {
				return nil, fmt.Errorf("%w: duplicate down migration for version %04d", ErrInvalidCatalog, parsed.version)
			}
			pair.downPath = filename
			pair.downSQL = body
			pair.downSHA256 = sum
		default:
			return nil, fmt.Errorf("%w: unknown migration direction %q", ErrInvalidCatalog, parsed.direction)
		}
	}

	if len(byVersion) == 0 {
		return nil, fmt.Errorf("%w: catalog has no SQL migrations", ErrInvalidCatalog)
	}

	versions := make([]int, 0, len(byVersion))
	for version := range byVersion {
		versions = append(versions, version)
	}
	sort.Ints(versions)

	catalog := make([]Migration, 0, len(versions))
	for i, version := range versions {
		want := i + 1
		if version != want {
			return nil, fmt.Errorf("%w: migration versions must be contiguous from 0001, got %04d at position %04d", ErrInvalidCatalog, version, want)
		}
		pair := byVersion[version]
		if pair.upPath == "" || pair.downPath == "" {
			return nil, fmt.Errorf("%w: migration version %04d must have up and down SQL files", ErrInvalidCatalog, version)
		}
		catalog = append(catalog, Migration{
			Version:    pair.version,
			Name:       pair.name,
			UpPath:     pair.upPath,
			DownPath:   pair.downPath,
			UpSQL:      pair.upSQL,
			DownSQL:    pair.downSQL,
			UpSHA256:   pair.upSHA256,
			DownSHA256: pair.downSHA256,
		})
	}
	if err := manifest.validateCatalog(catalog); err != nil {
		return nil, err
	}
	return catalog, nil
}

type migrationPair struct {
	version    int
	name       string
	upPath     string
	downPath   string
	upSQL      string
	downSQL    string
	upSHA256   string
	downSHA256 string
}

type parsedMigrationFilename struct {
	version   int
	versionID string
	name      string
	direction string
}

func parseMigrationFilename(filename string) (parsedMigrationFilename, error) {
	withoutSQL := strings.TrimSuffix(filename, ".sql")
	dot := strings.LastIndexByte(withoutSQL, '.')
	if dot < 0 {
		return parsedMigrationFilename{}, fmt.Errorf("%w: malformed migration filename %q", ErrInvalidCatalog, filename)
	}
	direction := withoutSQL[dot+1:]
	if direction != "up" && direction != "down" {
		return parsedMigrationFilename{}, fmt.Errorf("%w: malformed migration filename %q", ErrInvalidCatalog, filename)
	}
	versionAndName := withoutSQL[:dot]
	underscore := strings.IndexByte(versionAndName, '_')
	if underscore != 4 || len(versionAndName) <= 5 {
		return parsedMigrationFilename{}, fmt.Errorf("%w: malformed migration filename %q", ErrInvalidCatalog, filename)
	}
	versionID := versionAndName[:underscore]
	name := versionAndName[underscore+1:]
	for _, r := range versionID {
		if r < '0' || r > '9' {
			return parsedMigrationFilename{}, fmt.Errorf("%w: malformed migration version in %q", ErrInvalidCatalog, filename)
		}
	}
	if versionID == "0000" {
		return parsedMigrationFilename{}, fmt.Errorf("%w: migration version must start at 0001 in %q", ErrInvalidCatalog, filename)
	}
	if !validMigrationName(name) {
		return parsedMigrationFilename{}, fmt.Errorf("%w: malformed migration name in %q", ErrInvalidCatalog, filename)
	}
	version, err := strconv.Atoi(versionID)
	if err != nil {
		return parsedMigrationFilename{}, fmt.Errorf("%w: malformed migration version in %q", ErrInvalidCatalog, filename)
	}
	return parsedMigrationFilename{version: version, versionID: versionID, name: name, direction: direction}, nil
}

func validMigrationName(name string) bool {
	if name == "" || strings.HasPrefix(name, "_") || strings.HasSuffix(name, "_") || strings.Contains(name, "__") {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func validateMigrationHeader(parsed parsedMigrationFilename, body string) error {
	trimmed := strings.TrimLeft(body, " \t\r\n")
	line := trimmed
	if newline := strings.IndexByte(line, '\n'); newline >= 0 {
		line = line[:newline]
	}
	line = strings.TrimRight(line, "\r")
	want := fmt.Sprintf("-- go-metin2 migration: %s %s %s", parsed.versionID, parsed.name, parsed.direction)
	if line != want {
		return fmt.Errorf("%w: migration %04d %s must start with header %q", ErrInvalidCatalog, parsed.version, parsed.direction, want)
	}
	return nil
}

type manifestCatalog struct {
	entries map[int]migrationManifestEntry
}

type migrationManifestPayload struct {
	Format     string                   `json:"format"`
	Migrations []migrationManifestEntry `json:"migrations"`
}

type migrationManifestEntry struct {
	Version    int    `json:"version"`
	Name       string `json:"name"`
	UpPath     string `json:"up_path"`
	DownPath   string `json:"down_path"`
	UpSHA256   string `json:"up_sha256"`
	DownSHA256 string `json:"down_sha256"`
}

func loadManifest(fsys fs.FS) (manifestCatalog, error) {
	raw, err := fs.ReadFile(fsys, manifestFilename)
	if err != nil {
		return manifestCatalog{}, fmt.Errorf("%w: read migration manifest %q: %w", ErrInvalidCatalog, manifestFilename, err)
	}
	if !utf8.Valid(raw) {
		return manifestCatalog{}, fmt.Errorf("%w: migration manifest %q is not valid UTF-8", ErrInvalidCatalog, manifestFilename)
	}

	var payload migrationManifestPayload
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return manifestCatalog{}, fmt.Errorf("%w: decode migration manifest %q: %w", ErrInvalidCatalog, manifestFilename, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return manifestCatalog{}, fmt.Errorf("%w: migration manifest %q has trailing JSON", ErrInvalidCatalog, manifestFilename)
	}
	if payload.Format != manifestFormat {
		return manifestCatalog{}, fmt.Errorf("%w: migration manifest %q has unsupported format %q", ErrInvalidCatalog, manifestFilename, payload.Format)
	}
	if len(payload.Migrations) == 0 {
		return manifestCatalog{}, fmt.Errorf("%w: migration manifest %q has no entries", ErrInvalidCatalog, manifestFilename)
	}

	manifest := manifestCatalog{entries: make(map[int]migrationManifestEntry, len(payload.Migrations))}
	for _, entry := range payload.Migrations {
		if entry.Version <= 0 {
			return manifestCatalog{}, fmt.Errorf("%w: migration manifest %q has invalid version %d", ErrInvalidCatalog, manifestFilename, entry.Version)
		}
		if !validMigrationName(entry.Name) {
			return manifestCatalog{}, fmt.Errorf("%w: migration manifest %q has malformed name %q", ErrInvalidCatalog, manifestFilename, entry.Name)
		}
		if _, exists := manifest.entries[entry.Version]; exists {
			return manifestCatalog{}, fmt.Errorf("%w: migration manifest %q has duplicate version %04d", ErrInvalidCatalog, manifestFilename, entry.Version)
		}
		wantUpPath := fmt.Sprintf("%04d_%s.up.sql", entry.Version, entry.Name)
		wantDownPath := fmt.Sprintf("%04d_%s.down.sql", entry.Version, entry.Name)
		if entry.UpPath != wantUpPath || entry.DownPath != wantDownPath {
			return manifestCatalog{}, fmt.Errorf("%w: migration manifest %q path mismatch for version %04d", ErrInvalidCatalog, manifestFilename, entry.Version)
		}
		if !validSHA256Hex(entry.UpSHA256) || !validSHA256Hex(entry.DownSHA256) {
			return manifestCatalog{}, fmt.Errorf("%w: migration manifest %q has invalid checksum for version %04d", ErrInvalidCatalog, manifestFilename, entry.Version)
		}
		manifest.entries[entry.Version] = entry
	}
	return manifest, nil
}

func (manifest manifestCatalog) validateSQLFile(parsed parsedMigrationFilename, path, sum string) error {
	entry, ok := manifest.entries[parsed.version]
	if !ok {
		return fmt.Errorf("%w: migration %04d %s is missing from %s", ErrInvalidCatalog, parsed.version, parsed.direction, manifestFilename)
	}
	if entry.Name != parsed.name {
		return fmt.Errorf("%w: migration %04d name %q does not match %s entry %q", ErrInvalidCatalog, parsed.version, parsed.name, manifestFilename, entry.Name)
	}
	switch parsed.direction {
	case "up":
		if path != entry.UpPath {
			return fmt.Errorf("%w: migration %04d up path %q does not match %s entry %q", ErrInvalidCatalog, parsed.version, path, manifestFilename, entry.UpPath)
		}
		if sum != entry.UpSHA256 {
			return fmt.Errorf("%w: migration %04d up checksum does not match %s", ErrInvalidCatalog, parsed.version, manifestFilename)
		}
	case "down":
		if path != entry.DownPath {
			return fmt.Errorf("%w: migration %04d down path %q does not match %s entry %q", ErrInvalidCatalog, parsed.version, path, manifestFilename, entry.DownPath)
		}
		if sum != entry.DownSHA256 {
			return fmt.Errorf("%w: migration %04d down checksum does not match %s", ErrInvalidCatalog, parsed.version, manifestFilename)
		}
	default:
		return fmt.Errorf("%w: unknown migration direction %q", ErrInvalidCatalog, parsed.direction)
	}
	return nil
}

func (manifest manifestCatalog) validateCatalog(catalog []Migration) error {
	if len(catalog) != len(manifest.entries) {
		return fmt.Errorf("%w: migration manifest %s has %d entries but catalog has %d migrations", ErrInvalidCatalog, manifestFilename, len(manifest.entries), len(catalog))
	}
	for _, migration := range catalog {
		entry, ok := manifest.entries[migration.Version]
		if !ok {
			return fmt.Errorf("%w: migration %04d missing from %s", ErrInvalidCatalog, migration.Version, manifestFilename)
		}
		if migration.Name != entry.Name || migration.UpPath != entry.UpPath || migration.DownPath != entry.DownPath || migration.UpSHA256 != entry.UpSHA256 || migration.DownSHA256 != entry.DownSHA256 {
			return fmt.Errorf("%w: migration %04d does not match %s", ErrInvalidCatalog, migration.Version, manifestFilename)
		}
	}
	return nil
}

func sha256Hex(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func validSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return false
	}
	return true
}
