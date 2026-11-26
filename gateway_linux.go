//go:build linux
// +build linux

package gateway

import (
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
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
	bytes, procErr := readRoutes()
	if procErr == nil {
		return parseLinuxGatewayIPs(bytes)
	}
	rib, nlErr := syscall.NetlinkRIB(syscall.RTM_GETROUTE, syscall.AF_INET)
	if nlErr != nil {
		return nil, procErr
	}
	ips, nlErr = parseLinuxNetlinkGatewayIPs(rib)
	if nlErr != nil {
		return nil, procErr
	}
	return ips, nil
}

func discoverGatewayInterfaceOSSpecific() (ip net.IP, err error) {
	bytes, procErr := readRoutes()
	if procErr == nil {
		return parseLinuxInterfaceIP(bytes)
	}
	rib, nlErr := syscall.NetlinkRIB(syscall.RTM_GETROUTE, syscall.AF_INET)
	if nlErr != nil {
		return nil, procErr
	}
	ip, nlErr = parseLinuxNetlinkInterfaceIP(rib)
	if nlErr != nil {
		return nil, procErr
	}
	return ip, nil
}

func discoverGatewaysIPv6OSSpecific() (ips []net.IP, err error) {
	bytes, procErr := readRoutesIPv6()
	if procErr == nil {
		return parseLinuxIPv6GatewayIPs(bytes)
	}
	rib, nlErr := syscall.NetlinkRIB(syscall.RTM_GETROUTE, syscall.AF_INET6)
	if nlErr != nil {
		return nil, procErr
	}
	ips, nlErr = parseLinuxNetlinkIPv6GatewayIPs(rib)
	if nlErr != nil {
		return nil, procErr
	}
	return ips, nil
}

func discoverGatewayInterfaceIPv6OSSpecific() (ip net.IP, err error) {
	bytes, procErr := readRoutesIPv6()
	if procErr == nil {
		return parseLinuxIPv6InterfaceIP(bytes)
	}
	rib, nlErr := syscall.NetlinkRIB(syscall.RTM_GETROUTE, syscall.AF_INET6)
	if nlErr != nil {
		return nil, procErr
	}
	ip, nlErr = parseLinuxNetlinkIPv6InterfaceIP(rib)
	if nlErr != nil {
		return nil, procErr
	}
	return ip, nil
}
