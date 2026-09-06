package migratecli

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

const (
	backupTreeStatusStatusFormat = "go-metin2-backup-tree-status-status-v1"
	maxBackupTreeStatusBytes     = 128 * 1024
)

type backupTreeStatusStatus struct {
	Format                 string            `json:"format"`
	Present                bool              `json:"present"`
	BackupTreeStatusSHA256 string            `json:"backup_tree_status_sha256,omitempty"`
	Status                 *backupTreeStatus `json:"status,omitempty"`
}

func runBackupTreeStatusStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("backup-tree-status-status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var backupTreeStatusPath string
	var requireStoresComplete bool
	var requireNoCrashTemps bool
	flags.StringVar(&backupTreeStatusPath, "backup-tree-status", "", "path to a retained backup-tree-status JSON snapshot")
	flags.BoolVar(&requireStoresComplete, "require-stores-complete", false, "fail closed unless inner stores_complete is true")
	flags.BoolVar(&requireNoCrashTemps, "require-no-crash-temps", false, "fail closed unless every inner store has omitted or zero crash_temp_count")
	flags.Usage = func() { printBackupTreeStatusStatusUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected backup-tree-status-status argument %q\n", flags.Arg(0))
		printBackupTreeStatusStatusUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(backupTreeStatusPath) == "" {
		fmt.Fprintln(stderr, "--backup-tree-status is required for backup-tree-status-status")
		printBackupTreeStatusStatusUsage(stderr)
		return exitUsage
	}

	raw, present, err := readOptionalBackupTreeStatusFile(backupTreeStatusPath, maxBackupTreeStatusBytes)
	if err != nil {
		fmt.Fprintf(stderr, "backup-tree-status-status: %v\n", err)
		return exitError
	}
	if !present {
		if err := enforceBackupTreeStatusRequireGates(backupTreeStatus{Present: false}, requireStoresComplete, requireNoCrashTemps); err != nil {
			fmt.Fprintf(stderr, "backup-tree-status-status: %v\n", err)
			return exitError
		}
		return writeJSON(stdout, stderr, backupTreeStatusStatus{
			Format:  backupTreeStatusStatusFormat,
			Present: false,
		})
	}

	var inner backupTreeStatus
	if err := decodeStrictExportTreeStatusJSON(raw, &inner, "backup-tree-status"); err != nil {
		fmt.Fprintf(stderr, "backup-tree-status-status: %v\n", err)
		return exitError
	}
	if err := validateRetainedBackupTreeStatus(inner); err != nil {
		fmt.Fprintf(stderr, "backup-tree-status-status: %v\n", err)
		return exitError
	}
	if err := enforceBackupTreeStatusRequireGates(inner, requireStoresComplete, requireNoCrashTemps); err != nil {
		fmt.Fprintf(stderr, "backup-tree-status-status: %v\n", err)
		return exitError
	}
	return writeJSON(stdout, stderr, backupTreeStatusStatus{
		Format:                 backupTreeStatusStatusFormat,
		Present:                true,
		BackupTreeStatusSHA256: sha256Hex(raw),
		Status:                 &inner,
	})
}

func validateRetainedBackupTreeStatus(status backupTreeStatus) error {
	if status.Format != backupTreeStatusFormat {
		return fmt.Errorf("%w: unexpected format %q", ErrBackupTreeStatus, status.Format)
	}
	if !status.Present {
		if status.BackupTree != "" ||
			status.StoreCount != 0 ||
			status.StorePresentCount != 0 ||
			status.StoresComplete != nil ||
			len(status.Stores) != 0 {
			return fmt.Errorf("%w: absent backup-tree status has extra fields", ErrBackupTreeStatus)
		}
		return nil
	}

	normalizedTree, err := normalizeImportExportDrillAbsolutePath(status.BackupTree, "backup-tree")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBackupTreeStatus, err)
	}
	if normalizedTree != status.BackupTree {
		return fmt.Errorf("%w: backup_tree must be an absolute cleaned path", ErrBackupTreeStatus)
	}
	if status.StoreCount != len(backupTreeStoreSpecs) || len(status.Stores) != len(backupTreeStoreSpecs) {
		return fmt.Errorf("%w: store_count mismatch", ErrBackupTreeStatus)
	}

	presentCount := 0
	for i, spec := range backupTreeStoreSpecs {
		entry := status.Stores[i]
		if entry.Kind != spec.Kind {
			return fmt.Errorf("%w: kind mismatch at index %d: got %q want %q", ErrBackupTreeStatus, i, entry.Kind, spec.Kind)
		}
		if err := validateRetainedBackupTreeStore(entry, spec); err != nil {
			return err
		}
		if entry.Present {
			presentCount++
		}
	}
	if status.StorePresentCount != presentCount {
		return fmt.Errorf("%w: store_present_count mismatch", ErrBackupTreeStatus)
	}
	if status.StoresComplete == nil {
		return fmt.Errorf("%w: stores_complete mismatch", ErrBackupTreeStatus)
	}
	if *status.StoresComplete != (presentCount == len(backupTreeStoreSpecs)) {
		return fmt.Errorf("%w: stores_complete mismatch", ErrBackupTreeStatus)
	}
	return nil
}

func validateRetainedBackupTreeStore(entry backupTreeStoreStatus, spec backupTreeStoreSpec) error {
	if !entry.Present {
		if entry.Valid ||
			entry.Path != "" ||
			entry.ManifestFilename != "" ||
			entry.Format != "" ||
			entry.ManifestSHA256 != "" ||
			entry.CrashTempCount != 0 ||
			entry.AccountCount != 0 ||
			entry.CharacterCount != 0 ||
			entry.TicketCount != 0 ||
			entry.TemplateCount != 0 ||
			entry.DefinitionCount != 0 ||
			entry.ActorCount != 0 ||
			entry.FlagCount != 0 ||
			entry.GroundItemCount != 0 ||
			entry.CellCount != 0 {
			return fmt.Errorf("%w: missing %s store has extra fields", ErrBackupTreeStatus, spec.Kind)
		}
		return nil
	}
	if !entry.Valid {
		return fmt.Errorf("%w: present %s store must be valid", ErrBackupTreeStatus, spec.Kind)
	}
	if entry.Path != spec.Subdir {
		return fmt.Errorf("%w: %s path mismatch", ErrBackupTreeStatus, spec.Kind)
	}
	if entry.ManifestFilename != spec.ManifestFilename {
		return fmt.Errorf("%w: %s manifest_filename mismatch", ErrBackupTreeStatus, spec.Kind)
	}
	if entry.Format != spec.ManifestFormat {
		return fmt.Errorf("%w: %s format mismatch", ErrBackupTreeStatus, spec.Kind)
	}
	if !validBackupTreeManifestSHA256(entry.ManifestSHA256) {
		return fmt.Errorf("%w: %s manifest_sha256 mismatch", ErrBackupTreeStatus, spec.Kind)
	}
	if entry.CrashTempCount < 0 {
		return fmt.Errorf("%w: crash_temp_count", ErrBackupTreeStatus)
	}
	if err := validateBackupTreeStoreCounts(entry); err != nil {
		return err
	}
	return nil
}

func validateBackupTreeStoreCounts(entry backupTreeStoreStatus) error {
	allowed := map[string]bool{}
	switch entry.Kind {
	case "accounts":
		allowed["account_count"] = true
		allowed["character_count"] = true
	case "login-tickets":
		allowed["ticket_count"] = true
		allowed["character_count"] = true
	case "item-templates":
		allowed["template_count"] = true
	case "interaction-store":
		allowed["definition_count"] = true
	case "static-actors":
		allowed["actor_count"] = true
	case "quest-state":
		allowed["flag_count"] = true
	case "ground-items":
		allowed["ground_item_count"] = true
	case "safebox":
		allowed["character_count"] = true
		allowed["cell_count"] = true
	default:
		return fmt.Errorf("%w: unknown kind %q", ErrBackupTreeStatus, entry.Kind)
	}

	counts := []struct {
		name  string
		value int
	}{
		{"account_count", entry.AccountCount},
		{"character_count", entry.CharacterCount},
		{"ticket_count", entry.TicketCount},
		{"template_count", entry.TemplateCount},
		{"definition_count", entry.DefinitionCount},
		{"actor_count", entry.ActorCount},
		{"flag_count", entry.FlagCount},
		{"ground_item_count", entry.GroundItemCount},
		{"cell_count", entry.CellCount},
	}
	for _, count := range counts {
		if count.value < 0 {
			return fmt.Errorf("%w: %s", ErrBackupTreeStatus, count.name)
		}
		if count.value != 0 && !allowed[count.name] {
			return fmt.Errorf("%w: %s", ErrBackupTreeStatus, count.name)
		}
	}
	return nil
}

func validBackupTreeManifestSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func readOptionalBackupTreeStatusFile(path string, maxBytes int) ([]byte, bool, error) {
	trimmed := strings.TrimSpace(path)
	info, err := os.Lstat(trimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("%w: stat backup-tree-status: %v", ErrBackupTreeStatus, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, fmt.Errorf("%w: backup-tree-status must not be a symlink: %s", ErrBackupTreeStatus, trimmed)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%w: backup-tree-status must be a regular file: %s", ErrBackupTreeStatus, trimmed)
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return nil, false, fmt.Errorf("%w: open backup-tree-status: %v", ErrBackupTreeStatus, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("%w: stat opened backup-tree-status: %v", ErrBackupTreeStatus, err)
	}
	if !openedInfo.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%w: opened backup-tree-status must be a regular file: %s", ErrBackupTreeStatus, trimmed)
	}
	raw, err := io.ReadAll(io.LimitReader(file, int64(maxBytes)+1))
	if err != nil {
		return nil, false, fmt.Errorf("%w: read backup-tree-status: %v", ErrBackupTreeStatus, err)
	}
	if len(raw) > maxBytes {
		return nil, false, fmt.Errorf("%w: backup-tree-status exceeds %d bytes", ErrBackupTreeStatus, maxBytes)
	}
	if !utf8.Valid(raw) {
		return nil, false, fmt.Errorf("%w: backup-tree-status is not valid UTF-8", ErrBackupTreeStatus)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, false, fmt.Errorf("%w: backup-tree-status is empty", ErrBackupTreeStatus)
	}
	return raw, true, nil
}

func printBackupTreeStatusStatusUsage(w io.Writer) {
	fmt.Fprintln(w, "backup-tree-status-status usage:")
	fmt.Fprintln(w, "  metin2-migrate backup-tree-status-status --backup-tree-status <path> [--require-stores-complete] [--require-no-crash-temps]")
}
