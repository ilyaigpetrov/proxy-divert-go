package main

import (
  "io"
  "io/ioutil"
  "net"
  "sync"
  "fmt"
  "flag"
  "strings"
  "bytes"
  "golang.org/x/net/ipv4"
  //"encoding/hex"
  "os"
  "os/signal"

  log "github.com/Sirupsen/logrus"
  "github.com/google/gopacket"
  "github.com/google/gopacket/layers"
)

const SO_ORIGINAL_DST = 80

type Proxy struct {
  from string
  fromTCP *net.TCPAddr
  done chan struct{}
  log  *log.Entry
}

func NewProxy(from string) *Proxy {

  log.SetLevel(log.InfoLevel)
  return &Proxy{
    from: from,
    done: make(chan struct{}),
    log: log.WithFields(log.Fields{
      "from": from,
    }),
  }

}

func (p *Proxy) Start() error {
  p.log.Infoln("Starting proxy")
  var err error
  p.fromTCP, err = net.ResolveTCPAddr("tcp", p.from)
  if (err != nil) {
    panic(err)
  }
  listener, err := net.ListenTCP("tcp", p.fromTCP)
  if err != nil {
    return err
  }
  go p.run(*listener)
  return nil
}

func (p *Proxy) Stop() {
  p.log.Infoln("Stopping proxy")
  if p.done == nil {
    return
  }
  close(p.done)
  p.done = nil
}


func (p *Proxy) run(listener net.TCPListener) {
  for {
    select {
    case <-p.done:
      return
    default:
      connection, err := listener.AcceptTCP()
      if connection == nil {
        p.log.WithField("err", err).Errorln("Nil connection")
        panic(err)
      }
      la := connection.LocalAddr()
      if (la == nil) {
        panic("Connection lost!")
      }
      fmt.Printf("Connection from %s\n", la.String())

      if err == nil {
        go p.handle(*connection)
      } else {
        p.log.WithField("err", err).Errorln("Error accepting conn")
      }
    }
  }
}

func isLocalIP(ip string) bool {
    if strings.HasPrefix(ip, "127.0.0.") || ip == "0.0.0.0" {
      return true
    }
    addrs, err := net.InterfaceAddrs()
    if err != nil {
        return false
    }
    for _, address := range addrs {
        // check the address type and if it is not a loopback the display it
        if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
            if ipnet.IP.To4() != nil {
              if ip == ipnet.IP.String() {
                return true
              }
            }
        }
    }
    return false
}

func (p *Proxy) handle(connection net.TCPConn) {

  defer connection.Close()
  p.log.Debugln("Handling", connection)
  defer p.log.Debugln("Done handling", connection)

  buf := make([]byte, 0, 8186) // big buffer
  tmp := make([]byte, 4096)     // using small tmo buffer for demonstrating
  for {
    n, err := connection.Read(tmp)
    if err != nil {
      if err != io.EOF {
            fmt.Println("read error:", err)
        }
        break
    }
    fmt.Println("got", n, "bytes.")
    buf = append(buf, tmp[:n]...)
    header, err := ipv4.ParseHeader(buf)
    if err != nil {
      fmt.Println("Couldn't parse packet, dropping connnection.")
      return
    }
    if header.TotalLen == 0 && len(buf) > 0 {
      fmt.Println("Buffer is not parserable!")
      return
    }
    if (header.TotalLen > len(buf)) {
      fmt.Println("Reading more up to %d\n", header.TotalLen)
      continue
    }
    packetData := buf[0:header.TotalLen]
    fmt.Printf("PACKET LEN:%d, bufLen:%d\n", header.TotalLen, len(buf))

    buf = buf[header.TotalLen:]

    fmt.Printf("Packet to %s\n", header.Dst)

    go func(){

      var src, dst string
      var srcIP, dstIP net.IP

      packet := gopacket.NewPacket(packetData, layers.LayerTypeIPv4, gopacket.Default)
      ipLayer := packet.Layer(layers.LayerTypeIPv4)
      if ipLayer != nil {
          fmt.Println("IPv4 layer detected.")
          ip, _ := ipLayer.(*layers.IPv4)

          srcIP = ip.SrcIP
          dstIP = ip.DstIP
          src = srcIP.String()
          dst = dstIP.String()
      } else {
        fmt.Println("No IP layer!")
        return
      }

      var srcPort, dstPort layers.TCPPort
      tcpLayer := packet.Layer(layers.LayerTypeTCP)
      if tcpLayer != nil {
          tcp, _ := tcpLayer.(*layers.TCP)
          srcPort = tcp.SrcPort
          dstPort = tcp.DstPort
          dst = fmt.Sprintf("%s:%d", dst, dstPort)
          src = fmt.Sprintf("%s:%d", src, srcPort)
      } else {
        fmt.Println("NOT TCP!")
        return
      }
      fmt.Printf("From %s to %s\n", src, dst)

      if isLocalIP(dstIP.String()) {
        fmt.Printf("DESTINATION IS SELF: %s\n", dstIP.String())
        return // NO SELF CONNECTIONS
      }

      addr, err := net.ResolveTCPAddr("tcp", dst)
      if err != nil {
        p.log.Errorln(err)
        return
      }
      fmt.Printf("Connection to %s\n", dst)
      remote, err := net.DialTCP("tcp", nil, addr)
      if err != nil {
        p.log.WithField("err", err).Errorln("Error dialing remote host")
        return
      }
      defer remote.Close()
      wg := &sync.WaitGroup{}
      wg.Add(2)
      go func() {
        io.Copy(remote, bytes.NewReader(tcpLayer.LayerPayload()))
        wg.Done()
      }()
      go func() {
        tcpReply, _ := ioutil.ReadAll(remote)

        options := gopacket.SerializeOptions{
          ComputeChecksums: true,
        }
        replyBuffer := gopacket.NewSerializeBuffer()
        gopacket.SerializeLayers(replyBuffer, options,
          &layers.Ethernet{},
          &layers.IPv4{
            SrcIP: dstIP,
            DstIP: srcIP,
          },
          &layers.TCP{
            SrcPort: dstPort,
            DstPort: srcPort,
          },
          gopacket.Payload(tcpReply),
        )
        connection.Write(replyBuffer.Bytes())
        wg.Done()
      }()
      wg.Wait()

    }()

  }

}

func (p *Proxy) copy(from, to net.TCPConn, wg *sync.WaitGroup) {
  defer wg.Done()
  select {
  case <-p.done:
    return
  default:
    if _, err := io.Copy(&to, &from); err != nil {
      p.log.WithField("err", err).Errorln("Error from copy")
      p.Stop()
      return
    }
  }
}

func itod(i uint) string {
        if i == 0 {
                return "0"
        }

        // Assemble decimal in reverse order.
        var b [32]byte
        bp := len(b)
        for ; i > 0; i /= 10 {
                bp--
                b[bp] = byte(i%10) + '0'
        }

        return string(b[bp:])
}

var remoteAddr *string = flag.String("r", "boom", "remote address")

func main() {

    controlC := make(chan os.Signal)
    signal.Notify(controlC, os.Interrupt)
    go func(){
      <-controlC
      fmt.Println("Exiting after Ctrl+C")
      os.Exit(0)
    }()


    flag.Parse();
    log.SetLevel(log.InfoLevel)

    NewProxy(*remoteAddr).Start()
    fmt.Println("Server started.")
    select{}
}
