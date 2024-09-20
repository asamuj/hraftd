package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/pkg/errors"
	cli "github.com/urfave/cli/v2"

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

	if err := s.Open(c.String("join") == "", nodeID); err != nil {
		return errors.Wrap(err, "failed to open store")
	}

	h := httpd.New(httpAddr, s)
	if err := h.Start(); err != nil {
		return errors.Wrap(err, "failed to start HTTP service")
	}

	// If join was specified, make the join request.
	if joinAddr := c.String("join"); joinAddr != "" {
		if err := join(joinAddr, raftAddr, nodeID); err != nil {
			return errors.Wrapf(err, "failed to join node at %s", joinAddr)
		}
	}

	// We're up and running!
	log.Printf("hraftd started successfully, listening on http://%s", httpAddr)

	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt)
	<-terminate
	log.Println("hraftd exiting")

	return nil
}

func join(joinAddr, raftAddr, nodeID string) error {
	b, err := json.Marshal(map[string]string{"addr": raftAddr, "id": nodeID})
	if err != nil {
		return err
	}
	resp, err := http.Post(fmt.Sprintf("http://%s/join", joinAddr), "application-type/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
