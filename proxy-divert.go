package proxyDivert

import (
  "net"
  "strings"
  "fmt"
  "errors"
  "github.com/clmul/go-windivert"
)

var DIVERT_NO_LOCALNETS_DST = `(
  (ip.DstAddr < 127.0.0.1 or ip.DstAddr > 127.255.255.255) and
  (ip.DstAddr < 10.0.0.0 or ip.DstAddr > 10.255.255.255) and
  (ip.DstAddr < 192.168.0.0 or ip.DstAddr > 192.168.255.255) and
  (ip.DstAddr < 172.16.0.0 or ip.DstAddr > 172.31.255.255) and
  (ip.DstAddr < 169.254.0.0 or ip.DstAddr > 169.254.255.255)
)`

func SubscribeToPacketsExcept(exceptions []string, packetHandler func([]byte)) (func(), error) {

  nop := func() {}

  filters := make([]string, 0, len(exceptions))
  for _, addr := range exceptions {
    parts := strings.Split(addr, ":")
    if len(parts) != 2 {
      return nop, errors.New(fmt.Sprintf(`"%s" must be in format hostname:port`, addr))
    }
    port := parts[1]
    portless := parts[0]
    ips, err := net.LookupHost(portless)
    if err != nil {
      return nop, err
    }
    ors := make([]string, 0, len(ips))
    for _, ip := range ips {
      f := fmt.Sprintf("ip.DstAddr != %s", ip)
      ors = append(ors, f)
    }
    filters = append(filters, "(" + strings.Join(ors, " or ") + ") and tcp.DstPort != " + port)
  }
  excepted := "(" + strings.Join(filters, ") and (") + ")"

  filter := "outbound and ip and tcp and (tcp.DstPort == 443 or tcp.DstPort == 80) and " +
    excepted + " and " +
    DIVERT_NO_LOCALNETS_DST;

  handle, err := windivert.Open(filter, windivert.LayerNetwork, 0, 0)
  if err != nil {
    return nop, err
  }

  maxPacketSize := 9016
  packetBuffer := make([]byte, maxPacketSize)
  go func() {
    for {
      n, _, err := handle.Recv(packetBuffer)
      if err != nil {
        fmt.Println(err)
        continue
      }
      packetData := packetBuffer[:n]
      packetHandler(packetData)

    }
  }()
  return func() {
    handle.Close()
  }, nil

}
