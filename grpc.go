package gateway

import (
	"fmt"
	"log"
	"net"

	"github.com/fullstorydev/grpchan"
	"github.com/fullstorydev/grpchan/inprocgrpc"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"pkg.moe/pkg/logger"

	gm "pkg.moe/pkg/gateway/model"
)

type ConfigGRPC struct {
	// port
	PortGRPC string

	// interceptor
	StreamInterceptor []grpc.StreamServerInterceptor
	UnaryInterceptor  []grpc.UnaryServerInterceptor
}

type GateWayGRPC struct {
	configGRPC     *ConfigGRPC
	serverHandlers grpchan.HandlerMap
	serverGRPC     *grpc.Server
	serverInproc   *inprocgrpc.Channel
}

func NewGateWayGRPC(configGRPC *ConfigGRPC) *GateWayGRPC {
	g := &GateWayGRPC{}
	g.configGRPC = configGRPC

	// panic error
	grpcPanicError := func(p interface{}) (err error) {
		logger.Get().Error("request panic", logger.FieldError(err), logger.Field("p", p))
		return status.Errorf(codes.Internal, "%s", p)
	}

	// Stream Interceptor
	streamServerInterceptor := []grpc.StreamServerInterceptor{}
	streamServerInterceptor = append(streamServerInterceptor, grpc_recovery.StreamServerInterceptor(
		grpc_recovery.WithRecoveryHandler(grpcPanicError),
	))
	streamServerInterceptor = append(streamServerInterceptor, grpc_prometheus.StreamServerInterceptor)
	streamServerInterceptor = append(streamServerInterceptor, configGRPC.StreamInterceptor...)

	// Unary Interceptor
	unaryServerInterceptor := []grpc.UnaryServerInterceptor{}
	unaryServerInterceptor = append(unaryServerInterceptor, grpc_recovery.UnaryServerInterceptor(
		grpc_recovery.WithRecoveryHandler(grpcPanicError),
	))
	unaryServerInterceptor = append(unaryServerInterceptor, grpc_prometheus.UnaryServerInterceptor)
	unaryServerInterceptor = append(unaryServerInterceptor, configGRPC.UnaryInterceptor...)

	// serverGRPC Server Option
	serverOption := []grpc.ServerOption{}
	serverOption = append(serverOption, grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamServerInterceptor...)))
	serverOption = append(serverOption, grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryServerInterceptor...)))

	// handler map
	g.serverHandlers = grpchan.HandlerMap{}

	// grpc server
	g.serverGRPC = grpc.NewServer(serverOption...)

	// inproc server
	g.serverInproc = &inprocgrpc.Channel{}
	g.serverInproc.WithServerUnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryServerInterceptor...)).WithServerStreamInterceptor(grpc_middleware.ChainStreamServer(streamServerInterceptor...))

	grpc_prometheus.Register(g.serverGRPC)

	return g
}

func (g *GateWayGRPC) GRPC() grpc.ServiceRegistrar {
	return g.serverHandlers
}

func (g *GateWayGRPC) Register(f gm.IRegister) {

}

func (g *GateWayGRPC) Serve(service ...gm.IService) {
	for _, f := range service {
		f(g)
	}
}

func (g *GateWayGRPC) Start() error {
	// register handler to grpc
	g.serverHandlers.ForEach(g.serverGRPC.RegisterService)

	// GRPC server
	log.Printf("Starting grpc	server on %s\n", g.configGRPC.PortGRPC)

	listen, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", g.configGRPC.PortGRPC))
	if err != nil {
		log.Fatalf("GRPC Listen %s error: %v", g.configGRPC.PortGRPC, err)
	}

	go func() {
		if err := g.serverGRPC.Serve(listen); err != nil {
			log.Fatalf("GRPC Serve %s error: %v", g.configGRPC.PortGRPC, err)
		}
	}()

	return nil
}

func (g *GateWayGRPC) Stop() {
	// stop grpc server
	g.serverGRPC.Stop()
}
