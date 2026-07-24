package accountstore

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
	"sort"
	"strings"

	"github.com/MikelCalvo/go-metin2-server/internal/inventory"
	"github.com/MikelCalvo/go-metin2-server/internal/loginticket"
	quickslotproto "github.com/MikelCalvo/go-metin2-server/internal/proto/quickslot"
)

const (
	BackupManifestFilename = "account-backup-manifest.json"
	BackupManifestFormat   = "go-metin2-account-backup-v1"
)

var (
	ErrStoreDirRequired       = errors.New("account store dir is required")
	ErrLoginRequired          = errors.New("account login is required")
	ErrAccountNotFound        = errors.New("account not found")
	ErrInvalidAccount         = errors.New("invalid account snapshot")
	ErrBackupDirRequired      = errors.New("account backup dir is required")
	ErrBackupDirNotEmpty      = errors.New("account backup dir is not empty")
	ErrBackupDirInsideStore   = errors.New("account backup dir is inside account store")
	ErrRestoreSourceRequired  = errors.New("account restore source dir is required")
	ErrRestoreSourceNotFound  = errors.New("account restore source dir not found")
	ErrRestoreDirNotEmpty     = errors.New("account restore dir is not empty")
	ErrRestoreDirInsideSource = errors.New("account restore dir is inside backup source")
	ErrBackupManifestRequired = errors.New("account backup manifest is required")
	ErrInvalidBackupManifest  = errors.New("invalid account backup manifest")
)

type Account struct {
	Login      string                  `json:"login"`
	Empire     uint8                   `json:"empire"`
	Characters []loginticket.Character `json:"characters"`
}

type SnapshotSummary struct {
	AccountCount   int      `json:"account_count"`
	CharacterCount int      `json:"character_count"`
	Logins         []string `json:"logins"`
	CrashTempCount int      `json:"crash_temp_count,omitempty"`
	CrashTempFiles []string `json:"crash_temp_files,omitempty"`
}

type BackupManifest struct {
	Format  string               `json:"format"`
	Summary SnapshotSummary      `json:"summary"`
	Files   []BackupManifestFile `json:"files"`
}

type BackupManifestFile struct {
	Login     string `json:"login"`
	Filename  string `json:"filename"`
	SizeBytes int64  `json:"size_bytes"`
	SHA256    string `json:"sha256"`
}

func normalizeAccountCharacters(characters []loginticket.Character) []loginticket.Character {
	cloned := loginticket.CloneCharacters(characters)
	for i := range cloned {
		cloned[i].NormalizeItemState()
	}
	return cloned
}

type Store interface {
	Load(login string) (Account, error)
	Save(account Account) error
}

func (s *FileStore) List() ([]Account, error) {
	if s.dir == "" {
		return nil, ErrStoreDirRequired
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Account{}, nil
		}
		return nil, fmt.Errorf("read account store dir: %w", err)
	}

	accounts := make([]Account, 0, len(entries))
	seenLogins := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.Name() == BackupManifestFilename {
			continue
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		login, err := decodeAccountFilenameLogin(entry.Name())
		if err != nil {
			return nil, err
		}
		if canonicalFilename := filepath.Base(s.accountPath(login)); entry.Name() != canonicalFilename {
			return nil, fmt.Errorf("%w: account filename %q is not canonical for login %q", ErrInvalidAccount, entry.Name(), login)
		}
		account, err := s.Load(login)
		if err != nil {
			return nil, err
		}
		normalizedLogin := strings.ToLower(account.Login)
		if previousLogin, ok := seenLogins[normalizedLogin]; ok {
			return nil, fmt.Errorf("%w: account login %q duplicates %q", ErrInvalidAccount, account.Login, previousLogin)
		}
		seenLogins[normalizedLogin] = account.Login
		accounts = append(accounts, account)
	}
	sort.Slice(accounts, func(i, j int) bool {
		return strings.ToLower(accounts[i].Login) < strings.ToLower(accounts[j].Login)
	})
	return accounts, nil
}

func (s *FileStore) Validate() (SnapshotSummary, error) {
	accounts, err := s.List()
	if err != nil {
		return SnapshotSummary{}, err
	}
	crashTempFiles, err := s.crashTempFiles()
	if err != nil {
		return SnapshotSummary{}, err
	}
	summary := SnapshotSummary{
		AccountCount:   len(accounts),
		Logins:         make([]string, 0, len(accounts)),
		CrashTempCount: len(crashTempFiles),
		CrashTempFiles: crashTempFiles,
	}
	for _, account := range accounts {
		summary.Logins = append(summary.Logins, account.Login)
		summary.CharacterCount += len(account.Characters)
	}
	return summary, nil
}

func (s *FileStore) CleanupCrashTempFiles() (SnapshotSummary, error) {
	if s.dir == "" {
		return SnapshotSummary{}, ErrStoreDirRequired
	}
	if _, err := s.Validate(); err != nil {
		return SnapshotSummary{}, err
	}
	crashTempFiles, err := s.crashTempFiles()
	if err != nil {
		return SnapshotSummary{}, err
	}
	if len(crashTempFiles) == 0 {
		return s.Validate()
	}
	for _, filename := range crashTempFiles {
		if err := os.Remove(filepath.Join(s.dir, filename)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return SnapshotSummary{}, fmt.Errorf("remove account crash temp file %q: %w", filename, err)
		}
	}
	if err := syncStoreDir(s.dir); err != nil {
		return SnapshotSummary{}, fmt.Errorf("sync account store dir after crash temp cleanup: %w", err)
	}
	return s.Validate()
}

func (s *FileStore) crashTempFiles() ([]string, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read account store crash temp files: %w", err)
	}
	files := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if isAccountCrashTempFilename(name) {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, nil
	}
	return files, nil
}

type FileStore struct {
	dir string
}

func NewFileStore(dir string) *FileStore {
	return &FileStore{dir: dir}
}

func (s *FileStore) Load(login string) (Account, error) {
	if s.dir == "" {
		return Account{}, ErrStoreDirRequired
	}
	if login == "" {
		return Account{}, ErrLoginRequired
	}

	raw, err := os.ReadFile(s.accountPath(login))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Account{}, ErrAccountNotFound
		}
		return Account{}, fmt.Errorf("read account: %w", err)
	}

	var account Account
	if err := decodeAccountStrict(raw, &account); err != nil {
		return Account{}, fmt.Errorf("%w: decode account: %v", ErrInvalidAccount, err)
	}
	account.Characters = normalizeAccountCharacters(account.Characters)
	if err := validateLoadedAccountForLogin(login, account); err != nil {
		return Account{}, err
	}
	return account, nil
}

func (s *FileStore) Save(account Account) error {
	if s.dir == "" {
		return ErrStoreDirRequired
	}
	if account.Login == "" {
		return ErrLoginRequired
	}
	account.Characters = normalizeAccountCharacters(account.Characters)
	if err := validateAccount(account); err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create account store dir: %w", err)
	}

	temp, err := os.CreateTemp(s.dir, ".account-*.json")
	if err != nil {
		return fmt.Errorf("create account temp file: %w", err)
	}
	defer func() {
		_ = temp.Close()
		_ = os.Remove(temp.Name())
	}()

	if err := json.NewEncoder(temp).Encode(account); err != nil {
		return fmt.Errorf("encode account: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync account temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close account temp file: %w", err)
	}
	if err := os.Rename(temp.Name(), s.accountPath(account.Login)); err != nil {
		return fmt.Errorf("commit account file: %w", err)
	}
	if err := syncStoreDir(s.dir); err != nil {
		return fmt.Errorf("sync account store dir: %w", err)
	}
	return nil
}

func (s *FileStore) BackupTo(dstDir string) error {
	if s.dir == "" {
		return ErrStoreDirRequired
	}
	if dstDir == "" {
		return ErrBackupDirRequired
	}
	if err := rejectBackupDestinationInsideStore(s.dir, dstDir); err != nil {
		return err
	}
	if err := ensureEmptyDir(dstDir, ErrBackupDirNotEmpty, "read account backup dir"); err != nil {
		return err
	}
	accounts, err := s.List()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("create account backup dir: %w", err)
	}
	backup := NewFileStore(dstDir)
	committed := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		committed = append(committed, account)
		if err := backup.Save(account); err != nil {
			return backup.rollbackBackupFailure(committed, fmt.Errorf("backup account %q: %w", account.Login, err))
		}
	}
	if err := backup.writeBackupManifest(accounts); err != nil {
		return backup.rollbackBackupFailure(committed, err)
	}
	if err := syncStoreDir(dstDir); err != nil {
		return backup.rollbackBackupFailure(committed, fmt.Errorf("sync account backup dir: %w", err))
	}
	return nil
}

func (s *FileStore) rollbackBackupFailure(accounts []Account, backupErr error) error {
	var rollbackErrs []error
	for _, account := range accounts {
		if err := os.Remove(s.accountPath(account.Login)); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("remove backup account %q: %w", account.Login, err))
		}
	}
	if err := os.Remove(filepath.Join(s.dir, BackupManifestFilename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("remove backup manifest: %w", err))
	}
	if err := syncStoreDir(s.dir); err != nil {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("sync account backup rollback dir: %w", err))
	}
	if len(rollbackErrs) == 0 {
		return backupErr
	}
	return errors.Join(append([]error{backupErr}, rollbackErrs...)...)
}

func (s *FileStore) writeBackupManifest(accounts []Account) error {
	manifest := BackupManifest{
		Format: BackupManifestFormat,
		Summary: SnapshotSummary{
			AccountCount: len(accounts),
			Logins:       make([]string, 0, len(accounts)),
		},
		Files: make([]BackupManifestFile, 0, len(accounts)),
	}
	for _, account := range accounts {
		manifest.Summary.Logins = append(manifest.Summary.Logins, account.Login)
		manifest.Summary.CharacterCount += len(account.Characters)
		filename := filepath.Base(s.accountPath(account.Login))
		raw, err := os.ReadFile(filepath.Join(s.dir, filename))
		if err != nil {
			return fmt.Errorf("read backup account %q for manifest: %w", account.Login, err)
		}
		checksum := sha256.Sum256(raw)
		manifest.Files = append(manifest.Files, BackupManifestFile{
			Login:     account.Login,
			Filename:  filename,
			SizeBytes: int64(len(raw)),
			SHA256:    hex.EncodeToString(checksum[:]),
		})
	}
	return writeJSONFileAtomically(s.dir, BackupManifestFilename, manifest, "account backup manifest")
}

func (s *FileStore) RestoreFrom(srcDir string) error {
	if s.dir == "" {
		return ErrStoreDirRequired
	}
	if srcDir == "" {
		return ErrRestoreSourceRequired
	}
	if err := rejectRestoreDestinationInsideSource(srcDir, s.dir); err != nil {
		return err
	}
	if err := ensureEmptyDir(s.dir, ErrRestoreDirNotEmpty, "read account restore dir"); err != nil {
		return err
	}
	accounts, err := s.loadBackupAccountsForRestore(srcDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create account restore dir: %w", err)
	}
	committed := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		committed = append(committed, account)
		if err := s.Save(account); err != nil {
			return s.rollbackRestoreFailure(committed, fmt.Errorf("restore account %q: %w", account.Login, err))
		}
	}
	if err := s.writeBackupManifest(accounts); err != nil {
		return s.rollbackRestoreFailure(committed, err)
	}
	if err := syncStoreDir(s.dir); err != nil {
		return s.rollbackRestoreFailure(committed, fmt.Errorf("sync account restore dir: %w", err))
	}
	return nil
}

func (s *FileStore) rollbackRestoreFailure(accounts []Account, restoreErr error) error {
	var rollbackErrs []error
	for _, account := range accounts {
		if err := os.Remove(s.accountPath(account.Login)); err != nil && !errors.Is(err, os.ErrNotExist) {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("remove restored account %q: %w", account.Login, err))
		}
	}
	if err := os.Remove(filepath.Join(s.dir, BackupManifestFilename)); err != nil && !errors.Is(err, os.ErrNotExist) {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("remove restored backup manifest: %w", err))
	}
	if err := syncStoreDir(s.dir); err != nil {
		rollbackErrs = append(rollbackErrs, fmt.Errorf("sync account restore rollback dir: %w", err))
	}
	if len(rollbackErrs) == 0 {
		return restoreErr
	}
	return errors.Join(append([]error{restoreErr}, rollbackErrs...)...)
}

func (s *FileStore) ValidateBackupFrom(srcDir string) (SnapshotSummary, error) {
	accounts, err := s.loadBackupAccountsForRestore(srcDir)
	if err != nil {
		return SnapshotSummary{}, err
	}
	summary := SnapshotSummary{AccountCount: len(accounts), Logins: make([]string, 0, len(accounts))}
	for _, account := range accounts {
		summary.Logins = append(summary.Logins, account.Login)
		summary.CharacterCount += len(account.Characters)
	}
	return summary, nil
}

func (s *FileStore) loadBackupAccountsForRestore(srcDir string) ([]Account, error) {
	if srcDir == "" {
		return nil, ErrRestoreSourceRequired
	}
	if _, err := os.Stat(srcDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrRestoreSourceNotFound
		}
		return nil, fmt.Errorf("stat account restore source dir: %w", err)
	}
	source := NewFileStore(srcDir)
	accounts, err := source.List()
	if err != nil {
		return nil, err
	}
	if err := source.validateBackupManifest(accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (s *FileStore) validateBackupManifest(accounts []Account) error {
	manifestPath := filepath.Join(s.dir, BackupManifestFilename)
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrBackupManifestRequired
		}
		return fmt.Errorf("read account backup manifest: %w", err)
	}

	var manifest BackupManifest
	if err := decodeBackupManifestStrict(raw, &manifest); err != nil {
		return fmt.Errorf("%w: decode manifest: %v", ErrInvalidBackupManifest, err)
	}
	if manifest.Format != BackupManifestFormat {
		return fmt.Errorf("%w: format %q", ErrInvalidBackupManifest, manifest.Format)
	}

	wantSummary := SnapshotSummary{AccountCount: len(accounts), Logins: make([]string, 0, len(accounts))}
	for _, account := range accounts {
		wantSummary.Logins = append(wantSummary.Logins, account.Login)
		wantSummary.CharacterCount += len(account.Characters)
	}
	if !snapshotSummariesEqual(manifest.Summary, wantSummary) {
		return fmt.Errorf("%w: summary does not match committed snapshots", ErrInvalidBackupManifest)
	}
	if len(manifest.Files) != len(accounts) {
		return fmt.Errorf("%w: manifest lists %d files for %d accounts", ErrInvalidBackupManifest, len(manifest.Files), len(accounts))
	}

	accountsByLogin := make(map[string]Account, len(accounts))
	seenFiles := make(map[string]struct{}, len(manifest.Files))
	for _, account := range accounts {
		accountsByLogin[strings.ToLower(account.Login)] = account
	}
	for _, file := range manifest.Files {
		account, ok := accountsByLogin[strings.ToLower(file.Login)]
		if !ok {
			return fmt.Errorf("%w: manifest references unknown login %q", ErrInvalidBackupManifest, file.Login)
		}
		if file.Login != account.Login {
			return fmt.Errorf("%w: manifest login %q does not match committed snapshot login %q", ErrInvalidBackupManifest, file.Login, account.Login)
		}
		if file.Filename == "" || filepath.Base(file.Filename) != file.Filename {
			return fmt.Errorf("%w: manifest filename %q is not a base name", ErrInvalidBackupManifest, file.Filename)
		}
		wantFilename := filepath.Base(s.accountPath(account.Login))
		if file.Filename != wantFilename {
			return fmt.Errorf("%w: manifest filename %q does not match login %q", ErrInvalidBackupManifest, file.Filename, file.Login)
		}
		if _, ok := seenFiles[file.Filename]; ok {
			return fmt.Errorf("%w: manifest repeats filename %q", ErrInvalidBackupManifest, file.Filename)
		}
		seenFiles[file.Filename] = struct{}{}

		raw, err := os.ReadFile(filepath.Join(s.dir, file.Filename))
		if err != nil {
			return fmt.Errorf("%w: read manifest account %q: %v", ErrInvalidBackupManifest, file.Login, err)
		}
		if int64(len(raw)) != file.SizeBytes {
			return fmt.Errorf("%w: account %q size mismatch", ErrInvalidBackupManifest, file.Login)
		}
		checksum := sha256.Sum256(raw)
		if got := hex.EncodeToString(checksum[:]); got != file.SHA256 {
			return fmt.Errorf("%w: account %q checksum mismatch", ErrInvalidBackupManifest, file.Login)
		}
	}
	if err := s.validateBackupDirectoryEntries(seenFiles); err != nil {
		return err
	}
	return nil
}

func (s *FileStore) validateBackupDirectoryEntries(manifestFiles map[string]struct{}) error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("read account backup dir for manifest coverage: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == BackupManifestFilename {
			continue
		}
		if _, ok := manifestFiles[name]; ok {
			continue
		}
		if isAccountCrashTempFilename(name) {
			continue
		}
		return fmt.Errorf("%w: backup contains untracked entry %q", ErrInvalidBackupManifest, name)
	}
	return nil
}

func isAccountCrashTempFilename(name string) bool {
	return strings.HasPrefix(name, ".account-") && strings.HasSuffix(name, ".json")
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
	return rejectPathInsideOrEqual(storeDir, dstDir, ErrBackupDirInsideStore, "account store", "account backup")
}

func rejectRestoreDestinationInsideSource(srcDir string, dstDir string) error {
	return rejectPathInsideOrEqual(srcDir, dstDir, ErrRestoreDirInsideSource, "account restore source", "account restore")
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
	temp, err := os.CreateTemp(dir, ".account-*.json")
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
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync %s temp file: %w", context, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close %s temp file: %w", context, err)
	}
	if err := os.Rename(temp.Name(), filepath.Join(dir, filename)); err != nil {
		return fmt.Errorf("commit %s file: %w", context, err)
	}
	return nil
}

var syncStoreDir = syncDir

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (s *FileStore) accountPath(login string) string {
	normalized := strings.ToLower(login)
	filename := hex.EncodeToString([]byte(normalized)) + ".json"
	return filepath.Join(s.dir, filename)
}

func decodeAccountFilenameLogin(filename string) (string, error) {
	encoded := strings.TrimSuffix(filename, ".json")
	decoded, err := hex.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("%w: account filename %q is not hex login JSON", ErrInvalidAccount, filename)
	}
	login := string(decoded)
	if login == "" {
		return "", fmt.Errorf("%w: account filename %q decodes to empty login", ErrInvalidAccount, filename)
	}
	return login, nil
}

func decodeAccountStrict(raw []byte, account *Account) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(account); err != nil {
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

func decodeBackupManifestStrict(raw []byte, manifest *BackupManifest) error {
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

func snapshotSummariesEqual(a, b SnapshotSummary) bool {
	if a.AccountCount != b.AccountCount || a.CharacterCount != b.CharacterCount || a.CrashTempCount != b.CrashTempCount || len(a.Logins) != len(b.Logins) || len(a.CrashTempFiles) != len(b.CrashTempFiles) {
		return false
	}
	for i := range a.Logins {
		if a.Logins[i] != b.Logins[i] {
			return false
		}
	}
	for i := range a.CrashTempFiles {
		if a.CrashTempFiles[i] != b.CrashTempFiles[i] {
			return false
		}
	}
	return true
}

func validateAccount(account Account) error {
	if err := validateUniqueCharacterIdentity(account.Characters); err != nil {
		return err
	}
	for _, character := range account.Characters {
		if err := validateCharacterItemPayloads(character); err != nil {
			return err
		}
		if err := validateCharacterUniqueItemInstanceIDs(character); err != nil {
			return err
		}
		if err := validateCharacterUniqueEquipmentSlots(character); err != nil {
			return err
		}
		if err := validateCharacterQuickslots(character); err != nil {
			return err
		}
	}
	return nil
}

func validateUniqueCharacterIdentity(characters []loginticket.Character) error {
	ids := make(map[uint32]string, len(characters))
	names := make(map[string]uint32, len(characters))
	for _, character := range characters {
		if previousName, ok := ids[character.ID]; ok {
			return fmt.Errorf("%w: character id %d is used by %q and %q", ErrInvalidAccount, character.ID, previousName, character.Name)
		}
		ids[character.ID] = character.Name

		normalizedName := strings.ToLower(strings.TrimSpace(character.Name))
		if normalizedName == "" {
			continue
		}
		if previousID, ok := names[normalizedName]; ok {
			return fmt.Errorf("%w: character name %q is used by id %d and id %d", ErrInvalidAccount, character.Name, previousID, character.ID)
		}
		names[normalizedName] = character.ID
	}
	return nil
}

func validateLoadedAccountForLogin(requestedLogin string, account Account) error {
	if account.Login == "" {
		return fmt.Errorf("%w: account login is empty", ErrInvalidAccount)
	}
	if !strings.EqualFold(account.Login, requestedLogin) {
		return fmt.Errorf("%w: snapshot login %q does not match requested login %q", ErrInvalidAccount, account.Login, requestedLogin)
	}
	return validateAccount(account)
}

func validateCharacterItemPayloads(character loginticket.Character) error {
	for _, item := range character.Inventory {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("%w: inventory item %d: %v", ErrInvalidAccount, item.ID, err)
		}
		if item.Slot >= inventory.CarriedInventorySlotCount {
			return fmt.Errorf("%w: inventory item %d: slot %d out of range", ErrInvalidAccount, item.ID, item.Slot)
		}
	}
	for _, item := range character.Equipment {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("%w: equipment item %d: %v", ErrInvalidAccount, item.ID, err)
		}
	}
	return nil
}

func validateCharacterUniqueItemInstanceIDs(character loginticket.Character) error {
	itemIDs := make(map[uint64]string, len(character.Inventory)+len(character.Equipment))
	for _, item := range character.Inventory {
		if previous, ok := itemIDs[item.ID]; ok {
			return fmt.Errorf("%w: item instance id %d appears in %s and inventory slot %d", ErrInvalidAccount, item.ID, previous, item.Slot)
		}
		itemIDs[item.ID] = fmt.Sprintf("inventory slot %d", item.Slot)
	}
	for _, item := range character.Equipment {
		if previous, ok := itemIDs[item.ID]; ok {
			return fmt.Errorf("%w: item instance id %d appears in %s and equipment slot %s", ErrInvalidAccount, item.ID, previous, item.EquipSlot.String())
		}
		itemIDs[item.ID] = fmt.Sprintf("equipment slot %s", item.EquipSlot.String())
	}
	return nil
}

func validateCharacterUniqueEquipmentSlots(character loginticket.Character) error {
	equipmentSlots := make(map[uint8]uint64, len(character.Equipment))
	for _, item := range character.Equipment {
		if previousID, ok := equipmentSlots[uint8(item.EquipSlot)]; ok {
			return fmt.Errorf("%w: equipment slot %s contains item %d and item %d", ErrInvalidAccount, item.EquipSlot.String(), previousID, item.ID)
		}
		equipmentSlots[uint8(item.EquipSlot)] = item.ID
	}
	return nil
}

func validateCharacterQuickslots(character loginticket.Character) error {
	quickslotPositions := make(map[uint8]loginticket.Quickslot, len(character.Quickslots))
	quickslotTuples := make(map[quickslotTuple]uint8, len(character.Quickslots))
	for _, quickslot := range character.Quickslots {
		if !validQuickslotTuple(quickslot) {
			return fmt.Errorf("%w: quickslot position %d has invalid type %d slot %d", ErrInvalidAccount, quickslot.Position, quickslot.Type, quickslot.Slot)
		}
		if previous, ok := quickslotPositions[quickslot.Position]; ok {
			return fmt.Errorf("%w: quickslot position %d contains type %d slot %d and type %d slot %d", ErrInvalidAccount, quickslot.Position, previous.Type, previous.Slot, quickslot.Type, quickslot.Slot)
		}
		if quickslot.Type == quickslotproto.TypeSkill || quickslot.Type == quickslotproto.TypeCommand {
			tuple := quickslotTuple{Type: quickslot.Type, Slot: quickslot.Slot}
			if previousPosition, ok := quickslotTuples[tuple]; ok {
				return fmt.Errorf("%w: quickslot type %d slot %d is bound at positions %d and %d", ErrInvalidAccount, quickslot.Type, quickslot.Slot, previousPosition, quickslot.Position)
			}
			quickslotTuples[tuple] = quickslot.Position
		}
		quickslotPositions[quickslot.Position] = quickslot
	}
	return nil
}

type quickslotTuple struct {
	Type uint8
	Slot uint8
}

func validQuickslotTuple(quickslot loginticket.Quickslot) bool {
	if quickslot.Position >= 36 {
		return false
	}
	switch quickslot.Type {
	case quickslotproto.TypeNone:
		return quickslot.Slot == 0
	case quickslotproto.TypeItem:
		return quickslot.Slot < uint8(inventory.CarriedInventorySlotCount)
	case quickslotproto.TypeSkill:
		return quickslot.Slot < 200
	case quickslotproto.TypeCommand:
		return quickslot.Slot < 60
	default:
		return false
	}
}
