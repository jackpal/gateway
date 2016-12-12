package gateway

import (
	"net"
	"os"
	"os/exec"
)

func DiscoverGateway() (ip net.IP, err error) {
	ip, err = discoverGatewayUsingRoute()
	if err != nil {
		ip, err = discoverGatewayUsingIp()
	}
	return
}

func discoverGatewayUsingIp() (net.IP, error) {
	ipPaths := []string{"/usr/bin/ip", "/bin/ip", "/usr/sbin/ip", "/sbin/ip"}
	output, err := execCommand(ipPaths, "show")
	if err != nil {
		return nil, err
	}

	return parseLinuxIPRoute(output)
}

func discoverGatewayUsingRoute() (net.IP, error) {
	routePaths := []string{"/usr/bin/route", "/bin/route", "/usr/sbin/route", "/sbin/route"}
	output, err := execCommand(routePaths, "-n")
	if err != nil {
		return nil, err
	}

	return parseLinuxRoute(output)
}

// try command at multiple possible paths. Return on first non "not found" error
func execCommand(cmdPaths []string, arg ...string) (output []byte, err error) {
	for _, cmdPath := range cmdPaths {
		output, err = exec.Command(cmdPath, arg...).CombinedOutput()
		if !os.IsNotExist(err) {
			break
		}
	}
	return
}
