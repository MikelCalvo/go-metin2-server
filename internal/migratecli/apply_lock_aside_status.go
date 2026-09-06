package migratecli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

const applyLockAsideStatusFormat = "go-metin2-migration-apply-lock-aside-status-v1"

var ErrApplyLockAsideStatus = errors.New("apply-lock-aside-status failed")

type applyLockAsideStatus struct {
	Format          string                   `json:"format"`
	Present         bool                     `json:"present"`
	AsideSHA256     string                   `json:"aside_sha256,omitempty"`
	AsidePathExists *bool                    `json:"aside_path_exists,omitempty"`
	LockFileExists  *bool                    `json:"lock_file_exists,omitempty"`
	Aside           *migrationApplyLockAside `json:"aside,omitempty"`
}

func runApplyLockAsideStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("apply-lock-aside-status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	var asidePath string
	var requireAsidePathExists bool
	var requireLockFileAbsent bool
	flags.StringVar(&asidePath, "aside", "", "path to a retained apply-lock-aside JSON file")
	flags.BoolVar(&requireAsidePathExists, "require-aside-path-exists", false, "fail closed unless aside_path is a present non-symlink regular file")
	flags.BoolVar(&requireLockFileAbsent, "require-lock-file-absent", false, "fail closed unless the original lock_file path is absent")
	flags.Usage = func() { printApplyLockAsideStatusUsage(stderr) }
	if err := flags.Parse(args); err != nil {
		return exitUsage
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected apply-lock-aside-status argument %q\n", flags.Arg(0))
		printApplyLockAsideStatusUsage(stderr)
		return exitUsage
	}
	if strings.TrimSpace(asidePath) == "" {
		fmt.Fprintln(stderr, "--aside is required for apply-lock-aside-status")
		printApplyLockAsideStatusUsage(stderr)
		return exitUsage
	}

	raw, present, err := readOptionalApplyLockAsideStatusFile(asidePath)
	if err != nil {
		fmt.Fprintf(stderr, "apply-lock-aside-status: %v\n", err)
		return exitError
	}
	if !present {
		if requireAsidePathExists {
			fmt.Fprintln(stderr, "apply-lock-aside-status: --require-aside-path-exists failed: aside is absent")
			return exitError
		}
		if requireLockFileAbsent {
			fmt.Fprintln(stderr, "apply-lock-aside-status: --require-lock-file-absent failed: aside is absent")
			return exitError
		}
		return writeJSON(stdout, stderr, applyLockAsideStatus{
			Format:  applyLockAsideStatusFormat,
			Present: false,
		})
	}

	aside, err := decodeApplyLockAside(raw)
	if err != nil {
		fmt.Fprintf(stderr, "apply-lock-aside-status: %v\n", err)
		return exitError
	}
	if err := validateRetainedApplyLockAside(&aside); err != nil {
		fmt.Fprintf(stderr, "apply-lock-aside-status: %v\n", err)
		return exitError
	}

	asidePathExists, err := regularNonSymlinkFileExists(aside.AsidePath)
	if err != nil {
		fmt.Fprintf(stderr, "apply-lock-aside-status: %v\n", err)
		return exitError
	}
	lockFileExists, err := pathExists(aside.LockFile)
	if err != nil {
		fmt.Fprintf(stderr, "apply-lock-aside-status: %v\n", err)
		return exitError
	}
	if requireAsidePathExists && !asidePathExists {
		fmt.Fprintln(stderr, "apply-lock-aside-status: --require-aside-path-exists failed: aside_path is not a present regular file")
		return exitError
	}
	if requireLockFileAbsent && lockFileExists {
		fmt.Fprintln(stderr, "apply-lock-aside-status: --require-lock-file-absent failed: lock_file still exists")
		return exitError
	}

	return writeJSON(stdout, stderr, applyLockAsideStatus{
		Format:          applyLockAsideStatusFormat,
		Present:         true,
		AsideSHA256:     sha256Hex(raw),
		AsidePathExists: boolPtr(asidePathExists),
		LockFileExists:  boolPtr(lockFileExists),
		Aside:           &aside,
	})
}

func readOptionalApplyLockAsideStatusFile(path string) ([]byte, bool, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, false, fmt.Errorf("%w: aside path is required", ErrApplyLockAsideStatus)
	}
	info, err := os.Lstat(trimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("%w: stat aside: %v", ErrApplyLockAsideStatus, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, fmt.Errorf("%w: aside must not be a symlink: %s", ErrApplyLockAsideStatus, trimmed)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%w: aside must be a regular file: %s", ErrApplyLockAsideStatus, trimmed)
	}
	file, err := os.Open(trimmed)
	if err != nil {
		return nil, false, fmt.Errorf("%w: open aside: %v", ErrApplyLockAsideStatus, err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("%w: stat opened aside: %v", ErrApplyLockAsideStatus, err)
	}
	if !openedInfo.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%w: opened aside must be a regular file: %s", ErrApplyLockAsideStatus, trimmed)
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxMigrationApplyLockBytes+1))
	if err != nil {
		return nil, false, fmt.Errorf("%w: read aside: %v", ErrApplyLockAsideStatus, err)
	}
	if len(raw) > maxMigrationApplyLockBytes {
		return nil, false, fmt.Errorf("%w: aside exceeds %d bytes", ErrApplyLockAsideStatus, maxMigrationApplyLockBytes)
	}
	if !utf8.Valid(raw) {
		return nil, false, fmt.Errorf("%w: aside is not valid UTF-8", ErrApplyLockAsideStatus)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, false, fmt.Errorf("%w: aside is empty", ErrApplyLockAsideStatus)
	}
	return raw, true, nil
}

func decodeApplyLockAside(raw []byte) (migrationApplyLockAside, error) {
	var formatEnvelope struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(raw, &formatEnvelope); err != nil {
		return migrationApplyLockAside{}, fmt.Errorf("%w: decode aside: %v", ErrApplyLockAsideStatus, err)
	}
	if formatEnvelope.Format != migrationApplyLockAsideFormat {
		return migrationApplyLockAside{}, fmt.Errorf("%w: unexpected format %q", ErrApplyLockAsideStatus, formatEnvelope.Format)
	}
	var aside migrationApplyLockAside
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&aside); err != nil {
		return migrationApplyLockAside{}, fmt.Errorf("%w: decode aside: %v", ErrApplyLockAsideStatus, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return migrationApplyLockAside{}, fmt.Errorf("%w: aside has trailing JSON", ErrApplyLockAsideStatus)
	}
	return aside, nil
}

func validateRetainedApplyLockAside(aside *migrationApplyLockAside) error {
	if aside == nil {
		return fmt.Errorf("%w: aside is required", ErrApplyLockAsideStatus)
	}
	if aside.Format != migrationApplyLockAsideFormat {
		return fmt.Errorf("%w: unexpected format %q", ErrApplyLockAsideStatus, aside.Format)
	}
	lockFile := strings.TrimSpace(aside.LockFile)
	if lockFile == "" {
		return fmt.Errorf("%w: lock_file is required", ErrApplyLockAsideStatus)
	}
	asidePath := strings.TrimSpace(aside.AsidePath)
	if asidePath == "" {
		return fmt.Errorf("%w: aside_path is required", ErrApplyLockAsideStatus)
	}
	renamedAt := strings.TrimSpace(aside.RenamedAt)
	if renamedAt == "" {
		return fmt.Errorf("%w: renamed_at is required", ErrApplyLockAsideStatus)
	}
	parsed, err := time.Parse(time.RFC3339Nano, renamedAt)
	if err != nil {
		return fmt.Errorf("%w: invalid renamed_at: %v", ErrApplyLockAsideStatus, err)
	}
	wantAsidePath := lockAsidePath(lockFile, parsed)
	if asidePath != wantAsidePath {
		return fmt.Errorf("%w: aside_path %q does not match lock_file and renamed_at", ErrApplyLockAsideStatus, aside.AsidePath)
	}
	if aside.Lock == nil {
		return fmt.Errorf("%w: lock is required", ErrApplyLockAsideStatus)
	}
	normalized, err := normalizeMigrationApplyLock(*aside.Lock)
	if err != nil {
		return fmt.Errorf("%w: invalid lock: %v", ErrApplyLockAsideStatus, err)
	}
	if aside.HolderPIDAlive == nil {
		return fmt.Errorf("%w: holder_pid_alive is required", ErrApplyLockAsideStatus)
	}
	if aside.HolderPIDCheck != migrationApplyLockHolderPIDCheck {
		return fmt.Errorf("%w: unexpected holder_pid_check %q", ErrApplyLockAsideStatus, aside.HolderPIDCheck)
	}
	if aside.HolderHostnameLocal == nil {
		return fmt.Errorf("%w: holder_hostname_local is required", ErrApplyLockAsideStatus)
	}
	if aside.HolderHostnameCheck != migrationApplyLockHolderHostnameCheck {
		return fmt.Errorf("%w: unexpected holder_hostname_check %q", ErrApplyLockAsideStatus, aside.HolderHostnameCheck)
	}
	if aside.HolderBuildMatches == nil {
		return fmt.Errorf("%w: holder_build_matches is required", ErrApplyLockAsideStatus)
	}
	if aside.HolderBuildCheck != migrationApplyLockHolderBuildCheck {
		return fmt.Errorf("%w: unexpected holder_build_check %q", ErrApplyLockAsideStatus, aside.HolderBuildCheck)
	}
	if aside.LockAgeSeconds == nil || *aside.LockAgeSeconds < 0 {
		return fmt.Errorf("%w: lock_age_seconds must be present and non-negative", ErrApplyLockAsideStatus)
	}
	if aside.LockAgeCheck != migrationApplyLockAgeCheck {
		return fmt.Errorf("%w: unexpected lock_age_check %q", ErrApplyLockAsideStatus, aside.LockAgeCheck)
	}
	if aside.ManualClearCandidate == nil || !*aside.ManualClearCandidate {
		return fmt.Errorf("%w: manual_clear_candidate must be true", ErrApplyLockAsideStatus)
	}
	if aside.ManualClearCheck != migrationApplyLockManualClearCheck {
		return fmt.Errorf("%w: unexpected manual_clear_check %q", ErrApplyLockAsideStatus, aside.ManualClearCheck)
	}
	aside.LockFile = lockFile
	aside.AsidePath = asidePath
	aside.RenamedAt = renamedAt
	aside.Lock = &normalized
	return nil
}

func pathExists(path string) (bool, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return false, fmt.Errorf("%w: path is required", ErrApplyLockAsideStatus)
	}
	_, err := os.Lstat(trimmed)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("%w: stat path: %v", ErrApplyLockAsideStatus, err)
}

func regularNonSymlinkFileExists(path string) (bool, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return false, fmt.Errorf("%w: path is required", ErrApplyLockAsideStatus)
	}
	info, err := os.Lstat(trimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("%w: stat path: %v", ErrApplyLockAsideStatus, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, nil
	}
	return info.Mode().IsRegular(), nil
}

func printApplyLockAsideStatusUsage(w io.Writer) {
	fmt.Fprintln(w, "apply-lock-aside-status usage:")
	fmt.Fprintln(w, "  metin2-migrate apply-lock-aside-status --aside <path> [--require-aside-path-exists] [--require-lock-file-absent]")
}
