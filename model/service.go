package gm

import (
	"github.com/fullstorydev/grpchan/inprocgrpc"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

type IGateWay interface {
	Register(IRegister)
	Serve(...IService)
}

type IGateWayRegister interface {
	GRPC() grpc.ServiceRegistrar
	MuxGRPC() *runtime.ServeMux
	ServerInproc() *inprocgrpc.Channel
}
