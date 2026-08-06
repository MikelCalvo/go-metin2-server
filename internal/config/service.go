package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Service struct {
	Name                  string
	PprofAddr             string
	LegacyAddr            string
	PublicAddr            string
	VisibilityMode        string
	VisibilityRadius      int32
	VisibilitySectorSize  int32
	LoginTicketStoreDir   string
	AccountStoreDir       string
	StaticActorStorePath  string
	InteractionStorePath  string
	ItemTemplateStorePath string
}

var (
	ErrPersistencePathRequired     = errors.New("persistence path is required")
	ErrPersistencePathOverlap      = errors.New("persistence paths overlap")
	ErrPersistencePathRoleConflict = errors.New("persistence path role conflicts with existing filesystem entry")
	ErrOpsAddrRequired             = errors.New("ops bind address is required")
	ErrOpsAddrInvalid              = errors.New("ops bind address is invalid")
	ErrOpsAddrNotLoopback          = errors.New("ops bind address must be loopback")
)

type persistencePathRole string

const (
	persistencePathRoleDir  persistencePathRole = "dir"
	persistencePathRoleFile persistencePathRole = "file"
)

type persistencePathSelection struct {
	Name string
	Role persistencePathRole
	Path string
}

// ValidateOpsConfig fails closed when the local operations/pprof listener is
// missing, malformed, or configured for a non-loopback bind address. Local-only
// operator endpoints share the same mux as pprof, so a wildcard pprof bind
// would also expose those sensitive recovery surfaces.
func ValidateOpsConfig(cfg Service) error {
	addr := strings.TrimSpace(cfg.PprofAddr)
	if addr == "" {
		return ErrOpsAddrRequired
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("%w: %q", ErrOpsAddrInvalid, cfg.PprofAddr)
	}
	parsedPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil || parsedPort == 0 {
		return fmt.Errorf("%w: %q", ErrOpsAddrInvalid, cfg.PprofAddr)
	}
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("%w: %q", ErrOpsAddrNotLoopback, cfg.PprofAddr)
	}
	return nil
}

// ValidatePersistenceConfig fails closed when bootstrap JSON stores are missing
// or configured to share the same filesystem boundary. Directory-backed stores
// own their full subtree, while file-backed stores own only their exact file
// path; any overlap is rejected before runtime code can validate, back up,
// restore, or mutate the wrong store.
func ValidatePersistenceConfig(cfg Service) error {
	return validatePersistencePathSelections([]persistencePathSelection{
		{Name: "login_ticket_store_dir", Role: persistencePathRoleDir, Path: cfg.LoginTicketStoreDir},
		{Name: "account_store_dir", Role: persistencePathRoleDir, Path: cfg.AccountStoreDir},
		{Name: "static_actor_store_path", Role: persistencePathRoleFile, Path: cfg.StaticActorStorePath},
		{Name: "interaction_store_path", Role: persistencePathRoleFile, Path: cfg.InteractionStorePath},
		{Name: "item_template_store_path", Role: persistencePathRoleFile, Path: cfg.ItemTemplateStorePath},
	})
}

// ValidateHandoffPersistenceConfig applies the same fail-closed filesystem
// overlap checks to the auth/login-ticket handoff stores used by authd.
func ValidateHandoffPersistenceConfig(cfg Service) error {
	return validatePersistencePathSelections([]persistencePathSelection{
		{Name: "login_ticket_store_dir", Role: persistencePathRoleDir, Path: cfg.LoginTicketStoreDir},
		{Name: "account_store_dir", Role: persistencePathRoleDir, Path: cfg.AccountStoreDir},
	})
}

func validatePersistencePathSelections(paths []persistencePathSelection) error {
	for i := range paths {
		canonical, err := canonicalPersistencePath(paths[i])
		if err != nil {
			return err
		}
		paths[i].Path = canonical
	}
	for i := range paths {
		for j := i + 1; j < len(paths); j++ {
			if persistencePathsOverlap(paths[i], paths[j]) {
				return fmt.Errorf("%w: %s %q overlaps %s %q", ErrPersistencePathOverlap, paths[i].Name, paths[i].Path, paths[j].Name, paths[j].Path)
			}
		}
	}
	return nil
}

func canonicalPersistencePath(selection persistencePathSelection) (string, error) {
	trimmed := strings.TrimSpace(selection.Path)
	if trimmed == "" {
		return "", fmt.Errorf("%w: %s", ErrPersistencePathRequired, selection.Name)
	}
	abs, err := filepath.Abs(filepath.Clean(trimmed))
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", selection.Name, err)
	}
	resolved, err := resolvePersistencePath(abs)
	if err != nil {
		return "", fmt.Errorf("resolve %s symlinks: %w", selection.Name, err)
	}
	if err := validatePersistencePathRole(selection, resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

func validatePersistencePathRole(selection persistencePathSelection, resolved string) error {
	info, err := os.Stat(resolved)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", selection.Name, err)
	}
	switch selection.Role {
	case persistencePathRoleDir:
		if !info.IsDir() {
			return fmt.Errorf("%w: %s %q is not a directory", ErrPersistencePathRoleConflict, selection.Name, resolved)
		}
	case persistencePathRoleFile:
		if info.IsDir() {
			return fmt.Errorf("%w: %s %q is a directory", ErrPersistencePathRoleConflict, selection.Name, resolved)
		}
	}
	return nil
}

func resolvePersistencePath(path string) (string, error) {
	return resolvePersistencePathAt(path, 0)
}

func resolvePersistencePathAt(path string, depth int) (string, error) {
	if depth > 255 {
		return "", errors.New("too many symlinks while resolving persistence path")
	}
	path, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", err
	}
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
		return resolvePersistencePathAt(target, depth+1)
	}
	if lstatErr != nil && !errors.Is(lstatErr, os.ErrNotExist) {
		return "", lstatErr
	}
	parent := filepath.Dir(path)
	if parent == path {
		return path, nil
	}
	parentResolved, err := resolvePersistencePathAt(parent, depth+1)
	if err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Clean(filepath.Join(parentResolved, filepath.Base(path))))
}

func persistencePathsOverlap(a, b persistencePathSelection) bool {
	if a.Path == b.Path {
		return true
	}
	if a.Role == persistencePathRoleDir && pathInsideDir(a.Path, b.Path) {
		return true
	}
	if b.Role == persistencePathRoleDir && pathInsideDir(b.Path, a.Path) {
		return true
	}
	return false
}

func pathInsideDir(root string, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func LoadService(name string, defaultPprofAddr string, defaultLegacyAddr string, defaultPublicAddr string) Service {
	upperName := strings.ToUpper(name)

	return Service{
		Name:                  name,
		PprofAddr:             loadOverride(upperName, "PPROF_ADDR", defaultPprofAddr),
		LegacyAddr:            loadOverride(upperName, "LEGACY_ADDR", defaultLegacyAddr),
		PublicAddr:            loadOverride(upperName, "PUBLIC_ADDR", defaultPublicAddr),
		VisibilityMode:        loadVisibilityModeOverride(upperName, "whole_map"),
		VisibilityRadius:      loadInt32Override(upperName, "VISIBILITY_RADIUS", 0),
		VisibilitySectorSize:  loadInt32Override(upperName, "VISIBILITY_SECTOR_SIZE", 0),
		LoginTicketStoreDir:   loadPathOverride(upperName, "LOGIN_TICKET_STORE_DIR", defaultLoginTicketStoreDir()),
		AccountStoreDir:       loadPathOverride(upperName, "ACCOUNT_STORE_DIR", defaultAccountStoreDir()),
		StaticActorStorePath:  loadPathOverride(upperName, "STATIC_ACTOR_STORE_PATH", defaultStaticActorStorePath()),
		InteractionStorePath:  loadPathOverride(upperName, "INTERACTION_STORE_PATH", defaultInteractionStorePath()),
		ItemTemplateStorePath: loadPathOverride(upperName, "ITEM_TEMPLATE_STORE_PATH", defaultItemTemplateStorePath()),
	}
}

func defaultLoginTicketStoreDir() string {
	return filepath.Join(os.TempDir(), "go-metin2-server-login-tickets")
}

func DefaultLoginTicketStoreDir() string {
	return defaultLoginTicketStoreDir()
}

func defaultAccountStoreDir() string {
	return filepath.Join(os.TempDir(), "go-metin2-server-accounts")
}

func DefaultAccountStoreDir() string {
	return defaultAccountStoreDir()
}

func defaultStaticActorStorePath() string {
	return filepath.Join(os.TempDir(), "go-metin2-server-static-actors.json")
}

func DefaultStaticActorStorePath() string {
	return defaultStaticActorStorePath()
}

func defaultInteractionStorePath() string {
	return filepath.Join(os.TempDir(), "go-metin2-server-interaction-definitions.json")
}

func DefaultInteractionStorePath() string {
	return defaultInteractionStorePath()
}

func defaultItemTemplateStorePath() string {
	return filepath.Join(os.TempDir(), "go-metin2-server-item-templates.json")
}

func DefaultItemTemplateStorePath() string {
	return defaultItemTemplateStorePath()
}

func loadOverride(upperName string, suffix string, fallback string) string {
	if value, ok := loadRawOverride(upperName, suffix); ok {
		return value
	}
	return fallback
}

func loadVisibilityModeOverride(upperName string, fallback string) string {
	value := loadOverride(upperName, "VISIBILITY_MODE", fallback)
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "-", "_")
	if value == "" {
		return fallback
	}
	return value
}

func loadPathOverride(upperName string, suffix string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv("METIN2_" + upperName + "_" + suffix)); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("METIN2_" + suffix)); value != "" {
		return value
	}
	return fallback
}

func loadInt32Override(upperName string, suffix string, fallback int32) int32 {
	value, ok := loadRawOverride(upperName, suffix)
	if !ok {
		return fallback
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return -1
	}
	return int32(parsed)
}

func loadRawOverride(upperName string, suffix string) (string, bool) {
	if serviceValue := os.Getenv("METIN2_" + upperName + "_" + suffix); serviceValue != "" {
		return serviceValue, true
	}

	if globalValue := os.Getenv("METIN2_" + suffix); globalValue != "" {
		return globalValue, true
	}

	return "", false
}
