package gateway

import (
	"context"
	"log"
	"reflect"

	"pkg.moe/pkg/gateway/model"
)

func (g *GateWay) Register(client interface{}, gateway interface{}) {
	ctxValue := reflect.ValueOf(context.Background())

	clientFunc := reflect.ValueOf(client)
	gatewayFunc := reflect.ValueOf(gateway)

	clientResult := clientFunc.Call([]reflect.Value{
		reflect.ValueOf(g.serverInproc),
	})

	gatewayResult := gatewayFunc.Call([]reflect.Value{
		ctxValue, reflect.ValueOf(g.muxGRPC), clientResult[0],
	})

	err := gatewayResult[0].Interface()

	switch err.(type) {
	case error:
		log.Fatalf("register gateway error: %v", err)
	}
}

func (g *GateWay) Serve(service ...gm.IService) {
	for _, f := range service {
		f(g)
	}
}
