//go:generate tools/generate-tables.sh

package gateway

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
)

// For tests where an IP is parsed directly from route table
type ipTestCase struct {
	// Name of route table (tes_route_tables.go)
	tableName string

	// True if valid data expected
	ok bool

	// Dotted IP to assert
	ifaceIP string

	// Expected error, or nil if none expected
	expectedError error
}

// For tests where an interface name is parsed from route table
type ifaceTestCase struct {
	// Name of route table (tes_route_tables.go)
	tableName string

	// Name of interface expected from route table
	ifaceName string

	// True if valid data expected
	ok bool

	// Dotted IP to assert
	ifaceIP string

	// Expected error, or nil if none expected
	expectedError error
}

func TestParseWindows(t *testing.T) {

	testcases := []ipTestCase{
		{windows, true, "10.88.88.2", nil},
		{windowsLocalized, true, "10.88.88.2", nil},
		{windowsMultipleGateways, true, "10.21.38.1", nil},
		{randomData, false, "", &ErrCantParse{}},
		{windowsNoRoute, false, "", &ErrNoGateway{}},
		{windowsNoDefaultRoute, false, "", &ErrNoGateway{}},
		{windowsBadRoute1, false, "", &ErrCantParse{}},
		{windowsBadRoute2, false, "", &ErrCantParse{}},
	}

	t.Run("parseWindowsGatewayIP", func(t *testing.T) {
		testGatewayAddress(t, testcases, parseWindowsGatewayIP)
	})

	// Note that even if the value in the gateway column is rubbish like "foo"
	// the interface name can still be looked up if the dest is 0.0.0.0
	interfaceTestCases := []ipTestCase{
		{windows, true, "10.88.88.149", nil},
		{windowsLocalized, true, "10.88.88.149", nil},
		{windowsMultipleGateways, true, "10.21.38.97", nil},
		{randomData, false, "", &ErrCantParse{}},
		{windowsNoRoute, false, "", &ErrNoGateway{}},
		{windowsNoDefaultRoute, false, "", &ErrNoGateway{}},
		{windowsBadRoute1, false, "", &ErrCantParse{}},
		{windowsBadRoute2, true, "10.88.88.149", nil},
	}

	t.Run("parseWindowsInterfaceIP", func(t *testing.T) {
		testGatewayAddress(t, interfaceTestCases, parseWindowsInterfaceIP)
	})
}

func TestParseLinux(t *testing.T) {
	// Linux route tables are extracted from  proc filesystem

	testcases := []ipTestCase{
		{linux, true, "192.168.8.1", nil},
		{linuxNoRoute, false, "", &ErrNoGateway{}},
	}

	t.Run("parseLinuxGatewayIP", func(t *testing.T) {
		testGatewayAddress(t, testcases, parseLinuxGatewayIP)
	})

	interfaceTestCases := []ifaceTestCase{
		{linux, "wlp4s0", true, "192.168.2.1", nil},
		{linuxNoRoute, "wlp4s0", false, "", &ErrNoGateway{}},
	}

	t.Run("parseLinuxInterfaceIP", func(t *testing.T) {
		testInterfaceAddress(t, interfaceTestCases, parseLinuxInterfaceIPImpl)
	})

	// ifData := []byte(`Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
	// eth0	00000000	00000000	0001	0	0	1000	0000FFFF	0	0	0
	// `)
	// interfaceTestCases := []testcase{
	// 	{ifData, true, "192.168.8.238"},
	// 	{noRoute, false, ""},
	// }

	// to run interface test in your local computer, change eth0 with your default interface name, and change the expected IP to be your default IP
	// test(t, interfaceTestCases, parseLinuxInterfaceIP)
}

func TestParseUnix(t *testing.T) {
	// Unix route tables are extracted from netstat -rn

	testcases := []ipTestCase{
		{darwin, true, "192.168.1.254", nil},
		{freeBSD, true, "10.88.88.2", nil},
		{netBSD, true, "172.31.16.1", nil},
		{solaris, true, "172.16.32.1", nil},
		{solarisNoInterface, true, "172.16.32.1", nil},
		{randomData, false, "", &ErrCantParse{}},
		{darwinNoRoute, false, "", &ErrNoGateway{}},
		{darwinBadRoute, false, "", &ErrCantParse{}},
		{freeBSDNoRoute, false, "", &ErrNoGateway{}},
		{freeBSDBadRoute, false, "", &ErrCantParse{}},
		{netBSDNoRoute, false, "", &ErrNoGateway{}},
		{netBSDBadRoute, false, "", &ErrCantParse{}},
		{solarisNoRoute, false, "", &ErrNoGateway{}},
		{solarisBadRoute, false, "", &ErrCantParse{}},
	}

	t.Run("parseUnixGatewayIP", func(t *testing.T) {
		testGatewayAddress(t, testcases, parseUnixGatewayIP)
	})

	// Note that even if the value in the gateway column is rubbish like "foo"
	// the interface name can still be looked up if the dest is 0.0.0.0
	interfaceTestCases := []ifaceTestCase{
		{darwin, "en0", true, "192.168.1.254", nil},
		{freeBSD, "ena0", true, "10.88.88.2", nil},
		{netBSD, "ena0", true, "172.31.16.1", nil},
		{solaris, "net0", true, "172.16.32.1", nil},
		{solarisNoInterface, "", true, "172.16.32.1", nil},
		{randomData, "", false, "", &ErrCantParse{}},
		{darwinNoRoute, "", false, "", &ErrNoGateway{}},
		{darwinBadRoute, "en0", true, "192.168.1.254", &ErrCantParse{}},
		{freeBSDNoRoute, "", false, "", &ErrNoGateway{}},
		{freeBSDBadRoute, "ena0", true, "10.88.88.2", nil},
		{netBSDNoRoute, "", false, "", &ErrNoGateway{}},
		{netBSDBadRoute, "ena0", true, "172.31.16.1", nil},
		{solarisNoRoute, "", false, "", &ErrNoGateway{}},
		{solarisBadRoute, "net0", true, "172.16.32.1", nil},
	}

	t.Run("parseUnixInterfaceIP", func(t *testing.T) {
		testInterfaceAddress(t, interfaceTestCases, parseUnixInterfaceIPImpl)
	})
}

func testGatewayAddress(t *testing.T, testcases []ipTestCase, fn func([]byte) (net.IP, error)) {
	for i, tc := range testcases {
		t.Run(tc.tableName, func(t *testing.T) {
			net, err := fn(routeTables[tc.tableName])
			if tc.ok {
				if err != nil {
					t.Errorf("Unexpected error in test #%d: %v", i, err)
				}
				if net.String() != tc.ifaceIP {
					t.Errorf("Unexpected gateway address %v != %s", net, tc.ifaceIP)
				}
			} else if err == nil {
				t.Errorf("Unexpected nil error in test #%d", i)
			} else if errors.Is(err, tc.expectedError) {
				// Correct error was retured
				return
			} else {
				t.Errorf("Expected error of type %T, got %T", tc.expectedError, err)
			}
		})
	}
}

func testInterfaceAddress(t *testing.T, testcases []ifaceTestCase, fn func([]byte, interfaceGetter) (net.IP, error)) {
	for i, tc := range testcases {
		mockGetter := newMockinterfaceGetter(t)

		if tc.ok {
			// If the test is exepected to pass, i.e. return an interface IP,
			// then these methods must be called with the given arguments.
			//
			// Mock assertions will ensure they are called (or not if they're not supposed to be)
			mockGetter.On("InterfaceByName", tc.ifaceName).Return(&net.Interface{}, nil)
			mockGetter.On("Addrs", mock.AnythingOfType("*net.Interface")).Return([]net.Addr{
				&net.IPNet{
					IP:   net.ParseIP("fe80::42:66ff:fe89:8a6b"),
					Mask: net.IPMask{},
				},
				&net.IPNet{
					IP:   net.ParseIP(tc.ifaceIP),
					Mask: net.IPMask{},
				},
			}, nil)
		}

		t.Run(tc.tableName, func(t *testing.T) {
			net, err := fn(routeTables[tc.tableName], mockGetter)
			if tc.ok {
				if err != nil {
					t.Errorf("Unexpected error in test #%d: %v", i, err)
				}
				if net.String() != tc.ifaceIP {
					t.Errorf("Unexpected interface address %v != %s", net, tc.ifaceIP)
				}
			} else if err == nil {
				t.Errorf("Unexpected nil error in test #%d", i)
			} else if errors.Is(err, tc.expectedError) {
				// Correct error was retured
				return
			} else {
				t.Errorf("Expected error of type %T, got %T", tc.expectedError, err)
			}
		})
	}
}

func TestFlagsContain(t *testing.T) {
	type testcase struct {
		flags           string
		required        []string
		expectectResult bool
	}

	testcases := []testcase{
		{"UGS", []string{"U", "G"}, true},
		{"UH", []string{"U", "G"}, false},
		{"U", []string{"U", "G"}, false},
		{"UHS", []string{"U", "G"}, false},
		{"UHl", []string{"U", "G"}, false},
		{"UGScIg", []string{"U", "G"}, true},
	}

	for _, testcase := range testcases {
		t.Run(fmt.Sprintf("%s: %s is %v", testcase.flags, strings.Join(testcase.required, ","), testcase.expectectResult), func(t *testing.T) {
			if !flagsContain(testcase.flags, testcase.required...) == testcase.expectectResult {
				t.Errorf("Expected %s to contain %v", testcase.flags, testcase.required)
			}
		})
	}
}

func TestDiscoverFields(t *testing.T) {

	type testcase struct {
		name       string
		routeTable []byte
	}

	testcases := []testcase{
		{"darwin", routeTables[darwin]},
		{"FreeBSD", routeTables[freeBSD]},
		{"NetBSD", routeTables[netBSD]},
		{"Solaris", routeTables[solaris]},
		{"Illumos", routeTables[solarisNoInterface]},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			lineNo, _ := discoverFields(testcase.routeTable)

			if lineNo == -1 {
				t.Error("Could not parse route table fields")
			}
		})
	}
}

func ExampleDiscoverGateway() {
	gateway, err := DiscoverGateway()
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("Gateway:", gateway.String())
	}
}
