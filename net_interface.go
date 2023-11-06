package gateway

import "net"

// Wrapper for net.InterfaceByName so it can be mocked in tests.
type interfaceGetter interface {
	InterfaceByName(name string) (*net.Interface, error)
	Addrs(iface *net.Interface) ([]net.Addr, error)
}

type intefaceGetterImpl struct{}

func (*intefaceGetterImpl) InterfaceByName(name string) (*net.Interface, error) {
	return net.InterfaceByName(name)
}

func (*intefaceGetterImpl) Addrs(iface *net.Interface) ([]net.Addr, error) {
	return iface.Addrs()
}
