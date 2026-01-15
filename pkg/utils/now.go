package utils

import "time"

// NowNano возвращает текущее время в наносекундах
// Используется для всех timestamp в системе
func NowNano() uint64 {
	return uint64(time.Now().UnixNano())
}
