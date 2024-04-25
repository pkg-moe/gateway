package gateway

import (
	"context"
	"log"

	"pkg.moe/pkg/gateway/model"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

func Register[T, V any](service V, server func(s grpc.ServiceRegistrar, srv V), client func(cc grpc.ClientConnInterface) T, gateway func(ctx context.Context, mux *runtime.ServeMux, client T) error) gm.IRegister {
	return func(g gm.IGateWayRegister) error {
		server(g.GRPC(), service)

		ctx := context.Background()

		in := g.ServerInproc()
		if in == nil {
			return nil
		}

		c := client(in)
		if err := gateway(ctx, g.MuxGRPC(), c); err != nil {
			return err
		}

		return nil
	}
}

func (g *GateWay) Register(f gm.IRegister) {
	if err := f(g); err != nil {
		log.Fatalf("register gateway error: %v", err)
	}
}

func (g *GateWay) Serve(service ...gm.IService) {
	for _, f := range service {
		f(g)
	}
}
