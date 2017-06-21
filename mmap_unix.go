// Copyright 2011 Evan Shaw. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE-MMAP-GO file.

// +build darwin dragonfly freebsd linux openbsd solaris netbsd

// Modifications (c) 2017 The Memory Authors.

package memory

import (
	"syscall"
	"unsafe"
)

var pageSize = 1 << 20

func mmap0(size int) ([]byte, error) {
	flags := syscall.MAP_SHARED | syscall.MAP_ANON
	prot := syscall.PROT_READ | syscall.PROT_WRITE
	b, err := syscall.Mmap(-1, 0, size, prot, flags)
	if err != nil {
		return nil, err
	}

	if uintptr(unsafe.Pointer(&b[0]))&uintptr(osPageMask) != 0 {
		panic("internal error")
	}

	return b, nil
}

func unmap(addr unsafe.Pointer, size int) error {
	_, _, errno := syscall.Syscall(syscall.SYS_MUNMAP, uintptr(addr), uintptr(size), 0)
	if errno != 0 {
		return errno
	}

	return nil
}

// pageSize aligned.
func mmap(size int) ([]byte, error) {
	size = roundup(size, osPageSize)
	b, err := mmap0(size + pageSize)
	if err != nil {
		return nil, err
	}

	mod := int(uintptr(unsafe.Pointer(&b[0]))) & pageMask
	if mod != 0 {
		n := pageSize - mod
		if err := unmap(unsafe.Pointer(&b[0]), n); err != nil {
			return nil, err
		}

		b = b[n:]
	}

	if uintptr(unsafe.Pointer(&b[0]))&uintptr(pageMask) != 0 {
		panic("internal error")
	}

	if err := unmap(unsafe.Pointer(&b[size]), len(b)-size); err != nil {
		return nil, err
	}

	return b[:size:size], nil
}
