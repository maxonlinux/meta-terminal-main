package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/maxonlinux/meta-terminal-go/internal/engine"
	"github.com/maxonlinux/meta-terminal-go/internal/mm"
	"github.com/maxonlinux/meta-terminal-go/internal/persistence"
	"github.com/maxonlinux/meta-terminal-go/internal/registry"
	"github.com/maxonlinux/meta-terminal-go/pkg/outbox"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

func main() {
	workdir, err := os.MkdirTemp("", "mm-outbox-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(workdir)

	reg := registry.New()
	for i := 0; i < 500; i++ {
		symbol := fmt.Sprintf("ASSET%03dUSDT", i)
		reg.SetInstrument(symbol, &types.Instrument{
			Symbol:     symbol,
			BaseAsset:  fmt.Sprintf("ASSET%03d", i),
			QuoteAsset: "USDT",
			MinQty:     types.Quantity(fixed.NewI(1, 0)),
			MaxQty:     types.Quantity(fixed.NewI(1000000, 0)),
			MinPrice:   types.Price(fixed.NewI(1, 0)),
			MaxPrice:   types.Price(fixed.NewI(1000000, 0)),
			TickSize:   types.Price(fixed.NewI(1, 0)),
			LotSize:    types.Quantity(fixed.NewI(1, 0)),
		})
		reg.SetPrice(symbol, registry.PriceTick{Price: types.Price(fixed.NewI(int64(100+i), 0))})
	}

	store, err := persistence.Open(workdir, reg)
	if err != nil {
		panic(err)
	}
	defer store.Close()

	batchSink := outbox.NewBatchSink(store, outbox.BatchOptions{
		BatchSize:  1000,
		FlushEvery: 100 * time.Millisecond,
	})
	defer batchSink.Stop()

	ob, err := outbox.OpenWithOptions(workdir, outbox.Options{EventSink: batchSink, SegmentSize: 16 << 20})
	if err != nil {
		panic(err)
	}
	defer ob.Close()

	eng, err := engine.NewEngine(ob, reg, nil)
	if err != nil {
		panic(err)
	}

	mmaker := mm.New(eng, reg, mm.Config{Levels: 15, CancelPercent: 0, SkipPercent: 0, MinBalance: 5000000})
	ob.Start()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mmaker.Start(ctx)

	logPath := filepath.Join(workdir, "outbox.aol")
	tailPath := filepath.Join(workdir, "outbox.bin")
	startSize := fileSize(logPath)
	start := time.Now()

	iterations := 12
	for i := 0; i < iterations; i++ {
		for j := 0; j < 500; j++ {
			symbol := fmt.Sprintf("ASSET%03dUSDT", j)
			price := types.Price(fixed.NewI(int64(101+i+j), 0))
			eng.OnPriceTick(symbol, price)
			mmaker.OnPriceTick(symbol, price)
		}
		fmt.Printf("tick %d size=%d tail=%d\n", i, fileSize(logPath), readTail(tailPath))
		time.Sleep(5 * time.Second)
	}

	cancel()
	time.Sleep(2 * time.Second)
	endSize := fileSize(logPath)
	elapsed := time.Since(start)

	fmt.Printf("outbox.aol size: %d -> %d bytes in %s\n", startSize, endSize, elapsed)
	if elapsed.Seconds() > 0 {
		fmt.Printf("approx growth: %.2f MB/min\n", float64(endSize-startSize)/1024.0/1024.0*(60.0/elapsed.Seconds()))
	}
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func readTail(path string) int64 {
	data, err := os.ReadFile(path)
	if err != nil || len(data) < 8 {
		return 0
	}
	return int64(data[0])<<56 | int64(data[1])<<48 | int64(data[2])<<40 | int64(data[3])<<32 | int64(data[4])<<24 | int64(data[5])<<16 | int64(data[6])<<8 | int64(data[7])
}
