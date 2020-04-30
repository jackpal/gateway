package main

import (
	"fmt"

	"github.com/jackpal/gateway"
	"github.com/opentracing/opentracing-go/log"
)

func main() {
	ip, err := gateway.Get()
	if err != nil {
		log.Error(err)
	}
	fmt.Println(ip)
}
