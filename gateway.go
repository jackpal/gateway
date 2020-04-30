package gateway

import (
	"net"
)

func Get() (ip net.IP, err error) {
	return DiscoverGateway()
}
