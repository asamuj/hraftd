// Package httpd provides the HTTP server for accessing the distributed key-value store.
// It also provides the endpoint for other nodes to join an existing cluster.
package httpd

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Store is the interface Raft-backed key-value stores must implement.
type Store interface {
	// Get returns the value for the given key.
	Get(key string) (string, error)

	// Set sets the value for the given key, via distributed consensus.
	Set(key, value string) error

	// Delete removes the given key, via distributed consensus.
	Delete(key string) error

	// Join joins the node, identitifed by nodeID and reachable at addr, to the cluster.
	Join(nodeID string, addr string) error
}

// Service provides HTTP service.
type Service struct {
	addr   string
	engine *gin.Engine

	store Store
}

// New returns an uninitialized HTTP service.
func New(addr string, store Store) *Service {
	engine := gin.Default()

	service := &Service{
		addr:  addr,
		store: store,
	}

	engine.GET("/key/:key", service.get)
	engine.POST("/key", service.set)
	engine.DELETE("/key/:key", service.delete)

	service.engine = engine
	return service
}

func (s *Service) set(c *gin.Context) {
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
	go func() {
		if err := s.engine.Run(s.addr); err != nil {
			log.Fatalln(err)
		}
	}()

	return nil
}

// Addr returns the address on which the Service is listening
func (s *Service) Addr() string {
	return s.addr
}
