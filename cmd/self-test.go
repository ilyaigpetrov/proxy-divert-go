package main

import (
  "github.com/ilyaigpetrov/proxy-divert-go"

  "fmt"
  "log"
  "os"
  "os/signal"
  //"github.com/google/gopacket"
  //"github.com/google/gopacket/layers"
)

var Error = log.New(os.Stderr,
    "ERROR: ",
    log.Ldate|log.Ltime|log.Lshortfile)

var Info = log.New(os.Stdout,
    "INFO: ",
    log.Ldate|log.Ltime|log.Lshortfile)


var injectPacket func([]byte) error

func packetHandler(packetData []byte) {

  /*
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
  */

  err := injectPacket(packetData)
  if err != nil {
    Error.Println(err)
  }

}

func main() {

  var err error
  injectPacket, err = proxyDivert.CreatePacketInjector()
  if err != nil {
    Error.Fatal(err)
  }

  unsub, err := proxyDivert.SubscribeToPacketsExcept([]string{}, packetHandler)
  if err != nil {
    Error.Fatal(err)
  }

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
