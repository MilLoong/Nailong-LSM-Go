package comparison

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"nailong-lsm-go/pkg/config"
	"nailong-lsm-go/pkg/lsm"
)

func TestConcurrentReadThroughput(t *testing.T) {
	const keys, requests = 4096, 1048576
	value := strings.Repeat("v", 128)
	db, err := lsm.NewEngine(config.DefaultConfig(), filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	names := make([]string, keys)
	for i := range names {
		names[i] = fmt.Sprintf("k%08d", i)
		if err := db.Put(names[i], value); err != nil {
			t.Fatal(err)
		}
	}
	trace := make([]string, requests)
	rng := rand.New(rand.NewSource(42))
	for i := range trace {
		trace[i] = names[rng.Intn(keys)]
	}
	for _, key := range names {
		got, found, err := db.Get(key)
		if err != nil || !found || got != value {
			t.Fatalf("warmup failed for %q: %v", key, err)
		}
	}
	for _, workers := range []int{1, 4, 8, 16, 32} {
		for trial := 1; trial <= 3; trial++ {
			start := make(chan struct{})
			var ready, done sync.WaitGroup
			ready.Add(workers)
			done.Add(workers)
			for w := 0; w < workers; w++ {
				go func(begin, end int) {
					defer done.Done()
					ready.Done()
					<-start
					for i := begin; i < end; i++ {
						got, found, err := db.Get(trace[i])
						if err != nil || !found || got != value {
							t.Errorf("unexpected result for %q: %v", trace[i], err)
							return
						}
					}
				}(w*requests/workers, (w+1)*requests/workers)
			}
			ready.Wait()
			tick := time.Now()
			close(start)
			done.Wait()
			elapsed := time.Since(tick)
			t.Logf("workers=%2d trial=%d ops/s=%.0f", workers, trial, requests/elapsed.Seconds())
		}
	}
	for _, level := range db.GetLevelInfo() {
		if level.NumFiles != 0 {
			t.Fatal("expected memtable-only workload")
		}
	}
}
