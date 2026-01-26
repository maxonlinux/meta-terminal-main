package outbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
	"github.com/maxonlinux/meta-terminal-go/pkg/persistence"
	"github.com/maxonlinux/meta-terminal-go/pkg/types"
	"github.com/robaho/fixed"
)

type preparedRecord struct {
	recordType byte
	txID       uint64
	key        []byte
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

	log, err := openAppendLog(filepath.Join(dir, "outbox.aol"))
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
			_, err := log.Append(rec.recordType, rec.txID, rec.key, rec.value)
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

	records := make([]preparedRecord, 0, 1+2+20)
	records = append(records, preparedRecord{recordType: logRecordBegin, txID: txID})
	records = append(records, preparedRecord{
		recordType: logRecordData,
		txID:       txID,
		key:        persistence.OrderKey(order.ID),
		value:      persistence.EncodeOrder(order),
	})

	for i := 0; i < 10; i++ {
		balance := &types.Balance{
			UserID:    types.UserID(42),
			Asset:     "USDT",
			Available: fixed.NewI(100000-int64(i), 0),
			Locked:    fixed.NewI(500+int64(i), 0),
			Margin:    fixed.NewI(200+int64(i), 0),
		}
		position := &types.Position{
			UserID:     types.UserID(42),
			Symbol:     "BTCUSDT",
			Size:       fixed.NewI(1+int64(i), 0),
			EntryPrice: fixed.NewI(50000+int64(i), 0),
			Leverage:   fixed.NewI(10, 0),
		}
		records = append(records, preparedRecord{
			recordType: logRecordData,
			txID:       txID,
			key:        persistence.BalanceKey(balance.UserID, balance.Asset),
			value:      persistence.EncodeBalance(balance),
		})
		records = append(records, preparedRecord{
			recordType: logRecordData,
			txID:       txID,
			key:        persistence.PositionKey(position.UserID, position.Symbol),
			value:      persistence.EncodePosition(position),
		})
	}
	records = append(records, preparedRecord{recordType: logRecordCommit, txID: txID})
	return records
}
