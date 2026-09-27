package store

import (
	"hash/fnv"
	"sync"
)

// BloomFilter tracks membership with possible false positives.
type BloomFilter struct {
	mu     sync.RWMutex
	bitset []bool
	m      uint
	k      uint
}

// NewBloomFilter creates a filter with m bits and k hash probes.
func NewBloomFilter(m, k uint) *BloomFilter {
	return &BloomFilter{
		bitset: make([]bool, m),
		m:      m,
		k:      k,
	}
}

func (bf *BloomFilter) Add(item string) {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	for i := uint(0); i < bf.k; i++ {
		index := hash(item, uint64(i)) % uint64(bf.m)
		bf.bitset[index] = true
	}
}

func (bf *BloomFilter) Contains(item string) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	for i := uint(0); i < bf.k; i++ {
		index := hash(item, uint64(i)) % uint64(bf.m)
		if !bf.bitset[index] {
			return false
		}
	}
	return true
}

func hash(data string, seed uint64) uint64 {
	h := fnv.New64a()
	h.Write([]byte(data))
	return h.Sum64() + seed
}
