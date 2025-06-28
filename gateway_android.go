//go:build android
// +build android

package gateway

import (
    "net"
    "os/exec"
    "strings"
)

func discoverGatewaysOSSpecific() (ips []net.IP, err error) {
	ips, err = discoverGatewayUsingRoute()
	if err != nil {
		ips, err = discoverGatewayUsingIpRouteShow()
	}
	if err != nil {
		ips, err = discoverGatewayUsingIpRouteGet()
	}
	return
}

func discoverGatewayUsingIpRouteShow() (net.IP, error) {
	routeCmd := exec.Command("ip", "route", "show")
	output, err := routeCmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return parseLinuxIPRouteShow(output)
}

func discoverGatewayUsingIpRouteGet() (net.IP, error) {
	routeCmd := exec.Command("ip", "route", "get", "8.8.8.8")
	output, err := routeCmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return parseLinuxIPRouteGet(output)
}

func discoverGatewayUsingRoute() (net.IP, error) {
	routeCmd := exec.Command("route", "-n")
	output, err := routeCmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return parseLinuxRoute(output)
}

func parseLinuxIPRouteGet(output []byte) (net.IP, error) {
	// Linux '/usr/bin/ip route get 8.8.8.8' format looks like this:
	// 8.8.8.8 via 10.0.1.1 dev eth0  src 10.0.1.36  uid 2000
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "via" {
			ip := net.ParseIP(fields[2])
			if ip != nil {
				return ip, nil
			}
		}
	}

	return nil, &ErrNoGateway{}
}

func parseLinuxRoute(output []byte) (net.IP, error) {
	// Linux route out format is always like this:
	// Kernel IP routing table
	// Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
	// 0.0.0.0         192.168.1.1     0.0.0.0         UG    0      0        0 eth0
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "0.0.0.0" {
			ip := net.ParseIP(fields[1])
			if ip != nil {
				return ip, nil
			}
		}
	}

	return nil, &ErrNoGateway{}
}
