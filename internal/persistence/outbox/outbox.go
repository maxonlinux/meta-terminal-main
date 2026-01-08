package outbox

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

type Event struct {
	Type string
	Ts   int64
	Data interface{}
}

type Outbox struct {
	file   *os.File
	writer *bufio.Writer
	path   string
}

func New(path string, batchSize, flushDuration int) (*Outbox, error) {
	os.MkdirAll(path, 0755)
	f, err := os.OpenFile(path+"/outbox.jsonl", os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &Outbox{
		file:   f,
		writer: bufio.NewWriter(f),
		path:   path,
	}, nil
}

func (o *Outbox) Enqueue(eventType string, data interface{}) error {
	event := Event{
		Type: eventType,
		Ts:   time.Now().UnixMilli(),
		Data: data,
	}
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = o.writer.Write(b)
	if err != nil {
		return err
	}
	_, err = o.writer.WriteString("\n")
	return err
}

func (o *Outbox) Flush() error {
	return o.writer.Flush()
}

func (o *Outbox) Close() error {
	o.writer.Flush()
	return o.file.Close()
}
