package migrations

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

//go:embed *.sql
var embeddedMigrations embed.FS

var ErrInvalidCatalog = errors.New("invalid migration catalog")

// Migration is one validated project-owned SQL migration pair.
//
// The catalog deliberately models only static migration metadata and SQL text.
// Applying migrations to a database is a later production-ops slice.
type Migration struct {
	Version  int
	Name     string
	UpPath   string
	DownPath string
	UpSQL    string
	DownSQL  string
}

// Catalog returns the embedded project-owned migration catalog after validating
// naming, pairing, version ordering, and per-file headers.
func Catalog() ([]Migration, error) {
	return LoadCatalog(embeddedMigrations)
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
// matching project-owned header comment:
//
//	-- go-metin2 migration: NNNN name up
//	-- go-metin2 migration: NNNN name down
func LoadCatalog(fsys fs.FS) ([]Migration, error) {
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
		case "down":
			if pair.downPath != "" {
				return nil, fmt.Errorf("%w: duplicate down migration for version %04d", ErrInvalidCatalog, parsed.version)
			}
			pair.downPath = filename
			pair.downSQL = body
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
			Version:  pair.version,
			Name:     pair.name,
			UpPath:   pair.upPath,
			DownPath: pair.downPath,
			UpSQL:    pair.upSQL,
			DownSQL:  pair.downSQL,
		})
	}
	return catalog, nil
}

type migrationPair struct {
	version  int
	name     string
	upPath   string
	downPath string
	upSQL    string
	downSQL  string
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
