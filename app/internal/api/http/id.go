package api

import (
	"strconv"
	"strings"
)

func parseInt64ID(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, strconv.ErrSyntax
	}
	return strconv.ParseInt(value, 10, 64)
}
