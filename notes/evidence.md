# Real world test outputs

## Test code

```go
package main

import (
	"fmt"
	"lib/gateway"
	"runtime"
)

func main() {

	fmt.Printf("OS: %s\n", runtime.GOOS)

	gw, err := gateway.DiscoverGateway()

	if err == nil {
		fmt.Printf("Gateway: %v\n", gw)
	} else {
		fmt.Printf("Gateway error: %s", err)
	}

	iface, err := gateway.DiscoverInterface()

	if err == nil {
		fmt.Printf("Iface: %v\n", iface)
	} else {
		fmt.Printf("Iface error: %s", err)
	}

}
```

## Darwin

I don't have a Mac, nor am I prepared to pay $26 to spin one up in AWS (minimum charge 24 hours), but since it's using the same code path as the BSDs below and the unit tests work, I expect it to work.

## Solaris

AWS doesn't have any Solaris instances I can find, however the same goes for unit tests as for Darwin.

## FreeBSD

```
ec2-user@freebsd:~/src/gateway-test $ go run test.go
OS: freebsd
Gateway: 172.31.32.1
Iface: 172.31.32.246

ec2-user@freebsd:~/src/gateway-test $ netstat -rn
Routing tables

Internet:
Destination        Gateway            Flags     Netif Expire
default            172.31.32.1        UGS         xn0
127.0.0.1          link#1             UH          lo0
172.31.32.0/20     link#2             U           xn0
172.31.32.246      link#2             UHS         lo0

Internet6:
Destination                       Gateway                       Flags     Netif Expire
::/96                             ::1                           URS         lo0
::1                               link#1                        UHS         lo0
::ffff:0.0.0.0/96                 ::1                           URS         lo0
fe80::/10                         ::1                           URS         lo0
fe80::%lo0/64                     link#1                        U           lo0
fe80::1%lo0                       link#1                        UHS         lo0
fe80::%xn0/64                     link#2                        U           xn0
fe80::8db:f4ff:fe5b:2fd3%xn0      link#2                        UHS         lo0
ff02::/16                         ::1                           URS         lo0

ec2-user@freebsd:~/src/gateway-test $ ifconfig
lo0: flags=8049<UP,LOOPBACK,RUNNING,MULTICAST> metric 0 mtu 16384
        options=680003<RXCSUM,TXCSUM,LINKSTATE,RXCSUM_IPV6,TXCSUM_IPV6>
        inet6 ::1 prefixlen 128
        inet6 fe80::1%lo0 prefixlen 64 scopeid 0x1
        inet 127.0.0.1 netmask 0xff000000
        groups: lo
        nd6 options=21<PERFORMNUD,AUTO_LINKLOCAL>
xn0: flags=8843<UP,BROADCAST,RUNNING,SIMPLEX,MULTICAST> metric 0 mtu 9001
        options=503<RXCSUM,TXCSUM,TSO4,LRO>
        ether 0a:db:f4:5b:2f:d3
        inet6 fe80::8db:f4ff:fe5b:2fd3%xn0 prefixlen 64 scopeid 0x2
        inet 172.31.32.246 netmask 0xfffff000 broadcast 172.31.47.255
        media: Ethernet manual
        status: active
        nd6 options=23<PERFORMNUD,ACCEPT_RTADV,AUTO_LINKLOCAL>

ec2-user@freebsd:~/src/gateway-test $ uname -a
FreeBSD freebsd 13.2-STABLE FreeBSD 13.2-STABLE stable/13-n256661-e9ad6b456b02 GENERIC amd64

```

## NetBSD

```
ip-172-31-22-132$ go run test.go
OS: netbsd
Gateway: 172.31.16.1
Iface: 172.31.22.132

ip-172-31-22-132$ netstat -rn
Routing tables

Internet:
Destination        Gateway            Flags    Refs      Use    Mtu Interface
default            172.31.16.1        UG          -        -   9001  ena0
127/8              127.0.0.1          UGRS        -        -  33624  lo0
127.0.0.1          lo0                UHl         -        -  33624  lo0
172.31.16/20       link#1             UC          -        -   9001  ena0
172.31.22.132      link#1             UHl         -        -      -  lo0
172.31.16.1        06:fd:6a:57:a9:12  UHL         -        -      -  ena0

Internet6:
Destination                             Gateway                        Flags    Refs      Use    Mtu Interface
::/104                                  ::1                            UGRS        -        -  33624  lo0
::/96                                   ::1                            UGRS        -        -  33624  lo0
::1                                     lo0                            UHl         -        -  33624  lo0
::127.0.0.0/104                         ::1                            UGRS        -        -  33624  lo0
::224.0.0.0/100                         ::1                            UGRS        -        -  33624  lo0
::255.0.0.0/104                         ::1                            UGRS        -        -  33624  lo0
::ffff:0.0.0.0/96                       ::1                            UGRS        -        -  33624  lo0
2001:db8::/32                           ::1                            UGRS        -        -  33624  lo0
2002::/24                               ::1                            UGRS        -        -  33624  lo0
2002:7f00::/24                          ::1                            UGRS        -        -  33624  lo0
2002:e000::/20                          ::1                            UGRS        -        -  33624  lo0
2002:ff00::/24                          ::1                            UGRS        -        -  33624  lo0
fe80::/10                               ::1                            UGRS        -        -  33624  lo0
fe80::%ena0/64                          link#1                         UC          -        -      -  ena0
fe80::b38c:bca1:ab84:2e97               link#1                         UHl         -        -      -  lo0
fe80::%lo0/64                           fe80::1                        U           -        -      -  lo0
fe80::1                                 lo0                            UHl         -        -      -  lo0
ff01:1::/32                             link#1                         UC          -        -      -  ena0
ff01:2::/32                             ::1                            UC          -        -  33624  lo0
ff02::%ena0/32                          link#1                         UC          -        -      -  ena0
ff02::%lo0/32                           ::1                            UC          -        -  33624  lo0

ip-172-31-22-132$ ifconfig
ena0: flags=0x8843<UP,BROADCAST,RUNNING,SIMPLEX,MULTICAST> mtu 1500
        capabilities=0x200<IP4CSUM_Tx>
        enabled=0x200<IP4CSUM_Tx>
        ec_capabilities=0x4<JUMBO_MTU>
        ec_enabled=0
        address: 06:74:f1:42:54:23
        media: Ethernet autoselect
        status: active
        inet6 fe80::b38c:bca1:ab84:2e97%ena0/64 flags 0 scopeid 0x1
        inet 172.31.22.132/20 broadcast 172.31.31.255 flags 0
lo0: flags=0x8049<UP,LOOPBACK,RUNNING,MULTICAST> mtu 33624
        status: active
        inet6 ::1/128 flags 0x20<NODAD>
        inet6 fe80::1%lo0/64 flags 0 scopeid 0x2
        inet 127.0.0.1/8 flags 0

ip-172-31-22-132$ uname -a
NetBSD ip-172-31-22-132.eu-west-1.compute.internal 9.99.100 NetBSD 9.99.100 (GENERIC64) #0: Sun Oct  2 23:36:41 UTC 2022  mkrepro@mkrepro.NetBSD.org:/usr/src/sys/arch/evbarm/compile/GENERIC64 evbarm
```