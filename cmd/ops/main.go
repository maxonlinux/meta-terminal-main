package main

import (
	"flag"
	"log"
	"os"

	"github.com/maxonlinux/meta-terminal-go/pkg/persistence"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: ops <replay|snapshot|wal> [flags]")
	}

	switch os.Args[1] {
	case "replay":
		runReplay(os.Args[2:])
	case "snapshot":
		runSnapshot(os.Args[2:])
	case "wal":
		runWAL(os.Args[2:])
	default:
		log.Fatalf("unknown command: %s", os.Args[1])
	}
}

func runReplay(args []string) {
	fs := flag.NewFlagSet("replay", flag.ExitOnError)
	snapshotPath := fs.String("snapshot", "", "Path to existing snapshot directory")
	outputPath := fs.String("output", "", "Path to write merged snapshot directory")
	natsURL := fs.String("nats-url", "", "NATS server URL")
	stream := fs.String("stream", "", "JetStream stream name")
	subject := fs.String("subject", "", "JetStream subject for events")
	fs.Parse(args)

	if *snapshotPath == "" || *outputPath == "" {
		log.Fatal("snapshot and output paths are required")
	}

	cfg := persistence.DefaultNATSConfig()
	if *natsURL != "" {
		cfg.URL = *natsURL
	}
	if *stream != "" {
		cfg.Stream = *stream
	}
	if *subject != "" {
		cfg.EventSubject = *subject
	}

	store, err := persistence.OpenJetStreamStore(cfg)
	if err != nil {
		log.Fatalf("open jetstream: %v", err)
	}
	defer store.Close()

	lastSeq, err := store.StreamLastSeq()
	if err != nil {
		log.Fatalf("fetch stream last seq: %v", err)
	}

	if err := persistence.ReplayToSnapshot(*snapshotPath, *outputPath, store, lastSeq); err != nil {
		log.Fatalf("recovery failed: %v", err)
	}
}

func runSnapshot(args []string) {
	fs := flag.NewFlagSet("snapshot", flag.ExitOnError)
	outputPath := fs.String("output", "", "Path to write snapshot directory")
	natsURL := fs.String("nats-url", "", "NATS server URL")
	stream := fs.String("stream", "", "JetStream stream name")
	subject := fs.String("subject", "", "JetStream subject for events")
	fromSeq := fs.Uint64("from-seq", 1, "JetStream sequence to start replay")
	fs.Parse(args)

	if *outputPath == "" {
		log.Fatal("output path is required")
	}

	cfg := persistence.DefaultNATSConfig()
	if *natsURL != "" {
		cfg.URL = *natsURL
	}
	if *stream != "" {
		cfg.Stream = *stream
	}
	if *subject != "" {
		cfg.EventSubject = *subject
	}

	store, err := persistence.OpenJetStreamStore(cfg)
	if err != nil {
		log.Fatalf("open jetstream: %v", err)
	}
	defer store.Close()

	lastSeq, err := store.StreamLastSeq()
	if err != nil {
		log.Fatalf("fetch stream last seq: %v", err)
	}

	if err := persistence.ReplayToSnapshotFromSeq(*outputPath, *fromSeq, store, lastSeq); err != nil {
		log.Fatalf("snapshot replay failed: %v", err)
	}
}

func runWAL(args []string) {
	fs := flag.NewFlagSet("wal", flag.ExitOnError)
	snapshotPath := fs.String("snapshot", "", "Snapshot directory to load")
	walPath := fs.String("wal", "", "WAL directory containing events.wal")
	outputPath := fs.String("output", "", "Output snapshot directory")
	fs.Parse(args)

	if err := persistence.ReplayWALToSnapshot(*snapshotPath, *walPath, *outputPath); err != nil {
		log.Fatalf("wal recovery failed: %v", err)
	}
}
