package snapshot

import (
	"os"
	"path/filepath"
)

type Store struct {
	path string
}

func New(path string) (*Store, error) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, err
	}
	return &Store{path: path}, nil
}

func (s *Store) filePath() string {
	return filepath.Join(s.path, "snapshot.bin")
}

func (s *Store) Save(snap *Snapshot) error {
	file, err := os.OpenFile(s.filePath(), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	return Encode(file, snap)
}

func (s *Store) Load() (*Snapshot, error) {
	file, err := os.Open(s.filePath())
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return Decode(file)
}

func (s *Store) Exists() bool {
	_, err := os.Stat(s.filePath())
	return err == nil
}
