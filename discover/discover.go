package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/grandcat/zeroconf"
)

type Notifee interface {
	HandleNodeFound(*RegisterInfo)
}

const (
	ServiceName = "_raft._udp"
	mdnsDomain  = "local."
	raftPrefix  = "raft="
)

type RegisterInfo struct {
	ID       string `json:"id"`
	RaftAddr string `json:"raft_addr"`
	HttpAddr string `json:"http_addr"`
}

type Service struct {
	ctx          context.Context
	cancel       context.CancelFunc
	instance     string
	serviceName  string
	registerInfo string
	server       *zeroconf.Server
	notifee      Notifee
	resolverWG   sync.WaitGroup
}

// NewService creates a new service.
func NewService(ctx context.Context, instance, serviceName, raftAddr, httpAddr string, notifee Notifee) (*Service, error) {
	if serviceName == "" {
		serviceName = ServiceName
	}

	ri := &RegisterInfo{
		ID:       instance,
		RaftAddr: raftAddr,
		HttpAddr: httpAddr,
	}

	registerInfoBytes, err := json.Marshal(ri)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)
	return &Service{
		ctx:          ctx,
		cancel:       cancel,
		instance:     instance,
		serviceName:  serviceName,
		registerInfo: string(registerInfoBytes),
		notifee:      notifee,
	}, nil
}

func (s *Service) startServer() error {
	txt := fmt.Sprintf("%s%s", raftPrefix, s.registerInfo)
	server, err := zeroconf.Register(s.instance, s.serviceName, mdnsDomain, 4001, []string{txt}, nil)
	if err != nil {
		return err
	}

	s.server = server
	return nil
}

func (s *Service) startResolver() error {
	s.resolverWG.Add(2)
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return err
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		defer s.resolverWG.Done()
		for entry := range entries {
			if entry.Instance == s.instance {
				continue
			}

			for _, txt := range entry.Text {
				if strings.HasPrefix(txt, raftPrefix) {
					registerInfo := strings.TrimPrefix(txt, raftPrefix)

					unescapedData, err := strconv.Unquote(`"` + registerInfo + `"`)
					if err != nil {
						log.Fatalf("Failed to unquote registerInfo: %s", err)
					}

					ri := &RegisterInfo{}
					if err := json.Unmarshal([]byte(unescapedData), ri); err != nil {
						log.Fatalf("Failed to unmarshal registerInfo: %s", err)
					}

					s.notifee.HandleNodeFound(ri)
				}
			}

		}

	}()

	go func() {
		defer s.resolverWG.Done()
		if err := resolver.Browse(s.ctx, s.serviceName, mdnsDomain, entries); err != nil {
			log.Fatalf("Failed to browse: %s", err)
		}
	}()

	return nil
}

// Start starts the service.
func (s *Service) Start() error {
	if err := s.startServer(); err != nil {
		return err
	}

	if err := s.startResolver(); err != nil {
		return err
	}

	return nil
}

// Stop stops the service.
func (s *Service) Stop() {
	s.cancel()
	s.server.Shutdown()
	s.resolverWG.Wait()
}

// GetLocalIP returns the non loopback local IP of the host
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback the display it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}
