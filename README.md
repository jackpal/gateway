# gateway

A simple library for discovering the IP address of the default gateway.

Example:

```go
package main

import (
    "fmt"

    "github.com/jackpal/gateway"
)

func main() {
    gateway, err := gateway.DiscoverGateway()
    if err != nil {
        fmt.Println(err)
    } else {
        fmt.Println("Gateway:", gateway.String())
   }
}
```

Provides implementations for:

+ Darwin (macOS)
+ Dragonfly
+ FreeBSD
+ Linux
+ NetBSD
+ OpenBSD
+ Solaris
+ Windows

Other platforms use an implementation that always returns an error.

Pull requests for other OSs happily considered!

## Versions

### v1.0.10

+ Fix non-BSD-based builds.
### v1.0.9

+ Add go.mod and go.sum files.
+ Use "golang.org/x/net/route" to implement all BSD variants.
  + As a side effect this adds support for Dragonfly and NetBSD. 
+ Add example to README.
+ Remove broken continuous integration.

### v1.0.8

+ Add support for OpenBSD
+ Linux parser now supports gateways with an IP address of 0.0.0.0
+ Fall back to `netstat` on darwin systems if `route` fails.
+ Simplify Linux /proc/net/route parsers.
