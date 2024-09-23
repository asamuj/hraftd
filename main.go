package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	cli "github.com/urfave/cli/v2"

	"github.com/asamuj/hraftd/discover"
	httpd "github.com/asamuj/hraftd/http"
	"github.com/asamuj/hraftd/store"
)

func main() {
	app := cli.App{
		Name:  "hraftd",
		Usage: "A simple distributed key-value store using Raft",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "inmem",
				Usage: "Use in-memory storage for Raft",
			},
			&cli.StringFlag{
				Name:  "haddr",
				Value: "127.0.0.1:11000",
				Usage: "Set the HTTP bind address",
			},
			&cli.StringFlag{
				Name:  "raddr",
				Value: "127.0.0.1:12000",
				Usage: "Set Raft bind address",
			},
			&cli.StringFlag{
				Name:  "join",
				Usage: "Set join address, if any",
			},
			&cli.StringFlag{
				Name:  "id",
				Usage: "Node ID. If not set, same as Raft bind address",
			},
			&cli.StringFlag{
				Name:  "path",
				Usage: "Raft storage path",
			},
		},
		Action: action,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func action(c *cli.Context) error {
	raftAddr := c.String("raddr")
	httpAddr := c.String("haddr")
	nodeID := c.String("id")
	if nodeID == "" {
		nodeID = uuid.New().String()
	}

	// Ensure Raft storage exists.
	raftDir := c.String("path")
	if raftDir == "" {
		return errors.New("No Raft storage directory specified")
	}

	if err := os.MkdirAll(raftDir, 0700); err != nil {
		return errors.Wrap(err, "failed to create path for Raft storage")
	}

	s := store.New(c.Bool("inmem"), raftDir, raftAddr)
	if err := s.Open(nodeID); err != nil {
		return errors.Wrap(err, "failed to open store")
	}

	h := httpd.New(httpAddr, s)
	if err := h.Start(); err != nil {
		return errors.Wrap(err, "failed to start HTTP service")
	}

	service, err := discover.NewService(c.Context, nodeID, "xlfs.tcp", raftAddr, &notifee{store: s, raftAddr: raftAddr})
	if err != nil {
		return errors.Wrap(err, "failed to create discover service")
	}
	service.Start()

	// We're up and running!
	log.Printf("hraftd started successfully, listening on http://%s", raftAddr)

	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate

	service.Stop()
	log.Println("hraftd exiting")

	return nil
}

type notifee struct {
	raftAddr string
	store    *store.Store
}

func (n *notifee) HandleNodeFound(id, addr string) {
	if string(n.store.Raft.Leader()) == n.raftAddr {
		fmt.Println("n.store.Raft.Leader()", n.store.Raft.Leader())
		if err := n.store.Join(id, addr); err != nil {
			log.Printf("failed to join node at %s: %s", addr, err)
			return
		}

		log.Printf("joined node at %s \n", addr)
	}
}
