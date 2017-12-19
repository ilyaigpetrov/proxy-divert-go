package main

import (
  "github.com/ilyaigpetrov/proxy-divert-go"

  "golang.org/x/net/ipv4"
  "path/filepath"
  "fmt"
  "log"
  "time"
  "bytes"
  "os"
  "io"
  //"encoding/hex"
  "net"
  "os/signal"
  "github.com/google/gopacket"
  "github.com/google/gopacket/layers"
)

var Error = log.New(os.Stderr,
    "ERROR: ",
    log.Ldate|log.Ltime|log.Lshortfile)

var Info = log.New(os.Stdout,
    "INFO: ",
    log.Ldate|log.Ltime|log.Lshortfile)

var remote net.Conn
var isDisconnected = make(chan struct{})
var isConnected = make(chan struct{}, 1)

var injectPacket func([]byte) error

func keepHandlingReply() {

  for {
    buf := make([]byte, 0, 8186) // big buffer
    tmp := make([]byte, 4096)     // using small tmo buffer for demonstrating
    for {
      n, err := remote.Read(tmp)
      if err != nil {
        if err != io.EOF {
          fmt.Println("read error:", err)
          isDisconnected <- struct{}{}
          <-isConnected
        }
        break
      }
      buf = append(buf, tmp[:n]...)
      header, err := ipv4.ParseHeader(buf)
      if err != nil {
        fmt.Println("Couldn't parse packet, dropping connnection.")
        break
      }
      if header.TotalLen == 0 && len(buf) > 0 {
        fmt.Println("Buffer is not parserable!")
        os.Exit(1)
      }
      if (header.TotalLen > len(buf)) {
        fmt.Println("Reading more up to %d\n", header.TotalLen)
        continue
      }
      packetData := buf[0:header.TotalLen]
      fmt.Println("Injecting packet...")
      injectPacket(packetData)

      buf = buf[header.TotalLen:]
    }
  }

}

func connectTo(serverPoint string) (ifConnected bool) {

  fmt.Printf("Dialing %s\n...", serverPoint)
  var err error
  if remote != nil {
    remote.Close()
    remote = nil
  }
  fmt.Println("REMOTE REDEFINED")
  remote, err = net.Dial("tcp", serverPoint)
  if err != nil {
    fmt.Println("Can't connect to the server!")
    return false
  }
  fmt.Println("Connected!")
  isConnected <- struct{}{}
  return true

}

func keepConnectedTo(serverPoint string) {

  if connectTo(serverPoint) == false {
    Error.Fatal("Failed to stick to this server.")
  }
  <-isConnected
  go keepHandlingReply()
  for _ = range isDisconnected {
    for {
      ok := connectTo(serverPoint)
      if ok {
        break
      }
      fmt.Println("Reconnect in 5 seconds")
      time.Sleep(time.Second * 5)
    }
  }

}

func packetHandler(packetData []byte) {
  if remote == nil {
    return
  }

  packet := gopacket.NewPacket(packetData, layers.LayerTypeIPv4, gopacket.Default)
  ipLayer := packet.Layer(layers.LayerTypeIPv4)
  if ipLayer != nil {
      fmt.Println("IPv4 layer detected.")
      ip, _ := ipLayer.(*layers.IPv4)

      fmt.Printf("From %s to %s\n", ip.SrcIP, ip.DstIP)
      fmt.Println("Protocol: ", ip.Protocol)
  } else {
    Error.Println("No IP layer!")
    return
  }

  var err error
  _, err = io.Copy(remote, bytes.NewReader(packetData))
  if err != nil {
    Error.Println(err)
    isDisconnected <- struct{}{}
  }

}

func main() {

  if len(os.Args) != 2 {
    fmt.Printf("Usage: %s proxy_address:port\n", filepath.Base(os.Args[0]))
    os.Exit(1)
  }

  serverAddr := os.Args[1]

  var unsub func()
  var err error
  unsub, injectPacket, err = proxyDivert.SubscribeToPacketsExcept([]string{serverAddr}, packetHandler)
  if err != nil {
    Error.Fatal(err)
  }

  go keepConnectedTo(serverAddr)

  fmt.Println("Traffic diverted.")

  controlC := make(chan os.Signal)
  signal.Notify(controlC, os.Interrupt)
  go func(){
    <-controlC
    unsub()
    Info.Println("Exiting after Ctrl+C")
    os.Exit(0)
  }()
  select{}

}
