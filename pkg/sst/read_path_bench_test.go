package sst

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"nailong-lsm-go/pkg/cache"
)

func BenchmarkSSTReadPath(b *testing.B) {
	for _, tc := range []struct {
		name               string
		hit, cached, bloom bool
	}{
		{"hit_no_cache", true, false, false},
		{"hit_warm_cache", true, true, false},
		{"miss_no_bloom", false, false, false},
		{"miss_in_memory_bloom", false, false, true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			builder := NewSSTBuilder(4096, tc.bloom)
			hits, misses := make([]string, 1024), make([]string, 1024)
			value := strings.Repeat("x", 128)
			for i := range hits {
				hits[i] = fmt.Sprintf("key-%06d", 2*i)
				misses[i] = fmt.Sprintf("key-%06d", 2*i+1)
				if err := builder.Add(hits[i], value, 1); err != nil {
					b.Fatal(err)
				}
			}
			misses[len(misses)-1] = misses[0] // Keep all misses inside the SST key range.
			var bc *cache.BlockCache
			if tc.cached {
				bc = cache.NewBlockCache(128)
			}
			table, err := builder.Build(1, filepath.Join(b.TempDir(), "bench.sst"), bc)
			if err != nil {
				b.Fatal(err)
			}
			defer table.Close()
			if tc.bloom {
				// Isolate the lookup path from the current footer loading issue.
				table.bloomFilter = builder.bloomFilter
			}
			keys := misses
			if tc.hit {
				keys = hits
			}
			rejected := 0
			for _, key := range keys {
				if tc.bloom && table.FindBlockIndex(key) < 0 {
					rejected++
				}
				it, err := table.Get(key, 1)
				if err != nil || (it != nil && it.Valid() && it.Key() == key) != tc.hit {
					b.Fatalf("unexpected result for %q: %v", key, err)
				}
				if it != nil {
					it.Close()
				}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				it, err := table.Get(keys[i%len(keys)], 1)
				if err != nil {
					b.Fatal(err)
				}
				if it != nil {
					it.Close()
				}
			}
			b.StopTimer()
			if tc.bloom {
				b.ReportMetric(100*float64(rejected)/float64(len(keys)), "rejected-%")
			}
		})
	}
}
