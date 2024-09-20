package discover

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"
)

var mdnsWildcardAddrIPv4 = &net.UDPAddr{
	IP:   net.ParseIP("224.0.0.251"),
	Port: 5353,
}

type discover struct {
	ctx      context.Context
	cancel   context.CancelFunc
	interval uint64
	addr     string
}

func New(ctx context.Context, internal uint64, addr string) *discover {
	ctx, cancel := context.WithCancel(ctx)
	d := &discover{
		ctx:      ctx,
		cancel:   cancel,
		interval: internal,
		addr:     addr,
	}

	go d.Run()
	go d.listen()

	return d
}

func (d *discover) Run() {
	ticker := time.NewTicker(time.Second * time.Duration(time.Second))
	defer ticker.Stop()

	if err := d.broadcast(); err != nil {
		log.Printf("Initail broadcast message failed, err: %s\n", err.Error())
	}

	for {
		select {
		case <-ticker.C:
			if err := d.broadcast(); err != nil {
				log.Printf("Failed to broadcast message, err: %s\n", err.Error())
			}
		case <-d.ctx.Done():
			return
		}
	}
}

func (d *discover) Stop() {
	d.cancel()
}

func (d *discover) broadcast() error {
	conn, err := net.DialUDP("udp4", nil, mdnsWildcardAddrIPv4)
	if err != nil {
		fmt.Println("Error setting up UDP connection:", err)
		return err
	}
	defer conn.Close()

	fmt.Println("send broadcast message")
	conn.Write([]byte("hello"))

	return nil
}

func (d *discover) listen() error {
	conn, err := net.ListenUDP("udp4", mdnsWildcardAddrIPv4)
	if err != nil {
		fmt.Println("Error setting up UDP listener:", err)
		return err
	}
	defer conn.Close()

	buf := make([]byte, 1024)

	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Error reading from UDP:", err)
			return err
		}

		if string(buf[:n]) == "hello" {
			fmt.Println("Received broadcast message from", remoteAddr)
		}

		fmt.Printf("Discovered node from %s: %d\n", remoteAddr, len(buf[:n]))
	}
}
