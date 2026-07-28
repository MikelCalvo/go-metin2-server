package config

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadServiceUsesDefaultsWhenEnvIsMissing(t *testing.T) {
	t.Setenv("METIN2_PPROF_ADDR", "")
	t.Setenv("METIN2_GAMED_PPROF_ADDR", "")
	t.Setenv("METIN2_LEGACY_ADDR", "")
	t.Setenv("METIN2_GAMED_LEGACY_ADDR", "")
	t.Setenv("METIN2_PUBLIC_ADDR", "")
	t.Setenv("METIN2_GAMED_PUBLIC_ADDR", "")

	cfg := LoadService("gamed", "127.0.0.1:6060", ":13000", "127.0.0.1")
	if cfg.PprofAddr != "127.0.0.1:6060" {
		t.Fatalf("expected loopback default pprof addr, got %q", cfg.PprofAddr)
	}
	if cfg.LegacyAddr != ":13000" {
		t.Fatalf("expected default legacy addr, got %q", cfg.LegacyAddr)
	}
	if cfg.PublicAddr != "127.0.0.1" {
		t.Fatalf("expected default public addr, got %q", cfg.PublicAddr)
	}
}

func TestServiceDefaultOpsAddrIsLoopback(t *testing.T) {
	cfg := Service{PprofAddr: "127.0.0.1:6060"}

	if err := ValidateOpsConfig(cfg); err != nil {
		t.Fatalf("expected loopback ops addr to validate, got %v", err)
	}
}

func TestValidateOpsConfigRejectsWildcardPprofAddr(t *testing.T) {
	for _, addr := range []string{":6060", "0.0.0.0:6060", "[::]:6060"} {
		t.Run(addr, func(t *testing.T) {
			err := ValidateOpsConfig(Service{PprofAddr: addr})
			if !errors.Is(err, ErrOpsAddrNotLoopback) {
				t.Fatalf("expected ErrOpsAddrNotLoopback for %q, got %v", addr, err)
			}
		})
	}
}

func TestValidateOpsConfigAcceptsLocalhostLiteral(t *testing.T) {
	if err := ValidateOpsConfig(Service{PprofAddr: "localhost:6060"}); err != nil {
		t.Fatalf("expected localhost ops addr to validate, got %v", err)
	}
}

func TestValidateOpsConfigRejectsNonLoopbackPprofAddr(t *testing.T) {
	for _, addr := range []string{"192.0.2.10:6060", "example.com:6060"} {
		t.Run(addr, func(t *testing.T) {
			err := ValidateOpsConfig(Service{PprofAddr: addr})
			if !errors.Is(err, ErrOpsAddrNotLoopback) {
				t.Fatalf("expected ErrOpsAddrNotLoopback for %q, got %v", addr, err)
			}
		})
	}
}

func TestValidateOpsConfigRejectsMissingOrMalformedPprofAddr(t *testing.T) {
	for _, tc := range []struct {
		addr    string
		wantErr error
	}{
		{addr: "   ", wantErr: ErrOpsAddrRequired},
		{addr: "127.0.0.1", wantErr: ErrOpsAddrInvalid},
		{addr: "127.0.0.1:notaport", wantErr: ErrOpsAddrInvalid},
		{addr: "127.0.0.1:0", wantErr: ErrOpsAddrInvalid},
	} {
		t.Run(tc.addr, func(t *testing.T) {
			err := ValidateOpsConfig(Service{PprofAddr: tc.addr})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v for %q, got %v", tc.wantErr, tc.addr, err)
			}
		})
	}
}

func TestValidateOpsConfigAcceptsReservedLoopbackAddr(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve loopback addr: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close reserved listener: %v", err)
	}

	if err := ValidateOpsConfig(Service{PprofAddr: addr}); err != nil {
		t.Fatalf("expected reserved loopback addr %q to validate, got %v", addr, err)
	}
}

func TestLoadServiceUsesBootstrapPersistenceDefaultsWhenEnvIsMissing(t *testing.T) {
	clearPersistenceEnv(t)

	cfg := LoadService("gamed", ":6060", ":13000", "127.0.0.1")
	if cfg.LoginTicketStoreDir != defaultLoginTicketStoreDir() {
		t.Fatalf("expected default login ticket store dir, got %q", cfg.LoginTicketStoreDir)
	}
	if cfg.AccountStoreDir != defaultAccountStoreDir() {
		t.Fatalf("expected default account store dir, got %q", cfg.AccountStoreDir)
	}
	if cfg.StaticActorStorePath != defaultStaticActorStorePath() {
		t.Fatalf("expected default static actor store path, got %q", cfg.StaticActorStorePath)
	}
	if cfg.InteractionStorePath != defaultInteractionStorePath() {
		t.Fatalf("expected default interaction store path, got %q", cfg.InteractionStorePath)
	}
	if cfg.ItemTemplateStorePath != defaultItemTemplateStorePath() {
		t.Fatalf("expected default item template store path, got %q", cfg.ItemTemplateStorePath)
	}
}

func TestLoadServiceUsesGlobalBootstrapPersistenceOverrides(t *testing.T) {
	clearPersistenceEnv(t)
	t.Setenv("METIN2_LOGIN_TICKET_STORE_DIR", " /global/tickets ")
	t.Setenv("METIN2_ACCOUNT_STORE_DIR", "/global/accounts")
	t.Setenv("METIN2_STATIC_ACTOR_STORE_PATH", "/global/static-actors.json")
	t.Setenv("METIN2_INTERACTION_STORE_PATH", "/global/interactions.json")
	t.Setenv("METIN2_ITEM_TEMPLATE_STORE_PATH", "/global/item-templates.json")

	cfg := LoadService("gamed", ":6060", ":13000", "127.0.0.1")
	if cfg.LoginTicketStoreDir != "/global/tickets" {
		t.Fatalf("expected global login ticket store dir, got %q", cfg.LoginTicketStoreDir)
	}
	if cfg.AccountStoreDir != "/global/accounts" {
		t.Fatalf("expected global account store dir, got %q", cfg.AccountStoreDir)
	}
	if cfg.StaticActorStorePath != "/global/static-actors.json" {
		t.Fatalf("expected global static actor store path, got %q", cfg.StaticActorStorePath)
	}
	if cfg.InteractionStorePath != "/global/interactions.json" {
		t.Fatalf("expected global interaction store path, got %q", cfg.InteractionStorePath)
	}
	if cfg.ItemTemplateStorePath != "/global/item-templates.json" {
		t.Fatalf("expected global item template store path, got %q", cfg.ItemTemplateStorePath)
	}
}

func TestLoadServiceFallsBackForWhitespaceBootstrapPersistenceOverrides(t *testing.T) {
	clearPersistenceEnv(t)
	t.Setenv("METIN2_LOGIN_TICKET_STORE_DIR", "   ")
	t.Setenv("METIN2_ACCOUNT_STORE_DIR", "/global/accounts")
	t.Setenv("METIN2_GAMED_ACCOUNT_STORE_DIR", "   ")

	cfg := LoadService("gamed", ":6060", ":13000", "127.0.0.1")
	if cfg.LoginTicketStoreDir != defaultLoginTicketStoreDir() {
		t.Fatalf("expected whitespace login ticket store override to fall back, got %q", cfg.LoginTicketStoreDir)
	}
	if cfg.AccountStoreDir != "/global/accounts" {
		t.Fatalf("expected whitespace service account store override to fall back to global override, got %q", cfg.AccountStoreDir)
	}
}

func TestLoadServicePrefersServiceSpecificBootstrapPersistenceOverrides(t *testing.T) {
	clearPersistenceEnv(t)
	t.Setenv("METIN2_LOGIN_TICKET_STORE_DIR", "/global/tickets")
	t.Setenv("METIN2_ACCOUNT_STORE_DIR", "/global/accounts")
	t.Setenv("METIN2_STATIC_ACTOR_STORE_PATH", "/global/static-actors.json")
	t.Setenv("METIN2_INTERACTION_STORE_PATH", "/global/interactions.json")
	t.Setenv("METIN2_ITEM_TEMPLATE_STORE_PATH", "/global/item-templates.json")
	t.Setenv("METIN2_GAMED_LOGIN_TICKET_STORE_DIR", "/service/tickets")
	t.Setenv("METIN2_GAMED_ACCOUNT_STORE_DIR", "/service/accounts")
	t.Setenv("METIN2_GAMED_STATIC_ACTOR_STORE_PATH", "/service/static-actors.json")
	t.Setenv("METIN2_GAMED_INTERACTION_STORE_PATH", "/service/interactions.json")
	t.Setenv("METIN2_GAMED_ITEM_TEMPLATE_STORE_PATH", "/service/item-templates.json")

	cfg := LoadService("gamed", ":6060", ":13000", "127.0.0.1")
	if cfg.LoginTicketStoreDir != "/service/tickets" {
		t.Fatalf("expected service-specific login ticket store dir, got %q", cfg.LoginTicketStoreDir)
	}
	if cfg.AccountStoreDir != "/service/accounts" {
		t.Fatalf("expected service-specific account store dir, got %q", cfg.AccountStoreDir)
	}
	if cfg.StaticActorStorePath != "/service/static-actors.json" {
		t.Fatalf("expected service-specific static actor store path, got %q", cfg.StaticActorStorePath)
	}
	if cfg.InteractionStorePath != "/service/interactions.json" {
		t.Fatalf("expected service-specific interaction store path, got %q", cfg.InteractionStorePath)
	}
	if cfg.ItemTemplateStorePath != "/service/item-templates.json" {
		t.Fatalf("expected service-specific item template store path, got %q", cfg.ItemTemplateStorePath)
	}
}

func TestValidatePersistenceConfigAcceptsDistinctExplicitPaths(t *testing.T) {
	root := t.TempDir()
	cfg := Service{
		Name:                  "gamed",
		LoginTicketStoreDir:   filepath.Join(root, "tickets"),
		AccountStoreDir:       filepath.Join(root, "accounts"),
		StaticActorStorePath:  filepath.Join(root, "content", "static-actors.json"),
		InteractionStorePath:  filepath.Join(root, "content", "interactions.json"),
		ItemTemplateStorePath: filepath.Join(root, "content", "item-templates.json"),
	}

	if err := ValidatePersistenceConfig(cfg); err != nil {
		t.Fatalf("expected distinct persistence paths to validate, got %v", err)
	}
}

func TestValidatePersistenceConfigRejectsMissingCriticalStorePath(t *testing.T) {
	root := t.TempDir()
	cfg := Service{
		Name:                  "gamed",
		LoginTicketStoreDir:   filepath.Join(root, "tickets"),
		AccountStoreDir:       "   ",
		StaticActorStorePath:  filepath.Join(root, "static-actors.json"),
		InteractionStorePath:  filepath.Join(root, "interactions.json"),
		ItemTemplateStorePath: filepath.Join(root, "item-templates.json"),
	}

	err := ValidatePersistenceConfig(cfg)
	if !errors.Is(err, ErrPersistencePathRequired) {
		t.Fatalf("expected ErrPersistencePathRequired, got %v", err)
	}
}

func TestValidatePersistenceConfigRejectsDirectoryStoresThatOverlap(t *testing.T) {
	root := t.TempDir()
	cfg := Service{
		Name:                  "gamed",
		LoginTicketStoreDir:   filepath.Join(root, "state"),
		AccountStoreDir:       filepath.Join(root, "state"),
		StaticActorStorePath:  filepath.Join(root, "static-actors.json"),
		InteractionStorePath:  filepath.Join(root, "interactions.json"),
		ItemTemplateStorePath: filepath.Join(root, "item-templates.json"),
	}

	err := ValidatePersistenceConfig(cfg)
	if !errors.Is(err, ErrPersistencePathOverlap) {
		t.Fatalf("expected ErrPersistencePathOverlap for same account/ticket dirs, got %v", err)
	}
}

func TestValidateHandoffPersistenceConfigRejectsDirectoryStoresThatOverlap(t *testing.T) {
	root := t.TempDir()
	cfg := Service{
		Name:                "authd",
		LoginTicketStoreDir: filepath.Join(root, "state"),
		AccountStoreDir:     filepath.Join(root, "state"),
	}

	err := ValidateHandoffPersistenceConfig(cfg)
	if !errors.Is(err, ErrPersistencePathOverlap) {
		t.Fatalf("expected ErrPersistencePathOverlap for same account/ticket dirs, got %v", err)
	}
}

func TestValidatePersistenceConfigRejectsFileStoreInsideDirectoryStore(t *testing.T) {
	root := t.TempDir()
	cfg := Service{
		Name:                  "gamed",
		LoginTicketStoreDir:   filepath.Join(root, "tickets"),
		AccountStoreDir:       filepath.Join(root, "accounts"),
		StaticActorStorePath:  filepath.Join(root, "accounts", "static-actors.json"),
		InteractionStorePath:  filepath.Join(root, "interactions.json"),
		ItemTemplateStorePath: filepath.Join(root, "item-templates.json"),
	}

	err := ValidatePersistenceConfig(cfg)
	if !errors.Is(err, ErrPersistencePathOverlap) {
		t.Fatalf("expected ErrPersistencePathOverlap for file store inside account dir, got %v", err)
	}
}

func TestValidatePersistenceConfigRejectsFileStoresThatSharePath(t *testing.T) {
	root := t.TempDir()
	sharedPath := filepath.Join(root, "content.json")
	cfg := Service{
		Name:                  "gamed",
		LoginTicketStoreDir:   filepath.Join(root, "tickets"),
		AccountStoreDir:       filepath.Join(root, "accounts"),
		StaticActorStorePath:  sharedPath,
		InteractionStorePath:  sharedPath,
		ItemTemplateStorePath: filepath.Join(root, "item-templates.json"),
	}

	err := ValidatePersistenceConfig(cfg)
	if !errors.Is(err, ErrPersistencePathOverlap) {
		t.Fatalf("expected ErrPersistencePathOverlap for shared file paths, got %v", err)
	}
}

func TestValidatePersistenceConfigRejectsFileStorePathAtDirectoryStore(t *testing.T) {
	root := t.TempDir()
	dirPath := filepath.Join(root, "accounts")
	cfg := Service{
		Name:                  "gamed",
		LoginTicketStoreDir:   filepath.Join(root, "tickets"),
		AccountStoreDir:       dirPath,
		StaticActorStorePath:  dirPath,
		InteractionStorePath:  filepath.Join(root, "interactions.json"),
		ItemTemplateStorePath: filepath.Join(root, "item-templates.json"),
	}

	err := ValidatePersistenceConfig(cfg)
	if !errors.Is(err, ErrPersistencePathOverlap) {
		t.Fatalf("expected ErrPersistencePathOverlap for file path equal to account dir, got %v", err)
	}
}

func TestValidatePersistenceConfigRejectsSymlinkResolvedFileStoreInsideDirectoryStore(t *testing.T) {
	root := t.TempDir()
	accountsDir := filepath.Join(root, "accounts")
	linkPath := filepath.Join(root, "static-actors-link.json")
	if err := os.MkdirAll(accountsDir, 0o755); err != nil {
		t.Fatalf("create accounts dir: %v", err)
	}
	if err := os.Symlink(filepath.Join(accountsDir, "static-actors.json"), linkPath); err != nil {
		t.Fatalf("create static actor symlink: %v", err)
	}
	cfg := Service{
		Name:                  "gamed",
		LoginTicketStoreDir:   filepath.Join(root, "tickets"),
		AccountStoreDir:       accountsDir,
		StaticActorStorePath:  linkPath,
		InteractionStorePath:  filepath.Join(root, "interactions.json"),
		ItemTemplateStorePath: filepath.Join(root, "item-templates.json"),
	}

	err := ValidatePersistenceConfig(cfg)
	if !errors.Is(err, ErrPersistencePathOverlap) {
		t.Fatalf("expected ErrPersistencePathOverlap for symlinked file store inside account dir, got %v", err)
	}
}

func clearPersistenceEnv(t *testing.T) {
	t.Helper()
	for _, suffix := range []string{
		"LOGIN_TICKET_STORE_DIR",
		"ACCOUNT_STORE_DIR",
		"STATIC_ACTOR_STORE_PATH",
		"INTERACTION_STORE_PATH",
		"ITEM_TEMPLATE_STORE_PATH",
	} {
		t.Setenv("METIN2_"+suffix, "")
		t.Setenv("METIN2_GAMED_"+suffix, "")
		t.Setenv("METIN2_AUTHD_"+suffix, "")
	}
}

func TestLoadServiceUsesGlobalOverrides(t *testing.T) {
	t.Setenv("METIN2_PPROF_ADDR", ":9999")
	t.Setenv("METIN2_GAMED_PPROF_ADDR", "")
	t.Setenv("METIN2_LEGACY_ADDR", ":13001")
	t.Setenv("METIN2_GAMED_LEGACY_ADDR", "")
	t.Setenv("METIN2_PUBLIC_ADDR", "192.168.1.101")
	t.Setenv("METIN2_GAMED_PUBLIC_ADDR", "")

	cfg := LoadService("gamed", ":6060", ":13000", "127.0.0.1")
	if cfg.PprofAddr != ":9999" {
		t.Fatalf("expected global pprof addr, got %q", cfg.PprofAddr)
	}
	if cfg.LegacyAddr != ":13001" {
		t.Fatalf("expected global legacy addr, got %q", cfg.LegacyAddr)
	}
	if cfg.PublicAddr != "192.168.1.101" {
		t.Fatalf("expected global public addr, got %q", cfg.PublicAddr)
	}
}

func TestLoadServiceResolvesEachAddressFamilyIndependently(t *testing.T) {
	t.Setenv("METIN2_PPROF_ADDR", "")
	t.Setenv("METIN2_GAMED_PPROF_ADDR", ":6067")
	t.Setenv("METIN2_LEGACY_ADDR", ":13001")
	t.Setenv("METIN2_GAMED_LEGACY_ADDR", "")
	t.Setenv("METIN2_PUBLIC_ADDR", "")
	t.Setenv("METIN2_GAMED_PUBLIC_ADDR", "10.22.2.125")

	cfg := LoadService("gamed", ":6060", ":13000", "127.0.0.1")
	if cfg.PprofAddr != ":6067" {
		t.Fatalf("expected service-specific pprof addr, got %q", cfg.PprofAddr)
	}
	if cfg.LegacyAddr != ":13001" {
		t.Fatalf("expected global legacy addr, got %q", cfg.LegacyAddr)
	}
	if cfg.PublicAddr != "10.22.2.125" {
		t.Fatalf("expected service-specific public addr, got %q", cfg.PublicAddr)
	}
}

func TestLoadServicePrefersServiceSpecificOverrides(t *testing.T) {
	t.Setenv("METIN2_PPROF_ADDR", ":9999")
	t.Setenv("METIN2_GAMED_PPROF_ADDR", ":6067")
	t.Setenv("METIN2_LEGACY_ADDR", ":13001")
	t.Setenv("METIN2_GAMED_LEGACY_ADDR", ":13077")
	t.Setenv("METIN2_PUBLIC_ADDR", "192.168.1.101")
	t.Setenv("METIN2_GAMED_PUBLIC_ADDR", "10.22.2.125")

	cfg := LoadService("gamed", ":6060", ":13000", "127.0.0.1")
	if cfg.PprofAddr != ":6067" {
		t.Fatalf("expected service-specific pprof addr, got %q", cfg.PprofAddr)
	}
	if cfg.LegacyAddr != ":13077" {
		t.Fatalf("expected service-specific legacy addr, got %q", cfg.LegacyAddr)
	}
	if cfg.PublicAddr != "10.22.2.125" {
		t.Fatalf("expected service-specific public addr, got %q", cfg.PublicAddr)
	}
}

func TestLoadServiceUsesVisibilityDefaultsWhenEnvIsMissing(t *testing.T) {
	t.Setenv("METIN2_VISIBILITY_MODE", "")
	t.Setenv("METIN2_GAMED_VISIBILITY_MODE", "")
	t.Setenv("METIN2_VISIBILITY_RADIUS", "")
	t.Setenv("METIN2_GAMED_VISIBILITY_RADIUS", "")
	t.Setenv("METIN2_VISIBILITY_SECTOR_SIZE", "")
	t.Setenv("METIN2_GAMED_VISIBILITY_SECTOR_SIZE", "")

	cfg := LoadService("gamed", ":6060", ":13000", "127.0.0.1")
	if cfg.VisibilityMode != "whole_map" {
		t.Fatalf("expected default visibility mode whole_map, got %q", cfg.VisibilityMode)
	}
	if cfg.VisibilityRadius != 0 {
		t.Fatalf("expected default visibility radius 0, got %d", cfg.VisibilityRadius)
	}
	if cfg.VisibilitySectorSize != 0 {
		t.Fatalf("expected default visibility sector size 0, got %d", cfg.VisibilitySectorSize)
	}
}

func TestLoadServiceUsesGlobalVisibilityOverrides(t *testing.T) {
	t.Setenv("METIN2_VISIBILITY_MODE", " radius ")
	t.Setenv("METIN2_GAMED_VISIBILITY_MODE", "")
	t.Setenv("METIN2_VISIBILITY_RADIUS", "600")
	t.Setenv("METIN2_GAMED_VISIBILITY_RADIUS", "")
	t.Setenv("METIN2_VISIBILITY_SECTOR_SIZE", "300")
	t.Setenv("METIN2_GAMED_VISIBILITY_SECTOR_SIZE", "")

	cfg := LoadService("gamed", ":6060", ":13000", "127.0.0.1")
	if cfg.VisibilityMode != "radius" {
		t.Fatalf("expected global visibility mode radius, got %q", cfg.VisibilityMode)
	}
	if cfg.VisibilityRadius != 600 {
		t.Fatalf("expected global visibility radius 600, got %d", cfg.VisibilityRadius)
	}
	if cfg.VisibilitySectorSize != 300 {
		t.Fatalf("expected global visibility sector size 300, got %d", cfg.VisibilitySectorSize)
	}
}

func TestLoadServicePrefersServiceSpecificVisibilityOverrides(t *testing.T) {
	t.Setenv("METIN2_VISIBILITY_MODE", "whole_map")
	t.Setenv("METIN2_GAMED_VISIBILITY_MODE", "radius")
	t.Setenv("METIN2_VISIBILITY_RADIUS", "600")
	t.Setenv("METIN2_GAMED_VISIBILITY_RADIUS", "450")
	t.Setenv("METIN2_VISIBILITY_SECTOR_SIZE", "300")
	t.Setenv("METIN2_GAMED_VISIBILITY_SECTOR_SIZE", "225")

	cfg := LoadService("gamed", ":6060", ":13000", "127.0.0.1")
	if cfg.VisibilityMode != "radius" {
		t.Fatalf("expected service-specific visibility mode radius, got %q", cfg.VisibilityMode)
	}
	if cfg.VisibilityRadius != 450 {
		t.Fatalf("expected service-specific visibility radius 450, got %d", cfg.VisibilityRadius)
	}
	if cfg.VisibilitySectorSize != 225 {
		t.Fatalf("expected service-specific visibility sector size 225, got %d", cfg.VisibilitySectorSize)
	}
}
