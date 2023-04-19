package gm

type IXDAPPRegister interface {
	AddWebInstanceMethods(obj interface{}, namespace string)
}

type IService func(IGateWay)
