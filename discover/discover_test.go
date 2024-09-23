package discover_test

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/asamuj/hraftd/discover"
	"github.com/stretchr/testify/require"
)

type notifee struct {
	nodes chan discover.RegisterInfo
}

func (n *notifee) HandleNodeFound(id string, addr string) {
	registerInfo := discover.RegisterInfo{
		ID:   id,
		Addr: addr,
	}
	n.nodes <- registerInfo
}

func TestMdns(t *testing.T) {
	n := &notifee{
		nodes: make(chan discover.RegisterInfo),
	}

	s, err := discover.NewService(context.Background(), "0", "xlfs.tcp", "127.0.0.1:9999", n)
	if err != nil {
		t.Fatalf("failed to create service: %s", err)
	}

	s.Start()

	count := 3
	listenAddresses := make([]string, count)
	for i := 0; i < count; i++ {
		port := 10000 + i
		listenAddresses[i] = fmt.Sprintf("%s:%d", discover.GetLocalIP(), port)
		s, err := discover.NewService(context.Background(), strconv.Itoa(i+1), "xlfs.tcp", listenAddresses[i], &notifee{})
		if err != nil {
			t.Fatalf("failed to create service: %s", err)
		}

		s.Start()
	}

	wg := sync.WaitGroup{}
	wg.Add(count)
	go func() {
		for node := range n.nodes {
			wg.Done()

			require.Contains(t, listenAddresses, node.Addr)
		}
	}()
	wg.Wait()
}
