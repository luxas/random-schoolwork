package main

import (
	"flag"
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const (
	ProtocolICMP   = 1
	MaxSendRetries = 5

	defaultMaxRTT   = 1 * time.Second
	defaultInterval = 1 * time.Second
	defaultTTL      = 64
)

var (
	maxRTTFlag   = flag.Duration("max-rtt", defaultMaxRTT, "The maximum time for a single roundtrip")
	intervalFlag = flag.Duration("interval", defaultInterval, "The interval time between sending requests")
	debugFlag    = flag.Bool("debug", false, "Whether to show debug information or not")
	listenAddr   = flag.String("listen-address", "0.0.0.0", "What IP address to listen to")
	ttl          = flag.Int("ttl", defaultTTL, "The maximum amount of network hops allowed")

	ps = &PingStats{}
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	flag.Parse()
	log.SetFlags(0)

	if len(flag.Args()) < 1 {
		return fmt.Errorf("Usage: ping [hostname or IP address]")
	}
	host := flag.Arg(0)
	if host == "" {
		return fmt.Errorf("host is empty!")
	}

	p, err := NewPinger(*intervalFlag, *maxRTTFlag, *debugFlag, *listenAddr, *ttl, handler)
	if err != nil {
		return err
	}

	ps.Start()

	c := make(chan os.Signal, 1)
	signal.Notify(c,
		// https://www.gnu.org/software/libc/manual/html_node/Termination-Signals.html
		syscall.SIGTERM, // "the normal way to politely ask a program to terminate"
		syscall.SIGINT,  // Ctrl+C
		syscall.SIGQUIT, // Ctrl-\
		syscall.SIGHUP,  // "terminal is disconnected"
	)
	var pingErr error
	go func() {
		pingErr = p.Ping(host)
		c <- syscall.SIGTERM
	}()
	<-c
	p.Stop()
	if pingErr != nil {
		return fmt.Errorf("error: %v", pingErr)
	}
	fmt.Println()
	log.Printf("--- %s ping statistics ---", host)
	s := ps.Calculate()
	divider := float64(1000000)
	log.Printf(
		"%d packets transmitted, %d received, %.0f%% packet loss, time %.0f ms",
		s.NumPackets,
		s.NumReceived,
		(float64(s.NumPackets-s.NumReceived)/float64(s.NumPackets))*100,
		float64(s.TotalDuration.Nanoseconds())/divider)

	log.Printf(
		"rtt min/avg/max/sdev = %.3f/%.3f/%.3f/%.3f ms",
		float64(s.MinRTT.Nanoseconds())/divider,
		float64(s.AvgRTT.Nanoseconds())/divider,
		float64(s.MaxRTT.Nanoseconds())/divider,
		float64(s.SdevRTT.Nanoseconds())/divider,
	)
	return nil
}

func handler(resp *response, err error) {
	log.Printf("%d bytes from %s: icmp_seq=%d ttl=%d time=%v", resp.bytelen, resp.addr.IP, resp.seq, resp.ttl, resp.rtt)
	ps.PacketReceived(resp.rtt)
}

type context struct {
	// stop is a message from the "owner" of a process to stop its execution
	stop chan bool
	// done is the "return value" from a process that's been executing
	done chan error
}

func newContext() *context {
	return &context{
		stop: make(chan bool, 1),
		done: make(chan error, 1),
	}
}

type packet struct {
	bytes []byte
	addr  net.Addr
}

type task struct {
	id       int
	seq      int
	sendTime time.Time
	addr     net.IPAddr
}

type response struct {
	addr    net.IPAddr
	rtt     time.Duration
	seq     int
	bytelen int
	ttl     int
}

type Pinger struct {
	conn       *icmp.PacketConn
	maxRTT     time.Duration
	interval   time.Duration
	mux        *sync.Mutex
	debug      bool
	recvCh     chan *packet
	mainCtx    *context
	recvCtx    *context
	processCtx *context
	ticker     *time.Ticker
	queue      map[int]task
	callback   ReceiveFunc
	seq        int
}

type ReceiveFunc func(resp *response, err error)

func NewPinger(interval, maxRTT time.Duration, debug bool, listenAddr string, ttl int, callback ReceiveFunc) (*Pinger, error) {
	conn, err := icmp.ListenPacket("ip4:icmp", listenAddr)
	if err != nil {
		return nil, err
	}

	conn.IPv4PacketConn().SetControlMessage(ipv4.FlagTTL, true)
	conn.IPv4PacketConn().SetTTL(ttl)
	return &Pinger{
		conn:       conn,
		maxRTT:     maxRTT,
		interval:   interval,
		mux:        &sync.Mutex{},
		debug:      debug,
		recvCh:     make(chan *packet),
		mainCtx:    newContext(),
		recvCtx:    newContext(),
		processCtx: newContext(),
		ticker:     nil,
		queue:      make(map[int]task),
		callback:   callback,
		seq:        0,
	}, nil
}

func (p *Pinger) Ping(host string) error {
	// Start listening for responses
	go p.receiveLoop()
	// Start processing data from the receive loop
	go p.processLoop()

	targetIP := net.IPAddr{IP: net.ParseIP(host)}

	if targetIP.IP == nil {
		targetIPs, err := net.LookupIP(host)
		if err != nil {
			return err
		}
		if len(targetIPs) == 0 {
			return fmt.Errorf("ping: cannot resolve %s: Unknown host", host)
		}

		for _, ip := range targetIPs {
			if len(ip) == net.IPv4len {
				targetIP = net.IPAddr{IP: ip}
				break
			}
		}
	}

	// Send the first ping "manually", without the timer
	if err := p.sendICMP(host, targetIP); err != nil {
		return err
	}

	p.ticker = time.NewTicker(p.interval)
	defer p.ticker.Stop()

	for {
		select {
		case sendErr := <-p.mainCtx.done:
			p.debugf("Ping(): <-p.mainCtx.done: err == %v", sendErr)
			p.recvCtx.stop <- true
			recvErr := <-p.recvCtx.done
			p.debugf("Ping(): <-p.recvCtx.done: err == %v", recvErr)
			p.processCtx.stop <- true
			log.Println("Ping process has stopped")
			// no error handling/shutdown code for the process loop
			return sendErr
		case recvErr := <-p.recvCtx.done:
			p.debugf("Ping(): <-p.recvCtx.done: err == %v", recvErr)
			return recvErr
		case processErr := <-p.processCtx.done:
			p.debugf("Ping(): <-p.processCtx.done: err == %v", processErr)
			return processErr
		case <-p.ticker.C:
			p.debugf("Run(): call sendICMP()")
			if err := p.sendICMP(host, targetIP); err != nil {
				p.mainCtx.done <- err
			}
		}
	}
}

func (p *Pinger) Stop() {
	p.mainCtx.done <- nil
}

func (p *Pinger) sendICMP(host string, target net.IPAddr) error {
	id := rand.Intn(0xffff)
	timestamp := time.Now()

	p.mux.Lock()
	seq := p.seq
	p.seq++
	p.queue[id] = task{
		id:       id,
		seq:      seq,
		sendTime: timestamp,
		addr:     target,
	}
	p.mux.Unlock()

	bytes, err := (&icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: id, Seq: seq,
			Data: timeToBytes(timestamp),
		},
	}).Marshal(nil)
	if err != nil {
		return err
	}

	if seq == 0 {
		log.Printf("PING %s (%s): %d data bytes", host, target.IP, len(bytes))
	}
	p.debugf("Send: ID %d, Seq: %d, Bytes: %d %x", id, seq, len(bytes), bytes)

	retries := 0
	for {
		if _, err := p.conn.WriteTo(bytes, &target); err != nil {
			if neterr, ok := err.(*net.OpError); ok {
				if neterr.Err == syscall.ENOBUFS {
					retries++
					if retries == MaxSendRetries {
						log.Printf("Failed to ping %s for seq=%d", target.IP, seq)
						break
					}
					continue
				}
			}
		}
		break
	}

	return nil
}

func (p *Pinger) receiveLoop() {
	for {
		select {
		case <-p.recvCtx.stop:
			p.debugf("receiveLoop(): <-p.recvCtx.stop")
			p.recvCtx.done <- nil
			return
		default:
		}

		_ = p.conn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
		buf := make([]byte, 64, 512)
		_, addr, err := p.conn.ReadFrom(buf)
		if err != nil {
			if neterr, ok := err.(*net.OpError); ok {
				if neterr.Timeout() {
					continue
				} else {
					p.debugf("receiveLoop(): OpError happen %v", err)
					p.recvCtx.done <- err
					return
				}
			}
		}

		p.debugf("Received package from addr: %s", addr.String())

		select {
		case p.recvCh <- &packet{bytes: buf, addr: addr}:
		case <-p.recvCtx.stop:
			log.Println("receiveLoop(): <-p.recvCtx.stop")
			return
		}
	}
}

func (p *Pinger) processLoop() {
	for {
		select {
		case <-p.processCtx.stop:
			p.debugf("processLoop(): <-p.processCtx.stop")
			return
		case r := <-p.recvCh:
			p.debugf("processLoop(): <-p.recvCh")
			if err := p.processRecv(r); err != nil {
				log.Printf("Error when receiving: %v\n", err)
			}
		default:
			p.mux.Lock()
			for id, t := range p.queue {
				if time.Now().After(t.sendTime.Add(p.maxRTT)) {
					ps.PacketLost()
					log.Printf("Request Timeout for icmp_seq=%d", t.seq)
					delete(p.queue, id)
				}
			}

			p.mux.Unlock()
		}
	}
}

func (p *Pinger) processRecv(recv *packet) error {
	var ipaddr net.IPAddr
	switch adr := recv.addr.(type) {
	case *net.IPAddr:
		ipaddr = *adr
	case *net.UDPAddr:
		ipaddr = net.IPAddr{IP: adr.IP, Zone: adr.Zone}
	default:
		return fmt.Errorf("Got unknown type of received packet: %v", adr)
	}

	m, err := icmp.ParseMessage(ProtocolICMP, recv.bytes)
	if err != nil {
		return fmt.Errorf("%v: %x", err, recv.bytes)
	}

	switch m.Type {
	case ipv4.ICMPTypeEchoReply:
		// no-op
	case ipv4.ICMPTypeTimeExceeded:
		// Mention we lost a packet, regardless of exit here
		defer ps.PacketLost()

		newBuf := recv.bytes[len(recv.bytes)-16:]
		origMsg, err := icmp.ParseMessage(ProtocolICMP, newBuf)
		if err != nil {
			return fmt.Errorf("From %s Time to live exceeded", ipaddr.IP)
		}
		pkt, ok := origMsg.Body.(*icmp.Echo)
		if !ok {
			return fmt.Errorf("From %s Time to live exceeded", ipaddr.IP)
		}

		// Remove the specified packet from the queue
		if _, err := p.unqueuePkt(pkt.ID); err != nil {
			return err
		}

		return fmt.Errorf("From %s icmp_seq=%d Time To Live exceeded", ipaddr.IP, pkt.Seq)
	default:
		return fmt.Errorf("invalid reply type %v", m.Type)
	}

	p.debugf("Type: %d. Code: %d. Len: %d. Payload: %x", m.Type, m.Code, len(recv.bytes), recv.bytes)
	var t task
	var rtt time.Duration
	switch pkt := m.Body.(type) {
	case *icmp.Echo:
		t, err = p.unqueuePkt(pkt.ID)
		if err != nil {
			return err
		}

		if pkt.Seq == t.seq {
			rtt = time.Since(t.sendTime)
		}

	default:
		return fmt.Errorf("invalid reply body type: %v", pkt)
	}

	if ipaddr.IP.String() != t.addr.IP.String() {
		return fmt.Errorf("Did not expect packet from host: %v", ipaddr.String())
	}

	if p.callback != nil {
		p.callback(&response{
			addr:    ipaddr,
			rtt:     rtt,
			seq:     t.seq,
			bytelen: len(recv.bytes),
			ttl:     0,
		}, nil)
	}

	return nil
}

func (p *Pinger) unqueuePkt(id int) (task, error) {
	p.mux.Lock()
	defer p.mux.Unlock()

	t, ok := p.queue[id]
	if !ok {
		p.mux.Unlock()
		return task{}, fmt.Errorf("Invalid ID: didn't send any request with id %v", id)
	}

	delete(p.queue, id)

	return t, nil
}

func (p *Pinger) debugf(format string, v ...interface{}) {
	if p.debug {
		log.Printf(format, v...)
	}
}

func timeToBytes(t time.Time) []byte {
	return big.NewInt(t.UnixNano()).Bytes()
}
