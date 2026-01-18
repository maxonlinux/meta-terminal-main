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
	snapshotPath := flag.String("snapshot", "", "Snapshot directory to restore on startup")
	snapshotOut := flag.String("snapshot-out", "", "Snapshot directory to write on shutdown")
	snapshotSeq := flag.Uint64("snapshot-seq", 0, "Last sequence to store in snapshot")
	flag.Parse()

	store := oms.NewService()
	eng := engine.NewEngine(store, nil, nil)

	// Restore engine state from snapshot on startup.
	if *snapshotPath != "" {
		if _, err := persistence.LoadSnapshot(*snapshotPath, eng); err != nil {
			log.Fatalf("restore snapshot: %v", err)
		}
	}

	// Placeholder server loop: wait for shutdown to capture snapshot.
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh

	// Write a snapshot before shutting down if requested.
	if *snapshotOut != "" {
		if err := persistence.SaveSnapshot(*snapshotOut, *snapshotSeq, eng); err != nil {
			log.Fatalf("snapshot write: %v", err)
		}
	}

	eng.Shutdown()
}
