package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/oms"
	"github.com/maxonlinux/meta-terminal-go/pkg/persistence"
)

func main() {
	dataDir := flag.String("data", "", "Data directory for PebbleKV persistence")
	flag.Parse()

	if *dataDir == "" {
		log.Fatal("data directory is required")
	}

	pkv, err := persistence.OpenPebbleKV(*dataDir)
	if err != nil {
		log.Fatalf("open pebblekv: %v", err)
	}
	defer pkv.Close()

	store := oms.NewService(pkv)
	eng := engine.NewEngine(store, nil)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh

	eng.Shutdown()
}
