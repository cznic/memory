// Copyright 2017 The Memory Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package memory implements a memory allocator.
package memory

import (
	"os"
	"reflect"
	"unsafe"

	"github.com/cznic/mathutil"
)

const (
	mallocAllign = 16 // Must be >= 16
)

var (
	headerSize  = roundup(int(unsafe.Sizeof(page{})), mallocAllign)
	maxSlotSize = pageAvail >> 1
	osPageMask  = osPageSize - 1
	osPageSize  = os.Getpagesize()
	pageAvail   = pageSize - headerSize
	pageMask    = pageSize - 1
)

// if n%m != 0 { n += m-n%m }. m must be a power of 2.
func roundup(n, m int) int { return (n + m - 1) &^ (m - 1) }

type node struct {
	prev, next *node
}

type page struct {
	brk  int
	log  uint
	size int
	used int
}

// Allocator allocates and frees memory. Its zero value is ready for use.
type Allocator struct {
	cap    [64]int
	lists  [64]*node
	pages  [64]*page
	allocs int // # of allocs.
	bytes  int // Asked from OS.
	mmaps  int // Asked from OS.
}

func (a *Allocator) newPage(size int) (*page, error) {
	size += headerSize
	b, err := mmap(size)
	if err != nil {
		return nil, err
	}

	a.bytes += len(b)
	a.mmaps++
	p := (*page)(unsafe.Pointer(&b[0]))
	p.size = len(b)
	p.log = 0
	return p, nil
}

func (a *Allocator) newSharedPage(log uint) (*page, error) {
	if a.cap[log] == 0 {
		a.cap[log] = pageAvail / (1 << log)
	}
	size := headerSize + a.cap[log]<<log
	b, err := mmap(size)
	if err != nil {
		return nil, err
	}

	a.bytes += len(b)
	a.mmaps++
	p := (*page)(unsafe.Pointer(&b[0]))
	a.pages[log] = p
	p.size = len(b)
	p.log = log
	return p, nil
}

// Calloc is like Malloc except the allocated memory is zeroed.
func (a *Allocator) Calloc(size int) ([]byte, error) {
	b, err := a.Malloc(size)
	if err != nil {
		return nil, err
	}

	for i := range b {
		b[i] = 0
	}
	return b, nil
}

// Free deallocates memory (as in C.free). The argument of Free must have been
// acquired from Calloc or Malloc or Realloc.
func (a *Allocator) Free(b []byte) error {
	b = b[:cap(b)]
	a.allocs--
	p := (*page)(unsafe.Pointer(uintptr(unsafe.Pointer(&b[0])) &^ uintptr(pageMask)))
	log := p.log
	if log == 0 {
		a.mmaps--
		a.bytes -= p.size
		return unmap(unsafe.Pointer(p), p.size)
	}

	n := (*node)(unsafe.Pointer(&b[0]))
	n.prev = nil
	n.next = a.lists[log]
	if n.next != nil {
		n.next.prev = n
	}
	a.lists[log] = n
	p.used--
	if p.used != 0 {
		return nil
	}

	for i := 0; i < p.brk; i++ {
		n := (*node)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + uintptr(headerSize+i<<log)))
		switch {
		case n.prev == nil:
			a.lists[log] = n.next
			if n.next != nil {
				n.next.prev = nil
			}
		case n.next == nil:
			n.prev.next = nil
		default:
			n.prev.next = n.next
			n.next.prev = n.prev
		}
	}

	if a.pages[log] == p {
		a.pages[log] = nil
	}
	a.mmaps--
	a.bytes -= p.size
	return unmap(unsafe.Pointer(p), p.size)
}

// Malloc allocates size bytes and returns a byte slice of the allocated
// memory. The memory is not initialized. Malloc panics for size < 0 and
// returns (nil, nil) for zero size.
//
// It's ok to reslice the returned slice but the result of appending to it
// cannot be passed to Free or Realloc as it may refer to a different backing
// array afterwards.
func (a *Allocator) Malloc(size int) (r []byte, _ error) {
	if size < 0 {
		panic("invalid malloc size")
	}

	if size == 0 {
		return nil, nil
	}

	a.allocs++
	log := uint(mathutil.BitLen(roundup(size, mallocAllign) - 1))
	if 1<<log > maxSlotSize {
		p, err := a.newPage(size)
		if err != nil {
			return nil, err
		}

		var b []byte
		sh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
		sh.Data = uintptr(unsafe.Pointer(p)) + uintptr(headerSize)
		sh.Len = size
		sh.Cap = size
		return b, nil
	}

	if a.lists[log] == nil && a.pages[log] == nil {
		if _, err := a.newSharedPage(log); err != nil {
			return nil, err
		}
	}

	if p := a.pages[log]; p != nil {
		p.used++
		var b []byte
		sh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
		sh.Data = uintptr(unsafe.Pointer(p)) + uintptr(headerSize+p.brk<<log)
		sh.Len = size
		sh.Cap = 1 << log
		p.brk++
		if p.brk == a.cap[log] {
			a.pages[log] = nil
		}
		return b, nil
	}

	n := a.lists[log]
	p := (*page)(unsafe.Pointer(uintptr(unsafe.Pointer(n)) &^ uintptr(pageMask)))
	a.lists[log] = n.next
	if n.next != nil {
		n.next.prev = nil
	}
	p.used++
	var b []byte
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh.Data = uintptr(unsafe.Pointer(n))
	sh.Len = size
	sh.Cap = 1 << log
	return b, nil
}

// Realloc changes the size of the backing array of b to size bytes or returns
// an error, if any.  The contents will be unchanged in the range from the
// start of the region up to the minimum of the old and new  sizes.   If the
// new size is larger than the old size, the added memory will not be
// initialized.  If b's backing array is of zero size, then the call is
// equivalent to Malloc(size), for all values of size; if size is equal to
// zero, and b's backing array is not of zero size, then the call is equivalent
// to Free(b).  Unless b's backing array is of zero size, it must have been
// returned by an earlier call to Malloc, Calloc or Realloc.  If the area
// pointed to was moved, a Free(b) is done.
func (a *Allocator) Realloc(b []byte, size int) ([]byte, error) {
	switch {
	case cap(b) == 0:
		return a.Malloc(size)
	case size == 0 && cap(b) != 0:
		return nil, a.Free(b)
	case size <= cap(b):
		return b[:size], nil
	}

	r, err := a.Malloc(size)
	if err != nil {
		return nil, err
	}

	copy(r, b)
	return r, a.Free(b)
}

// UsableSize reports the size of the memory block allocated at p, which must
// point to the first byte of a slice returned from Calloc, Malloc or Realloc.
// The allocated memory block size can be larger than the size originally
// requested from Calloc, Malloc or Realloc.
func UsableSize(p *byte) int {
	if p == nil {
		return 0
	}

	pg := (*page)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) &^ uintptr(pageMask)))
	if pg.log != 0 {
		return 1 << pg.log
	}

	return pg.size - headerSize
}
