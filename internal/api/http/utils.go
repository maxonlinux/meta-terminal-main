package httpapi

import (
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/anomalyco/meta-terminal-go/internal/constants"
)

var errBadRequest = errors.New("bad request")

func parseCategory(value string) (int8, error) {
	switch strings.ToUpper(value) {
	case "SPOT":
		return constants.CATEGORY_SPOT, nil
	case "LINEAR":
		return constants.CATEGORY_LINEAR, nil
	default:
		return 0, errBadRequest
	}
}

func parseSide(value string) (int8, error) {
	switch strings.ToUpper(value) {
	case "BUY":
		return constants.ORDER_SIDE_BUY, nil
	case "SELL":
		return constants.ORDER_SIDE_SELL, nil
	default:
		return 0, errBadRequest
	}
}

func parseType(value string) (int8, error) {
	switch strings.ToUpper(value) {
	case "LIMIT":
		return constants.ORDER_TYPE_LIMIT, nil
	case "MARKET":
		return constants.ORDER_TYPE_MARKET, nil
	default:
		return 0, errBadRequest
	}
}

func parseTIF(value string) (int8, error) {
	switch strings.ToUpper(value) {
	case "GTC":
		return constants.TIF_GTC, nil
	case "IOC":
		return constants.TIF_IOC, nil
	case "FOK":
		return constants.TIF_FOK, nil
	case "POST_ONLY":
		return constants.TIF_POST_ONLY, nil
	default:
		return 0, errBadRequest
	}
}

func parseUint64(value string) (uint64, error) {
	return strconv.ParseUint(value, 10, 64)
}

func parseInt64(value string) (int64, error) {
	if value == "" {
		return 0, errBadRequest
	}
	bigVal, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return 0, errBadRequest
	}
	if bigVal.BitLen() > 63 {
		return 0, errBadRequest
	}
	return bigVal.Int64(), nil
}

type flexibleNumber string

func (f *flexibleNumber) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*f = flexibleNumber(s)
		return nil
	}
	*f = flexibleNumber(string(data))
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
