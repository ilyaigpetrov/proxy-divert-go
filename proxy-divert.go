package proxyDivert

import (
  "net"
  "strings"
  "fmt"
  "errors"
  "github.com/clmul/go-windivert"
  "github.com/ilyaigpetrov/proxy-divert-go/vendor-local/nettest"
)

var DIVERT_NO_LOCALNETS_DST = `(
  (ip.DstAddr < 127.0.0.1 or ip.DstAddr > 127.255.255.255) and
  (ip.DstAddr < 10.0.0.0 or ip.DstAddr > 10.255.255.255) and
  (ip.DstAddr < 192.168.0.0 or ip.DstAddr > 192.168.255.255) and
  (ip.DstAddr < 172.16.0.0 or ip.DstAddr > 172.31.255.255) and
  (ip.DstAddr < 169.254.0.0 or ip.DstAddr > 169.254.255.255)
)`

var theInterface *net.Interface

func CreatePacketInjector() (func([]byte) error, error) {

  fmt.Println("CREATE INJECTOR")
  handle, err := windivert.Open("true", windivert.LayerNetwork, 1, 0)
  if err != nil {
    return nil, err
  }

  if theInterface == nil {
    rif := nettest.RoutedInterface("ip", net.FlagUp)
    if rif == nil {
      return nil, errors.New("No interface found for injecting packets.")
    }
    theInterface = rif
    fmt.Println("INTERFACE", rif)
  }

  addr := windivert.Address{
    Direction: windivert.DirectionInbound,
    IfIdx: uint32(theInterface.Index),
    SubIfIdx: 0,
  }
  fmt.Println("ADDRRRRRR", addr)
  return func(packetData []byte) error {

    _, err = handle.Send(packetData, addr)
    return err

  }, nil

}

func SubscribeToPacketsExcept(exceptions []string, packetHandler func([]byte)) (func(), func([]byte) error, error) {

  excepted := ""
  if len(exceptions) != 0 {
    filters := make([]string, 0, len(exceptions))
    for _, addr := range exceptions {
      parts := strings.Split(addr, ":")
      if len(parts) != 2 {
        return nil, nil, errors.New(fmt.Sprintf(`"%s" must be in format hostname:port`, addr))
      }
      port := parts[1]
      portless := parts[0]
      ips, err := net.LookupHost(portless)
      if err != nil {
        return nil, nil, err
      }
      ors := make([]string, 0, len(ips))
      for _, ip := range ips {
        f := fmt.Sprintf("ip.DstAddr != %s", ip)
        ors = append(ors, f)
      }
      filters = append(filters, "(" + strings.Join(ors, " or ") + ") and tcp.DstPort != " + port)
    }
    excepted = "(" + strings.Join(filters, ") and (") + ") and "
  }

  filter := "outbound and ip and tcp and (tcp.DstPort == 443 or tcp.DstPort == 80) and " +
    excepted +
    DIVERT_NO_LOCALNETS_DST;

  handle, err := windivert.Open(filter, windivert.LayerNetwork, -1, 0)
  if err != nil {
    return nil, nil, err
  }

  maxPacketSize := 9016
  packetBuffer := make([]byte, maxPacketSize)
  go func() {
    for {
      fmt.Println("Calling RECV")
      n, _, err := handle.Recv(packetBuffer)
      if err != nil {
        fmt.Println(err)
        continue
      }
      packetData := packetBuffer[:n]
      fmt.Println("Calling RECV hANdlerR")
      packetHandler(packetData)

    }
  }()

  if theInterface == nil {
    rif := nettest.RoutedInterface("ip", net.FlagUp)
    if rif == nil {
      return nil, nil, errors.New("No interface found for injecting packets.")
    }
    theInterface = rif
    fmt.Println("INTERFACE", rif)
  }

  addr := windivert.Address{
    Direction: windivert.DirectionOutbound,
    IfIdx: uint32(theInterface.Index),
    SubIfIdx: 0,
  }

  return func() {
    handle.Close()
  },
  func(packetData []byte) error {

    _, err = handle.Send(packetData, addr)
    return err

  }, nil

}
