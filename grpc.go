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
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
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

	ServerOption []grpc.ServerOption
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
		logger.Get().Error("request panic", logger.FieldError(err), logger.Field("p", p), logger.Field("stack", string(debug.Stack())))
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
	serverOption = append(serverOption, configGRPC.ServerOption...)

	// handler map
	g.serverHandlers = grpchan.HandlerMap{}

	// grpc server
	g.serverGRPC = grpc.NewServer(serverOption...)

	// inproc server
	g.serverInproc = &inprocgrpc.Channel{}
	g.serverInproc.WithServerUnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryServerInterceptor...)).WithServerStreamInterceptor(grpc_middleware.ChainStreamServer(streamServerInterceptor...))

	grpc_prometheus.Register(g.serverGRPC)

	// health check
	grpc_health_v1.RegisterHealthServer(g.serverHandlers, health.NewServer())

	return g
}

func (g *GateWayGRPC) MuxGRPC() *runtime.ServeMux {
	return nil
}

func (g *GateWayGRPC) ServerInproc() *inprocgrpc.Channel {
	return nil
}

func (g *GateWayGRPC) GRPC() grpc.ServiceRegistrar {
	return g.serverHandlers
}

func (g *GateWayGRPC) Register(f gm.IRegister) {
	if err := f(g); err != nil {
		log.Fatalf("register gateway error: %v", err)
	}
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

func RegisterGRPC[V any](service V, server func(s grpc.ServiceRegistrar, srv V)) gm.IRegister {
	return func(g gm.IGateWayRegister) error {
		server(g.GRPC(), service)

		return nil
	}
}
