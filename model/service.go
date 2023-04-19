package gm

import (
	"google.golang.org/grpc"
)

type IGateWay interface {
	GRPC() grpc.ServiceRegistrar
	Register(interface{}, interface{})
	Serve(...IService)
}
