package gateway

import (
	"bytes"
	"errors"
	"net"
	"strings"
)

var errNoGateway = errors.New("no gateway found")

func parseRoutePrint(output []byte) (net.IP, error) {
	// Windows route output format is always like this:
	// ===========================================================================
	// Active Routes:
	// Network Destination        Netmask          Gateway       Interface  Metric
	//           0.0.0.0          0.0.0.0      192.168.1.1    192.168.1.100     20
	// ===========================================================================
	// I'm trying to pick the active route,
	// then jump 2 lines and pick the third IP
	// Not using regex because output is quite standard from Windows XP to 8 (NEEDS TESTING)
	outputLines := bytes.Split(output, []byte("\n"))
	for idx, line := range outputLines {
		if bytes.Contains(line, []byte("Active Routes:")) {
			if len(outputLines) <= idx+2 {
				return nil, errNoGateway
			}

			ipFields := bytes.Fields(outputLines[idx+2])
			if len(ipFields) < 3 {
				return nil, errNoGateway
			}

			ip := net.ParseIP(string(ipFields[2]))
			return ip, nil
		}
	}
	return nil, errNoGateway
}

func parseNetstat(output []byte) (net.IP, error) {
	// netstat -rn produces the following on FreeBSD:
	// Routing tables
	//
	// Internet:
	// Destination        Gateway            Flags      Netif Expire
	// default            10.88.88.2         UGS         em0
	// 10.88.88.0/24      link#1             U           em0
	// 10.88.88.148       link#1             UHS         lo0
	// 127.0.0.1          link#2             UH          lo0
	//
	// Internet6:
	// Destination                       Gateway                       Flags      Netif Expire
	// ::/96                             ::1                           UGRS        lo0
	// ::1                               link#2                        UH          lo0
	// ::ffff:0.0.0.0/96                 ::1                           UGRS        lo0
	// fe80::/10                         ::1                           UGRS        lo0
	// ...
	outputLines := strings.Split(string(output), "\n")
	for _, line := range outputLines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "default" {
			ip := net.ParseIP(fields[1])
			if ip != nil {
				return ip, nil
			}
		}
	}

	return nil, errNoGateway
}
