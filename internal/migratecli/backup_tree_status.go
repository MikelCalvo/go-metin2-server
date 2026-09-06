package migratecli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/accountstore"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/itemstore"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	"github.com/MikelCalvo/go-metin2-server/internal/queststate"
	"github.com/MikelCalvo/go-metin2-server/internal/safeboxstore"
	"github.com/MikelCalvo/go-metin2-server/internal/staticstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

const backupTreeStatusFormat = "go-metin2-backup-tree-status-v1"

// ErrBackupTreeStatus reports a fail-closed backup-tree inspection failure.
var ErrBackupTreeStatus = errors.New("backup-tree-status failed")

type backupTreeStoreSpec struct {
	Kind             string
	Subdir           string
	ManifestFilename string
	ManifestFormat   string
	SnapshotBasename string
}

var backupTreeStoreSpecs = []backupTreeStoreSpec{
	{
		Kind:             "accounts",
		Subdir:           "accounts",
		ManifestFilename: accountstore.BackupManifestFilename,
		ManifestFormat:   accountstore.BackupManifestFormat,
	},
	{
		Kind:             "login-tickets",
		Subdir:           "login-tickets",
		ManifestFilename: loginticket.BackupManifestFilename,
		ManifestFormat:   loginticket.BackupManifestFormat,
	},
	{
		Kind:             "item-templates",
		Subdir:           "item-templates",
		ManifestFilename: itemstore.BackupManifestFilename,
		ManifestFormat:   itemstore.BackupManifestFormat,
		SnapshotBasename: "item-templates.json",
	},
	{
		Kind:             "interaction-store",
		Subdir:           "interaction-store",
		ManifestFilename: interactionstore.BackupManifestFilename,
		ManifestFormat:   interactionstore.BackupManifestFormat,
		SnapshotBasename: "interaction-definitions.json",
	},
	{
		Kind:             "static-actors",
		Subdir:           "static-actors",
		ManifestFilename: staticstore.BackupManifestFilename,
		ManifestFormat:   staticstore.BackupManifestFormat,
		SnapshotBasename: "static-actors.json",
	},
	{
		Kind:             "quest-state",
		Subdir:           "quest-state",
		ManifestFilename: queststate.BackupManifestFilename,
		ManifestFormat:   queststate.BackupManifestFormat,
		SnapshotBasename: "quest-state.json",
	},
	{
		Kind:             "ground-items",
		Subdir:           "ground-items",
		ManifestFilename: worldruntime.BackupManifestFilename,
		ManifestFormat:   worldruntime.BackupManifestFormat,
		SnapshotBasename: "ground-items.json",
	},
	{
		Kind:             "safebox",
		Subdir:           "safebox",
		ManifestFilename: safeboxstore.BackupManifestFilename,
		ManifestFormat:   safeboxstore.BackupManifestFormat,
		SnapshotBasename: "safebox.json",
	},
}

type backupTreeStoreStatus struct {
	Kind             string `json:"kind"`
	Present          bool   `json:"present"`
	Valid            bool   `json:"valid"`
	Path             string `json:"path,omitempty"`
	ManifestFilename string `json:"manifest_filename,omitempty"`
	Format           string `json:"format,omitempty"`
	ManifestSHA256   string `json:"manifest_sha256,omitempty"`
	CrashTempCount   int    `json:"crash_temp_count,omitempty"`
	AccountCount     int    `json:"account_count,omitempty"`
	CharacterCount   int    `json:"character_count,omitempty"`
	TicketCount      int    `json:"ticket_count,omitempty"`
	TemplateCount    int    `json:"template_count,omitempty"`
	DefinitionCount  int    `json:"definition_count,omitempty"`
	ActorCount       int    `json:"actor_count,omitempty"`
	FlagCount        int    `json:"flag_count,omitempty"`
	GroundItemCount  int    `json:"ground_item_count,omitempty"`
	CellCount        int    `json:"cell_count,omitempty"`
}

type backupTreeStatus struct {
	Format            string                  `json:"format"`
	Present           bool                    `json:"present"`
	BackupTree        string                  `json:"backup_tree,omitempty"`
	StoreCount        int                     `json:"store_count,omitempty"`
	StorePresentCount int                     `json:"store_present_count,omitempty"`
	StoresComplete    *bool                   `json:"stores_complete,omitempty"`
	Stores            []backupTreeStoreStatus `json:"stores,omitempty"`
}

func runBackupTreeStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("backup-tree-status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var backupTree string
	var requireStoresComplete bool
	flags.StringVar(&backupTree, "backup-tree", "", "absolute path to a retained backup-restore-drill tree")
	flags.BoolVar(&requireStoresComplete, "require-stores-complete", false, "fail closed unless stores_complete is true")
	flags.Usage = func() { printBackupTreeStatusUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected backup-tree-status argument %q\n", flags.Arg(0))
		printBackupTreeStatusUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(backupTree) == "" {
		fmt.Fprintln(stderr, "--backup-tree is required for backup-tree-status")
		printBackupTreeStatusUsage(stderr)
		return exitUsage
	}

	status, err := inspectBackupTreeStatus(backupTree)
	if err != nil {
		fmt.Fprintf(stderr, "backup-tree-status: %v\n", err)
		return exitError
	}
	if err := enforceBackupTreeStatusRequireGates(status, requireStoresComplete); err != nil {
		fmt.Fprintf(stderr, "backup-tree-status: %v\n", err)
		return exitError
	}
	return writeJSON(stdout, stderr, status)
}

func enforceBackupTreeStatusRequireGates(status backupTreeStatus, requireStoresComplete bool) error {
	if !requireStoresComplete {
		return nil
	}
	if !status.Present {
		return fmt.Errorf("--require-stores-complete failed: backup-tree is absent")
	}
	if status.StoresComplete == nil || !*status.StoresComplete {
		return fmt.Errorf("--require-stores-complete failed: stores_complete=false")
	}
	return nil
}

func inspectBackupTreeStatus(backupTree string) (backupTreeStatus, error) {
	normalizedTree, err := normalizeImportExportDrillAbsolutePath(backupTree, "backup-tree")
	if err != nil {
		return backupTreeStatus{}, fmt.Errorf("%w: %v", ErrBackupTreeStatus, err)
	}

	info, err := os.Lstat(normalizedTree)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return backupTreeStatus{
				Format:  backupTreeStatusFormat,
				Present: false,
			}, nil
		}
		return backupTreeStatus{}, fmt.Errorf("%w: stat backup-tree: %v", ErrBackupTreeStatus, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return backupTreeStatus{}, fmt.Errorf("%w: backup-tree must not be a symlink: %s", ErrBackupTreeStatus, normalizedTree)
	}
	if !info.IsDir() {
		return backupTreeStatus{}, fmt.Errorf("%w: backup-tree must be a directory: %s", ErrBackupTreeStatus, normalizedTree)
	}

	status := backupTreeStatus{
		Format:     backupTreeStatusFormat,
		Present:    true,
		BackupTree: normalizedTree,
		StoreCount: len(backupTreeStoreSpecs),
		Stores:     make([]backupTreeStoreStatus, 0, len(backupTreeStoreSpecs)),
	}
	for _, spec := range backupTreeStoreSpecs {
		entry, err := inspectBackupTreeStore(normalizedTree, spec)
		if err != nil {
			return backupTreeStatus{}, err
		}
		if entry.Present {
			status.StorePresentCount++
		}
		status.Stores = append(status.Stores, entry)
	}
	status.StoresComplete = boolPtr(status.StorePresentCount == len(backupTreeStoreSpecs))
	return status, nil
}

func inspectBackupTreeStore(tree string, spec backupTreeStoreSpec) (backupTreeStoreStatus, error) {
	storeDir := filepath.Join(tree, spec.Subdir)
	info, err := os.Lstat(storeDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return backupTreeStoreStatus{
				Kind:    spec.Kind,
				Present: false,
				Valid:   false,
			}, nil
		}
		return backupTreeStoreStatus{}, fmt.Errorf("%w: %s: stat store: %v", ErrBackupTreeStatus, spec.Kind, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return backupTreeStoreStatus{}, fmt.Errorf("%w: %s: store must not be a symlink: %s", ErrBackupTreeStatus, spec.Kind, spec.Subdir)
	}
	if !info.IsDir() {
		return backupTreeStoreStatus{}, fmt.Errorf("%w: %s: store must be a directory: %s", ErrBackupTreeStatus, spec.Kind, spec.Subdir)
	}

	entry, err := validateBackupTreeStore(spec, storeDir)
	if err != nil {
		return backupTreeStoreStatus{}, fmt.Errorf("%w: %s: %v", ErrBackupTreeStatus, spec.Kind, err)
	}
	return entry, nil
}

func validateBackupTreeStore(spec backupTreeStoreSpec, storeDir string) (backupTreeStoreStatus, error) {
	dummyPath := storeDir
	if spec.SnapshotBasename != "" {
		dummyPath = filepath.Join(storeDir, spec.SnapshotBasename)
	}

	entry := backupTreeStoreStatus{
		Kind:             spec.Kind,
		Present:          true,
		Valid:            true,
		Path:             filepath.ToSlash(spec.Subdir),
		ManifestFilename: spec.ManifestFilename,
		Format:           spec.ManifestFormat,
	}
	switch spec.Kind {
	case "accounts":
		summary, err := accountstore.NewFileStore(dummyPath).ValidateBackupFrom(storeDir)
		if err != nil {
			return backupTreeStoreStatus{}, err
		}
		entry.CrashTempCount = summary.CrashTempCount
		entry.AccountCount = summary.AccountCount
		entry.CharacterCount = summary.CharacterCount
	case "login-tickets":
		summary, err := loginticket.NewFileStore(dummyPath).ValidateBackupFrom(storeDir)
		if err != nil {
			return backupTreeStoreStatus{}, err
		}
		entry.CrashTempCount = summary.CrashTempCount
		entry.TicketCount = summary.TicketCount
		entry.CharacterCount = summary.CharacterCount
	case "item-templates":
		summary, err := itemstore.NewFileStore(dummyPath).ValidateBackupFrom(storeDir)
		if err != nil {
			return backupTreeStoreStatus{}, err
		}
		entry.CrashTempCount = summary.CrashTempCount
		entry.TemplateCount = summary.TemplateCount
	case "interaction-store":
		summary, err := interactionstore.NewFileStore(dummyPath).ValidateBackupFrom(storeDir)
		if err != nil {
			return backupTreeStoreStatus{}, err
		}
		entry.CrashTempCount = summary.CrashTempCount
		entry.DefinitionCount = summary.DefinitionCount
	case "static-actors":
		summary, err := staticstore.NewFileStore(dummyPath).ValidateBackupFrom(storeDir)
		if err != nil {
			return backupTreeStoreStatus{}, err
		}
		entry.CrashTempCount = summary.CrashTempCount
		entry.ActorCount = summary.ActorCount
	case "quest-state":
		summary, err := queststate.NewFileStore(dummyPath).ValidateBackupFrom(storeDir)
		if err != nil {
			return backupTreeStoreStatus{}, err
		}
		entry.CrashTempCount = summary.CrashTempCount
		entry.FlagCount = summary.FlagCount
	case "ground-items":
		summary, err := worldruntime.NewGroundItemFileStore(dummyPath).ValidateBackupFrom(storeDir)
		if err != nil {
			return backupTreeStoreStatus{}, err
		}
		entry.CrashTempCount = summary.CrashTempCount
		entry.GroundItemCount = summary.GroundItemCount
	case "safebox":
		summary, err := safeboxstore.NewFileStore(dummyPath).ValidateBackupFrom(storeDir)
		if err != nil {
			return backupTreeStoreStatus{}, err
		}
		entry.CrashTempCount = summary.CrashTempCount
		entry.CharacterCount = summary.CharacterCount
		entry.CellCount = summary.CellCount
	default:
		return backupTreeStoreStatus{}, fmt.Errorf("unknown backup-tree store kind %q", spec.Kind)
	}

	raw, err := os.ReadFile(filepath.Join(storeDir, spec.ManifestFilename))
	if err != nil {
		return backupTreeStoreStatus{}, fmt.Errorf("read manifest: %w", err)
	}
	entry.ManifestSHA256 = sha256Hex(raw)
	return entry, nil
}

func printBackupTreeStatusUsage(w io.Writer) {
	fmt.Fprintln(w, "backup-tree-status usage:")
	fmt.Fprintln(w, "  metin2-migrate backup-tree-status --backup-tree <absolute-path> [--require-stores-complete]")
}
