package outbox

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

var errInvalidOffset = errors.New("outbox: invalid offset")

func ReadOffset(path string) (int64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return 0, errInvalidOffset
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	if v < 0 {
		return 0, errInvalidOffset
	}
	return v, nil
}

func ReadOffsetOrZero(path string) (int64, error) {
	v, err := ReadOffset(path)
	if err == nil {
		return v, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	return 0, err
}

func WriteOffset(path string, offset int64) error {
	if offset < 0 {
		return errInvalidOffset
	}
	return os.WriteFile(path, []byte(strconv.FormatInt(offset, 10)), 0o644)
}
