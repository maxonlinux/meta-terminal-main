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
	segments, err := listSegments(path)
	if err != nil {
		return err
	}
	if len(segments) == 0 {
		return nil
	}

	pending := make(map[uint64][]logRecord)
	for _, seg := range segments {
		file, err := os.Open(seg.path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		reader := bufio.NewReaderSize(file, 1<<20)
		for {
			rec, _, err := readLogRecord(reader)
			if err == io.EOF {
				break
			}
			if err != nil {
				_ = file.Close()
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
						_ = file.Close()
						return nil
					}
				}
			case logRecordAbort:
				delete(pending, rec.txID)
			}
		}
		_ = file.Close()
	}
	return nil
}
