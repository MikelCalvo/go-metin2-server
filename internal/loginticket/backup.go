package loginticket

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	BackupManifestFilename = "login-ticket-backup-manifest.json"
	BackupManifestFormat   = "go-metin2-login-ticket-backup-v1"
)

var (
	ErrBackupDirRequired      = errors.New("login ticket backup dir is required")
	ErrBackupDirNotEmpty      = errors.New("login ticket backup dir is not empty")
	ErrBackupDirInsideStore   = errors.New("login ticket backup dir is inside login ticket store")
	ErrRestoreSourceRequired  = errors.New("login ticket restore source dir is required")
	ErrRestoreSourceNotFound  = errors.New("login ticket restore source dir not found")
	ErrRestoreDirNotEmpty     = errors.New("login ticket restore dir is not empty")
	ErrRestoreDirInsideSource = errors.New("login ticket restore dir is inside backup source")
	ErrBackupManifestRequired = errors.New("login ticket backup manifest is required")
	ErrInvalidBackupManifest  = errors.New("invalid login ticket backup manifest")
)

type BackupManifest struct {
	Format  string               `json:"format"`
	Summary SnapshotSummary      `json:"summary"`
	Files   []BackupManifestFile `json:"files"`
}

type BackupManifestFile struct {
	Login     string `json:"login"`
	LoginKey  uint32 `json:"login_key"`
	Filename  string `json:"filename"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

func (s *FileStore) BackupTo(dstDir string) error {
	if s.dir == "" {
		return ErrStoreDirRequired
	}
	if strings.TrimSpace(dstDir) == "" {
		return ErrBackupDirRequired
	}
	if err := rejectBackupDestinationInsideStore(s.dir, dstDir); err != nil {
		return err
	}
	if err := ensureEmptyDir(dstDir, ErrBackupDirNotEmpty, "read login ticket backup dir"); err != nil {
		return err
	}
	tickets, err := s.List()
	if err != nil {
		return err
	}
	if err := s.validateActiveBackupManifestForTickets(tickets); err != nil {
		return err
	}
	if _, err := s.crashTempFiles(); err != nil {
		return err
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("create login ticket backup dir: %w", err)
	}
	backup := NewFileStore(dstDir)
	committed := make([]Ticket, 0, len(tickets))
	for _, ticket := range tickets {
		committed = append(committed, ticket)
		if err := backup.Issue(ticket); err != nil {
			return backup.rollbackBackupFailure(committed, fmt.Errorf("backup login ticket %08x: %w", ticket.LoginKey, err))
		}
	}
	if err := backup.writeBackupManifest(tickets); err != nil {
		return backup.rollbackBackupFailure(committed, err)
	}
	if err := backup.syncStoreDir(); err != nil {
		return backup.rollbackBackupFailure(committed, fmt.Errorf("sync login ticket backup dir: %w", err))
	}
	return nil
}

func (s *FileStore) rollbackBackupFailure(tickets []Ticket, backupErr error) error {
	var rollbackErrs []error
	for _, ticket := range tickets {
		if err := os.Remove(s.ticketPath(ticket.LoginKey)); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("remove backup login ticket %08x: %w", ticket.LoginKey, err))
		}
	}
	if err := os.Remove(filepath.Join(s.dir, BackupManifestFilename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("remove backup manifest: %w", err))
	}
	if err := s.syncStoreDir(); err != nil {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("sync login ticket backup rollback dir: %w", err))
	}
	if len(rollbackErrs) == 0 {
		return backupErr
	}
	return errors.Join(append([]error{backupErr}, rollbackErrs...)...)
}

func (s *FileStore) writeBackupManifest(tickets []Ticket) error {
	manifest := BackupManifest{
		Format:  BackupManifestFormat,
		Summary: summarizeTickets(tickets, nil),
		Files:   make([]BackupManifestFile, 0, len(tickets)),
	}
	for _, ticket := range tickets {
		filename := filepath.Base(s.ticketPath(ticket.LoginKey))
		raw, err := os.ReadFile(filepath.Join(s.dir, filename))
		if err != nil {
			return fmt.Errorf("read backup login ticket %08x for manifest: %w", ticket.LoginKey, err)
		}
		checksum := sha256.Sum256(raw)
		manifest.Files = append(manifest.Files, BackupManifestFile{
			Login:     ticket.Login,
			LoginKey:  ticket.LoginKey,
			Filename:  filename,
			SizeBytes: int64(len(raw)),
			SHA256:    hex.EncodeToString(checksum[:]),
		})
	}
	return writeJSONFileAtomically(s.dir, BackupManifestFilename, manifest, "login ticket backup manifest")
}

func (s *FileStore) RestoreFrom(srcDir string) error {
	if s.dir == "" {
		return ErrStoreDirRequired
	}
	if strings.TrimSpace(srcDir) == "" {
		return ErrRestoreSourceRequired
	}
	if err := rejectRestoreDestinationInsideSource(srcDir, s.dir); err != nil {
		return err
	}
	if err := ensureEmptyDir(s.dir, ErrRestoreDirNotEmpty, "read login ticket restore dir"); err != nil {
		return err
	}
	tickets, err := s.loadBackupTicketsForRestore(srcDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create login ticket restore dir: %w", err)
	}
	committed := make([]Ticket, 0, len(tickets))
	for _, ticket := range tickets {
		committed = append(committed, ticket)
		if err := s.Issue(ticket); err != nil {
			return s.rollbackRestoreFailure(committed, fmt.Errorf("restore login ticket %08x: %w", ticket.LoginKey, err))
		}
	}
	if err := s.writeBackupManifest(tickets); err != nil {
		return s.rollbackRestoreFailure(committed, err)
	}
	if err := s.syncStoreDir(); err != nil {
		return s.rollbackRestoreFailure(committed, fmt.Errorf("sync login ticket restore dir: %w", err))
	}
	return nil
}

func (s *FileStore) rollbackRestoreFailure(tickets []Ticket, restoreErr error) error {
	var rollbackErrs []error
	for _, ticket := range tickets {
		if err := os.Remove(s.ticketPath(ticket.LoginKey)); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("remove restored login ticket %08x: %w", ticket.LoginKey, err))
		}
	}
	if err := os.Remove(filepath.Join(s.dir, BackupManifestFilename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("remove restored backup manifest: %w", err))
	}
	if err := s.syncStoreDir(); err != nil {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("sync login ticket restore rollback dir: %w", err))
	}
	if len(rollbackErrs) == 0 {
		return restoreErr
	}
	return errors.Join(append([]error{restoreErr}, rollbackErrs...)...)
}

func (s *FileStore) ValidateBackupFrom(srcDir string) (SnapshotSummary, error) {
	tickets, err := s.loadBackupTicketsForRestore(srcDir)
	if err != nil {
		return SnapshotSummary{}, err
	}
	crashTempFiles, err := NewFileStore(srcDir).crashTempFiles()
	if err != nil {
		return SnapshotSummary{}, err
	}
	return summarizeTickets(tickets, crashTempFiles), nil
}

func (s *FileStore) loadBackupTicketsForRestore(srcDir string) ([]Ticket, error) {
	if strings.TrimSpace(srcDir) == "" {
		return nil, ErrRestoreSourceRequired
	}
	if _, err := os.Stat(srcDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrRestoreSourceNotFound
		}
		return nil, fmt.Errorf("stat login ticket restore source dir: %w", err)
	}
	source := NewFileStore(srcDir)
	tickets, err := source.List()
	if err != nil {
		if errors.Is(err, ErrInvalidTicket) {
			return nil, errors.Join(fmt.Errorf("%w: load backup login ticket snapshots", ErrInvalidBackupManifest), err)
		}
		return nil, err
	}
	if err := source.validateBackupManifest(tickets); err != nil {
		return nil, err
	}
	return tickets, nil
}

func (s *FileStore) validateBackupManifest(tickets []Ticket) error {
	return s.validateBackupManifestForTickets(tickets, true)
}

func (s *FileStore) validateActiveBackupManifest() error {
	tickets, err := s.List()
	if err != nil {
		return err
	}
	return s.validateActiveBackupManifestForTickets(tickets)
}

func (s *FileStore) validateActiveBackupManifestForTickets(tickets []Ticket) error {
	if s.dir == "" {
		return ErrStoreDirRequired
	}
	manifestPath := filepath.Join(s.dir, BackupManifestFilename)
	if _, err := os.Lstat(manifestPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat active login ticket backup manifest: %w", err)
	}
	return s.validateBackupManifestForTickets(tickets, false)
}

func (s *FileStore) validateBackupManifestForTickets(tickets []Ticket, requireClosedDirectory bool) error {
	manifestPath := filepath.Join(s.dir, BackupManifestFilename)
	if err := rejectBackupEntrySymlink(manifestPath, "login ticket backup manifest"); err != nil {
		return err
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrBackupManifestRequired
		}
		return fmt.Errorf("read login ticket backup manifest: %w", err)
	}

	var manifest BackupManifest
	if err := decodeBackupManifestStrict(raw, &manifest); err != nil {
		return fmt.Errorf("%w: decode manifest: %v", ErrInvalidBackupManifest, err)
	}
	if manifest.Format != BackupManifestFormat {
		return fmt.Errorf("%w: format %q", ErrInvalidBackupManifest, manifest.Format)
	}

	wantSummary := summarizeTickets(tickets, nil)
	summaryHasEmptyCharacterSlotCount, err := backupManifestSummaryHasField(raw, "empty_character_slot_count")
	if err != nil {
		return fmt.Errorf("%w: inspect manifest summary: %v", ErrInvalidBackupManifest, err)
	}
	if !snapshotSummariesEqual(manifest.Summary, wantSummary, !summaryHasEmptyCharacterSlotCount) {
		return fmt.Errorf("%w: summary does not match committed snapshots", ErrInvalidBackupManifest)
	}
	if len(manifest.Files) != len(tickets) {
		return fmt.Errorf("%w: manifest lists %d files for %d tickets", ErrInvalidBackupManifest, len(manifest.Files), len(tickets))
	}

	ticketsByKey := make(map[uint32]Ticket, len(tickets))
	seenFiles := make(map[string]struct{}, len(manifest.Files))
	for _, ticket := range tickets {
		ticketsByKey[ticket.LoginKey] = ticket
	}
	for _, file := range manifest.Files {
		ticket, ok := ticketsByKey[file.LoginKey]
		if !ok {
			return fmt.Errorf("%w: manifest references unknown login key %08x", ErrInvalidBackupManifest, file.LoginKey)
		}
		if file.Login != ticket.Login {
			return fmt.Errorf("%w: manifest login %q does not match committed snapshot login %q for key %08x", ErrInvalidBackupManifest, file.Login, ticket.Login, file.LoginKey)
		}
		if file.Filename == "" || filepath.Base(file.Filename) != file.Filename {
			return fmt.Errorf("%w: manifest filename %q is not a base name", ErrInvalidBackupManifest, file.Filename)
		}
		wantFilename := filepath.Base(s.ticketPath(ticket.LoginKey))
		if file.Filename != wantFilename {
			return fmt.Errorf("%w: manifest filename %q does not match login key %08x", ErrInvalidBackupManifest, file.Filename, file.LoginKey)
		}
		if _, ok := seenFiles[file.Filename]; ok {
			return fmt.Errorf("%w: manifest repeats filename %q", ErrInvalidBackupManifest, file.Filename)
		}
		seenFiles[file.Filename] = struct{}{}

		snapshotPath := filepath.Join(s.dir, file.Filename)
		if err := rejectBackupEntrySymlink(snapshotPath, fmt.Sprintf("login ticket backup snapshot %q", file.Filename)); err != nil {
			return err
		}
		raw, err := os.ReadFile(snapshotPath)
		if err != nil {
			return fmt.Errorf("%w: read manifest login ticket %08x: %v", ErrInvalidBackupManifest, file.LoginKey, err)
		}
		if int64(len(raw)) != file.SizeBytes {
			return fmt.Errorf("%w: login ticket %08x size mismatch", ErrInvalidBackupManifest, file.LoginKey)
		}
		checksum := sha256.Sum256(raw)
		if got := hex.EncodeToString(checksum[:]); got != file.SHA256 {
			return fmt.Errorf("%w: login ticket %08x checksum mismatch", ErrInvalidBackupManifest, file.LoginKey)
		}
	}
	if requireClosedDirectory {
		if err := s.validateBackupDirectoryEntries(seenFiles); err != nil {
			return err
		}
	}
	return nil
}

func (s *FileStore) validateBackupDirectoryEntries(manifestFiles map[string]struct{}) error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("read login ticket backup dir for manifest coverage: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == BackupManifestFilename {
			continue
		}
		if _, ok := manifestFiles[name]; ok {
			continue
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: backup contains symlink entry %q", ErrInvalidBackupManifest, name)
		}
		if entry.IsDir() {
			return fmt.Errorf("%w: backup contains untracked directory %q", ErrInvalidBackupManifest, name)
		}
		if isLoginTicketCrashTempFilename(name) {
			continue
		}
		return fmt.Errorf("%w: backup contains untracked entry %q", ErrInvalidBackupManifest, name)
	}
	return nil
}

func removeBackupManifest(dir string) error {
	if err := os.Remove(filepath.Join(dir, BackupManifestFilename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func rejectBackupEntrySymlink(path string, context string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s is a symlink", ErrInvalidBackupManifest, context)
	}
	return nil
}

func isLoginTicketCrashTempFilename(name string) bool {
	return strings.HasPrefix(name, ".ticket-") && strings.HasSuffix(name, ".json")
}

func ensureEmptyDir(path string, nonEmptyErr error, readContext string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("%s: %w", readContext, err)
	}
	if len(entries) != 0 {
		return nonEmptyErr
	}
	return nil
}

func rejectBackupDestinationInsideStore(storeDir string, dstDir string) error {
	return rejectPathInsideOrEqual(storeDir, dstDir, ErrBackupDirInsideStore, "login ticket store", "login ticket backup")
}

func rejectRestoreDestinationInsideSource(srcDir string, dstDir string) error {
	return rejectPathInsideOrEqual(srcDir, dstDir, ErrRestoreDirInsideSource, "login ticket restore source", "login ticket restore")
}

func rejectPathInsideOrEqual(root string, candidate string, rejectedErr error, rootContext string, candidateContext string) error {
	rootPath, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return fmt.Errorf("resolve %s dir: %w", rootContext, err)
	}
	candidatePath, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return fmt.Errorf("resolve %s dir: %w", candidateContext, err)
	}
	inside, err := pathInsideOrEqual(rootPath, candidatePath)
	if err != nil {
		return fmt.Errorf("compare %s dir: %w", candidateContext, err)
	}
	if inside {
		return rejectedErr
	}

	resolvedRootPath, err := resolveExistingPath(rootPath)
	if err != nil {
		return fmt.Errorf("resolve %s symlinks: %w", rootContext, err)
	}
	resolvedCandidatePath, err := resolveExistingPath(candidatePath)
	if err != nil {
		return fmt.Errorf("resolve %s symlinks: %w", candidateContext, err)
	}
	inside, err = pathInsideOrEqual(resolvedRootPath, resolvedCandidatePath)
	if err != nil {
		return fmt.Errorf("compare resolved %s dir: %w", candidateContext, err)
	}
	if inside {
		return rejectedErr
	}
	return nil
}

func pathInsideOrEqual(root string, candidate string) (bool, error) {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false, err
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))), nil
}

func resolveExistingPath(path string) (string, error) {
	path, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	for range 255 {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			return filepath.Abs(filepath.Clean(resolved))
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		info, lstatErr := os.Lstat(path)
		if lstatErr == nil && info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return "", err
			}
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(path), target)
			}
			path = filepath.Clean(target)
			continue
		}
		if lstatErr != nil && !errors.Is(lstatErr, os.ErrNotExist) {
			return "", lstatErr
		}
		parent := filepath.Dir(path)
		if parent == path {
			return filepath.Abs(filepath.Clean(path))
		}
		parentResolved, err := resolveExistingPath(parent)
		if err != nil {
			return "", err
		}
		return filepath.Abs(filepath.Clean(filepath.Join(parentResolved, filepath.Base(path))))
	}
	return "", errors.New("too many symlinks while resolving path")
}

func writeJSONFileAtomically(dir, filename string, value any, context string) error {
	temp, err := os.CreateTemp(dir, ".ticket-*.json")
	if err != nil {
		return fmt.Errorf("create %s temp file: %w", context, err)
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()
	encoder := json.NewEncoder(temp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("encode %s: %w", context, err)
	}
	if !durableSyncDisabledForTest {
		if err := temp.Sync(); err != nil {
			return fmt.Errorf("sync %s temp file: %w", context, err)
		}
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close %s temp file: %w", context, err)
	}
	if err := os.Rename(temp.Name(), filepath.Join(dir, filename)); err != nil {
		return fmt.Errorf("commit %s file: %w", context, err)
	}
	return nil
}

func decodeBackupManifestStrict(raw []byte, manifest *BackupManifest) error {
	if !utf8.Valid(raw) {
		return errors.New("invalid utf-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(manifest); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return errors.New("unexpected trailing JSON value")
		}
		return err
	}
	return nil
}

func backupManifestSummaryHasField(raw []byte, field string) (bool, error) {
	var envelope struct {
		Summary map[string]json.RawMessage `json:"summary"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return false, err
	}
	_, ok := envelope.Summary[field]
	return ok, nil
}

func snapshotSummariesEqual(a, b SnapshotSummary, allowMissingEmptyCharacterSlotCount bool) bool {
	if a.TicketCount != b.TicketCount || a.CharacterCount != b.CharacterCount || !emptyCharacterSlotCountsCompatible(a.EmptyCharacterSlotCount, b.EmptyCharacterSlotCount, allowMissingEmptyCharacterSlotCount) || a.CrashTempCount != b.CrashTempCount || len(a.Logins) != len(b.Logins) || len(a.LoginKeys) != len(b.LoginKeys) || len(a.CrashTempFiles) != len(b.CrashTempFiles) {
		return false
	}
	for i := range a.Logins {
		if a.Logins[i] != b.Logins[i] {
			return false
		}
	}
	for i := range a.LoginKeys {
		if a.LoginKeys[i] != b.LoginKeys[i] {
			return false
		}
	}
	for i := range a.CrashTempFiles {
		if a.CrashTempFiles[i] != b.CrashTempFiles[i] {
			return false
		}
	}
	if !issuedAtPointersEqual(a.OldestIssuedAt, b.OldestIssuedAt) || !issuedAtPointersEqual(a.NewestIssuedAt, b.NewestIssuedAt) {
		return false
	}
	return true
}

func emptyCharacterSlotCountsCompatible(got, want int, allowMissing bool) bool {
	if got == want {
		return true
	}
	return allowMissing && got == 0
}

func issuedAtPointersEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.UTC().Equal(b.UTC())
}
