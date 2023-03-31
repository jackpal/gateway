package gateway

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

type windowsRouteStruct struct {
	Gateway   string
	Interface string
}

type linuxRouteStruct struct {
	Iface   string
	Gateway string
}

func parseToWindowsRouteStruct(output []byte) (windowsRouteStruct, error) {
	// Windows route output format is always like this:
	// ===========================================================================
	// Interface List
	// 8 ...00 12 3f a7 17 ba ...... Intel(R) PRO/100 VE Network Connection
	// 1 ........................... Software Loopback Interface 1
	// ===========================================================================
	// IPv4 Route Table
	// ===========================================================================
	// Active Routes:
	// Network Destination        Netmask          Gateway       Interface  Metric
	//           0.0.0.0          0.0.0.0      192.168.1.1    192.168.1.100     20
	// ===========================================================================
	//
	// Windows commands are localized, so we can't just look for "Active Routes:" string
	// I'm trying to pick the active route,
	// then jump 2 lines and get the row
	// Not using regex because output is quite standard from Windows XP to 8 (NEEDS TESTING)
	lines := strings.Split(string(output), "\n")
	sep := 0
	for idx, line := range lines {
		if sep == 3 {
			// We just entered the 2nd section containing "Active Routes:"
			if len(lines) <= idx+2 {
				return windowsRouteStruct{}, errNoGateway
			}

			fields := strings.Fields(lines[idx+2])
			if len(fields) < 5 {
				return windowsRouteStruct{}, errCantParse
			}

			return windowsRouteStruct{
				Gateway:   fields[2],
				Interface: fields[3],
			}, nil
		}
		if strings.HasPrefix(line, "=======") {
			sep++
			continue
		}
	}
	return windowsRouteStruct{}, errNoGateway
}

func parseToLinuxRouteStruct(output []byte) (linuxRouteStruct, error) {
	// parseLinuxProcNetRoute parses the route file located at /proc/net/route
	// and returns the IP address of the default gateway. The default gateway
	// is the one with Destination value of 0.0.0.0.
	//
	// The Linux route file has the following format:
	//
	// $ cat /proc/net/route
	//
	// Iface   Destination Gateway     Flags   RefCnt  Use Metric  Mask
	// eno1    00000000    C900A8C0    0003    0   0   100 00000000    0   00
	// eno1    0000A8C0    00000000    0001    0   0   100 00FFFFFF    0   00
	const (
		sep              = "\t" // field separator
		destinationField = 1    // field containing hex destination address
		gatewayField     = 2    // field containing hex gateway address
		maskField        = 7    // field containing hex mask
	)
	scanner := bufio.NewScanner(bytes.NewReader(output))

	// Skip header line
	if !scanner.Scan() {
		return linuxRouteStruct{}, errors.New("Invalid linux route file")
	}

	for scanner.Scan() {
		row := scanner.Text()
		tokens := strings.Split(row, sep)
		if len(tokens) < 11 {
			return linuxRouteStruct{}, fmt.Errorf("invalid row %q in route file: doesn't have 11 fields", row)
		}

		// The default interface is the one that's 0 for both destination and mask.
		if !(tokens[destinationField] == "00000000" && tokens[maskField] == "00000000") {
			continue
		}

		return linuxRouteStruct{
			Iface:   tokens[0],
			Gateway: tokens[2],
		}, nil
	}
	return linuxRouteStruct{}, errors.New("interface with default destination not found")
}

func parseWindowsGatewayIP(output []byte) (net.IP, error) {
	parsedOutput, err := parseToWindowsRouteStruct(output)
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(parsedOutput.Gateway)
	if ip == nil {
		return nil, errCantParse
	}
	return ip, nil
}

func parseWindowsInterfaceIP(output []byte) (net.IP, error) {
	parsedOutput, err := parseToWindowsRouteStruct(output)
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(parsedOutput.Interface)
	if ip == nil {
		return nil, errCantParse
	}
	return ip, nil
}

func parseLinuxGatewayIP(output []byte) (net.IP, error) {
	parsedStruct, err := parseToLinuxRouteStruct(output)
	if err != nil {
		return nil, err
	}

	// cast hex address to uint32
	d, err := strconv.ParseUint(parsedStruct.Gateway, 16, 32)
	if err != nil {
		return nil, fmt.Errorf(
			"parsing default interface address field hex %q: %w",
			parsedStruct.Gateway,
			err,
		)
	}
	// make net.IP address from uint32
	ipd32 := make(net.IP, 4)
	binary.LittleEndian.PutUint32(ipd32, uint32(d))
	return ipd32, nil
}

func parseLinuxInterfaceIP(output []byte) (net.IP, error) {
	parsedStruct, err := parseToLinuxRouteStruct(output)
	if err != nil {
		return nil, err
	}

	iface, err := net.InterfaceByName(parsedStruct.Iface)
	if err != nil {
		return nil, err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, err
	}

	// Return the first IPv4 address we encounter.
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}

		ip := ipnet.IP.To4()
		if ip != nil {
			return ip, nil
		}
	}

	return nil, fmt.Errorf("no IPv4 address found for interface %v",
		parsedStruct.Iface)
}

func parseDarwinRouteGet(output []byte) (net.IP, error) {
	// Darwin route out format is always like this:
	//    route to: default
	// destination: default
	//        mask: default
	//     gateway: 192.168.1.1
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "gateway:" {
			ip := net.ParseIP(fields[1])
			if ip != nil {
				return ip, nil
			}
		}
	}

	return nil, errNoGateway
}

func parseDarwinNetstat(output []byte) (net.IP, error) {
	// Darwin netstat -nr out format is always like this:
	// Routing tables

	// Internet:
	// Destination        Gateway            Flags           Netif Expire
	// default            link#17            UCSg            utun3
	// default            192.168.1.1      	 UGScIg            en0
	outputLines := strings.Split(string(output), "\n")
	for _, line := range outputLines {
		fields := strings.Fields(line)

		if len(fields) >= 3 && fields[0] == "default" {
			// validate routing table flags:
			// https://library.netapp.com/ecmdocs/ECMP1155586/html/GUID-07F1F043-7AB7-4749-8F8D-727929233E62.html
			//
			// U = Up—Route is valid
			isUp := strings.Contains(fields[2], "U")
			// G = Gateway—Route is to a gateway router rather than to a directly connected network or host
			isGateway := strings.Contains(fields[2], "G")

			if isUp && isGateway {
				ip := net.ParseIP(fields[1])
				if ip != nil {
					return ip, nil
				}
			}
		}
	}
	return nil, errNoGateway
}

func parseBSDSolarisNetstat(output []byte) (net.IP, error) {
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
