package ops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	stdpprof "net/http/pprof"
	"net/url"
	"strconv"
	"strings"
	"time"

	contentbundle "github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
	"github.com/MikelCalvo/go-metin2-server/internal/worldruntime"
)

type localRelocationRequest struct {
	Name     string `json:"name"`
	MapIndex uint32 `json:"map_index"`
	X        int32  `json:"x"`
	Y        int32  `json:"y"`
}

type localAccountStoreBackupRequest struct {
	DstDir string `json:"dst_dir"`
}

type localAccountStoreRestoreRequest struct {
	SrcDir string `json:"src_dir"`
}

type localStaticActorRequest struct {
	Name            string `json:"name"`
	MapIndex        uint32 `json:"map_index"`
	X               int32  `json:"x"`
	Y               int32  `json:"y"`
	RaceNum         uint32 `json:"race_num"`
	InteractionKind string `json:"interaction_kind"`
	InteractionRef  string `json:"interaction_ref"`
	CombatProfile   string `json:"combat_profile"`
}

type localStaticActorCombatProfileRequest struct {
	Profile               string                      `json:"profile"`
	MaxHP                 uint8                       `json:"max_hp"`
	DamagePerNormalAttack uint8                       `json:"damage_per_normal_attack"`
	AttackValue           uint16                      `json:"attack_value"`
	DefenseValue          uint16                      `json:"defense_value"`
	Level                 uint16                      `json:"level"`
	Rank                  uint8                       `json:"rank"`
	RespawnDelayMs        int64                       `json:"respawn_delay_ms"`
	DeathReward           localStaticActorDeathReward `json:"death_reward"`
}

type localStaticActorDeathReward struct {
	Experience uint64   `json:"experience"`
	Gold       uint64   `json:"gold"`
	DropVnums  []uint32 `json:"drop_vnums"`
}

type localStaticActorCombatProfileResponse struct {
	Profile               string                              `json:"profile"`
	MaxHP                 uint8                               `json:"max_hp"`
	DamagePerNormalAttack uint8                               `json:"damage_per_normal_attack"`
	AttackValue           uint16                              `json:"attack_value"`
	DefenseValue          uint16                              `json:"defense_value"`
	Level                 uint16                              `json:"level"`
	Rank                  uint8                               `json:"rank"`
	RespawnDelayMs        int64                               `json:"respawn_delay_ms"`
	DeathReward           worldruntime.StaticActorDeathReward `json:"death_reward"`
}

type localStaticActorCombatProfileListResponse struct {
	Profiles []localStaticActorCombatProfileResponse `json:"profiles"`
}

type localInteractionDefinitionRequest struct {
	Kind     string                                  `json:"kind"`
	Ref      string                                  `json:"ref"`
	Text     string                                  `json:"text"`
	Title    string                                  `json:"title"`
	Catalog  []interactionstore.MerchantCatalogEntry `json:"catalog"`
	MapIndex uint32                                  `json:"map_index"`
	X        int32                                   `json:"x"`
	Y        int32                                   `json:"y"`
}

const (
	maxLocalAccountStoreMutationBodyBytes  = 4096
	maxLocalInteractionDefinitionBodyBytes = 4096
)

func NewPprofMux(serviceName string) *http.ServeMux {
	return NewPprofMuxWithLocalRuntimeIntrospection(serviceName, nil, nil, nil, nil, nil, nil, nil)
}

func NewPprofMuxWithLocalNotice(serviceName string, broadcastNotice func(string) int) *http.ServeMux {
	return NewPprofMuxWithLocalRuntimeIntrospection(serviceName, broadcastNotice, nil, nil, nil, nil, nil, nil)
}

func NewPprofMuxWithLocalRelocation(serviceName string, broadcastNotice func(string) int, relocateCharacter func(string, uint32, int32, int32) bool) *http.ServeMux {
	return NewPprofMuxWithLocalRuntimeIntrospection(serviceName, broadcastNotice, relocateCharacter, nil, nil, nil, nil, nil)
}

func NewPprofMuxWithLocalRuntimeSnapshot(serviceName string, broadcastNotice func(string) int, relocateCharacter func(string, uint32, int32, int32) bool, connectedCharacters func() any) *http.ServeMux {
	return NewPprofMuxWithLocalRuntimeIntrospection(serviceName, broadcastNotice, relocateCharacter, nil, nil, connectedCharacters, nil, nil)
}

func RegisterLocalAccountStoreValidateEndpoint(mux *http.ServeMux, validate func() (any, error)) *http.ServeMux {
	if mux == nil || validate == nil {
		return mux
	}

	mux.HandleFunc("/local/account-store/validate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		summary, err := validate()
		if err != nil {
			slog.Warn("local account store validation failed", "err", err)
			w.WriteHeader(http.StatusConflict)
			return
		}
		writeLocalJSONMutationResponse(w, summary, http.StatusOK)
	})
	return mux
}

func RegisterLocalAccountStoreCrashTempCleanupEndpoint(mux *http.ServeMux, cleanup func() (any, error)) *http.ServeMux {
	if mux == nil || cleanup == nil {
		return mux
	}

	mux.HandleFunc("/local/account-store/crash-temps/cleanup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		status, ok := requireEmptyLocalAccountStoreMutationBody(r)
		if !ok {
			w.WriteHeader(status)
			return
		}
		summary, err := cleanup()
		if err != nil {
			slog.Warn("local account store crash temp cleanup failed", "err", err)
			w.WriteHeader(http.StatusConflict)
			return
		}
		writeLocalJSONMutationResponse(w, summary, http.StatusOK)
	})
	return mux
}

func RegisterLocalLoginTicketStoreValidateEndpoint(mux *http.ServeMux, validate func() (any, error)) *http.ServeMux {
	if mux == nil || validate == nil {
		return mux
	}

	mux.HandleFunc("/local/login-tickets/validate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		summary, err := validate()
		if err != nil {
			slog.Warn("local login ticket store validation failed", "err", err)
			w.WriteHeader(http.StatusConflict)
			return
		}
		writeLocalJSONMutationResponse(w, summary, http.StatusOK)
	})
	return mux
}

func RegisterLocalItemTemplateStoreValidateEndpoint(mux *http.ServeMux, validate func() (any, error)) *http.ServeMux {
	if mux == nil || validate == nil {
		return mux
	}

	mux.HandleFunc("/local/item-templates/validate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		summary, err := validate()
		if err != nil {
			slog.Warn("local item template store validation failed", "err", err)
			w.WriteHeader(http.StatusConflict)
			return
		}
		writeLocalJSONMutationResponse(w, summary, http.StatusOK)
	})
	return mux
}

func RegisterLocalItemTemplateStoreCrashTempCleanupEndpoint(mux *http.ServeMux, cleanup func() (any, error)) *http.ServeMux {
	if mux == nil || cleanup == nil {
		return mux
	}

	mux.HandleFunc("/local/item-templates/crash-temps/cleanup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		status, ok := requireEmptyLocalAccountStoreMutationBody(r)
		if !ok {
			w.WriteHeader(status)
			return
		}
		summary, err := cleanup()
		if err != nil {
			slog.Warn("local item template store crash temp cleanup failed", "err", err)
			w.WriteHeader(http.StatusConflict)
			return
		}
		writeLocalJSONMutationResponse(w, summary, http.StatusOK)
	})
	return mux
}

func RegisterLocalAccountStoreBackupEndpoint(mux *http.ServeMux, backup func(string) (any, error)) *http.ServeMux {
	if mux == nil || backup == nil {
		return mux
	}

	mux.HandleFunc("/local/account-store/backup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		request, status, ok := decodeLocalAccountStoreBackupRequest(r)
		if !ok {
			w.WriteHeader(status)
			return
		}
		summary, err := backup(request.DstDir)
		if err != nil {
			slog.Warn("local account store backup failed", "err", err)
			w.WriteHeader(http.StatusConflict)
			return
		}
		writeLocalJSONMutationResponse(w, summary, http.StatusOK)
	})
	return mux
}

func RegisterLocalAccountStoreBackupValidateEndpoint(mux *http.ServeMux, validate func(string) (any, error)) *http.ServeMux {
	if mux == nil || validate == nil {
		return mux
	}

	mux.HandleFunc("/local/account-store/backup/validate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		request, status, ok := decodeLocalAccountStoreRestoreRequest(r)
		if !ok {
			w.WriteHeader(status)
			return
		}
		summary, err := validate(request.SrcDir)
		if err != nil {
			slog.Warn("local account store backup validation failed", "err", err)
			w.WriteHeader(http.StatusConflict)
			return
		}
		writeLocalJSONMutationResponse(w, summary, http.StatusOK)
	})
	return mux
}

func RegisterLocalAccountStoreRestoreEndpoint(mux *http.ServeMux, restore func(string) (any, error)) *http.ServeMux {
	if mux == nil || restore == nil {
		return mux
	}

	mux.HandleFunc("/local/account-store/restore", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		request, status, ok := decodeLocalAccountStoreRestoreRequest(r)
		if !ok {
			w.WriteHeader(status)
			return
		}
		summary, err := restore(request.SrcDir)
		if err != nil {
			slog.Warn("local account store restore failed", "err", err)
			w.WriteHeader(http.StatusConflict)
			return
		}
		writeLocalJSONMutationResponse(w, summary, http.StatusOK)
	})
	return mux
}

func RegisterLocalRuntimeConfigEndpoint(mux *http.ServeMux, runtimeConfig func() any) *http.ServeMux {
	if mux == nil || runtimeConfig == nil {
		return mux
	}

	mux.HandleFunc("/local/runtime-config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(runtimeConfig()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	return mux
}

func RegisterLocalGroundItemsEndpoint(mux *http.ServeMux, groundItems func() any) *http.ServeMux {
	if mux == nil || groundItems == nil {
		return mux
	}

	mux.HandleFunc("/local/ground-items", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(groundItems()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	return mux
}

func RegisterLocalGroundItemEndpoint(mux *http.ServeMux, groundItem func(uint32) (any, bool)) *http.ServeMux {
	if mux == nil || groundItem == nil {
		return mux
	}
	mux.HandleFunc("GET /local/ground-items/", func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		vid, ok := decodeLocalGroundItemVID(r)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		value, ok := groundItem(vid)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeLocalJSONMutationResponse(w, value, http.StatusOK)
	})
	return mux
}

func RegisterLocalInventoryEndpoint(mux *http.ServeMux, inventorySnapshot func(string) (any, bool)) *http.ServeMux {
	return registerLocalNamedSnapshotEndpoint(mux, "GET /local/inventory/", "/local/inventory/", inventorySnapshot)
}

func RegisterLocalEquipmentEndpoint(mux *http.ServeMux, equipmentSnapshot func(string) (any, bool)) *http.ServeMux {
	return registerLocalNamedSnapshotEndpoint(mux, "GET /local/equipment/", "/local/equipment/", equipmentSnapshot)
}

func RegisterLocalCurrencyEndpoint(mux *http.ServeMux, currencySnapshot func(string) (any, bool)) *http.ServeMux {
	return registerLocalNamedSnapshotEndpoint(mux, "GET /local/currency/", "/local/currency/", currencySnapshot)
}

func RegisterLocalQuickslotsEndpoint(mux *http.ServeMux, quickslotsSnapshot func(string) (any, bool)) *http.ServeMux {
	return registerLocalNamedSnapshotEndpoint(mux, "GET /local/quickslots/", "/local/quickslots/", quickslotsSnapshot)
}

func RegisterLocalCombatTargetEndpoint(mux *http.ServeMux, combatTargetSnapshot func(string) (any, bool)) *http.ServeMux {
	return registerLocalNamedSnapshotEndpoint(mux, "GET /local/combat-target/", "/local/combat-target/", combatTargetSnapshot)
}

func RegisterLocalCombatTargetsEndpoint(mux *http.ServeMux, combatTargetSnapshots func() any) *http.ServeMux {
	if mux == nil || combatTargetSnapshots == nil {
		return mux
	}
	mux.HandleFunc("/local/combat-targets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(combatTargetSnapshots()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	return mux
}

func registerLocalNamedSnapshotEndpoint(mux *http.ServeMux, pattern string, prefix string, snapshot func(string) (any, bool)) *http.ServeMux {
	if mux == nil || snapshot == nil {
		return mux
	}
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		name, ok := decodeLocalCharacterName(r, prefix)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		value, ok := snapshot(name)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeLocalJSONMutationResponse(w, value, http.StatusOK)
	})
	return mux
}

func RegisterLocalStaticActorEndpoints(mux *http.ServeMux, staticActors func() any, registerStaticActor func(string, uint32, int32, int32, uint32, string, string, string) (any, bool)) *http.ServeMux {
	if mux == nil || (staticActors == nil && registerStaticActor == nil) {
		return mux
	}

	mux.HandleFunc("/local/static-actors", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if staticActors == nil {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if err := json.NewEncoder(w).Encode(staticActors()); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		case http.MethodPost:
			if registerStaticActor == nil {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			request, ok := decodeLocalStaticActorRequest(r)
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			actor, ok := registerStaticActor(request.Name, request.MapIndex, request.X, request.Y, request.RaceNum, request.InteractionKind, request.InteractionRef, request.CombatProfile)
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if err := json.NewEncoder(w).Encode(actor); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	return mux
}

func RegisterLocalStaticActorDeleteEndpoint(mux *http.ServeMux, removeStaticActor func(uint64) (any, bool)) *http.ServeMux {
	if mux == nil || removeStaticActor == nil {
		return mux
	}

	mux.HandleFunc("DELETE /local/static-actors/", func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		entityID, ok := decodeLocalStaticActorEntityID(r)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		actor, ok := removeStaticActor(entityID)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(actor); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	return mux
}

func RegisterLocalStaticActorUpdateEndpoint(mux *http.ServeMux, updateStaticActor func(uint64, string, uint32, int32, int32, uint32, string, string, string) (any, bool)) *http.ServeMux {
	if mux == nil || updateStaticActor == nil {
		return mux
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		entityID, ok := decodeLocalStaticActorEntityID(r)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		request, ok := decodeLocalStaticActorRequest(r)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		actor, ok := updateStaticActor(entityID, request.Name, request.MapIndex, request.X, request.Y, request.RaceNum, request.InteractionKind, request.InteractionRef, request.CombatProfile)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(actor); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
	mux.HandleFunc("PATCH /local/static-actors/", handler)
	mux.HandleFunc("PUT /local/static-actors/", handler)
	return mux
}

func RegisterLocalStaticActorCombatProfileEndpoint(mux *http.ServeMux) *http.ServeMux {
	if mux == nil {
		return mux
	}
	mux.HandleFunc("/local/static-actor-combat-profiles", func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if err := json.NewEncoder(w).Encode(worldruntime.StaticActorCombatProfileSnapshots()); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		case http.MethodPost:
			profile, defaults, ok := decodeLocalStaticActorCombatProfileRequest(r)
			if !ok || !worldruntime.RegisterStaticActorCombatProfile(profile, defaults) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			registered, ok := worldruntime.BootstrapStaticActorCombatProfileDefaults(profile)
			if !ok {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			writeLocalJSONMutationResponse(w, localStaticActorCombatProfileResponse{
				Profile:               profile,
				MaxHP:                 registered.MaxHP,
				DamagePerNormalAttack: registered.DamagePerNormalAttack,
				AttackValue:           registered.AttackValue,
				DefenseValue:          registered.DefenseValue,
				Level:                 registered.Level,
				Rank:                  registered.Rank,
				RespawnDelayMs:        registered.RespawnDelay.Milliseconds(),
				DeathReward:           registered.DeathReward,
			}, http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	return mux
}

func RegisterLocalInteractionDefinitionEndpoints(mux *http.ServeMux, interactionDefinitions func() any, createInteractionDefinition func(interactionstore.Definition) (any, int)) *http.ServeMux {
	if mux == nil || (interactionDefinitions == nil && createInteractionDefinition == nil) {
		return mux
	}

	mux.HandleFunc("/local/interactions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if interactionDefinitions == nil {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if err := json.NewEncoder(w).Encode(interactionDefinitions()); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		case http.MethodPost:
			if createInteractionDefinition == nil {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			request, decodeStatus, ok := decodeLocalInteractionDefinitionRequest(r)
			if !ok {
				w.WriteHeader(decodeStatus)
				return
			}
			definition, status := createInteractionDefinition(request)
			writeLocalJSONMutationResponse(w, definition, status)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	return mux
}

func RegisterLocalInteractionDefinitionUpdateEndpoint(mux *http.ServeMux, upsertInteractionDefinition func(interactionstore.Definition) (any, int)) *http.ServeMux {
	if mux == nil || upsertInteractionDefinition == nil {
		return mux
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		kind, ref, ok := decodeLocalInteractionDefinitionIdentity(r)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		request, decodeStatus, ok := decodeLocalInteractionDefinitionRequest(r)
		if !ok || request.Kind != kind || request.Ref != ref {
			if ok {
				decodeStatus = http.StatusBadRequest
			}
			w.WriteHeader(decodeStatus)
			return
		}
		definition, status := upsertInteractionDefinition(request)
		writeLocalJSONMutationResponse(w, definition, status)
	}
	mux.HandleFunc("PATCH /local/interactions/", handler)
	mux.HandleFunc("PUT /local/interactions/", handler)
	return mux
}

func RegisterLocalInteractionDefinitionDeleteEndpoint(mux *http.ServeMux, removeInteractionDefinition func(string, string) (any, int)) *http.ServeMux {
	if mux == nil || removeInteractionDefinition == nil {
		return mux
	}

	mux.HandleFunc("DELETE /local/interactions/", func(w http.ResponseWriter, r *http.Request) {
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		kind, ref, ok := decodeLocalInteractionDefinitionIdentity(r)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		definition, status := removeInteractionDefinition(kind, ref)
		writeLocalJSONMutationResponse(w, definition, status)
	})
	return mux
}

func RegisterLocalInteractionVisibilityEndpoint(mux *http.ServeMux, interactionVisibility func() any) *http.ServeMux {
	if mux == nil || interactionVisibility == nil {
		return mux
	}

	mux.HandleFunc("/local/interaction-visibility", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(interactionVisibility()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	})
	return mux
}

func RegisterLocalContentBundleEndpoint(mux *http.ServeMux, exportContentBundle func() (any, int), importContentBundle func(contentbundle.Bundle) (any, int)) *http.ServeMux {
	if mux == nil || (exportContentBundle == nil && importContentBundle == nil) {
		return mux
	}

	mux.HandleFunc("/local/content-bundle", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if exportContentBundle == nil {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			result, status := exportContentBundle()
			if status >= 200 && status < 300 {
				bundle, ok := result.(contentbundle.Bundle)
				if !ok {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				normalized, err := contentbundle.Canonicalize(bundle)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				result = normalized
			}
			writeLocalJSONMutationResponse(w, result, status)
		case http.MethodPost:
			if importContentBundle == nil {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			bundle, status, ok := decodeLocalContentBundleRequest(r)
			if !ok {
				w.WriteHeader(status)
				return
			}
			normalized, err := contentbundle.Canonicalize(bundle)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			result, status := importContentBundle(normalized)
			writeLocalJSONMutationResponse(w, result, status)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	return mux
}

func RegisterLocalContentBundleSummaryEndpoint(mux *http.ServeMux, exportContentBundleSummary func() (any, int)) *http.ServeMux {
	if mux == nil || exportContentBundleSummary == nil {
		return mux
	}

	mux.HandleFunc("/local/content-bundle/summary", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			summary, status := exportContentBundleSummary()
			writeLocalJSONMutationResponse(w, summary, status)
		case http.MethodPost:
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			bundle, status, ok := decodeLocalContentBundleRequest(r)
			if !ok {
				w.WriteHeader(status)
				return
			}
			summary, err := contentbundle.Summarize(bundle)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			writeLocalJSONMutationResponse(w, summary, http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	return mux
}

func RegisterLocalContentBundleValidateEndpoint(mux *http.ServeMux) *http.ServeMux {
	if mux == nil {
		return mux
	}

	mux.HandleFunc("/local/content-bundle/validate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !isLoopbackRemoteAddr(r.RemoteAddr) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		bundle, status, ok := decodeLocalContentBundleRequest(r)
		if !ok {
			w.WriteHeader(status)
			return
		}
		normalized, err := contentbundle.Canonicalize(bundle)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeLocalJSONMutationResponse(w, normalized, http.StatusOK)
	})
	return mux
}

func NewPprofMuxWithLocalRuntimeIntrospection(serviceName string, broadcastNotice func(string) int, relocateCharacter func(string, uint32, int32, int32) bool, previewRelocation func(string, uint32, int32, int32) (any, bool), transferCharacter func(string, uint32, int32, int32) (any, bool), connectedCharacters func() any, characterVisibility func() any, mapOccupancy func() any) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(w, "%s ok\n", serviceName)
	})

	mux.HandleFunc("/debug/pprof/", stdpprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", stdpprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", stdpprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", stdpprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", stdpprof.Trace)
	mux.Handle("/debug/pprof/allocs", stdpprof.Handler("allocs"))
	mux.Handle("/debug/pprof/block", stdpprof.Handler("block"))
	mux.Handle("/debug/pprof/goroutine", stdpprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/heap", stdpprof.Handler("heap"))
	mux.Handle("/debug/pprof/mutex", stdpprof.Handler("mutex"))
	mux.Handle("/debug/pprof/threadcreate", stdpprof.Handler("threadcreate"))

	if broadcastNotice != nil {
		mux.HandleFunc("/local/notice", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			message := strings.TrimSpace(string(body))
			if message == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = fmt.Fprintf(w, "queued %d\n", broadcastNotice(message))
		})
	}

	if relocateCharacter != nil {
		mux.HandleFunc("/local/relocate", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			request, ok := decodeLocalRelocationRequest(r)
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if !relocateCharacter(request.Name, request.MapIndex, request.X, request.Y) {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = io.WriteString(w, "relocated 1\n")
		})
	}

	if previewRelocation != nil {
		mux.HandleFunc("/local/relocate-preview", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			request, ok := decodeLocalRelocationRequest(r)
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			preview, ok := previewRelocation(request.Name, request.MapIndex, request.X, request.Y)
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if err := json.NewEncoder(w).Encode(preview); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		})
	}

	if transferCharacter != nil {
		mux.HandleFunc("/local/transfer", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			request, ok := decodeLocalRelocationRequest(r)
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			result, ok := transferCharacter(request.Name, request.MapIndex, request.X, request.Y)
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if err := json.NewEncoder(w).Encode(result); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		})
	}

	if connectedCharacters != nil {
		mux.HandleFunc("/local/players", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if err := json.NewEncoder(w).Encode(connectedCharacters()); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		})
	}

	if characterVisibility != nil {
		mux.HandleFunc("/local/visibility", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if err := json.NewEncoder(w).Encode(characterVisibility()); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		})
	}

	if mapOccupancy != nil {
		mux.HandleFunc("/local/maps", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if !isLoopbackRemoteAddr(r.RemoteAddr) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			if err := json.NewEncoder(w).Encode(mapOccupancy()); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
		})
	}

	return mux
}

func decodeLocalRelocationRequest(r *http.Request) (localRelocationRequest, bool) {
	var request localRelocationRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return localRelocationRequest{}, false
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return localRelocationRequest{}, false
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" || request.MapIndex == 0 {
		return localRelocationRequest{}, false
	}
	return request, true
}

func decodeLocalStaticActorRequest(r *http.Request) (localStaticActorRequest, bool) {
	var request localStaticActorRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return localStaticActorRequest{}, false
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return localStaticActorRequest{}, false
	}
	request.Name = strings.TrimSpace(request.Name)
	request.InteractionKind = strings.TrimSpace(request.InteractionKind)
	request.InteractionRef = strings.TrimSpace(request.InteractionRef)
	if request.Name == "" || request.MapIndex == 0 || request.RaceNum == 0 {
		return localStaticActorRequest{}, false
	}
	if (request.InteractionKind == "") != (request.InteractionRef == "") {
		return localStaticActorRequest{}, false
	}
	if !worldruntime.ValidStaticActorInteractionMetadata(request.InteractionKind, request.InteractionRef) {
		return localStaticActorRequest{}, false
	}
	return request, true
}

func decodeLocalStaticActorCombatProfileRequest(r *http.Request) (string, worldruntime.StaticActorCombatProfileDefaults, bool) {
	var request localStaticActorCombatProfileRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return "", worldruntime.StaticActorCombatProfileDefaults{}, false
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", worldruntime.StaticActorCombatProfileDefaults{}, false
	}
	profile := request.Profile
	if strings.TrimSpace(profile) == "" || request.RespawnDelayMs <= 0 {
		return "", worldruntime.StaticActorCombatProfileDefaults{}, false
	}
	defaults := worldruntime.StaticActorCombatProfileDefaults{
		MaxHP:                 request.MaxHP,
		DamagePerNormalAttack: request.DamagePerNormalAttack,
		AttackValue:           request.AttackValue,
		DefenseValue:          request.DefenseValue,
		Level:                 request.Level,
		Rank:                  request.Rank,
		RespawnDelay:          time.Duration(request.RespawnDelayMs) * time.Millisecond,
		DeathReward: worldruntime.StaticActorDeathReward{
			Experience: request.DeathReward.Experience,
			Gold:       request.DeathReward.Gold,
			DropVnums:  request.DeathReward.DropVnums,
		},
	}
	return profile, defaults, true
}

func decodeLocalAccountStoreBackupRequest(r *http.Request) (localAccountStoreBackupRequest, int, bool) {
	raw, status, ok := readNonEmptyLocalAccountStoreMutationBody(r)
	if !ok {
		return localAccountStoreBackupRequest{}, status, false
	}
	var request localAccountStoreBackupRequest
	if !decodeStrictLocalAccountStoreMutationRequest(raw, &request) {
		return localAccountStoreBackupRequest{}, http.StatusBadRequest, false
	}
	request.DstDir = strings.TrimSpace(request.DstDir)
	if request.DstDir == "" {
		return localAccountStoreBackupRequest{}, http.StatusBadRequest, false
	}
	return request, http.StatusOK, true
}

func decodeLocalAccountStoreRestoreRequest(r *http.Request) (localAccountStoreRestoreRequest, int, bool) {
	raw, status, ok := readNonEmptyLocalAccountStoreMutationBody(r)
	if !ok {
		return localAccountStoreRestoreRequest{}, status, false
	}
	var request localAccountStoreRestoreRequest
	if !decodeStrictLocalAccountStoreMutationRequest(raw, &request) {
		return localAccountStoreRestoreRequest{}, http.StatusBadRequest, false
	}
	request.SrcDir = strings.TrimSpace(request.SrcDir)
	if request.SrcDir == "" {
		return localAccountStoreRestoreRequest{}, http.StatusBadRequest, false
	}
	return request, http.StatusOK, true
}

func readNonEmptyLocalAccountStoreMutationBody(r *http.Request) ([]byte, int, bool) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxLocalAccountStoreMutationBodyBytes+1))
	if err != nil {
		return nil, http.StatusBadRequest, false
	}
	if len(raw) > maxLocalAccountStoreMutationBodyBytes {
		return nil, http.StatusRequestEntityTooLarge, false
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, http.StatusBadRequest, false
	}
	return raw, http.StatusOK, true
}

func requireEmptyLocalAccountStoreMutationBody(r *http.Request) (int, bool) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxLocalAccountStoreMutationBodyBytes+1))
	if err != nil {
		return http.StatusBadRequest, false
	}
	if len(raw) > maxLocalAccountStoreMutationBodyBytes {
		return http.StatusRequestEntityTooLarge, false
	}
	if len(bytes.TrimSpace(raw)) != 0 {
		return http.StatusBadRequest, false
	}
	return http.StatusOK, true
}

func decodeStrictLocalAccountStoreMutationRequest(raw []byte, request any) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(request); err != nil {
		return false
	}
	var trailing struct{}
	return decoder.Decode(&trailing) == io.EOF
}

func decodeLocalInteractionDefinitionRequest(r *http.Request) (interactionstore.Definition, int, bool) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxLocalInteractionDefinitionBodyBytes+1))
	if err != nil {
		return interactionstore.Definition{}, http.StatusBadRequest, false
	}
	if len(raw) > maxLocalInteractionDefinitionBodyBytes {
		return interactionstore.Definition{}, http.StatusRequestEntityTooLarge, false
	}
	var request localInteractionDefinitionRequest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return interactionstore.Definition{}, http.StatusBadRequest, false
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return interactionstore.Definition{}, http.StatusBadRequest, false
	}
	definition := interactionstore.NormalizeDefinition(interactionstore.Definition{
		Kind:     strings.TrimSpace(request.Kind),
		Ref:      strings.TrimSpace(request.Ref),
		Text:     request.Text,
		Title:    request.Title,
		Catalog:  request.Catalog,
		MapIndex: request.MapIndex,
		X:        request.X,
		Y:        request.Y,
	})
	if !interactionstore.ValidDefinition(definition) {
		return interactionstore.Definition{}, http.StatusBadRequest, false
	}
	return definition, http.StatusOK, true
}

func decodeLocalInteractionDefinitionIdentity(r *http.Request) (string, string, bool) {
	raw := strings.TrimPrefix(r.URL.Path, "/local/interactions/")
	raw = strings.TrimSpace(raw)
	parts := strings.Split(raw, "/")
	if len(parts) != 2 {
		return "", "", false
	}
	kind, err := url.PathUnescape(parts[0])
	if err != nil {
		return "", "", false
	}
	ref, err := url.PathUnescape(parts[1])
	if err != nil {
		return "", "", false
	}
	kind = strings.TrimSpace(kind)
	ref = strings.TrimSpace(ref)
	if kind == "" || ref == "" || strings.Contains(kind, "/") || strings.Contains(ref, "/") {
		return "", "", false
	}
	return kind, ref, true
}

func decodeLocalContentBundleRequest(r *http.Request) (contentbundle.Bundle, int, bool) {
	const maxContentBundleBodyBytes = 1 << 20
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxContentBundleBodyBytes+1))
	if err != nil {
		return contentbundle.Bundle{}, http.StatusBadRequest, false
	}
	if len(raw) > maxContentBundleBodyBytes {
		return contentbundle.Bundle{}, http.StatusRequestEntityTooLarge, false
	}
	var bundle contentbundle.Bundle
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil {
		return contentbundle.Bundle{}, http.StatusBadRequest, false
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return contentbundle.Bundle{}, http.StatusBadRequest, false
	}
	return bundle, http.StatusOK, true
}

func decodeLocalStaticActorEntityID(r *http.Request) (uint64, bool) {
	entityIDRaw := strings.TrimPrefix(r.URL.Path, "/local/static-actors/")
	entityIDRaw = strings.TrimSpace(entityIDRaw)
	if entityIDRaw == "" || strings.Contains(entityIDRaw, "/") {
		return 0, false
	}
	entityID, err := strconv.ParseUint(entityIDRaw, 10, 64)
	if err != nil || entityID == 0 {
		return 0, false
	}
	return entityID, true
}

func decodeLocalGroundItemVID(r *http.Request) (uint32, bool) {
	vidRaw := strings.TrimPrefix(r.URL.Path, "/local/ground-items/")
	vidRaw = strings.TrimSpace(vidRaw)
	if vidRaw == "" || strings.Contains(vidRaw, "/") {
		return 0, false
	}
	vid, err := strconv.ParseUint(vidRaw, 0, 32)
	if err != nil || vid == 0 {
		return 0, false
	}
	return uint32(vid), true
}

func decodeLocalCharacterName(r *http.Request, prefix string) (string, bool) {
	raw := strings.TrimPrefix(r.URL.Path, prefix)
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.Contains(raw, "/") {
		return "", false
	}
	name, err := url.PathUnescape(raw)
	if err != nil {
		return "", false
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "/") {
		return "", false
	}
	return name, true
}

func writeLocalJSONMutationResponse(w http.ResponseWriter, value any, status int) {
	if status == 0 {
		status = http.StatusOK
	}
	if status < 200 || status >= 300 {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func isLoopbackRemoteAddr(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
