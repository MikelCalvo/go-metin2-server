package ops

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	stdpprof "net/http/pprof"
	"net/url"
	"strconv"
	"strings"

	contentbundle "github.com/MikelCalvo/go-metin2-server/internal/contentbundle"
	"github.com/MikelCalvo/go-metin2-server/internal/interactionstore"
)

type localRelocationRequest struct {
	Name     string `json:"name"`
	MapIndex uint32 `json:"map_index"`
	X        int32  `json:"x"`
	Y        int32  `json:"y"`
}

type localStaticActorRequest struct {
	Name            string `json:"name"`
	MapIndex        uint32 `json:"map_index"`
	X               int32  `json:"x"`
	Y               int32  `json:"y"`
	RaceNum         uint32 `json:"race_num"`
	InteractionKind string `json:"interaction_kind"`
	InteractionRef  string `json:"interaction_ref"`
}

type localInteractionDefinitionRequest struct {
	Kind     string `json:"kind"`
	Ref      string `json:"ref"`
	Text     string `json:"text"`
	MapIndex uint32 `json:"map_index"`
	X        int32  `json:"x"`
	Y        int32  `json:"y"`
}

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

func RegisterLocalStaticActorEndpoints(mux *http.ServeMux, staticActors func() any, registerStaticActor func(string, uint32, int32, int32, uint32, string, string) (any, bool)) *http.ServeMux {
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
			actor, ok := registerStaticActor(request.Name, request.MapIndex, request.X, request.Y, request.RaceNum, request.InteractionKind, request.InteractionRef)
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

func RegisterLocalStaticActorUpdateEndpoint(mux *http.ServeMux, updateStaticActor func(uint64, string, uint32, int32, int32, uint32, string, string) (any, bool)) *http.ServeMux {
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
		actor, ok := updateStaticActor(entityID, request.Name, request.MapIndex, request.X, request.Y, request.RaceNum, request.InteractionKind, request.InteractionRef)
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
			request, ok := decodeLocalInteractionDefinitionRequest(r)
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
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
		request, ok := decodeLocalInteractionDefinitionRequest(r)
		if !ok || request.Kind != kind || request.Ref != ref {
			w.WriteHeader(http.StatusBadRequest)
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
			bundle, ok := decodeLocalContentBundleRequest(r)
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
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
	return request, true
}

func decodeLocalInteractionDefinitionRequest(r *http.Request) (interactionstore.Definition, bool) {
	var request localInteractionDefinitionRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return interactionstore.Definition{}, false
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return interactionstore.Definition{}, false
	}
	definition := interactionstore.Definition{
		Kind:     strings.TrimSpace(request.Kind),
		Ref:      strings.TrimSpace(request.Ref),
		Text:     request.Text,
		MapIndex: request.MapIndex,
		X:        request.X,
		Y:        request.Y,
	}
	if !interactionstore.ValidDefinition(definition) {
		return interactionstore.Definition{}, false
	}
	return definition, true
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

func decodeLocalContentBundleRequest(r *http.Request) (contentbundle.Bundle, bool) {
	var bundle contentbundle.Bundle
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&bundle); err != nil {
		return contentbundle.Bundle{}, false
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return contentbundle.Bundle{}, false
	}
	return bundle, true
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
