// Copyright 2017 The Memory Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package memory

import (
	"reflect"
	"syscall"
	"unsafe"
)

const (
	_MEM_COMMIT   = 0x1000
	_MEM_RESERVE  = 0x2000
	_MEM_DECOMMIT = 0x4000
	_MEM_RELEASE  = 0x8000

	_PAGE_READWRITE = 0x0004
	_PAGE_NOACCESS  = 0x0001
)

var (
	pageSize = 1 << 16

	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procVirtualAlloc = modkernel32.NewProc("VirtualAlloc")
	procVirtualFree  = modkernel32.NewProc("VirtualFree")
)

// pageSize aligned.
func mmap(size int) ([]byte, error) {
	size = roundup(size, pageSize)
	addr, _, err := procVirtualAlloc.Call(0, uintptr(size), _MEM_COMMIT|_MEM_RESERVE, _PAGE_READWRITE)
	if addr == 0 {
		return nil, err
	}

	if addr&uintptr(pageMask) != 0 {
		panic("internal error")
	}

	var b []byte
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh.Data = addr
	sh.Len = size
	sh.Cap = size
	return b, nil
}

func unmap(addr unsafe.Pointer, size int) error {
	r, _, err := procVirtualFree.Call(uintptr(addr), 0, _MEM_RELEASE)
	if r == 0 {
		return err
	}

	return nil
}
