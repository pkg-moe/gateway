package gateway

import (
	"fmt"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

type GateWayInt struct {
	PortInt string

	serverInt *fasthttp.Server
	muxInt    *http.ServeMux
}

func NewGateWayInt(portInt string) *GateWayInt {
	g := &GateWayInt{
		PortInt: portInt,
		muxInt:  pprofServeMux(),
	}

	// mux internal
	g.muxInt.Handle("/metrics", promhttp.Handler())

	return g
}

// internal server
func (g *GateWayInt) Start() error {
	log.Printf("Starting pprof	server on %s\n", g.PortInt)
	go func() {
		fastHandler := fasthttpadaptor.NewFastHTTPHandler(g.muxInt)
		g.serverInt = &fasthttp.Server{
			Handler: func(ctx *fasthttp.RequestCtx) {
				fastHandler(ctx)
			},
		}
		_ = g.serverInt.ListenAndServe(fmt.Sprintf("0.0.0.0:%s", g.PortInt))
	}()

	return nil
}

func (g *GateWayInt) Stop() {
	if err := g.serverInt.Shutdown(); err != nil {
		log.Fatalf("internal service Shutdown error: %v", err)
	}
}
