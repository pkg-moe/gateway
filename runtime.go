package gateway

import (
	"fmt"
	"log"

	"github.com/rs/cors"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
)

func (g *GateWay) Start() error {
	// handler register inproc
	g.serverHandlers.ForEach(g.serverInproc.RegisterService)

	// start internal server
	if err := g.GateWayInt.Start(); err != nil {
		return err
	}

	// GRPC server
	if err := g.GateWayGRPC.Start(); err != nil {
		return err
	}

	// http server
	log.Printf("Starting http	server on %s\n", g.config.PortHttp)
	//g.muxHttp.Handle("/api/", gziphandler.GzipHandler(g.httpXor(g.httpReqGzip(g.muxGRPC))))
	g.muxHttp.Handle("/api/", g.httpXor(g.muxGRPC))

	// set cors
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	fastHandler := fasthttpadaptor.NewFastHTTPHandler(c.Handler(g.muxHttp))

	g.serverHttp = &fasthttp.Server{
		Handler: func(ctx *fasthttp.RequestCtx) {
			fastHandler(ctx)
		},
	}

	return g.serverHttp.ListenAndServe(fmt.Sprintf("0.0.0.0:%s", g.config.PortHttp))
}

func (g *GateWay) Stop() {
	// stop http server
	{
		if err := g.serverHttp.Shutdown(); err != nil {
			log.Fatalf("http service Shutdown error: %v", err)
		}
	}

	// stop grpc server
	g.serverGRPC.Stop()

	// stop internal server
	g.GateWayInt.Stop()
}
