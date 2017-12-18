package main

import (
	"fmt"
	"net"
)

func main() {
	fmt.Println(net.LookupHost("127.0.0.1"))
}

