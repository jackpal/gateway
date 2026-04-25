package gateway

// References
// * https://superuser.com/questions/622144/what-does-netstat-r-on-osx-tell-you-about-gateways
// * https://man.freebsd.org/cgi/man.cgi?query=netstat&sektion=1

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

const (
	ns_destination = "Destination"
	ns_flags       = "Flags"
	ns_netif       = "Netif"
	ns_gateway     = "Gateway"
	ns_interface   = "Interface"
)

type netstatFields map[string]int

type windowsRouteStruct struct {
	// Dotted IP address
	Gateway string

	// Dotted IP address
	Interface string
}

type linuxRouteStruct struct {
	// Name of interface
	Iface string

	// big-endian hex string
	Gateway string
}

type unixRouteStruct struct {
	// Name of interface
	Iface string

	// Dotted IP address
	Gateway string
}

func fieldNum(fields []string, names ...string) int {
	// Return the zero-based index of given field in slice of field names
	for num, field := range fields {
		for _, name := range names {
			if name == field {
				return num
			}
		}
	}

	return -1
}

func discoverFields(output []byte) (int, netstatFields) {
	// Discover positions of fields of interest in netstat output
	nf := make(netstatFields, 4)

	outputLines := strings.Split(string(output), "\n")
	for lineNo, line := range outputLines {
		fields := strings.Fields(line)

		if len(fields) > 3 {
			d := fieldNum(fields, ns_destination, "Destination/Mask")
			f := fieldNum(fields, ns_flags)
			g := fieldNum(fields, ns_gateway)
			n := fieldNum(fields, ns_netif, "If", ns_interface)
			if d >= 0 && f >= 0 && g >= 0 && n >= 0 {
				nf[ns_destination] = d
				nf[ns_flags] = f
				nf[ns_gateway] = g
				nf[ns_netif] = n

				return lineNo, nf
			}
		}
	}

	// Unable to parse column headers
	return -1, nil
}

func flagsContain(flags string, flag ...string) bool {
	// Check route table flags field for existence of specific flags
	contain := true

	for _, f := range flag {
		contain = contain && strings.Contains(flags, f)
	}

	return contain
}

func parseToWindowsRouteStruct(output []byte) ([]windowsRouteStruct, error) {
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
	//
	// If multiple default gateways are present, then the one with the lowest metric is returned.
	type gatewayEntry struct {
		gateway string
		iface   string
		metric  int
	}

	ipRegex := regexp.MustCompile(`^(((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.|$)){4})`)
	defaultRoutes := make([]gatewayEntry, 0, 2)
	lines := strings.Split(string(output), "\n")
	sep := 0
	for idx, line := range lines {
		if sep == 3 {
			// We just entered the 2nd section containing "Active Routes:"
			if len(lines) <= idx+2 {
				return nil, &ErrNoGateway{}
			}

			inputLine := lines[idx+2]
			if strings.HasPrefix(inputLine, "=======") {
				// End of routes
				break
			}
			fields := strings.Fields(inputLine)
			// Some Windows commands are localized, so we need to handle the fields in a logical way.
			// Basically, fields that start with a number will be treated as-is, but consecutive fields
			// that start with a letter will be combined into a single field.
			{
				var logicalFields []string
				for f := 0; f < len(fields); f++ {
					field := fields[f]
					if len(field) > 0 && unicode.IsLetter(rune(field[0])) {
						for f+1 < len(fields) {
							nextField := fields[f+1]
							if len(nextField) > 0 && unicode.IsLetter(rune(nextField[0])) {
								field += " " + nextField
								f++
							} else {
								break
							}
						}
					}
					logicalFields = append(logicalFields, field)
				}
				fields = logicalFields
			}
			if len(fields) < 5 || !ipRegex.MatchString(fields[0]) {
				return nil, &ErrCantParse{}
			}

			if fields[0] != "0.0.0.0" {
				// Routes to 0.0.0.0 are listed first
				// so we are done
				break
			}

			metric, err := strconv.Atoi(fields[4])

			if err != nil {
				return nil, err
			}

			defaultRoutes = append(defaultRoutes, gatewayEntry{
				gateway: fields[2],
				iface:   fields[3],
				metric:  metric,
			})
		}
		if strings.HasPrefix(line, "=======") {
			sep++
			continue
		}
	}

	if sep == 0 {
		// We saw no separator lines, so input must have been garbage.
		return nil, &ErrCantParse{}
	}

	if len(defaultRoutes) == 0 {
		return nil, &ErrNoGateway{}
	}

	slices.SortFunc(defaultRoutes,
		func(a, b gatewayEntry) int {
			return a.metric - b.metric
		})

	result := make([]windowsRouteStruct, 0, len(defaultRoutes))
	for _, defaultRoute := range defaultRoutes {
		result = append(result, windowsRouteStruct{
			Gateway:   defaultRoute.gateway,
			Interface: defaultRoute.iface,
		})
	}
	if len(result) == 0 {
		return nil, &ErrNoGateway{}
	}
	return result, nil
}

func parseToLinuxRouteStructs(output []byte) ([]linuxRouteStruct, error) {
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
		err := scanner.Err()
		if err == nil {
			return nil, &ErrNoGateway{}
		}

		return nil, err
	}

	var result []linuxRouteStruct
	for scanner.Scan() {
		row := scanner.Text()
		tokens := strings.Split(row, sep)
		if len(tokens) < 11 {
			return nil, &ErrInvalidRouteFileFormat{row: row}
		}

		// The default interface is the one that's 0 for both destination and mask.
		if !(tokens[destinationField] == "00000000" && tokens[maskField] == "00000000") {
			continue
		}

		result = append(result, linuxRouteStruct{
			Iface:   tokens[0],
			Gateway: tokens[2],
		})
	}
	if len(result) == 0 {
		return nil, &ErrNoGateway{}
	}
	return result, nil
}

func parseWindowsGatewayIPs(output []byte) ([]net.IP, error) {
	parsedOutputs, err := parseToWindowsRouteStruct(output)
	if err != nil {
		return nil, err
	}

	result := make([]net.IP, 0, len(parsedOutputs))
	for _, parsedOutput := range parsedOutputs {
		// Skip "On-link" gateways (these will start with a letter; not all languages will print "On-link").
		if len(parsedOutput.Gateway) > 0 && unicode.IsLetter(rune(parsedOutput.Gateway[0])) {
			continue
		}
		ip := net.ParseIP(parsedOutput.Gateway)
		if ip == nil {
			return nil, &ErrCantParse{}
		}
		result = append(result, ip)
	}
	if len(result) == 0 {
		return nil, &ErrNoGateway{}
	}
	return result, nil
}

func parseWindowsInterfaceIP(output []byte) ([]net.IP, error) {
	parsedOutputs, err := parseToWindowsRouteStruct(output)
	if err != nil {
		return nil, err
	}

	result := make([]net.IP, 0, len(parsedOutputs))
	for _, parsedOutput := range parsedOutputs {
		ip := net.ParseIP(parsedOutput.Interface)
		if ip == nil {
			return nil, &ErrCantParse{}
		}
		result = append(result, ip)
	}
	return result, nil
}

func parseWindowsIPv6GatewayIPs(output []byte) ([]net.IP, error) {
	// Windows IPv6 route table format (from 'route print -6'):
	//
	// ===========================================================================
	// IPv6 Route Table
	// ===========================================================================
	// Active Routes:
	//  If Metric Network Destination      Gateway
	//  12    281  ::/0                    fe80::1
	//  12    281  ::1/128                 On-link
	// ===========================================================================

	lines := strings.Split(string(output), "\n")
	inActiveRoutes := false
	seen := make(map[string]bool)
	var result []net.IP

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "Active Routes:") || strings.HasPrefix(line, "Rutas activas:") {
			inActiveRoutes = true
			continue
		}

		if !inActiveRoutes {
			continue
		}

		// End of active routes section
		if strings.HasPrefix(line, "====") || strings.HasPrefix(line, "Persistent") || strings.HasPrefix(line, "Rutas persistentes") || line == "" {
			if len(result) > 0 {
				break
			}
			continue
		}

		// Skip column header line
		if strings.HasPrefix(line, "If") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		// Fields: If, Metric, Network Destination, Gateway
		dest := fields[2]
		gateway := fields[3]

		// Look for default route ::/0
		if dest != "::/0" {
			continue
		}

		// Skip "On-link" or other text gateways
		ip := net.ParseIP(gateway)
		if ip == nil {
			continue
		}

		key := ip.String()
		if !seen[key] {
			seen[key] = true
			result = append(result, ip)
		}
	}

	if len(result) == 0 {
		return nil, &ErrNoGateway{}
	}
	return result, nil
}

func parseWindowsIPv6InterfaceIP(output []byte) (net.IP, error) {
	return parseWindowsIPv6InterfaceIPImpl(output, &intefaceGetterImpl{})
}

func parseWindowsIPv6InterfaceIPImpl(output []byte, ifaceGetter interfaceGetter) (net.IP, error) {
	// Parse the Windows IPv6 route table to find the interface index
	// for the default route (::/0), then resolve it to an IPv6 address.
	lines := strings.Split(string(output), "\n")
	inActiveRoutes := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "Active Routes:") || strings.HasPrefix(line, "Rutas activas:") {
			inActiveRoutes = true
			continue
		}

		if !inActiveRoutes {
			continue
		}

		if strings.HasPrefix(line, "====") || strings.HasPrefix(line, "Persistent") || strings.HasPrefix(line, "Rutas persistentes") || line == "" {
			continue
		}

		if strings.HasPrefix(line, "If") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		// Fields: If, Metric, Network Destination, Gateway
		ifIndex := fields[0]
		dest := fields[2]

		if dest != "::/0" {
			continue
		}

		// Parse the interface index
		idx, err := strconv.Atoi(ifIndex)
		if err != nil {
			continue
		}

		// Look up the interface by index
		iface, err := ifaceGetter.InterfaceByIndex(idx)
		if err != nil {
			return nil, err
		}

		addrs, err := ifaceGetter.Addrs(iface)
		if err != nil {
			return nil, err
		}

		// Find an IPv6 address on this interface, preferring global over link-local
		var linkLocal net.IP
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			if ipnet.IP.To4() != nil {
				continue
			}
			ip := ipnet.IP.To16()
			if ip == nil {
				continue
			}
			if !ip.IsLinkLocalUnicast() {
				return ip, nil
			}
			if linkLocal == nil {
				linkLocal = ip
			}
		}
		if linkLocal != nil {
			return linkLocal, nil
		}
	}

	return nil, &ErrNoGateway{}
}

func parseLinuxGatewayIPs(output []byte) ([]net.IP, error) {
	parsedStructs, err := parseToLinuxRouteStructs(output)
	if err != nil {
		return nil, err
	}

	result := make([]net.IP, 0, len(parsedStructs))
	for _, parsedStruct := range parsedStructs {
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
		result = append(result, ipd32)
	}
	return result, nil
}

func parseLinuxInterfaceIP(output []byte) (net.IP, error) {
	// Return the first IPv4 address we encounter.
	return parseLinuxInterfaceIPImpl(output, &intefaceGetterImpl{})
}

func parseLinuxInterfaceIPImpl(output []byte, ifaceGetter interfaceGetter) (net.IP, error) {
	// Mockable implemenation
	parsedStructs, err := parseToLinuxRouteStructs(output)
	if err != nil {
		return nil, err
	}

	return getInterfaceIP4(parsedStructs[0].Iface, ifaceGetter)
}

// linuxIPv6RouteStruct represents a parsed entry from /proc/net/ipv6_route
type linuxIPv6RouteStruct struct {
	// Name of interface
	Iface string

	// 32-character hex string representing 128-bit IPv6 address
	Gateway string
}

func parseToLinuxIPv6RouteStructs(output []byte) ([]linuxIPv6RouteStruct, error) {
	// Parse /proc/net/ipv6_route which has the format:
	//
	// dest dest_prefix src src_prefix nexthop metric refcnt use flags iface
	// 00000000000000000000000000000000 00 00000000000000000000000000000000 00 fe800000000000000242acfffe110003 00000064 00000000 00000000 00000003 eth0
	//
	// Fields are space-separated. All hex values.
	// The default route has destination all zeros with prefix length 00.
	const (
		destinationField    = 0
		destinationPrefField = 1
		gatewayField        = 4
		ifaceField          = 9
		allZeros            = "00000000000000000000000000000000"
	)
	scanner := bufio.NewScanner(bytes.NewReader(output))

	var result []linuxIPv6RouteStruct
	for scanner.Scan() {
		row := scanner.Text()
		fields := strings.Fields(row)
		if len(fields) < 10 {
			continue
		}

		// Default route: destination is all zeros, prefix length is 00
		if fields[destinationField] != allZeros || fields[destinationPrefField] != "00" {
			continue
		}

		// Skip if gateway is also all zeros (no real gateway)
		if fields[gatewayField] == allZeros {
			continue
		}

		result = append(result, linuxIPv6RouteStruct{
			Iface:   fields[ifaceField],
			Gateway: fields[gatewayField],
		})
	}
	if len(result) == 0 {
		return nil, &ErrNoGateway{}
	}
	return result, nil
}

func parseIPv6Hex(hexStr string) (net.IP, error) {
	if len(hexStr) != 32 {
		return nil, fmt.Errorf("invalid IPv6 hex string length: %d", len(hexStr))
	}
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("parsing IPv6 hex %q: %w", hexStr, err)
	}
	return net.IP(b), nil
}

func parseLinuxIPv6GatewayIPs(output []byte) ([]net.IP, error) {
	parsedStructs, err := parseToLinuxIPv6RouteStructs(output)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	result := make([]net.IP, 0, len(parsedStructs))
	for _, parsedStruct := range parsedStructs {
		ip, err := parseIPv6Hex(parsedStruct.Gateway)
		if err != nil {
			return nil, err
		}
		key := ip.String()
		if !seen[key] {
			seen[key] = true
			result = append(result, ip)
		}
	}
	return result, nil
}

func parseLinuxIPv6InterfaceIP(output []byte) (net.IP, error) {
	return parseLinuxIPv6InterfaceIPImpl(output, &intefaceGetterImpl{})
}

func parseLinuxIPv6InterfaceIPImpl(output []byte, ifaceGetter interfaceGetter) (net.IP, error) {
	parsedStructs, err := parseToLinuxIPv6RouteStructs(output)
	if err != nil {
		return nil, err
	}

	return getInterfaceIP6(parsedStructs[0].Iface, ifaceGetter)
}

func parseUnixInterfaceIP(output []byte) (net.IP, error) {
	// Return the first IPv4 address we encounter.
	return parseUnixInterfaceIPImpl(output, &intefaceGetterImpl{})
}

func parseUnixInterfaceIPImpl(output []byte, ifaceGetter interfaceGetter) (net.IP, error) {
	// Mockable implemenation
	parsedStructs, err := parseNetstatToRouteStruct(output)
	if err != nil {
		return nil, err
	}

	return getInterfaceIP4(parsedStructs[0].Iface, ifaceGetter)
}

func getInterfaceIP4(name string, ifaceGetter interfaceGetter) (net.IP, error) {
	// Given interface name and an interface to "net" package
	// lookup ip4 for the given interface
	iface, err := ifaceGetter.InterfaceByName(name)
	if err != nil {
		return nil, err
	}

	addrs, err := ifaceGetter.Addrs(iface)
	if err != nil {
		return nil, err
	}

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
		name)
}

func parseUnixInterfaceIPv6(output []byte) (net.IP, error) {
	// Return the first IPv6 address we encounter.
	return parseUnixInterfaceIPv6Impl(output, &intefaceGetterImpl{})
}

func parseUnixInterfaceIPv6Impl(output []byte, ifaceGetter interfaceGetter) (net.IP, error) {
	// Mockable implementation
	parsedStructs, err := parseNetstatToRouteStruct(output)
	if err != nil {
		return nil, err
	}

	return getInterfaceIP6(parsedStructs[0].Iface, ifaceGetter)
}

func getInterfaceIP6(name string, ifaceGetter interfaceGetter) (net.IP, error) {
	// Given interface name and an interface to "net" package
	// lookup ip6 for the given interface
	iface, err := ifaceGetter.InterfaceByName(name)
	if err != nil {
		return nil, err
	}

	addrs, err := ifaceGetter.Addrs(iface)
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}

		// Skip IPv4 addresses
		if ipnet.IP.To4() != nil {
			continue
		}

		ip := ipnet.IP.To16()
		if ip != nil && !ip.IsLinkLocalUnicast() {
			return ip, nil
		}
	}

	// Fall back to link-local if no global address found
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}

		if ipnet.IP.To4() != nil {
			continue
		}

		ip := ipnet.IP.To16()
		if ip != nil {
			return ip, nil
		}
	}

	return nil, fmt.Errorf("no IPv6 address found for interface %v",
		name)
}

func parseUnixGatewayIPs(output []byte) ([]net.IP, error) {
	// Extract default gateway IP from netstat route table
	parsedStructs, err := parseNetstatToRouteStruct(output)
	if err != nil {
		return nil, err
	}

	result := make([]net.IP, 0, len(parsedStructs))
	for _, parsedStruct := range parsedStructs {
		ip := net.ParseIP(parsedStruct.Gateway)
		if ip == nil {
			return nil, &ErrCantParse{}
		}
		result = append(result, ip)
	}
	return result, nil
}

// Parse any netstat -rn output
func parseNetstatToRouteStruct(output []byte) ([]unixRouteStruct, error) {
	startLine, nsFields := discoverFields(output)

	if startLine == -1 {
		// Unable to find required column headers in netstat output
		return nil, &ErrCantParse{}
	}

	outputLines := strings.Split(string(output), "\n")

	var result []unixRouteStruct
	for lineNo, line := range outputLines {
		if lineNo <= startLine || strings.Contains(line, "-----") {
			// Skip until past column headers and heading underlines (solaris)
			continue
		}

		fields := strings.Fields(line)

		if len(fields) < 4 {
			// past route entries (got to end or blank line prior to ip6 entries)
			if len(result) > 0 {
				break
			}
			continue
		}

		if fields[nsFields[ns_destination]] == "default" && flagsContain(fields[nsFields[ns_flags]], "U", "G") {
			iface := ""
			if ifaceIdx := nsFields[ns_netif]; ifaceIdx < len(fields) {
				iface = fields[nsFields[ns_netif]]
			}
			result = append(result, unixRouteStruct{
				Iface:   iface,
				Gateway: fields[nsFields[ns_gateway]],
			})
		}
	}
	if len(result) == 0 {
		return nil, &ErrNoGateway{}
	}
	return result, nil
}

func parseSolarisIPv6GatewayIPs(output []byte) ([]net.IP, error) {
	// Solaris netstat -rn output has a section "Routing Table: IPv6"
	idx := bytes.Index(output, []byte("Routing Table: IPv6"))
	if idx != -1 {
		output = output[idx:]
	}

	parsedStructs, err := parseNetstatToRouteStruct(output)
	if err != nil {
		return nil, err
	}

	result := make([]net.IP, 0, len(parsedStructs))
	for _, parsedStruct := range parsedStructs {
		ip := net.ParseIP(parsedStruct.Gateway)
		if ip != nil {
			result = append(result, ip)
		}
	}
	if len(result) == 0 {
		return nil, &ErrNoGateway{}
	}
	return result, nil
}

func parseSolarisIPv6InterfaceIP(output []byte) (net.IP, error) {
	return parseSolarisIPv6InterfaceIPImpl(output, &intefaceGetterImpl{})
}

func parseSolarisIPv6InterfaceIPImpl(output []byte, ifaceGetter interfaceGetter) (net.IP, error) {
	idx := bytes.Index(output, []byte("Routing Table: IPv6"))
	if idx != -1 {
		output = output[idx:]
	}

	parsedStructs, err := parseNetstatToRouteStruct(output)
	if err != nil {
		return nil, err
	}

	return getInterfaceIP6(parsedStructs[0].Iface, ifaceGetter)
}

