package events

import "github.com/anomalyco/meta-terminal-go/internal/id"

const (
	TypeBalanceReserved = "BALANCE_RESERVED"
	TypeBalanceReleased = "BALANCE_RELEASED"
	TypeBalanceDeducted = "BALANCE_DEDUCTED"
	TypeBalanceAdded    = "BALANCE_ADDED"

	TypePositionOpened  = "POSITION_OPENED"
	TypePositionUpdated = "POSITION_UPDATED"
	TypePositionClosed  = "POSITION_CLOSED"
)

type BalanceReserved struct {
	TxID   id.TxID   `json:"tx_id"`
	UserID id.UserID `json:"user_id"`
	Asset  string    `json:"asset"`
	Amount int64     `json:"amount"`
	Bucket int8      `json:"bucket"`
}

type BalanceDeducted struct {
	TxID   id.TxID   `json:"tx_id"`
	UserID id.UserID `json:"user_id"`
	Asset  string    `json:"asset"`
	Amount int64     `json:"amount"`
	Bucket int8      `json:"bucket"`
}

type BalanceAdded struct {
	TxID   id.TxID   `json:"tx_id"`
	UserID id.UserID `json:"user_id"`
	Asset  string    `json:"asset"`
	Amount int64     `json:"amount"`
	Bucket int8      `json:"bucket"`
}

type PositionOpened struct {
	TxID       id.TxID       `json:"tx_id"`
	UserID     id.UserID     `json:"user_id"`
	Symbol     string        `json:"symbol"`
	PositionID id.PositionID `json:"position_id"`
	Side       int8          `json:"side"`
	Size       int64         `json:"size"`
	EntryPrice int64         `json:"entry_price"`
	Leverage   int8          `json:"leverage"`
}

type PositionUpdated struct {
	TxID       id.TxID       `json:"tx_id"`
	UserID     id.UserID     `json:"user_id"`
	Symbol     string        `json:"symbol"`
	PositionID id.PositionID `json:"position_id"`
	Size       int64         `json:"size"`
	EntryPrice int64         `json:"entry_price"`
}
