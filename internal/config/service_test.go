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
