package gateway

import "testing"

func TestGateway(t *testing.T) {
	ip, err := DiscoverGateway()
	if err != nil {
		t.Errorf("DiscoverGateway() = %v,%v", ip, err)
	} else {
		t.Logf("ip %v\n", ip)
	}

}

func TestParseRoutePrint(t *testing.T) {
	correctData := []byte(`
IPv4 Route Table
===========================================================================
Active Routes:
Network Destination        Netmask          Gateway       Interface  Metric
          0.0.0.0          0.0.0.0       10.88.88.2     10.88.88.149     10
===========================================================================
Persistent Routes:
`)
	randomData := []byte(`
Lorem ipsum dolor sit amet, consectetur adipiscing elit,
sed do eiusmod tempor incididunt ut labore et dolore magna
aliqua. Ut enim ad minim veniam, quis nostrud exercitation
`)
	noRoute := []byte(`
IPv4 Route Table
===========================================================================
Active Routes:
`)
	badRoute := []byte(`
IPv4 Route Table
===========================================================================
Active Routes:
===========================================================================
Persistent Routes:
`)

	testcases := []struct {
		output  []byte
		ok      bool
		gateway string
	}{
		{correctData, true, "10.88.88.2"},
		{randomData, false, ""},
		{noRoute, false, ""},
		{badRoute, false, ""},
	}

	for i, tc := range testcases {
		net, err := parseRoutePrint(tc.output)
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
	}
}

func TestParseNetstat(t *testing.T) {
	correctDataFreeBSD := []byte(`
Routing tables

Internet:
Destination        Gateway            Flags      Netif Expire
default            10.88.88.2         UGS         em0
10.88.88.0/24      link#1             U           em0
10.88.88.148       link#1             UHS         lo0
127.0.0.1          link#2             UH          lo0

Internet6:
Destination                       Gateway                       Flags      Netif Expire
::/96                             ::1                           UGRS        lo0
::1                               link#2                        UH          lo0
::ffff:0.0.0.0/96                 ::1                           UGRS        lo0
fe80::/10                         ::1                           UGRS        lo0
`)
	correctDataSolaris := []byte(`
Routing Table: IPv4
  Destination           Gateway           Flags  Ref     Use     Interface
-------------------- -------------------- ----- ----- ---------- ---------
default              172.16.32.1          UG        2      76419 net0
127.0.0.1            127.0.0.1            UH        2         36 lo0
172.16.32.0          172.16.32.17         U         4       8100 net0

Routing Table: IPv6
  Destination/Mask            Gateway                   Flags Ref   Use    If
--------------------------- --------------------------- ----- --- ------- -----
::1                         ::1                         UH      3   75382 lo0
2001:470:deeb:32::/64       2001:470:deeb:32::17        U       3    2744 net0
fe80::/10                   fe80::6082:52ff:fedc:7df0   U       3    8430 net0
`)
	randomData := []byte(`
Lorem ipsum dolor sit amet, consectetur adipiscing elit,
sed do eiusmod tempor incididunt ut labore et dolore magna
aliqua. Ut enim ad minim veniam, quis nostrud exercitation
`)
	noRoute := []byte(`
Internet:
Destination        Gateway            Flags      Netif Expire
10.88.88.0/24      link#1             U           em0
10.88.88.148       link#1             UHS         lo0
127.0.0.1          link#2             UH          lo0
`)
	badRoute := []byte(`
Internet:
Destination        Gateway            Flags      Netif Expire
default            foo                UGS         em0
10.88.88.0/24      link#1             U           em0
10.88.88.148       link#1             UHS         lo0
127.0.0.1          link#2             UH          lo0
`)

	testcases := []struct {
		output  []byte
		ok      bool
		gateway string
	}{
		{correctDataFreeBSD, true, "10.88.88.2"},
		{correctDataSolaris, true, "172.16.32.1"},
		{randomData, false, ""},
		{noRoute, false, ""},
		{badRoute, false, ""},
	}

	for i, tc := range testcases {
		net, err := parseNetstat(tc.output)
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
	}
}
