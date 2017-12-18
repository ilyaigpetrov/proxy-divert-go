package main

import (
  "github.com/ilyaigpetrov/proxy-divert-go"

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
    Error.Fatal("Failed to stick to this server.")
  }
  for _ = range isDisconnected {
    if remote != nil {
      remote.Close()
    }
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

  unsub, err := proxyDivert.SubscribeToPacketsExcept([]string{serverAddr}, packetHandler)
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
