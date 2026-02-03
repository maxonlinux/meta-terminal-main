package api

import (
	"errors"
	"strconv"
)

var (
	errInvalidLimit  = errors.New("invalid limit")
	errInvalidOffset = errors.New("invalid offset")
)

func parseLimit(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, errInvalidLimit
	}
	return parsed, nil
}

func parseOffset(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, errInvalidOffset
	}
	return parsed, nil
}
