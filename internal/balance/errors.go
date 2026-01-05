package balance

import "errors"

var ErrInsufficientBalance = errors.New("insufficient balance")
var ErrBalanceNotFound = errors.New("balance not found")
var ErrInvalidUnlockAmount = errors.New("invalid unlock amount")
var ErrInvalidMarginAmount = errors.New("invalid margin amount")
