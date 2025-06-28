//go:build android
// +build android

package gateway

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
)

const (
	// See http://man7.org/linux/man-pages/man8/route.8.html
	file  = "/proc/net/route"
)

func discoverGatewaysOSSpecific() (ips []net.IP, err error) {
	ips, err = wrapIP(discoverGatewayUsingRoute())
	if err != nil {
		ips, err = wrapIP(discoverGatewayUsingIpRouteShow())
	}
	if err != nil {
		ips, err = wrapIP(discoverGatewayUsingIpRouteGet())
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

func discoverGatewayInterfaceOSSpecific() (ip net.IP, err error) {
	bytes, err := readRoutes()
	if err != nil {
		return nil, err
	}
	return parseLinuxInterfaceIP(bytes)
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

func wrapIP(ip net.IP, err error) ([]net.IP, error) {
    if err != nil {
        return nil, err
    }
    return []net.IP{ip}, nil
}
