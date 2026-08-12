package migrations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"unicode/utf8"
)

const LedgerSnapshotFormat = "go-metin2-schema-migrations-ledger-v1"

var ErrInvalidLedgerSnapshot = errors.New("invalid migration ledger snapshot")

// LedgerSnapshot is a metadata-only JSON shape for offline schema_migrations
// preflight. It carries only the applied ledger rows needed by the migration
// planner and deliberately omits executable SQL.
type LedgerSnapshot struct {
	Format  string        `json:"format"`
	Entries []LedgerEntry `json:"entries"`
}

// ReadJSONLedgerSnapshot decodes a strict metadata-only schema_migrations ledger
// snapshot. The returned entries are validated enough to be safe input for the
// dry-run planner; catalog-specific checks are still performed by PlanCatalog*.
func ReadJSONLedgerSnapshot(r io.Reader) ([]LedgerEntry, error) {
	if readerIsNil(r) {
		return nil, fmt.Errorf("%w: reader is required", ErrInvalidLedgerSnapshot)
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("%w: read ledger snapshot: %w", ErrInvalidLedgerSnapshot, err)
	}
	if !utf8.Valid(raw) {
		return nil, fmt.Errorf("%w: ledger snapshot is not valid UTF-8", ErrInvalidLedgerSnapshot)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("%w: ledger snapshot is empty", ErrInvalidLedgerSnapshot)
	}

	var snapshot LedgerSnapshot
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("%w: decode ledger snapshot: %w", ErrInvalidLedgerSnapshot, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: ledger snapshot has trailing JSON", ErrInvalidLedgerSnapshot)
	}
	entries, err := validateLedgerSnapshot(snapshot)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// MarshalJSONLedgerSnapshot encodes ledger entries into the strict offline
// snapshot format in deterministic version order without mutating the input.
func MarshalJSONLedgerSnapshot(entries []LedgerEntry) ([]byte, error) {
	validated, err := validateLedgerSnapshotEntries(entries)
	if err != nil {
		return nil, err
	}
	if validated == nil {
		validated = []LedgerEntry{}
	}
	snapshot := LedgerSnapshot{Format: LedgerSnapshotFormat, Entries: validated}
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("%w: marshal ledger snapshot: %w", ErrInvalidLedgerSnapshot, err)
	}
	return append(raw, '\n'), nil
}

// LedgerSnapshotFromSQLLedger reads schema_migrations through the narrow SQL
// ledger seam and returns the strict metadata-only offline snapshot shape. It is
// intended for operator/export preflights and does not read or expose executable
// migration SQL.
func LedgerSnapshotFromSQLLedger(ctx context.Context, querier SQLLedgerQuerier) (LedgerSnapshot, error) {
	entries, err := ReadSQLLedgerEntries(ctx, querier)
	if err != nil {
		return LedgerSnapshot{}, err
	}
	validated, err := validateLedgerSnapshotEntries(entries)
	if err != nil {
		return LedgerSnapshot{}, err
	}
	if validated == nil {
		validated = []LedgerEntry{}
	}
	return LedgerSnapshot{Format: LedgerSnapshotFormat, Entries: validated}, nil
}

// PlanCatalogUpToLatestFromJSONLedgerSnapshot reads an offline ledger snapshot
// and returns the metadata-only up plan to the supplied catalog's latest version.
func PlanCatalogUpToLatestFromJSONLedgerSnapshot(catalog []Migration, r io.Reader) (Plan, error) {
	if err := validatePlanCatalog(catalog); err != nil {
		return Plan{}, err
	}
	return PlanCatalogToVersionFromJSONLedgerSnapshot(catalog, r, catalog[len(catalog)-1].Version)
}

// PlanUpToLatestFromJSONLedgerSnapshot reads an offline ledger snapshot and
// returns the metadata-only up plan to the embedded catalog's latest version.
func PlanUpToLatestFromJSONLedgerSnapshot(r io.Reader) (Plan, error) {
	catalog, err := Catalog()
	if err != nil {
		return Plan{}, err
	}
	return PlanCatalogUpToLatestFromJSONLedgerSnapshot(catalog, r)
}

// PlanCatalogToVersionFromJSONLedgerSnapshot reads an offline ledger snapshot
// and returns a metadata-only plan to targetVersion without executing SQL.
func PlanCatalogToVersionFromJSONLedgerSnapshot(catalog []Migration, r io.Reader, targetVersion int) (Plan, error) {
	ledger, err := ReadJSONLedgerSnapshot(r)
	if err != nil {
		return Plan{}, err
	}
	return PlanCatalogToVersion(catalog, ledger, targetVersion)
}

// PlanCatalogToVersionFromLedgerSnapshot validates an already-decoded offline
// ledger snapshot and returns a metadata-only plan to targetVersion.
func PlanCatalogToVersionFromLedgerSnapshot(catalog []Migration, snapshot LedgerSnapshot, targetVersion int) (Plan, error) {
	ledger, err := validateLedgerSnapshot(snapshot)
	if err != nil {
		return Plan{}, err
	}
	return PlanCatalogToVersion(catalog, ledger, targetVersion)
}

// PlanToVersionFromLedgerSnapshot validates an already-decoded offline ledger
// snapshot and returns a metadata-only plan against the embedded catalog.
func PlanToVersionFromLedgerSnapshot(snapshot LedgerSnapshot, targetVersion int) (Plan, error) {
	catalog, err := Catalog()
	if err != nil {
		return Plan{}, err
	}
	return PlanCatalogToVersionFromLedgerSnapshot(catalog, snapshot, targetVersion)
}

// PlanToVersionFromJSONLedgerSnapshot reads an offline ledger snapshot and
// returns a metadata-only plan against the embedded catalog.
func PlanToVersionFromJSONLedgerSnapshot(r io.Reader, targetVersion int) (Plan, error) {
	catalog, err := Catalog()
	if err != nil {
		return Plan{}, err
	}
	return PlanCatalogToVersionFromJSONLedgerSnapshot(catalog, r, targetVersion)
}

func validateLedgerSnapshotEntries(entries []LedgerEntry) ([]LedgerEntry, error) {
	validated := append([]LedgerEntry(nil), entries...)
	sort.Slice(validated, func(i, j int) bool {
		return validated[i].Version < validated[j].Version
	})
	seen := make(map[int]struct{}, len(validated))
	for _, entry := range validated {
		if entry.Version <= 0 {
			return nil, fmt.Errorf("%w: ledger entry version must be positive, got %d", ErrInvalidLedgerSnapshot, entry.Version)
		}
		if _, ok := seen[entry.Version]; ok {
			return nil, fmt.Errorf("%w: duplicate ledger entry version %04d", ErrInvalidLedgerSnapshot, entry.Version)
		}
		seen[entry.Version] = struct{}{}
		if !validMigrationName(entry.Name) {
			return nil, fmt.Errorf("%w: ledger entry version %04d has malformed name %q", ErrInvalidLedgerSnapshot, entry.Version, entry.Name)
		}
		if !validSHA256Hex(entry.UpSHA256) {
			return nil, fmt.Errorf("%w: ledger entry version %04d has invalid checksum", ErrInvalidLedgerSnapshot, entry.Version)
		}
	}
	return validated, nil
}

func validateLedgerSnapshot(snapshot LedgerSnapshot) ([]LedgerEntry, error) {
	if snapshot.Format != LedgerSnapshotFormat {
		return nil, fmt.Errorf("%w: unsupported format %q", ErrInvalidLedgerSnapshot, snapshot.Format)
	}
	if snapshot.Entries == nil {
		return nil, fmt.Errorf("%w: entries are required", ErrInvalidLedgerSnapshot)
	}
	return validateLedgerSnapshotEntries(snapshot.Entries)
}

func readerIsNil(r io.Reader) bool {
	if r == nil {
		return true
	}
	value := reflect.ValueOf(r)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
