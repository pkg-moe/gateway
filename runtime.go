package gateway

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/rs/cors"

	"pkg.moe/pkg/gateway/grpc_websocket"
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
	g.muxHttp.Handle("/api/", g.httpXor(grpc_websocket.WebsocketProxy(g.muxGRPC, grpc_websocket.WithMaxRespBodyBufferSize(4*1024*1024))))

	// set cors
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	g.serverHttp = &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%s", g.config.PortHttp),
		Handler: c.Handler(g.muxHttp),
	}

	return g.serverHttp.ListenAndServe()
}

func (g *GateWay) Stop() {
	// stop http server
	{
		if err := g.serverHttp.Shutdown(context.TODO()); err != nil {
			log.Fatalf("http service Shutdown error: %v", err)
		}
	}

	// stop grpc server
	g.serverGRPC.Stop()

	// stop internal server
	g.GateWayInt.Stop()
}
