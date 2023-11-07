//go:generate tools/generate-tables.sh

package gateway

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
)

type testcase struct {
	output  []byte
	ok      bool
	gateway string
}

type testcase2 struct {
	tableName string
	ok        bool
	ifaceIP   string
}

type ifaceTestCase struct {
	tableName string
	ifaceName string
	ok        bool
	ifaceIP   string
}

func TestParseWindows(t *testing.T) {

	testcases := []testcase2{
		{windows, true, "10.88.88.2"},
		{windowsLocalized, true, "10.88.88.2"},
		{randomData, false, ""},
		{windowsNoRoute, false, ""},
		{windowsBadRoute1, false, ""},
		{windowsBadRoute2, false, ""},
	}

	t.Run("parseWindowsGatewayIP", func(t *testing.T) {
		testGatewayAddress(t, testcases, parseWindowsGatewayIP)
	})

	interfaceTestCases := []testcase2{
		{windows, true, "10.88.88.149"},
		{windowsLocalized, true, "10.88.88.149"},
		{randomData, false, ""},
		{windowsNoRoute, false, ""},
		{windowsBadRoute1, false, ""},
		{windowsBadRoute2, true, "10.88.88.149"},
	}

	t.Run("parseWindowsInterfaceIP", func(t *testing.T) {
		testGatewayAddress(t, interfaceTestCases, parseWindowsInterfaceIP)
	})
}

func TestParseLinux(t *testing.T) {
	// Linux ruote tables are extracted from  proc filesystem

	testcases := []testcase2{
		{linux, true, "192.168.8.1"},
		{linuxNoRoute, false, ""},
	}

	t.Run("parseLinuxGatewayIP", func(t *testing.T) {
		testGatewayAddress(t, testcases, parseLinuxGatewayIP)
	})

	interfaceTestCases := []ifaceTestCase{
		{linux, "wlp4s0", true, "192.168.2.1"},
		{linuxNoRoute, "wlp4s0", false, ""},
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

func TestParseDarwinRouteGet(t *testing.T) {
	correctData := []byte(`
   route to: 0.0.0.0
destination: default
       mask: default
    gateway: 172.16.32.1
  interface: en0
      flags: <UP,GATEWAY,DONE,STATIC,PRCLONING>
 recvpipe  sendpipe  ssthresh  rtt,msec    rttvar  hopcount      mtu     expire
       0         0         0         0         0         0      1500         0
`)
	randomData := []byte(`
test
Lorem ipsum dolor sit amet, consectetur adipiscing elit,
sed do eiusmod tempor incididunt ut labore et dolore magna
aliqua. Ut enim ad minim veniam, quis nostrud exercitation
`)
	noRoute := []byte(`
   route to: 0.0.0.0
destination: default
       mask: default
`)
	badRoute := []byte(`
   route to: 0.0.0.0
destination: default
       mask: default
    gateway: foo
  interface: en0
      flags: <UP,GATEWAY,DONE,STATIC,PRCLONING>
 recvpipe  sendpipe  ssthresh  rtt,msec    rttvar  hopcount      mtu     expire
       0         0         0         0         0         0      1500         0
`)

	testcases := []testcase{
		{correctData, true, "172.16.32.1"},
		{randomData, false, ""},
		{noRoute, false, ""},
		{badRoute, false, ""},
	}

	test(t, testcases, parseDarwinRouteGet)
}

func TestParseUnix(t *testing.T) {
	// Unix route tables are extracted from netstat -rn

	testcases := []testcase2{
		{darwin, true, "192.168.1.254"},
		{freeBSD, true, "10.88.88.2"},
		{netBSD, true, "172.31.16.1"},
		{solaris, true, "172.16.32.1"},
		{randomData, false, ""},
		{darwinNoRoute, false, ""},
		{darwinBadRoute, false, ""},
		{freeBSDNoRoute, false, ""},
		{freeBSDBadRoute, false, ""},
		{netBSDNoRoute, false, ""},
		{netBSDBadRoute, false, ""},
		{solarisNoRoute, false, ""},
		{solarisBadRoute, false, ""},
	}

	t.Run("parseUnixGatewayIP", func(t *testing.T) {
		testGatewayAddress(t, testcases, parseUnixGatewayIP)
	})

	// Note that even if the value in the gateway column if rubbish like "foo"
	// the interface name can still be looked up
	interfaceTestCases := []ifaceTestCase{
		{darwin, "en0", true, "192.168.1.254"},
		{freeBSD, "ena0", true, "10.88.88.2"},
		{netBSD, "ena0", true, "172.31.16.1"},
		{solaris, "net0", true, "172.16.32.1"},
		{randomData, "", false, ""},
		{darwinNoRoute, "", false, ""},
		{darwinBadRoute, "en0", true, "192.168.1.254"},
		{freeBSDNoRoute, "", false, ""},
		{freeBSDBadRoute, "ena0", true, "10.88.88.2"},
		{netBSDNoRoute, "", false, ""},
		{netBSDBadRoute, "ena0", true, "172.31.16.1"},
		{solarisNoRoute, "", false, ""},
		{solarisBadRoute, "net0", true, "172.16.32.1"},
	}

	t.Run("parseUnixInterfaceIP", func(t *testing.T) {
		testInterfaceAddress(t, interfaceTestCases, parseUnixInterfaceIPImpl)
	})
}

func test(t *testing.T, testcases []testcase, fn func([]byte) (net.IP, error)) {
	for i, tc := range testcases {
		t.Run("unixGateway", func(t *testing.T) {
			net, err := fn(tc.output)
			if tc.ok {
				if err != nil {
					t.Errorf("Unexpected error in test #%d: %v", i, err)
				}
				if net.String() != tc.gateway {
					t.Errorf("Unexpected gateway address %v != %s", net, tc.gateway)
				}
			} else if err == nil {
				t.Errorf("Unexpected nil error in test #%d", i)
			}
		})
	}
}

func testGatewayAddress(t *testing.T, testcases []testcase2, fn func([]byte) (net.IP, error)) {
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
