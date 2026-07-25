package config

import (
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
