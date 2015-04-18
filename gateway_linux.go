package gateway

import (
	"bytes"
	"io/ioutil"
	"net"
	"os/exec"
)

func DiscoverGateway() (ip net.IP, err error) {
	routeCmd := exec.Command("route", "-n")
	stdOut, err := routeCmd.StdoutPipe()
	if err != nil {
		return
	}
	if err = routeCmd.Start(); err != nil {
		return
	}
	output, err := ioutil.ReadAll(stdOut)
	if err != nil {
		return
	}

	// Linux route out format is always like this:
	// Kernel IP routing table
	// Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
	// 0.0.0.0         192.168.1.1     0.0.0.0         UG    0      0        0 eth0
	outputLines := bytes.Split(output, []byte("\n"))
	for _, line := range outputLines {
		if bytes.Contains(line, []byte("0.0.0.0")) {
			ipFields := bytes.Fields(line)
			ip = net.ParseIP(string(ipFields[1]))
			break
		}
	}
	err = routeCmd.Wait()
	return
}
