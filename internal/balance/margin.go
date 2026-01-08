package balance

import (
	"github.com/anomalyco/meta-terminal-go/internal/utils"
)

func CalculateMargin(qty, price int64, leverage int8) int64 {
	return utils.MulDiv(qty, price, int64(leverage))
}
