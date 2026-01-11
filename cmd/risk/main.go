package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/anomalyco/meta-terminal-go/internal/config"
	"github.com/anomalyco/meta-terminal-go/internal/messaging"
	"github.com/anomalyco/meta-terminal-go/internal/risk"
)

func main() {
	env := config.Load()

	omsClient, err := messaging.NewOMSClient(messaging.Config{
		URL:          env.NATSURL,
		StreamPrefix: env.StreamPrefix,
	})
	if err != nil {
		log.Fatalf("oms client init: %v", err)
	}
	defer omsClient.Close()

	svc, err := risk.New(risk.Config{
		NATSURL:      env.NATSURL,
		StreamPrefix: env.StreamPrefix,
	}, nil, omsClient)
	if err != nil {
		log.Fatalf("risk init: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := svc.Start(ctx); err != nil {
		log.Fatalf("risk start: %v", err)
	}

	<-ctx.Done()
	svc.Stop()
}
