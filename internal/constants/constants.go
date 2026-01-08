package constants

const (
	CATEGORY_SPOT   int8 = 0
	CATEGORY_LINEAR int8 = 1

	ORDER_TYPE_LIMIT  int8 = 0
	ORDER_TYPE_MARKET int8 = 1

	ORDER_SIDE_BUY  int8 = 0
	ORDER_SIDE_SELL int8 = 1

	TIF_GTC       int8 = 0
	TIF_IOC       int8 = 1
	TIF_FOK       int8 = 2
	TIF_POST_ONLY int8 = 3

	ORDER_STATUS_NEW                       int8 = 0
	ORDER_STATUS_PARTIALLY_FILLED          int8 = 1
	ORDER_STATUS_FILLED                    int8 = 2
	ORDER_STATUS_CANCELED                  int8 = 3
	ORDER_STATUS_PARTIALLY_FILLED_CANCELED int8 = 4
	ORDER_STATUS_UNTRIGGERED               int8 = 5
	ORDER_STATUS_TRIGGERED                 int8 = 6
	ORDER_STATUS_DEACTIVATED               int8 = 7

	STOP_ORDER_TYPE_NORMAL      int8 = 0
	STOP_ORDER_TYPE_STOP        int8 = 1
	STOP_ORDER_TYPE_TP          int8 = 2
	STOP_ORDER_TYPE_SL          int8 = 3
	STOP_ORDER_TYPE_LIQUIDATION int8 = 4

	BUCKET_AVAILABLE int8 = 0
	BUCKET_LOCKED    int8 = 1
	BUCKET_MARGIN    int8 = 2

	SIDE_NONE  int8 = -1
	SIDE_LONG  int8 = 0
	SIDE_SHORT int8 = 1

	MAINTENANCE_MARGIN_RATIO int64 = 10
)
