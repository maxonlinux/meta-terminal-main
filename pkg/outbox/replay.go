package outbox

import (
	"bufio"
	"io"
	"os"
	"path/filepath"

	"github.com/maxonlinux/meta-terminal-go/pkg/events"
)

func IterateEvents(dir string, fn func(events.Event) bool) error {
	path := filepath.Join(dir, "outbox.aol")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	reader := bufio.NewReaderSize(file, 1<<20)
	pending := make(map[uint64][]logRecord)
	for {
		rec, _, err := readLogRecord(reader)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch rec.recordType {
		case logRecordBegin:
			if _, ok := pending[rec.txID]; !ok {
				pending[rec.txID] = nil
			}
		case logRecordData:
			pending[rec.txID] = append(pending[rec.txID], rec)
		case logRecordCommit:
			records := pending[rec.txID]
			delete(pending, rec.txID)
			for i := range records {
				value := records[i].value
				if len(value) == 0 {
					continue
				}
				ev := events.Event{Type: events.Type(value[0]), Data: value[1:]}
				if !fn(ev) {
					return nil
				}
			}
		case logRecordAbort:
			delete(pending, rec.txID)
		}
	}
	return nil
}
