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
	nodes chan *discover.RegisterInfo
}

func (n *notifee) HandleNodeFound(ri *discover.RegisterInfo) {
	n.nodes <- ri
}

func TestMdns(t *testing.T) {
	n := &notifee{
		nodes: make(chan *discover.RegisterInfo),
	}

	s, err := discover.NewService(context.Background(), "0", "xlfs.tcp", "127.0.0.1:9999", "127.0.0.1:8888", n)
	if err != nil {
		t.Fatalf("failed to create service: %s", err)
	}

	s.Start()

	count := 3
	raftAddrs := make([]string, count)
	for i := 0; i < count; i++ {
		raftAddrs[i] = fmt.Sprintf("%s:%d", discover.GetLocalIP(), 10000+i)

		httpAddr := fmt.Sprintf("%s:%d", discover.GetLocalIP(), 10000-i)
		s, err := discover.NewService(context.Background(), strconv.Itoa(i+1), "xlfs.tcp", raftAddrs[i], httpAddr, &notifee{})
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
			require.Contains(t, raftAddrs, node.RaftAddr)
		}
	}()
	wg.Wait()
}
