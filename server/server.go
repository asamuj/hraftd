package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/asamuj/hraftd/discover"
	"github.com/asamuj/hraftd/store"
	"github.com/gin-gonic/gin"
	"github.com/hashicorp/raft"
)

type Service struct {
	ctx    context.Context
	cancel context.CancelFunc
	store  *store.Store
	raddr  string // raft bind address
	hAddr  string // http bind address
	peerCh chan *discover.RegisterInfo
	wg     sync.WaitGroup
	server *http.Server

	clients map[string]string
}

func New(ctx context.Context, store *store.Store, raddr, hAddr string) *Service {
	ctx, cancel := context.WithCancel(ctx)
	clients := make(map[string]string)
	clients[raddr] = hAddr
	ser := &Service{
		ctx:     ctx,
		cancel:  cancel,
		store:   store,
		raddr:   raddr,
		hAddr:   hAddr,
		peerCh:  make(chan *discover.RegisterInfo, 10),
		clients: clients,
	}

	ser.server = ser.createHTTPServer()
	return ser
}

func (s *Service) HandleNodeFound(ri *discover.RegisterInfo) {
	s.peerCh <- ri
}

func (s *Service) run() {
	for {
		select {
		case ri := <-s.peerCh:
			for _, ser := range s.store.Raft.GetConfiguration().Configuration().Servers {
				if ser.Address == raft.ServerAddress(s.raddr) {
					break
				}
			}

			if err := s.store.Join(ri.ID, ri.RaftAddr); err != nil {
				s.peerCh <- ri
				time.Sleep(5 * time.Second)
				continue
			}

			log.Printf("node joined: %s", ri.ID)
			s.clients[ri.RaftAddr] = ri.HttpAddr

		case <-s.ctx.Done():
			log.Println("http server exiting")
			return
		}
	}
}

// New returns an uninitialized HTTP service.
func (s *Service) createHTTPServer() *http.Server {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.Default()

	engine.GET("/key/:key", s.get)
	engine.POST("/key", s.set)
	engine.DELETE("/key/:key", s.delete)

	return &http.Server{
		Addr:    s.hAddr,
		Handler: engine,
	}
}

func (s *Service) set(c *gin.Context) {
	if s.store.Raft.Leader() == raft.ServerAddress(s.raddr) {
		m := map[string]string{}
		if err := json.NewDecoder(c.Request.Body).Decode(&m); err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}

		for k, v := range m {
			if err := s.store.Set(k, v); err != nil {
				c.AbortWithError(http.StatusBadRequest, err)
				return
			}
		}
		return
	}

	if err := forward(s.clients[string(s.store.Raft.Leader())], c); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
}

func (s *Service) get(c *gin.Context) {
	k := c.Params.ByName("key")
	v, err := s.store.Get(k)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	b, err := json.Marshal(map[string]string{k: v})
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	_, err = c.Writer.Write(b)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
}

func (s *Service) delete(c *gin.Context) {
	k := c.Params.ByName("key")
	if err := s.store.Delete(k); err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
}

// Start starts the service.
func (s *Service) Start() error {
	s.wg.Add(2)
	go func() {
		defer s.wg.Done()
		s.run()
	}()

	go func() {
		defer s.wg.Done()
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	return nil
}

func (s *Service) Stop() {
	s.cancel()
	if err := s.server.Shutdown(s.ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	s.wg.Wait()
}
