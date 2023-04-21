package gm

type IXDAPPRegister interface {
	AddWebInstanceMethods(obj interface{}, namespace string)
}

type IService func(IGateWay)

type IRegister func(IGateWayRegister) error
