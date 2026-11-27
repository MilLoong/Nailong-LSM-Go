package comparison

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
	"nailong-lsm-go/pkg/config"
	"nailong-lsm-go/pkg/lsm"
)

type backend struct {
	put   func(string, string) error
	get   func(string, string, bool) bool
	seal  func() error
	close func() error
}

func openTiny(path string) (backend, error) {
	db, err := lsm.NewEngine(config.DefaultConfig(), path)
	if err != nil {
		return backend{}, err
	}
	return backend{db.Put, func(key, want string, hit bool) bool {
		value, found, err := db.Get(key)
		return err == nil && found == hit && (!hit || value == want)
	}, func() error { return nil }, db.Close}, nil
}

func openLevelDB(path string) (backend, error) {
	db, err := leveldb.OpenFile(path, &opt.Options{
		Filter: filter.NewBloomFilter(10), Compression: opt.NoCompression,
	})
	if err != nil {
		return backend{}, err
	}
	return backend{func(key, value string) error {
		return db.Put([]byte(key), []byte(value), nil)
	}, func(key, want string, hit bool) bool {
		value, err := db.Get([]byte(key), nil)
		if !hit {
			return err == leveldb.ErrNotFound
		}
		return err == nil && string(value) == want
	}, func() error { return db.CompactRange(util.Range{}) }, db.Close}, nil
}

func BenchmarkPointRead(b *testing.B) {
	const records = 4096
	value := strings.Repeat("v", 128)
	for _, engine := range []struct {
		name string
		open func(string) (backend, error)
	}{{"TinyLSM", openTiny}, {"GoLevelDB", openLevelDB}} {
		path := filepath.Join(b.TempDir(), engine.name)
		db, err := engine.open(path)
		check(b, err)
		for i := 0; i < records; i++ {
			check(b, db.put(fmt.Sprintf("k%08d", i*2), value))
		}
		check(b, db.seal())
		check(b, db.close())
		ext := ".sst"
		if engine.name == "GoLevelDB" {
			ext = ".ldb"
		}
		tables := 0
		check(b, filepath.WalkDir(path, func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() && filepath.Ext(path) == ext {
				tables++
			}
			return err
		}))
		if tables == 0 {
			b.Fatalf("%s: fixture has no on-disk tables", engine.name)
		}
		db, err = engine.open(path)
		check(b, err)
		b.Cleanup(func() { check(b, db.close()) })
		for _, hit := range []bool{true, false} {
			mode, gap := "hit", 0
			if !hit {
				mode, gap = "in_range_miss", 1
			}
			keys := make([]string, records-1)
			for i, n := range rand.New(rand.NewSource(42)).Perm(len(keys)) {
				keys[i] = fmt.Sprintf("k%08d", n*2+gap)
				if !db.get(keys[i], value, hit) {
					b.Fatalf("%s: unexpected result for %s", engine.name, keys[i])
				}
			}
			b.Run(engine.name+"/"+mode, func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if !db.get(keys[i%len(keys)], value, hit) {
						b.Fatal("incorrect lookup")
					}
				}
			})
		}
	}
}

func check(b *testing.B, err error) {
	b.Helper()
	if err != nil {
		b.Fatal(err)
	}
}
