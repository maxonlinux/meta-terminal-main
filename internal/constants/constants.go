package constants

import "errors"

// Balance errors
var ErrInsufficientBalance = errors.New("insufficient balance")

// Order validation errors
var (
	ErrNoPositionForRO        = errors.New("reduce-only order requires existing position")
	ErrROSideMismatch         = errors.New("reduce-only side doesn't match position direction")
	ErrInvalidTriggerForBuy   = errors.New("buy trigger must be below current price")
	ErrInvalidTriggerForSell  = errors.New("sell trigger must be above current price")
	ErrPostOnlyWouldMatch     = errors.New("post-only order would match immediately")
	ErrNoPriceForConditional  = errors.New("no price data for conditional order validation")
	ErrSelfMatch              = errors.New("order would match with own order")
	ErrMarketOrderRequiresTIF = errors.New("market orders must be IOC or FOK")
)

const (
	// Category
	CATEGORY_SPOT   = 0
	CATEGORY_LINEAR = 1

	// Order Type
	ORDER_TYPE_LIMIT  = 0
	ORDER_TYPE_MARKET = 1

	// Order Side
	ORDER_SIDE_BUY  = 0
	ORDER_SIDE_SELL = 1

	// SIDE_NONE is used when user has no position
	SIDE_NONE  = -1
	SIDE_LONG  = 0
	SIDE_SHORT = 1

	// TIF (Time In Force)
	TIF_GTC       = 0
	TIF_IOC       = 1
	TIF_FOK       = 2
	TIF_POST_ONLY = 3

	// Order Status
	ORDER_STATUS_NEW                       = 0
	ORDER_STATUS_PARTIALLY_FILLED          = 1
	ORDER_STATUS_FILLED                    = 2
	ORDER_STATUS_CANCELED                  = 3
	ORDER_STATUS_PARTIALLY_FILLED_CANCELED = 4
	ORDER_STATUS_UNTRIGGERED               = 5
	ORDER_STATUS_TRIGGERED                 = 6
	ORDER_STATUS_DEACTIVATED               = 7

	// Balance Buckets
	BUCKET_AVAILABLE = 0
	BUCKET_LOCKED    = 1
	BUCKET_MARGIN    = 2

	// Stop Order Types (Bybit-compatible)
	// OCO, TP, SL — все CloseOnTrigger=true, различаются только stopOrderType
	STOP_OPRDER_NONE            = -1 // No type (not stop order)
	STOP_ORDER_TYPE_NORMAL      = 0  // Standard conditional order (Stop)
	STOP_ORDER_TYPE_STOP        = 1  // Standard stop order
	STOP_ORDER_TYPE_TAKE_PROFIT = 2  // Take profit order
	STOP_ORDER_TYPE_STOP_LOSS   = 3  // Stop loss order

	// Position Mode
	POSITION_MODE_ONE_WAY = 0 // One-Way Mode: can only hold one direction at a time
	POSITION_MODE_HEDGE   = 1 // Hedge Mode: can hold both long and short simultaneously

	// Margin Configuration
	MM_RATIO         = 0.1 // Maintenance Margin Ratio = 10%
	DEFAULT_LEVERAGE = 2   // Default leverage if not specified

	// Persistence
	BATCH_FLUSH_INTERVAL_MS = 5000 // Flush to DB every 5 seconds
	BATCH_FLUSH_SIZE        = 1000 // Or every 1000 records
	OUTBOX_RETENTION_DAYS   = 30   // Delete processed outbox records after 30 days
)
