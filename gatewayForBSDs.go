//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package gateway

import (
	"net"
	"os/exec"
	"syscall"

	"golang.org/x/net/route"
)

func readNetstat() ([]byte, error) {
	routeCmd := exec.Command("netstat", "-rn")
	return routeCmd.CombinedOutput()
}

func discoverGatewaysByFamily(family int) ([]net.IP, error) {
	rib, err := route.FetchRIB(family, syscall.NET_RT_DUMP, 0)
	if err != nil {
		return nil, err
	}

	msgs, err := route.ParseRIB(syscall.NET_RT_DUMP, rib)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var result []net.IP
	for _, m := range msgs {
		rm, ok := m.(*route.RouteMessage)
		if !ok {
			continue
		}
		if len(rm.Addrs) <= syscall.RTAX_GATEWAY || rm.Addrs[syscall.RTAX_GATEWAY] == nil {
			continue
		}
		var ip net.IP
		switch sa := rm.Addrs[syscall.RTAX_GATEWAY].(type) {
		case *route.Inet4Addr:
			ip = net.IPv4(sa.IP[0], sa.IP[1], sa.IP[2], sa.IP[3])
		case *route.Inet6Addr:
			ip = make(net.IP, net.IPv6len)
			copy(ip, sa.IP[:])
		}
		if ip != nil {
			key := ip.String()
			if !seen[key] {
				seen[key] = true
				result = append(result, ip)
			}
		}
	}
	if len(result) == 0 {
		return nil, &ErrNoGateway{}
	}
	return result, nil
}

func discoverGatewaysOSSpecific() (ips []net.IP, err error) {
	return discoverGatewaysByFamily(syscall.AF_INET)
}

func discoverGatewaysIPv6OSSpecific() (ips []net.IP, err error) {
	return discoverGatewaysByFamily(syscall.AF_INET6)
}

func discoverGatewayInterfaceOSSpecific() (ip net.IP, err error) {
	bytes, err := readNetstat()
	if err != nil {
		return nil, err
	}

	return parseUnixInterfaceIP(bytes)
}

func discoverGatewayInterfaceIPv6OSSpecific() (ip net.IP, err error) {
	bytes, err := readNetstat()
	if err != nil {
		return nil, err
	}

	return parseUnixInterfaceIPv6(bytes)
}
