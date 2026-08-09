package migrations

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const DirectionUp = "up"

var ErrInvalidLedger = errors.New("invalid migration ledger")

// LedgerEntry is the durable schema_migrations state a future migrator or
// database preflight reads from storage. The dry-run planner validates it
// against the project-owned catalog before reporting any pending steps.
type LedgerEntry struct {
	Version  int    `json:"version"`
	Name     string `json:"name"`
	UpSHA256 string `json:"up_sha256"`
}

// PlanStep describes one pending migration without applying it. It exposes
// metadata only; callers that later apply SQL must fetch the validated catalog
// explicitly instead of treating this dry-run plan as an execution script.
type PlanStep struct {
	Version   int    `json:"version"`
	Name      string `json:"name"`
	Direction string `json:"direction"`
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
}

// Plan describes the checked catalog/ledger state and the remaining migration
// steps required to reach the latest embedded project-owned migration.
type Plan struct {
	CurrentVersion int        `json:"current_version"`
	LatestVersion  int        `json:"latest_version"`
	UpToDate       bool       `json:"up_to_date"`
	Pending        []PlanStep `json:"pending"`
}

// PlanUpToLatest validates the embedded catalog and the supplied ledger entries
// and returns a dry-run plan. It performs no database I/O and applies nothing.
func PlanUpToLatest(ledger []LedgerEntry) (Plan, error) {
	catalog, err := Catalog()
	if err != nil {
		return Plan{}, err
	}
	return PlanCatalogUpToLatest(catalog, ledger)
}

// PlanCatalogUpToLatest validates a catalog/ledger pair and returns the pending
// up-migration steps needed to reach the catalog's latest version. This is a
// read-only planning boundary for future operator preflights and CLI tooling;
// it intentionally does not execute SQL.
func PlanCatalogUpToLatest(catalog []Migration, ledger []LedgerEntry) (Plan, error) {
	if err := validatePlanCatalog(catalog); err != nil {
		return Plan{}, err
	}
	currentVersion, err := validateLedgerAgainstCatalog(catalog, ledger)
	if err != nil {
		return Plan{}, err
	}

	plan := Plan{
		CurrentVersion: currentVersion,
		LatestVersion:  catalog[len(catalog)-1].Version,
		UpToDate:       currentVersion == catalog[len(catalog)-1].Version,
		Pending:        make([]PlanStep, 0, len(catalog)-currentVersion),
	}
	for _, migration := range catalog[currentVersion:] {
		plan.Pending = append(plan.Pending, PlanStep{
			Version:   migration.Version,
			Name:      migration.Name,
			Direction: DirectionUp,
			Path:      migration.UpPath,
			SHA256:    migration.UpSHA256,
		})
	}
	return plan, nil
}

func validatePlanCatalog(catalog []Migration) error {
	if len(catalog) == 0 {
		return fmt.Errorf("%w: plan catalog has no migrations", ErrInvalidCatalog)
	}
	for i, migration := range catalog {
		wantVersion := i + 1
		if migration.Version != wantVersion {
			return fmt.Errorf("%w: plan catalog versions must be contiguous from 0001, got %04d at position %04d", ErrInvalidCatalog, migration.Version, wantVersion)
		}
		if !validMigrationName(migration.Name) {
			return fmt.Errorf("%w: plan catalog migration %04d has malformed name %q", ErrInvalidCatalog, migration.Version, migration.Name)
		}
		wantUpPath := fmt.Sprintf("%04d_%s.up.sql", migration.Version, migration.Name)
		wantDownPath := fmt.Sprintf("%04d_%s.down.sql", migration.Version, migration.Name)
		if migration.UpPath != wantUpPath || migration.DownPath != wantDownPath {
			return fmt.Errorf("%w: plan catalog migration %04d has path drift", ErrInvalidCatalog, migration.Version)
		}
		if !validSHA256Hex(migration.UpSHA256) || !validSHA256Hex(migration.DownSHA256) {
			return fmt.Errorf("%w: plan catalog migration %04d has invalid checksum", ErrInvalidCatalog, migration.Version)
		}
		if strings.TrimSpace(migration.UpSQL) == "" || strings.TrimSpace(migration.DownSQL) == "" {
			return fmt.Errorf("%w: plan catalog migration %04d has empty SQL", ErrInvalidCatalog, migration.Version)
		}
		if sha256Hex([]byte(migration.UpSQL)) != migration.UpSHA256 || sha256Hex([]byte(migration.DownSQL)) != migration.DownSHA256 {
			return fmt.Errorf("%w: plan catalog migration %04d checksum does not match SQL text", ErrInvalidCatalog, migration.Version)
		}
		versionID := fmt.Sprintf("%04d", migration.Version)
		if err := validateMigrationHeader(parsedMigrationFilename{version: migration.Version, versionID: versionID, name: migration.Name, direction: DirectionUp}, migration.UpSQL); err != nil {
			return err
		}
		if err := validateMigrationHeader(parsedMigrationFilename{version: migration.Version, versionID: versionID, name: migration.Name, direction: "down"}, migration.DownSQL); err != nil {
			return err
		}
	}
	return nil
}

func validateLedgerAgainstCatalog(catalog []Migration, ledger []LedgerEntry) (int, error) {
	if len(ledger) == 0 {
		return 0, nil
	}
	entries := append([]LedgerEntry(nil), ledger...)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Version < entries[j].Version
	})
	seen := make(map[int]struct{}, len(entries))
	for i, entry := range entries {
		if entry.Version <= 0 {
			return 0, fmt.Errorf("%w: ledger version must be positive, got %d", ErrInvalidLedger, entry.Version)
		}
		if _, ok := seen[entry.Version]; ok {
			return 0, fmt.Errorf("%w: duplicate ledger version %04d", ErrInvalidLedger, entry.Version)
		}
		seen[entry.Version] = struct{}{}
		wantVersion := i + 1
		if entry.Version != wantVersion {
			return 0, fmt.Errorf("%w: ledger versions must be contiguous from 0001, got %04d at position %04d", ErrInvalidLedger, entry.Version, wantVersion)
		}
		if entry.Version > len(catalog) {
			return 0, fmt.Errorf("%w: ledger version %04d is newer than catalog latest %04d", ErrInvalidLedger, entry.Version, catalog[len(catalog)-1].Version)
		}
		migration := catalog[entry.Version-1]
		if entry.Name != migration.Name {
			return 0, fmt.Errorf("%w: ledger version %04d name %q does not match catalog name %q", ErrInvalidLedger, entry.Version, entry.Name, migration.Name)
		}
		if entry.UpSHA256 != migration.UpSHA256 {
			return 0, fmt.Errorf("%w: ledger version %04d checksum does not match catalog", ErrInvalidLedger, entry.Version)
		}
	}
	return len(entries), nil
}
