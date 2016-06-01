package gateway

import "testing"

func TestGateway(t *testing.T) {
	ip, err := DiscoverGateway()
	if err != nil {
		t.Errorf("DiscoverGateway() = %v,%v", ip, err)
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
