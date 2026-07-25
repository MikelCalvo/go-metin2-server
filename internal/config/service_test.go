package config

import "testing"

func TestLoadServiceUsesDefaultsWhenEnvIsMissing(t *testing.T) {
	t.Setenv("METIN2_PPROF_ADDR", "")
	t.Setenv("METIN2_GAMED_PPROF_ADDR", "")
	t.Setenv("METIN2_LEGACY_ADDR", "")
	t.Setenv("METIN2_GAMED_LEGACY_ADDR", "")
	t.Setenv("METIN2_PUBLIC_ADDR", "")
	t.Setenv("METIN2_GAMED_PUBLIC_ADDR", "")

	cfg := LoadService("gamed", ":6060", ":13000", "127.0.0.1")
	if cfg.PprofAddr != ":6060" {
		t.Fatalf("expected default pprof addr, got %q", cfg.PprofAddr)
	}
	if cfg.LegacyAddr != ":13000" {
		t.Fatalf("expected default legacy addr, got %q", cfg.LegacyAddr)
	}
	if cfg.PublicAddr != "127.0.0.1" {
		t.Fatalf("expected default public addr, got %q", cfg.PublicAddr)
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
