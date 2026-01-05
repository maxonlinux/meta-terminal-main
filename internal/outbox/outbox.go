package outbox

import "os"

type Outbox struct {
	path      string
	batchSize int
}

func New(path string, batchSize int) (*Outbox, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}

	return &Outbox{
		path:      path,
		batchSize: batchSize,
	}, nil
}

func (o *Outbox) Enqueue(events []byte) error {
	return nil
}

func (o *Outbox) Flush() error {
	return nil
}

func (o *Outbox) Close() error {
	return nil
}
