package oms

import "github.com/maxonlinux/meta-terminal-go/pkg/constants"

// ShardIndex computes the shard index for a given symbol using the same hash function
// as OrderBook.shard(). This ensures consistent sharding across the OMS and OrderBook,
// which is critical for maintaining order placement correctness during matching.
func ShardIndex(symbol string) uint8 {
	var h uint32
	for _, c := range symbol {
		h = h*31 + uint32(c)
	}
	return uint8(h % constants.OMS_SHARD_COUNT)
}
