package gateway

import (
	"net"
	"os/exec"
)

// some android versions has no default gateway ip. or route table. it can have multiple '0.0.0.0' gates with same metrics at the same time.

func DiscoverGateway() (ip net.IP, err error) {
	ip, err = discoverGatewayUsingRoute()
	if err != nil {
		ip, err = discoverGatewayUsingIp()
		if err != nil {
			ip, err = discoverGatewayUsingPing()
		}
	}
	return
}

func discoverGatewayUsingIp() (net.IP, error) {
	routeCmd := exec.Command("/system/bin/ip", "route", "show")
	output, err := routeCmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return parseLinuxIPRoute(output)
}

func discoverGatewayUsingRoute() (net.IP, error) {
	routeCmd := exec.Command("/system/bin/route", "-n")
	output, err := routeCmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	return parseLinuxRoute(output)
}

// unsucessful ping discover can take up to 1 sec
func discoverGatewayUsingPing() (net.IP, error) {
	routeCmd := exec.Command("/system/bin/ping", "-n", "-c", "1", "-t", "1", "-W", "1", "198.41.0.4") // a.root-servers.net
	output, err := routeCmd.CombinedOutput()
	if len(output) == 0 { // err will be 1
		return nil, err
	}

	return parseLinuxPing(output)
}
