package constants

import "errors"

const (
	CATEGORY_SPOT   = 0
	CATEGORY_LINEAR = 1

	ORDER_TYPE_LIMIT  = 0
	ORDER_TYPE_MARKET = 1

	ORDER_SIDE_BUY  = 0
	ORDER_SIDE_SELL = 1

	TIF_GTC       = 0
	TIF_IOC       = 1
	TIF_FOK       = 2
	TIF_POST_ONLY = 3

	ORDER_STATUS_NEW                       = 0
	ORDER_STATUS_PARTIALLY_FILLED          = 1
	ORDER_STATUS_FILLED                    = 2
	ORDER_STATUS_CANCELED                  = 3
	ORDER_STATUS_PARTIALLY_FILLED_CANCELED = 4
	ORDER_STATUS_UNTRIGGERED               = 5
	ORDER_STATUS_TRIGGERED                 = 6
	ORDER_STATUS_DEACTIVATED               = 7

	BUCKET_AVAILABLE = 0
	BUCKET_LOCKED    = 1
	BUCKET_MARGIN    = 2

	DEFAULT_LEVERAGE = 2
	MM_RATIO         = 0.1

	STOP_ORDER_TYPE_NORMAL      = 0
	STOP_ORDER_TYPE_STOP        = 1
	STOP_ORDER_TYPE_TAKE_PROFIT = 2
	STOP_ORDER_TYPE_STOP_LOSS   = 3
	STOP_ORDER_TYPE_TRAILING    = 4

	OMS_SHARD_COUNT = 256

	// TradeBufferSize defines the rolling trade buffer size.
	TRADE_BUFFER_SIZE = 50
	// EngineCommandQueueSize defines the engine command channel size.
	ENGINE_COMMAND_QUEUE_SIZE = 1000

	FUNDING_TYPE_DEPOSIT    = "DEPOSIT"
	FUNDING_TYPE_WITHDRAWAL = "WITHDRAWAL"

	FUNDING_STATUS_PENDING   = "PENDING"
	FUNDING_STATUS_COMPLETED = "COMPLETED"
	FUNDING_STATUS_CANCELED  = "CANCELED"

	FUNDING_CREATED_BY_USER     = "USER"
	FUNDING_CREATED_BY_ADMIN    = "ADMIN"
	FUNDING_CREATED_BY_PLATFORM = "PLATFORM"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	// ErrInvalidCategory guards market isolation between SPOT and LINEAR.
	ErrInvalidCategory = errors.New("invalid category: must be SPOT or LINEAR")
	// ErrInvalidTIF guards unsupported time-in-force values.
	ErrInvalidTIF               = errors.New("invalid time-in-force")
	ErrInvalidQuantity          = errors.New("invalid quantity")
	ErrNoPositionForRO          = errors.New("reduce-only requires existing position")
	ErrROSideMismatch           = errors.New("reduce-only side doesn't match position")
	ErrInvalidTriggerForBuy     = errors.New("buy trigger must be below current price")
	ErrInvalidTriggerForSell    = errors.New("sell trigger must be above current price")
	ErrPostOnlyWouldMatch       = errors.New("post-only order would match immediately")
	ErrMarketOrderRequiresTIF   = errors.New("market orders must be IOC or FOK")
	ErrReduceOnlyExceedsPos     = errors.New("reduce-only exceeds position size")
	ErrConditionalSpot          = errors.New("conditional orders not allowed for SPOT")
	ErrReduceOnlySpot           = errors.New("reduce-only not allowed for SPOT")
	ErrFOKInsufficientLiquidity = errors.New("FOK: insufficient liquidity in orderbook")
	// ErrLeverageTooHigh indicates leverage would trigger immediate liquidation.
	ErrLeverageTooHigh = errors.New("leverage would cause immediate liquidation")
	// ErrPriceUnavailable indicates required pricing data is missing.
	ErrPriceUnavailable = errors.New("price unavailable")
)
