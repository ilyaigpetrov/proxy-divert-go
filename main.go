package main

import (
  "path/filepath"
  "fmt"
  "log"
  "time"
  "os"
  "net"
  "os/signal"
  "github.com/clmul/go-windivert"
  "github.com/google/gopacket"
  "github.com/google/gopacket/layers"
)

var DIVERT_NO_LOCALNETS_DST = `(
  (ip.DstAddr < 127.0.0.1 or ip.DstAddr > 127.255.255.255) and
  (ip.DstAddr < 10.0.0.0 or ip.DstAddr > 10.255.255.255) and
  (ip.DstAddr < 192.168.0.0 or ip.DstAddr > 192.168.255.255) and
  (ip.DstAddr < 172.16.0.0 or ip.DstAddr > 172.31.255.255) and
  (ip.DstAddr < 169.254.0.0 or ip.DstAddr > 169.254.255.255)
)`

var remote net.Conn
var isDisconnected = make(chan struct{})

func connectTo(serverPoint string) (ifConnected bool) {

  fmt.Printf("Dialing %s\n...", serverPoint)
  var err error
  remote, err = net.Dial("tcp", serverPoint)
  if err != nil {
    fmt.Println("Can't connect to the server!")
    return false
  }
  fmt.Println("Connected!")
  return true

}

func keepConnectedTo(serverPoint string) {

  if connectTo(serverPoint) == false {
    //Error.Fatal("Failed to stick to this server.")
  }
  for _ = range isDisconnected {
    if remote != nil {
      remote.Close()
    }
    for {
      ok := connectTo(serverPoint)
      if !ok {
        fmt.Println("Reconnect in 5 seconds")
        time.Sleep(time.Second * 5)
      }
    }
  }

}

func main() {

  Error := log.New(os.Stderr,
    "ERROR: ",
    log.Ldate|log.Ltime|log.Lshortfile)

  Info := log.New(os.Stdout,
    "INFO: ",
    log.Ldate|log.Ltime|log.Lshortfile)

  if len(os.Args) != 2 {
    fmt.Printf("Usage: %s proxy_address:port\n", filepath.Base(os.Args[0]))
    os.Exit(1)
  }

  tcpAddr, err := net.ResolveTCPAddr("tcp", os.Args[1])
  if err != nil {
    Error.Fatal(err)
  }

  serverIp := tcpAddr.IP.String()
  serverPort := fmt.Sprintf("%d", tcpAddr.Port)
  fmt.Printf("Server addr is %s:%s\n", serverIp, serverPort)

  filter := "outbound and ip and and tcp and (tcp.DstPort == 443 or tcp.DstPort == 80) and " +
    "(ip.DstAddr != " + serverIp + " and tcp.DstPort == " + serverPort + ") and " +
    DIVERT_NO_LOCALNETS_DST;

  serverPoint := fmt.Sprintf("%s:%d", serverIp, tcpAddr.Port)
  go keepConnectedTo(serverPoint)

  select{}

  handle, err := windivert.Open(filter, windivert.LayerNetwork, 0, 0)
  if err != nil {
    Error.Fatal(err)
  }

  controlC := make(chan os.Signal)
  signal.Notify(controlC, os.Interrupt)
  go func(){
    <-controlC
    handle.Close()
    Info.Println("Exiting after Ctrl+C")
    os.Exit(0)
  }()

  maxPacketSize := 9016
  data := make([]byte, maxPacketSize)
  for {
    _, _, err := handle.Recv(data)
    if err != nil {
      Error.Println(err)
      continue
    }

    packet := gopacket.NewPacket(data, layers.LayerTypeIPv4, gopacket.Default)
    ipLayer := packet.Layer(layers.LayerTypeIPv4)
    if ipLayer != nil {
        fmt.Println("IPv4 layer detected.")
        ip, _ := ipLayer.(*layers.IPv4)

        // IP layer variables:
        // Version (Either 4 or 6)
        // IHL (IP Header Length in 32-bit words)
        // TOS, Length, Id, Flags, FragOffset, TTL, Protocol (TCP?),
        // Checksum, SrcIP, DstIP
        fmt.Printf("From %s to %s\n", ip.SrcIP, ip.DstIP)
        fmt.Println("Protocol: ", ip.Protocol)
        fmt.Println()
    }

  }

}
