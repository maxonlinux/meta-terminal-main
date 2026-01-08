package snapshot

import (
	"encoding/binary"
	"encoding/gob"
	"os"
)

type Snapshot struct {
	path string
}

func New(path string) *Snapshot {
	os.MkdirAll(path, 0755)
	return &Snapshot{path: path}
}

func (s *Snapshot) Save(state interface{}, offset int64) error {
	f, err := os.Create(s.path + "/snapshot.dat")
	if err != nil {
		return err
	}
	defer f.Close()

	if err := binary.Write(f, binary.LittleEndian, offset); err != nil {
		return err
	}
	enc := gob.NewEncoder(f)
	return enc.Encode(state)
}

func (s *Snapshot) Load() (interface{}, int64, error) {
	f, err := os.Open(s.path + "/snapshot.dat")
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	var offset int64
	if err := binary.Read(f, binary.LittleEndian, &offset); err != nil {
		return nil, 0, err
	}

	var state interface{}
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&state); err != nil {
		return nil, 0, err
	}
	return state, offset, nil
}
