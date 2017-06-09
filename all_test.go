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

const quota = 16 << 20

var max = 2 * pageSize

func Test1(t *testing.T) {
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
	t.Logf("allocs %v, pages %v, bytes %v, overhead %v bytes.", alloc.nallocs, alloc.npages, alloc.nbytes, alloc.nbytes-quota)
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
	if alloc.nallocs != 0 || alloc.npages != 0 || alloc.nbytes != 0 {
		t.Fatalf("%+v", alloc)
	}
}

func Test2(t *testing.T) {
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
	t.Logf("allocs %v, pages %v, bytes %v, overhead %v bytes.", alloc.nallocs, alloc.npages, alloc.nbytes, alloc.nbytes-quota)
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
	if alloc.nallocs != 0 || alloc.npages != 0 || alloc.nbytes != 0 {
		t.Fatalf("%+v", alloc)
	}
}

func Test3(t *testing.T) {
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
	t.Logf("allocs %v, pages %v, bytes %v, overhead %v bytes.", alloc.nallocs, alloc.npages, alloc.nbytes, alloc.nbytes-quota)
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
	if alloc.nallocs != 0 || alloc.npages != 0 || alloc.nbytes != 0 {
		t.Fatalf("%+v", alloc)
	}
}
