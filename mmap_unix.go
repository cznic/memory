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
