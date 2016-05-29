package gateway

import (
	"bufio"
	"net"
	"os/exec"
	"strings"
)

func DiscoverGateway() (ip net.IP, err error) {
	routeCmd := exec.Command("/sbin/route", "-n", "get", "0.0.0.0")
	stdOut, err := routeCmd.StdoutPipe()
	if err != nil {
		return
	}
	if err = routeCmd.Start(); err != nil {
		return
	}

	// Darwin route out format is always like this:
	//    route to: default
	// destination: default
	//        mask: default
	//     gateway: 192.168.1.1
	for cmdScanner := bufio.NewScanner(stdOut); ; cmdScanner.Scan() {
		if line := cmdScanner.Text(); strings.Contains(line, "gateway:") {
			gatewayFields := strings.Fields(line)
			ip = net.ParseIP(gatewayFields[1])
			break
		}
	}

	err = routeCmd.Wait()
	return
}
