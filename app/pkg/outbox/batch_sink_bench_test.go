package outbox

import (
	"testing"

	"github.com/maxonlinux/meta-terminal-go/pkg/events"
)

type noopSink struct{}

func (noopSink) Apply(_ []events.Event) error { return nil }

func BenchmarkBatchSink(b *testing.B) {
	sink := NewBatchSink(noopSink{}, BatchOptions{BatchSize: 1000})
	batch := make([]events.Event, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := sink.Apply(batch); err != nil {
			b.Fatalf("apply: %v", err)
		}
	}
}
