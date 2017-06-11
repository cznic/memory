// Copyright 2017 The Memory Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package memory

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"github.com/cznic/mathutil"
)

func caller(s string, va ...interface{}) {
	if s == "" {
		s = strings.Repeat("%v ", len(va))
	}
	_, fn, fl, _ := runtime.Caller(2)
	fmt.Fprintf(os.Stderr, "# caller: %s:%d: ", path.Base(fn), fl)
	fmt.Fprintf(os.Stderr, s, va...)
	fmt.Fprintln(os.Stderr)
	_, fn, fl, _ = runtime.Caller(1)
	fmt.Fprintf(os.Stderr, "# \tcallee: %s:%d: ", path.Base(fn), fl)
	fmt.Fprintln(os.Stderr)
	os.Stderr.Sync()
}

func dbg(s string, va ...interface{}) {
	if s == "" {
		s = strings.Repeat("%v ", len(va))
	}
	_, fn, fl, _ := runtime.Caller(1)
	fmt.Fprintf(os.Stderr, "# dbg %s:%d: ", path.Base(fn), fl)
	fmt.Fprintf(os.Stderr, s, va...)
	fmt.Fprintln(os.Stderr)
	os.Stderr.Sync()
}

func TODO(...interface{}) string { //TODOOK
	_, fn, fl, _ := runtime.Caller(1)
	return fmt.Sprintf("# TODO: %s:%d:\n", path.Base(fn), fl) //TODOOK
}

func use(...interface{}) {}

func init() {
	use(caller, dbg, TODO) //TODOOK
}

// ============================================================================

const quota = 128 << 20

var (
	max    = 2 * osPageSize
	bigMax = 2 * pageSize
)

func test1(t *testing.T, max int) {
	var alloc Allocator
	rem := quota
	var a [][]byte
	rng, err := mathutil.NewFC32(0, math.MaxInt32, true)
	if err != nil {
		t.Fatal(err)
	}

	rng.Seed(42)
	pos := rng.Pos()
	// Allocate
	for rem > 0 {
		size := rng.Next()%max + 1
		rem -= size
		b, err := alloc.Malloc(size)
		if err != nil {
			t.Fatal(err)
		}

		a = append(a, b)
		for i := range b {
			b[i] = byte(rng.Next())
		}
	}
	t.Logf("allocs %v, mmaps %v, bytes %v, overhead %v (%.2f%%).", alloc.allocs, alloc.mmaps, alloc.bytes, alloc.bytes-quota, 100*float64(alloc.bytes-quota)/quota)
	rng.Seek(pos)
	// Verify
	for i, b := range a {
		if g, e := len(b), rng.Next()%max+1; g != e {
			t.Fatal(i, g, e)
		}
		for i, g := range b {
			if e := byte(rng.Next()); g != e {
				t.Fatalf("%v %p: %#02x %#02x", i, &b[i], g, e)
			}

			b[i] = 0
		}
	}
	// Shuffle
	for i := range a {
		j := rng.Next() % len(a)
		a[i], a[j] = a[j], a[i]
	}
	// Free
	for _, b := range a {
		if err := alloc.Free(b); err != nil {
			t.Fatal(err)
		}
	}
	if alloc.allocs != 0 || alloc.mmaps != 0 || alloc.bytes != 0 {
		t.Fatalf("%+v", alloc)
	}
}

func Test1Small(t *testing.T) { test1(t, max) }
func Test1Big(t *testing.T)   { test1(t, bigMax) }

func test2(t *testing.T, max int) {
	var alloc Allocator
	rem := quota
	var a [][]byte
	rng, err := mathutil.NewFC32(0, math.MaxInt32, true)
	if err != nil {
		t.Fatal(err)
	}

	rng.Seed(42)
	pos := rng.Pos()
	// Allocate
	for rem > 0 {
		size := rng.Next()%max + 1
		rem -= size
		b, err := alloc.Malloc(size)
		if err != nil {
			t.Fatal(err)
		}

		a = append(a, b)
		for i := range b {
			b[i] = byte(rng.Next())
		}
	}
	t.Logf("allocs %v, mmaps %v, bytes %v, overhead %v (%.2f%%).", alloc.allocs, alloc.mmaps, alloc.bytes, alloc.bytes-quota, 100*float64(alloc.bytes-quota)/quota)
	rng.Seek(pos)
	// Verify & free
	for i, b := range a {
		if g, e := len(b), rng.Next()%max+1; g != e {
			t.Fatal(i, g, e)
		}
		for i, g := range b {
			if e := byte(rng.Next()); g != e {
				t.Fatalf("%v %p: %#02x %#02x", i, &b[i], g, e)
			}

			b[i] = 0
		}
		if err := alloc.Free(b); err != nil {
			t.Fatal(err)
		}
	}
	if alloc.allocs != 0 || alloc.mmaps != 0 || alloc.bytes != 0 {
		t.Fatalf("%+v", alloc)
	}
}

func Test2Small(t *testing.T) { test2(t, max) }
func Test2Big(t *testing.T)   { test2(t, bigMax) }

func test3(t *testing.T, max int) {
	var alloc Allocator
	rem := quota
	m := map[*[]byte][]byte{}
	rng, err := mathutil.NewFC32(1, max, true)
	if err != nil {
		t.Fatal(err)
	}

	for rem > 0 {
		switch rng.Next() % 3 {
		case 0, 1: // 2/3 allocate
			size := rng.Next()
			rem -= size
			b, err := alloc.Malloc(size)
			if err != nil {
				t.Fatal(err)
			}

			m[&b] = append([]byte(nil), b...)
		default: // 1/3 free
			for k := range m {
				b := *k
				for i := range b {
					b[i] = 0
				}
				rem += len(b)
				alloc.Free(b)
				delete(m, k)
				break
			}
		}
	}
	t.Logf("allocs %v, mmaps %v, bytes %v, overhead %v (%.2f%%).", alloc.allocs, alloc.mmaps, alloc.bytes, alloc.bytes-quota, 100*float64(alloc.bytes-quota)/quota)
	for k, v := range m {
		b := *k
		if !bytes.Equal(b, v) {
			t.Fatal("corrupted heap")
		}

		for i := range b {
			b[i] = 0
		}
		alloc.Free(b)
	}
	if alloc.allocs != 0 || alloc.mmaps != 0 || alloc.bytes != 0 {
		t.Fatalf("%+v", alloc)
	}
}

func Test3Small(t *testing.T) { test3(t, max) }
func Test3Big(t *testing.T)   { test3(t, bigMax) }

func TestFree(t *testing.T) {
	var alloc Allocator
	b, err := alloc.Malloc(1)
	if err != nil {
		t.Fatal(err)
	}

	if err := alloc.Free(b[:0]); err != nil {
		t.Fatal(err)
	}

	if alloc.allocs != 0 || alloc.mmaps != 0 || alloc.bytes != 0 {
		t.Fatalf("%+v", alloc)
	}
}

func TestMalloc(t *testing.T) {
	var alloc Allocator
	b, err := alloc.Malloc(maxSlotSize)
	if err != nil {
		t.Fatal(err)
	}

	p := (*page)(unsafe.Pointer(uintptr(unsafe.Pointer(&b[0])) &^ uintptr(osPageMask)))
	if 1<<p.log > maxSlotSize {
		t.Fatal(1<<p.log, maxSlotSize)
	}

	if err := alloc.Free(b[:0]); err != nil {
		t.Fatal(err)
	}

	if alloc.allocs != 0 || alloc.mmaps != 0 || alloc.bytes != 0 {
		t.Fatalf("%+v", alloc)
	}
}

func benchmarkFree(b *testing.B, size int) {
	var alloc Allocator
	m := make(map[*[]byte]struct{}, b.N)
	for i := 0; i < b.N; i++ {
		p, err := alloc.Malloc(size)
		if err != nil {
			b.Fatal(err)
		}

		m[&p] = struct{}{}
	}
	b.ResetTimer()
	for k := range m {
		alloc.Free(*k)
	}
	b.StopTimer()
	if alloc.allocs != 0 || alloc.mmaps != 0 || alloc.bytes != 0 {
		b.Fatalf("%+v", alloc)
	}
}

func BenchmarkFree16(b *testing.B) { benchmarkFree(b, 1<<4) }
func BenchmarkFree32(b *testing.B) { benchmarkFree(b, 1<<5) }
func BenchmarkFree64(b *testing.B) { benchmarkFree(b, 1<<6) }

func benchmarkCalloc(b *testing.B, size int) {
	var alloc Allocator
	m := make(map[*[]byte]struct{}, b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p, err := alloc.Calloc(size)
		if err != nil {
			b.Fatal(err)
		}

		m[&p] = struct{}{}
	}
	b.StopTimer()
	for k := range m {
		alloc.Free(*k)
	}
	if alloc.allocs != 0 || alloc.mmaps != 0 || alloc.bytes != 0 {
		b.Fatalf("%+v", alloc)
	}
}

func BenchmarkCalloc16(b *testing.B) { benchmarkCalloc(b, 1<<4) }
func BenchmarkCalloc32(b *testing.B) { benchmarkCalloc(b, 1<<5) }
func BenchmarkCalloc64(b *testing.B) { benchmarkCalloc(b, 1<<6) }

func benchmarkMalloc(b *testing.B, size int) {
	var alloc Allocator
	m := make(map[*[]byte]struct{}, b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p, err := alloc.Malloc(size)
		if err != nil {
			b.Fatal(err)
		}

		m[&p] = struct{}{}
	}
	b.StopTimer()
	for k := range m {
		alloc.Free(*k)
	}
	if alloc.allocs != 0 || alloc.mmaps != 0 || alloc.bytes != 0 {
		b.Fatalf("%+v", alloc)
	}
}

func BenchmarkMalloc16(b *testing.B) { benchmarkMalloc(b, 1<<4) }
func BenchmarkMalloc32(b *testing.B) { benchmarkMalloc(b, 1<<5) }
func BenchmarkMalloc64(b *testing.B) { benchmarkMalloc(b, 1<<6) }
