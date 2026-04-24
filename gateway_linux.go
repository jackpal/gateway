//go:build linux
// +build linux

package gateway

import (
	"fmt"
	"io"
	"net"
	"os"
)

const (
	// See http://man7.org/linux/man-pages/man8/route.8.html
	file     = "/proc/net/route"
	fileIPv6 = "/proc/net/ipv6_route"
)

func readRoutes() ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("can't access %s", file)
	}
	defer f.Close()

	bytes, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("can't read %s", file)
	}

	return bytes, nil
}

func readRoutesIPv6() ([]byte, error) {
	f, err := os.Open(fileIPv6)
	if err != nil {
		return nil, fmt.Errorf("can't access %s", fileIPv6)
	}
	defer f.Close()

	bytes, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("can't read %s", fileIPv6)
	}

	return bytes, nil
}

func discoverGatewaysOSSpecific() (ips []net.IP, err error) {
	bytes, err := readRoutes()
	if err != nil {
		return nil, err
	}
	return parseLinuxGatewayIPs(bytes)
}

func discoverGatewayInterfaceOSSpecific() (ip net.IP, err error) {
	bytes, err := readRoutes()
	if err != nil {
		return nil, err
	}
	return parseLinuxInterfaceIP(bytes)
}

func discoverGatewaysIPv6OSSpecific() (ips []net.IP, err error) {
	bytes, err := readRoutesIPv6()
	if err != nil {
		return nil, err
	}
	return parseLinuxIPv6GatewayIPs(bytes)
}

func discoverGatewayInterfaceIPv6OSSpecific() (ip net.IP, err error) {
	bytes, err := readRoutesIPv6()
	if err != nil {
		return nil, err
	}
	return parseLinuxIPv6InterfaceIP(bytes)
}
