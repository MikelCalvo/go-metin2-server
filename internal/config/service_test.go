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
