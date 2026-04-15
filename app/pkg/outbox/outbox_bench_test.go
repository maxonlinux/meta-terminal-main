package outbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/events"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

type preparedRecord struct {
	recordType byte
	txID       uint64
	value      []byte
}

func BenchmarkOutboxAppendPrepared(b *testing.B) {
	dir, err := os.MkdirTemp("", "outbox-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	log, err := openAppendLog(filepath.Join(dir, "outbox.aol"), 1<<20, defaultLogFlushEvery, false, nil)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		_ = log.Close()
	})

	records := buildPreparedRecords()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := range records {
			rec := records[j]
			_, err := log.Append(rec.recordType, rec.txID, rec.value)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
	b.StopTimer()
}

func buildPreparedRecords() []preparedRecord {
	txID := uint64(1)
	order := &types.Order{
		ID:         types.OrderID(1),
		UserID:     types.UserID(42),
		Symbol:     "BTCUSDT",
		Category:   constants.CATEGORY_LINEAR,
		Side:       constants.ORDER_SIDE_BUY,
		Type:       constants.ORDER_TYPE_LIMIT,
		TIF:        constants.TIF_GTC,
		Status:     constants.ORDER_STATUS_PARTIALLY_FILLED,
		Price:      fixed.NewI(50000, 0),
		Quantity:   fixed.NewI(5, 0),
		Filled:     fixed.NewI(3, 0),
		ReduceOnly: true,
	}

	records := make([]preparedRecord, 0, 3)
	records = append(records, preparedRecord{recordType: logRecordBegin, txID: txID})
	event := events.EncodeOrderPlaced(events.OrderPlacedEvent{Order: order})
	value := append([]byte{byte(event.Type)}, event.Data...)
	records = append(records, preparedRecord{
		recordType: logRecordData,
		txID:       txID,
		value:      value,
	})
	records = append(records, preparedRecord{recordType: logRecordCommit, txID: txID})
	return records
}
