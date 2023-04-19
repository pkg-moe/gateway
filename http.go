package gateway

import (
	"net/http"
	"net/http/pprof"
)

func httpServeMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Not Found"))
	})

	return mux
}

func pprofServeMux() *http.ServeMux {
	httpServer := httpServeMux()

	handlerFuncMap := map[string]http.HandlerFunc{
		"/debug/pprof/":        pprof.Index,
		"/debug/pprof/cmdline": pprof.Cmdline,
		"/debug/pprof/profile": pprof.Profile,
		"/debug/pprof/symbol":  pprof.Symbol,
		"/debug/pprof/trace":   pprof.Trace,
	}

	for pattern, handlerFunc := range handlerFuncMap {
		httpServer.HandleFunc(pattern, handlerFunc)
	}

	return httpServer
}
