package intern

import (
	"sync"
)

var (
	symbols     = make(map[string]string)
	symbolsLock sync.RWMutex
	assets      = make(map[string]string)
	assetsLock  sync.RWMutex
)

func Symbol(s string) string {
	if s == "" {
		return s
	}
	symbolsLock.RLock()
	if existing, ok := symbols[s]; ok {
		symbolsLock.RUnlock()
		return existing
	}
	symbolsLock.RUnlock()

	symbolsLock.Lock()
	if existing, ok := symbols[s]; ok {
		symbolsLock.Unlock()
		return existing
	}
	symbols[s] = s
	symbolsLock.Unlock()
	return s
}

func Asset(a string) string {
	if a == "" {
		return a
	}
	assetsLock.RLock()
	if existing, ok := assets[a]; ok {
		assetsLock.RUnlock()
		return existing
	}
	assetsLock.RUnlock()

	assetsLock.Lock()
	if existing, ok := assets[a]; ok {
		assetsLock.Unlock()
		return existing
	}
	assets[a] = a
	assetsLock.Unlock()
	return a
}
