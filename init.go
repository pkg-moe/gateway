package gateway

import (
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/valyala/fasthttp"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
)

type GateWay struct {
	GateWayInt
	GateWayGRPC

	config *Config

	serverHttp *fasthttp.Server

	muxHttp *http.ServeMux
	muxGRPC *runtime.ServeMux
}

// Config is Gateway Config
type Config struct {
	// grpc
	ConfigGRPC

	// port
	PortHttp string
	PortInt  string
}

func NewGateWay(config *Config) *GateWay {
	if config == nil {
		return nil
	}

	g := &GateWay{}
	g.config = config

	// proto raw marshaler
	protoRaw := &runtime.ProtoMarshaller{}

	// mux grpc
	g.muxGRPC = runtime.NewServeMux(
		// register header matcher
		runtime.WithIncomingHeaderMatcher(HeaderMatcher),

		// register proto raw
		runtime.WithMarshalerOption(protoRaw.ContentType(1), protoRaw),
	)

	// mux http
	g.muxHttp = httpServeMux()
	g.GateWayInt = *NewGateWayInt(config.PortInt)

	// mux grpc
	g.GateWayGRPC = *NewGateWayGRPC(&config.ConfigGRPC)

	return g
}

func (g *GateWay) GRPC() grpc.ServiceRegistrar {
	return g.GateWayGRPC.GRPC()
}
