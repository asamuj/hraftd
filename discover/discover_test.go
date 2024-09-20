package discover_test

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/asamuj/hraftd/discover"
)

func TestXxx(t *testing.T) {
	d := discover.New(context.Background(), 5, "127.0.0.1:8080")

	go d.Run()

	d1 := discover.New(context.Background(), 5, "127.0.0.1:8080")

	d1.Run()

	select {}
}

var mdnsWildcardAddrIPv4 = &net.UDPAddr{
	IP:   net.ParseIP("224.0.0.251"),
	Port: 5353,
}

func TestListen(t *testing.T) {
	conn, err := net.ListenUDP("udp4", mdnsWildcardAddrIPv4)
	if err != nil {
		fmt.Println("Error setting up UDP listener:", err)
		panic(err)
	}
	defer conn.Close()

	println("Listening on", conn.LocalAddr().String())
	buf := make([]byte, 1024)

	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("Error reading from UDP:", err)
			panic(err)
		}

		if string(buf[:n]) == "hello" {
			fmt.Println("Received broadcast message from", remoteAddr)
		}

		fmt.Printf("Discovered node from %s: %d\n", remoteAddr, len(buf[:n]))
	}
}

func TestBroadcast(t *testing.T) {
	conn, err := net.DialUDP("udp4", nil, mdnsWildcardAddrIPv4)
	if err != nil {
		fmt.Println("Error setting up UDP connection:", err)
		panic(err)
	}
	defer conn.Close()

	fmt.Println("send broadcast message")
	_, err = conn.Write([]byte("hello"))
	if err != nil {
		panic(err)
	}
}
