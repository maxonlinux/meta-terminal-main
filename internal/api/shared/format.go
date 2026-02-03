package shared

import (
	"errors"
	"strings"

	"github.com/maxonlinux/meta-terminal-go/pkg/constants"
)

func ParseCategoryParam(value string) (int8, error) {
	switch strings.ToUpper(value) {
	case "SPOT":
		return constants.CATEGORY_SPOT, nil
	case "LINEAR":
		return constants.CATEGORY_LINEAR, nil
	default:
		return 0, errors.New("invalid category")
	}
}

func CategoryToString(category int8) string {
	if category == constants.CATEGORY_LINEAR {
		return "LINEAR"
	}
	return "SPOT"
}

func SideToString(side int8) string {
	if side == constants.ORDER_SIDE_SELL {
		return "SELL"
	}
	return "BUY"
}

func ParseSide(value string) (int8, error) {
	switch strings.ToUpper(value) {
	case "BUY":
		return constants.ORDER_SIDE_BUY, nil
	case "SELL":
		return constants.ORDER_SIDE_SELL, nil
	default:
		return 0, errors.New("invalid side")
	}
}

func OrderTypeToString(value int8) string {
	if value == constants.ORDER_TYPE_MARKET {
		return "MARKET"
	}
	return "LIMIT"
}

func ParseOrderType(value string) (int8, error) {
	switch strings.ToUpper(value) {
	case "LIMIT":
		return constants.ORDER_TYPE_LIMIT, nil
	case "MARKET":
		return constants.ORDER_TYPE_MARKET, nil
	default:
		return 0, errors.New("invalid order type")
	}
}

func TifToString(value int8) string {
	switch value {
	case constants.TIF_IOC:
		return "IOC"
	case constants.TIF_FOK:
		return "FOK"
	case constants.TIF_POST_ONLY:
		return "POST_ONLY"
	default:
		return "GTC"
	}
}

func ParseTif(value string) (int8, error) {
	switch strings.ToUpper(value) {
	case "IOC":
		return constants.TIF_IOC, nil
	case "FOK":
		return constants.TIF_FOK, nil
	case "POST_ONLY":
		return constants.TIF_POST_ONLY, nil
	case "GTC":
		return constants.TIF_GTC, nil
	default:
		return 0, errors.New("invalid time in force")
	}
}

func OrderStatusToString(value int8) string {
	switch value {
	case constants.ORDER_STATUS_PARTIALLY_FILLED:
		return "PARTIALLY_FILLED"
	case constants.ORDER_STATUS_FILLED:
		return "FILLED"
	case constants.ORDER_STATUS_CANCELED:
		return "CANCELED"
	case constants.ORDER_STATUS_PARTIALLY_FILLED_CANCELED:
		return "PARTIALLY_FILLED_CANCELED"
	case constants.ORDER_STATUS_UNTRIGGERED:
		return "UNTRIGGERED"
	case constants.ORDER_STATUS_TRIGGERED:
		return "TRIGGERED"
	case constants.ORDER_STATUS_DEACTIVATED:
		return "DEACTIVATED"
	default:
		return "NEW"
	}
}

func StopOrderTypeToString(value int8) string {
	switch value {
	case constants.STOP_ORDER_TYPE_TAKE_PROFIT:
		return "TakeProfit"
	case constants.STOP_ORDER_TYPE_STOP_LOSS:
		return "StopLoss"
	case constants.STOP_ORDER_TYPE_TRAILING:
		return "TrailingStop"
	case constants.STOP_ORDER_TYPE_STOP:
		return "Stop"
	default:
		return "Stop"
	}
}

func ParseStopOrderType(value string) (int8, error) {
	switch strings.ToUpper(value) {
	case "TAKEPROFIT":
		return constants.STOP_ORDER_TYPE_TAKE_PROFIT, nil
	case "STOPLOSS":
		return constants.STOP_ORDER_TYPE_STOP_LOSS, nil
	case "TRAILINGSTOP":
		return constants.STOP_ORDER_TYPE_TRAILING, nil
	case "STOP":
		return constants.STOP_ORDER_TYPE_STOP, nil
	default:
		return 0, errors.New("invalid stop order type")
	}
}

func OriginToString(value int8) string {
	if value == constants.ORDER_ORIGIN_SYSTEM {
		return "SYSTEM"
	}
	return "USER"
}
