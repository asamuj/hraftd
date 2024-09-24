package main

import (
	"log"
	"os"
	"os/signal"

	"github.com/pkg/errors"
	cli "github.com/urfave/cli/v2"

	"github.com/asamuj/hraftd/discover"
	"github.com/asamuj/hraftd/server"
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
			&cli.BoolFlag{
				Name:  "bootstrap",
				Usage: "Bootstrap the cluster",
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
	ctx := c.Context
	raftAddr := c.String("raddr")
	httpAddr := c.String("haddr")
	nodeID := c.String("id")
	if nodeID == "" {
		nodeID = raftAddr
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
	if err := s.Open(c.Bool("bootstrap"), nodeID); err != nil {
		return errors.Wrap(err, "failed to open store")
	}

	ser := server.New(ctx, s, raftAddr, httpAddr)
	ser.Start()

	discoverService, err := discover.NewService(ctx, nodeID, "xlfs.tcp", raftAddr, httpAddr, ser)
	if err != nil {
		return errors.Wrap(err, "failed to create discover service")
	}
	if err := discoverService.Start(); err != nil {
		return errors.Wrap(err, "failed to start discover service")
	}

	log.Printf("hraftd started successfully, listening on http://%s", raftAddr)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig

	discoverService.Stop()
	ser.Stop()
	log.Println("hraftd exiting")

	return nil
}
