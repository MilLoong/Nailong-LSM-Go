package cache

import (
	"testing"

	"nailong-lsm-go/pkg/block"
)

func TestBlockCacheScanResistance(t *testing.T) {
	t.Run("four_warm_reads", func(t *testing.T) { checkScanResistance(t, 4) })
	t.Run("seven_warm_reads", func(t *testing.T) { checkScanResistance(t, 7) })
}

func checkScanResistance(t *testing.T, reads int) {
	t.Helper()
	c := NewBlockCache(32)
	payload := new(block.Block)
	for key := uint64(0); key < 8; key++ {
		c.Put(1, key, payload)
		for i := 0; i < reads; i++ {
			c.Get(1, key)
		}
	}
	for key := uint64(8); key < 136; key++ {
		c.Put(1, key, payload)
	}
	hits := 0
	for key := uint64(0); key < 8; key++ {
		if c.Get(1, key) != nil {
			hits++
		}
	}
	t.Logf("post-scan hot hits: %d/8", hits)
	if hits != 8 {
		t.Fatalf("got %d hits, want 8", hits)
	}
}
