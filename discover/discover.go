package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/grandcat/zeroconf"
)

type Notifee interface {
	HandleNodeFound(string, string)
}

const (
	ServiceName = "_raft._udp"
	mdnsDomain  = "local."
	raftPrefix  = "raft="
)

type RegisterInfo struct {
	ID   string `json:"id"`
	Addr string `json:"addr"`
}

type Service struct {
	ctx          context.Context
	cancel       context.CancelFunc
	instance     string
	serviceName  string
	registerInfo string
	server       *zeroconf.Server
	notifee      Notifee
}

// NewService creates a new service.
func NewService(ctx context.Context, instance, serviceName, listenAddress string, notifee Notifee) (*Service, error) {
	if serviceName == "" {
		serviceName = ServiceName
	}

	ri := &RegisterInfo{
		ID:   instance,
		Addr: listenAddress,
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

func (s *Service) startBrowser() error {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return err
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func() {
		for entry := range entries {
			log.Printf("local instance: %s, remote instance: %s", s.instance, entry.Instance)
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

					ri := RegisterInfo{}
					if err := json.Unmarshal([]byte(unescapedData), &ri); err != nil {
						log.Fatalf("Failed to unmarshal registerInfo: %s", err)
					}

					log.Printf("Found node %s at %s\n", ri.ID, ri.Addr)
					s.notifee.HandleNodeFound(ri.ID, ri.Addr)
				}
			}

		}

	}()

	if err := resolver.Browse(s.ctx, s.serviceName, mdnsDomain, entries); err != nil {
		return err
	}

	return nil
}

// Start starts the service.
func (s *Service) Start() {
	if err := s.startServer(); err != nil {
		log.Fatalf("Failed to start server: %s", err)
	}

	go func() {
		if err := s.startBrowser(); err != nil {
			log.Fatalf("Failed to browse: %s", err)
		}
	}()
}

// Stop stops the service.
func (s *Service) Stop() {
	s.cancel()
	s.server.Shutdown()
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
