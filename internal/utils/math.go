package utils

import "math"

func Mul(a, b int64) int64 {
	return a * b
}

func Div(a, b int64) int64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func MulDiv(a, b, c int64) int64 {
	if c == 0 {
		return 0
	}
	return a * b / c
}

func Avg(a1, c1, a2, c2 int64) int64 {
	sum := a1*c1 + a2*c2
	count := c1 + c2
	if count == 0 {
		return 0
	}
	return sum / count
}

func Sub(a, b int64) int64 {
	return a - b
}

func Add(a, b int64) int64 {
	return a + b
}

// RoundToPrecision округляет цену до указанной точности
// price: исходная цена в int64 (например, 50000123 для 50000.123)
// precision: количество знаков после запятой (0, 1, 2, 3, 4, 5, 8)
func RoundToPrecision(price int64, precision int8) int64 {
	if precision <= 0 {
		return price
	}

	divisor := int64(1)
	for i := int8(0); i < precision; i++ {
		divisor *= 10
	}

	return (price/divisor + ((price%divisor + divisor/2) / divisor)) * divisor
}

// PriceToFloat конвертирует int64 цену в float64
// precision: количество знаков после запятой
func PriceToFloat(price int64, precision int8) float64 {
	divisor := float64(1)
	for i := int8(0); i < precision; i++ {
		divisor *= 10
	}
	return float64(price) / divisor
}

// FloatToPrice конвертирует float64 цену в int64
// precision: количество знаков после запятой
func FloatToPrice(price float64, precision int8) int64 {
	divisor := float64(1)
	for i := int8(0); i < precision; i++ {
		divisor *= 10
	}
	return int64(math.Round(price * divisor))
}

// CalcAvgPrice вычисляет среднюю цену позиции
// existingPrice: теща цена входа
// existingQty: текущий размер
// newPrice: новая цена
// newQty: новый размер
func CalcAvgPrice(existingPrice int64, existingQty int64, newPrice int64, newQty int64) int64 {
	if existingQty == 0 {
		return newPrice
	}
	if newQty == 0 {
		return existingPrice
	}
	totalValue := existingPrice*existingQty + newPrice*newQty
	totalQty := existingQty + newQty
	return totalValue / totalQty
}

// CalcUnrealizedPnL вычисляет нереализованный PnL
// positionSize: размер позиции (всегда положительный)
// entryPrice: цена входа
// currentPrice: текущая цена
// side: 0 = LONG, 1 = SHORT
func CalcUnrealizedPnL(positionSize int64, entryPrice int64, currentPrice int64, side int8) int64 {
	if side == 0 {
		return (currentPrice - entryPrice) * positionSize
	}
	return (entryPrice - currentPrice) * positionSize
}

// CalcLiquidationPrice вычисляет цену ликвидации
// entryPrice: цена входа
// size: размер позиции
// leverage: плечо
// side: 0 = LONG, 1 = SHORT
func CalcLiquidationPrice(entryPrice int64, size int64, leverage int8, side int8) int64 {
	if size == 0 {
		return 0
	}

	// IM = size * price / leverage
	// MM = IM / 10
	// buffer = IM - MM = IM * 0.9
	// liquidation = entryPrice ± buffer / size

	initialMargin := entryPrice * size / int64(leverage)
	maintenanceMargin := initialMargin / 10
	buffer := initialMargin - maintenanceMargin

	if side == 0 {
		// LONG ликвидируется когда цена падает
		return entryPrice - buffer/size
	}
	// SHORT ликвидируется когда цена растёт
	return entryPrice + buffer/size
}
