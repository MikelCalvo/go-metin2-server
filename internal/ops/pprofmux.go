package ops

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	stdpprof "net/http/pprof"
	"strings"
)

type localRelocationRequest struct {
	Name     string `json:"name"`
	MapIndex uint32 `json:"map_index"`
	X        int32  `json:"x"`
	Y        int32  `json:"y"`
}

type localStaticActorRequest struct {
	Name     string `json:"name"`
	MapIndex uint32 `json:"map_index"`
	X        int32  `json:"x"`
	Y        int32  `json:"y"`
	RaceNum  uint32 `json:"race_num"`
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

func RegisterLocalStaticActorEndpoints(mux *http.ServeMux, staticActors func() any, registerStaticActor func(string, uint32, int32, int32, uint32) (any, bool)) *http.ServeMux {
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
			actor, ok := registerStaticActor(request.Name, request.MapIndex, request.X, request.Y, request.RaceNum)
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
	if request.Name == "" || request.MapIndex == 0 || request.RaceNum == 0 {
		return localStaticActorRequest{}, false
	}
	return request, true
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
