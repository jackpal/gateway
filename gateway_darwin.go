// +build darwin

package gateway

import (
	"net"
	"os/exec"
)

func discoverGatewayOSSpecific() (net.IP, error) {
	routeCmd := exec.Command("/sbin/route", "-n", "get", "0.0.0.0")
	output, err := routeCmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	ip, err := parseDarwinRouteGet(output)
	if err != nil {
		// If we fail to retrieve the gateway using route then
		// try to fallback to netstat if it's on the system.
		nsPath, err := exec.LookPath("netstat")
		if err != nil {
			return nil, errNoGateway
		}

		nsCmd := exec.Command(nsPath, "-nr")
		output, err = nsCmd.CombinedOutput()
		if err != nil {
			return nil, errNoGateway
		}
		return parseDarwinNetstat(output)
	}
	return ip, nil
}

func discoverGatewayInterfaceOSSpecific() (ip net.IP, err error) {
	return nil, errNotImplemented
}
