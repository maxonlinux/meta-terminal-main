package oms

import "github.com/maxonlinux/meta-terminal-go/pkg/constants"

// ShardIndex computes the shard index for a given symbol.
func ShardIndex(symbol string) uint8 {
	var h uint32
	for _, c := range symbol {
		h = h*31 + uint32(c)
	}
	return uint8(h % constants.OMS_SHARD_COUNT)
}
