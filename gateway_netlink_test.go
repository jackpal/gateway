package gateway

import (
	"encoding/binary"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/mock"
)

type netlinkAttrSpec struct {
	typ uint16
	val []byte
}

func attrBytes(typ uint16, val []byte) netlinkAttrSpec {
	return netlinkAttrSpec{typ: typ, val: val}
}

func attrUint32(typ uint16, v uint32) netlinkAttrSpec {
	b := make([]byte, 4)
	binary.NativeEndian.PutUint32(b, v)
	return netlinkAttrSpec{typ: typ, val: b}
}

func buildNetlinkRouteMsg(family, dstLen, table, rtType uint8, attrs ...netlinkAttrSpec) []byte {
	var attrBuf []byte
	for _, a := range attrs {
		rawLen := rtAttrHdrLen + len(a.val)
		paddedLen := nlAlign4(rawLen)
		buf := make([]byte, paddedLen)
		binary.NativeEndian.PutUint16(buf[0:2], uint16(rawLen))
		binary.NativeEndian.PutUint16(buf[2:4], a.typ)
		copy(buf[rtAttrHdrLen:], a.val)
		attrBuf = append(attrBuf, buf...)
	}

	msgLen := nlmsgHdrLen + rtMsgLen + len(attrBuf)
	msg := make([]byte, nlAlign4(msgLen))
	binary.NativeEndian.PutUint32(msg[0:4], uint32(msgLen))
	binary.NativeEndian.PutUint16(msg[4:6], rtmNewRoute)
	msg[nlmsgHdrLen+0] = family
	msg[nlmsgHdrLen+1] = dstLen
	msg[nlmsgHdrLen+4] = table
	msg[nlmsgHdrLen+7] = rtType
	copy(msg[nlmsgHdrLen+rtMsgLen:], attrBuf)
	return msg
}

func buildNetlinkCtrlMsg(msgType uint16) []byte {
	msg := make([]byte, nlmsgHdrLen)
	binary.NativeEndian.PutUint32(msg[0:4], nlmsgHdrLen)
	binary.NativeEndian.PutUint16(msg[4:6], msgType)
	return msg
}

func concatMsgs(msgs ...[]byte) []byte {
	var out []byte
	for _, m := range msgs {
		out = append(out, m...)
	}
	return out
}

func TestParseLinuxNetlinkGatewayIPs(t *testing.T) {
	t.Run("single IPv4 default gateway", func(t *testing.T) {
		rib := concatMsgs(
			buildNetlinkRouteMsg(nlAFInet, 0, rtTableMain, rtnUnicast,
				attrBytes(rtaGateway, net.IPv4(192, 168, 1, 1).To4()),
				attrUint32(rtaOif, 2),
			),
			buildNetlinkCtrlMsg(nlmsgDone),
		)

		ips, err := parseLinuxNetlinkGatewayIPs(rib)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ips) != 1 || ips[0].String() != "192.168.1.1" {
			t.Fatalf("got %v, want [192.168.1.1]", ips)
		}
	})

	t.Run("sorts by priority and deduplicates", func(t *testing.T) {
		rib := concatMsgs(
			buildNetlinkRouteMsg(nlAFInet, 0, rtTableMain, rtnUnicast,
				attrBytes(rtaGateway, net.IPv4(10, 0, 0, 1).To4()),
				attrUint32(rtaPriority, 200),
				attrUint32(rtaOif, 3),
			),
			buildNetlinkRouteMsg(nlAFInet, 0, rtTableMain, rtnUnicast,
				attrBytes(rtaGateway, net.IPv4(192, 168, 8, 1).To4()),
				attrUint32(rtaPriority, 50),
				attrUint32(rtaOif, 2),
			),
			buildNetlinkRouteMsg(nlAFInet, 0, rtTableMain, rtnUnicast,
				attrBytes(rtaGateway, net.IPv4(192, 168, 8, 1).To4()),
				attrUint32(rtaPriority, 100),
				attrUint32(rtaOif, 4),
			),
			buildNetlinkCtrlMsg(nlmsgDone),
		)

		ips, err := parseLinuxNetlinkGatewayIPs(rib)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ips) != 2 || ips[0].String() != "192.168.8.1" || ips[1].String() != "10.0.0.1" {
			t.Fatalf("got %v, want [192.168.8.1 10.0.0.1]", ips)
		}
	})

	t.Run("ignores non-default non-main non-unicast and wrong family", func(t *testing.T) {
		rib := concatMsgs(
			// Non-default prefix length
			buildNetlinkRouteMsg(nlAFInet, 24, rtTableMain, rtnUnicast,
				attrBytes(rtaGateway, net.IPv4(10, 0, 0, 2).To4()),
			),
			// Non-zero RTA_DST despite dstLen == 0
			buildNetlinkRouteMsg(nlAFInet, 0, rtTableMain, rtnUnicast,
				attrBytes(rtaDst, net.IPv4(10, 0, 0, 0).To4()),
				attrBytes(rtaGateway, net.IPv4(10, 0, 0, 3).To4()),
			),
			// Local table (255)
			buildNetlinkRouteMsg(nlAFInet, 0, 255, rtnUnicast,
				attrBytes(rtaGateway, net.IPv4(10, 0, 0, 4).To4()),
			),
			// Custom RTA_TABLE overriding rtTableMain
			buildNetlinkRouteMsg(nlAFInet, 0, rtTableUnspec, rtnUnicast,
				attrUint32(rtaTable, 100),
				attrBytes(rtaGateway, net.IPv4(10, 0, 0, 5).To4()),
			),
			// Non-unicast route type (RTN_LOCAL = 2)
			buildNetlinkRouteMsg(nlAFInet, 0, rtTableMain, 2,
				attrBytes(rtaGateway, net.IPv4(10, 0, 0, 6).To4()),
			),
			// IPv6 route in IPv4 query
			buildNetlinkRouteMsg(nlAFInet6, 0, rtTableMain, rtnUnicast,
				attrBytes(rtaGateway, net.ParseIP("fe80::1").To16()),
			),
			// Valid default route specified via RTA_TABLE = rtTableMain
			buildNetlinkRouteMsg(nlAFInet, 0, rtTableUnspec, rtnUnicast,
				attrUint32(rtaTable, rtTableMain),
				attrBytes(rtaGateway, net.IPv4(172, 16, 0, 1).To4()),
			),
			buildNetlinkCtrlMsg(nlmsgDone),
		)

		ips, err := parseLinuxNetlinkGatewayIPs(rib)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ips) != 1 || ips[0].String() != "172.16.0.1" {
			t.Fatalf("got %v, want [172.16.0.1]", ips)
		}
	})

	t.Run("no default route returns ErrNoGateway", func(t *testing.T) {
		for _, rib := range [][]byte{
			nil,
			concatMsgs(
				buildNetlinkRouteMsg(nlAFInet, 24, rtTableMain, rtnUnicast,
					attrBytes(rtaGateway, net.IPv4(192, 168, 1, 1).To4()),
				),
				buildNetlinkCtrlMsg(nlmsgDone),
			),
		} {
			_, err := parseLinuxNetlinkGatewayIPs(rib)
			var noGW *ErrNoGateway
			if !errors.As(err, &noGW) {
				t.Fatalf("got err=%v (%T), want *ErrNoGateway", err, err)
			}
		}
	})

	t.Run("malformed payloads and NLMSG_ERROR return ErrCantParse", func(t *testing.T) {
		badHdrLen := make([]byte, nlmsgHdrLen)
		binary.NativeEndian.PutUint32(badHdrLen[0:4], 4)
		binary.NativeEndian.PutUint16(badHdrLen[4:6], rtmNewRoute)

		truncRoute := make([]byte, nlmsgHdrLen+4)
		binary.NativeEndian.PutUint32(truncRoute[0:4], uint32(len(truncRoute)))
		binary.NativeEndian.PutUint16(truncRoute[4:6], rtmNewRoute)

		badAttr := buildNetlinkRouteMsg(nlAFInet, 0, rtTableMain, rtnUnicast)
		// Append a truncated attribute header and update message length.
		badAttr = append(badAttr, 0x08, 0x00, 0x05, 0x00)
		binary.NativeEndian.PutUint32(badAttr[0:4], uint32(len(badAttr)))

		cases := [][]byte{
			{1, 2, 3},
			badHdrLen,
			truncRoute,
			badAttr,
			buildNetlinkCtrlMsg(nlmsgError),
		}
		for i, rib := range cases {
			_, err := parseLinuxNetlinkGatewayIPs(rib)
			var cantParse *ErrCantParse
			if !errors.As(err, &cantParse) {
				t.Fatalf("case %d: got err=%v (%T), want *ErrCantParse", i, err, err)
			}
		}
	})
}

func TestParseLinuxNetlinkIPv6GatewayIPs(t *testing.T) {
	t.Run("single and multi priority IPv6 gateways", func(t *testing.T) {
		rib := concatMsgs(
			buildNetlinkRouteMsg(nlAFInet6, 0, rtTableMain, rtnUnicast,
				attrBytes(rtaGateway, net.IPv6zero),
				attrUint32(rtaPriority, 10),
			),
			buildNetlinkRouteMsg(nlAFInet6, 0, rtTableMain, rtnUnicast,
				attrBytes(rtaGateway, net.ParseIP("fe80::2").To16()),
				attrUint32(rtaPriority, 200),
			),
			buildNetlinkRouteMsg(nlAFInet6, 0, rtTableMain, rtnUnicast,
				attrBytes(rtaGateway, net.ParseIP("fe80::1").To16()),
				attrUint32(rtaPriority, 50),
			),
			buildNetlinkRouteMsg(nlAFInet6, 0, rtTableMain, rtnUnicast,
				attrBytes(rtaGateway, net.ParseIP("fe80::1").To16()),
				attrUint32(rtaPriority, 100),
			),
			buildNetlinkCtrlMsg(nlmsgDone),
		)

		ips, err := parseLinuxNetlinkIPv6GatewayIPs(rib)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ips) != 2 || ips[0].String() != "fe80::1" || ips[1].String() != "fe80::2" {
			t.Fatalf("got %v, want [fe80::1 fe80::2]", ips)
		}
	})

	t.Run("no IPv6 default route returns ErrNoGateway", func(t *testing.T) {
		rib := concatMsgs(
			buildNetlinkRouteMsg(nlAFInet6, 64, rtTableMain, rtnUnicast,
				attrBytes(rtaGateway, net.ParseIP("fe80::1").To16()),
			),
			buildNetlinkCtrlMsg(nlmsgDone),
		)
		_, err := parseLinuxNetlinkIPv6GatewayIPs(rib)
		var noGW *ErrNoGateway
		if !errors.As(err, &noGW) {
			t.Fatalf("got err=%v (%T), want *ErrNoGateway", err, err)
		}
	})
}

func TestParseLinuxNetlinkInterfaceIP(t *testing.T) {
	t.Run("resolves IPv4 of lowest-priority default route OIF", func(t *testing.T) {
		rib := concatMsgs(
			buildNetlinkRouteMsg(nlAFInet, 0, rtTableMain, rtnUnicast,
				attrBytes(rtaGateway, net.IPv4(10, 0, 0, 1).To4()),
				attrUint32(rtaPriority, 200),
				attrUint32(rtaOif, 9),
			),
			buildNetlinkRouteMsg(nlAFInet, 0, rtTableMain, rtnUnicast,
				attrBytes(rtaGateway, net.IPv4(192, 168, 1, 1).To4()),
				attrUint32(rtaPriority, 50),
				attrUint32(rtaOif, 2),
			),
			buildNetlinkCtrlMsg(nlmsgDone),
		)

		mockGetter := newMockinterfaceGetter(t)
		mockGetter.On("InterfaceByIndex", 2).Return(&net.Interface{Index: 2, Name: "eth0"}, nil)
		mockGetter.On("Addrs", mock.AnythingOfType("*net.Interface")).Return([]net.Addr{
			&net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)},
			&net.IPNet{IP: net.ParseIP("192.168.1.42"), Mask: net.CIDRMask(24, 32)},
		}, nil)

		ip, err := parseLinuxNetlinkInterfaceIPImpl(rib, mockGetter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ip.String() != "192.168.1.42" {
			t.Fatalf("got %s, want 192.168.1.42", ip)
		}
	})

	t.Run("missing OIF returns ErrNoGateway", func(t *testing.T) {
		rib := concatMsgs(
			buildNetlinkRouteMsg(nlAFInet, 0, rtTableMain, rtnUnicast,
				attrBytes(rtaGateway, net.IPv4(192, 168, 1, 1).To4()),
			),
			buildNetlinkCtrlMsg(nlmsgDone),
		)

		mockGetter := newMockinterfaceGetter(t)
		_, err := parseLinuxNetlinkInterfaceIPImpl(rib, mockGetter)
		var noGW *ErrNoGateway
		if !errors.As(err, &noGW) {
			t.Fatalf("got err=%v (%T), want *ErrNoGateway", err, err)
		}
	})
}

func TestParseLinuxNetlinkIPv6InterfaceIP(t *testing.T) {
	rib := concatMsgs(
		buildNetlinkRouteMsg(nlAFInet6, 0, rtTableMain, rtnUnicast,
			attrBytes(rtaGateway, net.ParseIP("fe80::1").To16()),
			attrUint32(rtaOif, 3),
		),
		buildNetlinkCtrlMsg(nlmsgDone),
	)

	t.Run("prefers global IPv6 over link-local", func(t *testing.T) {
		mockGetter := newMockinterfaceGetter(t)
		mockGetter.On("InterfaceByIndex", 3).Return(&net.Interface{Index: 3, Name: "eth0"}, nil)
		mockGetter.On("Addrs", mock.AnythingOfType("*net.Interface")).Return([]net.Addr{
			&net.IPNet{IP: net.ParseIP("192.168.1.42"), Mask: net.CIDRMask(24, 32)},
			&net.IPNet{IP: net.ParseIP("fe80::42"), Mask: net.CIDRMask(64, 128)},
			&net.IPNet{IP: net.ParseIP("2001:db8::10"), Mask: net.CIDRMask(64, 128)},
		}, nil)

		ip, err := parseLinuxNetlinkIPv6InterfaceIPImpl(rib, mockGetter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ip.String() != "2001:db8::10" {
			t.Fatalf("got %s, want 2001:db8::10", ip)
		}
	})

	t.Run("falls back to link-local IPv6", func(t *testing.T) {
		mockGetter := newMockinterfaceGetter(t)
		mockGetter.On("InterfaceByIndex", 3).Return(&net.Interface{Index: 3, Name: "eth0"}, nil)
		mockGetter.On("Addrs", mock.AnythingOfType("*net.Interface")).Return([]net.Addr{
			&net.IPNet{IP: net.ParseIP("fe80::42"), Mask: net.CIDRMask(64, 128)},
		}, nil)

		ip, err := parseLinuxNetlinkIPv6InterfaceIPImpl(rib, mockGetter)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ip.String() != "fe80::42" {
			t.Fatalf("got %s, want fe80::42", ip)
		}
	})
}
