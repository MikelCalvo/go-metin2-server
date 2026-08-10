package migrations

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	DirectionUp   = "up"
	DirectionDown = "down"
)

var (
	ErrInvalidLedger          = errors.New("invalid migration ledger")
	ErrInvalidMigrationTarget = errors.New("invalid migration target")
)

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
	return PlanCatalogToVersion(catalog, ledger, len(catalog))
}

// PlanToVersion validates the embedded catalog and the supplied ledger entries,
// then returns a metadata-only dry-run plan toward targetVersion. A target equal
// to the current ledger version is up to date, a higher target yields up steps,
// and a lower target yields down steps in reverse version order.
func PlanToVersion(ledger []LedgerEntry, targetVersion int) (Plan, error) {
	catalog, err := Catalog()
	if err != nil {
		return Plan{}, err
	}
	return PlanCatalogToVersion(catalog, ledger, targetVersion)
}

// PlanCatalogToVersion validates a catalog/ledger pair and returns the pending
// metadata-only steps needed to reach targetVersion without executing SQL. The
// target must stay inside the embedded catalog boundary, including zero for a
// complete rollback preview.
func PlanCatalogToVersion(catalog []Migration, ledger []LedgerEntry, targetVersion int) (Plan, error) {
	if err := validatePlanCatalog(catalog); err != nil {
		return Plan{}, err
	}
	currentVersion, err := validateLedgerAgainstCatalog(catalog, ledger)
	if err != nil {
		return Plan{}, err
	}
	latestVersion := catalog[len(catalog)-1].Version
	if targetVersion < 0 || targetVersion > latestVersion {
		return Plan{}, fmt.Errorf("%w: target version %d outside catalog range 0..%d", ErrInvalidMigrationTarget, targetVersion, latestVersion)
	}

	plan := Plan{
		CurrentVersion: currentVersion,
		LatestVersion:  latestVersion,
		UpToDate:       currentVersion == targetVersion,
		Pending:        planStepsToVersion(catalog, currentVersion, targetVersion),
	}
	return plan, nil
}

func planStepsToVersion(catalog []Migration, currentVersion int, targetVersion int) []PlanStep {
	if currentVersion == targetVersion {
		return []PlanStep{}
	}
	if targetVersion > currentVersion {
		steps := make([]PlanStep, 0, targetVersion-currentVersion)
		for _, migration := range catalog[currentVersion:targetVersion] {
			steps = append(steps, PlanStep{
				Version:   migration.Version,
				Name:      migration.Name,
				Direction: DirectionUp,
				Path:      migration.UpPath,
				SHA256:    migration.UpSHA256,
			})
		}
		return steps
	}

	steps := make([]PlanStep, 0, currentVersion-targetVersion)
	for version := currentVersion; version > targetVersion; version-- {
		migration := catalog[version-1]
		steps = append(steps, PlanStep{
			Version:   migration.Version,
			Name:      migration.Name,
			Direction: DirectionDown,
			Path:      migration.DownPath,
			SHA256:    migration.DownSHA256,
		})
	}
	return steps
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
