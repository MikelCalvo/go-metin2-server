package ops

import (
	"fmt"
	"net/http"
	stdpprof "net/http/pprof"
)

func NewPprofMux(serviceName string) *http.ServeMux {
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

	return mux
}
