package history

import "errors"

var (
	errInvalidOrderClosedRecord = errors.New("history: invalid order closed record")
	errInvalidTradeRecord       = errors.New("history: invalid trade record")
	errInvalidPnLRecord         = errors.New("history: invalid pnl record")
	errUnknownRecordKind        = errors.New("history: unknown record kind")
)

func ErrInvalidOrderClosedRecord() error { return errInvalidOrderClosedRecord }
func ErrInvalidTradeRecord() error       { return errInvalidTradeRecord }
func ErrInvalidPnLRecord() error         { return errInvalidPnLRecord }
func ErrUnknownRecordKind() error        { return errUnknownRecordKind }
