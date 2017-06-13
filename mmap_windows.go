// Copyright 2011 Evan Shaw. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE-MMAP-GO file.

// Modifications (c) 2017 The Memory Authors.

package memory

import (
	"errors"
	"os"
	"reflect"
	"syscall"
	"unsafe"
)

var pageSize = os.Getpagesize()

// mmap on Windows is a two-step process.
// First, we call CreateFileMapping to get a handle.
// Then, we call MapviewToFile to get an actual pointer into memory.

// We keep this map so that we can get back the original handle from the memory address.
var handleMap = map[uintptr]syscall.Handle{}

func mmap0(size int) ([]byte, error) {
	flProtect := uint32(syscall.PAGE_READWRITE)
	dwDesiredAccess := uint32(syscall.FILE_MAP_WRITE)

	// The maximum size is the area of the file, starting from 0,
	// that we wish to allow to be mappable. It is the sum of
	// the length the user requested, plus the offset where that length
	// is starting from. This does not map the data into memory.
	maxSizeHigh := uint32(int64(size) >> 32)
	maxSizeLow := uint32(int64(size) & 0xFFFFFFFF)
	// TODO: Do we need to set some security attributes? It might help portability.
	h, errno := syscall.CreateFileMapping(syscall.Handle(^uintptr(0)), nil, flProtect, maxSizeHigh, maxSizeLow, nil)
	if h == 0 {
		return nil, os.NewSyscallError("CreateFileMapping", errno)
	}

	// Actually map a view of the data into memory. The view's size
	// is the length the user requested.
	addr, errno := syscall.MapViewOfFile(h, dwDesiredAccess, 0, 0, uintptr(size))
	if addr == 0 {
		return nil, os.NewSyscallError("MapViewOfFile", errno)
	}

	if addr&uintptr(osPageMask) != 0 {
		panic("internal error")
	}

	handleMap[addr] = h
	var b []byte
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh.Data = addr
	sh.Len = size
	sh.Cap = size
	return b, nil
}

func unmap(addr unsafe.Pointer, size int) error {
	// Lock the UnmapViewOfFile along with the handleMap deletion.
	// As soon as we unmap the view, the OS is free to give the
	// same addr to another new map. We don't want another goroutine
	// to insert and remove the same addr into handleMap while
	// we're trying to remove our old addr/handle pair.
	err := syscall.UnmapViewOfFile(uintptr(addr))
	if err != nil {
		return err
	}

	handle, ok := handleMap[uintptr(addr)]
	if !ok {
		// should be impossible; we would've errored above
		return errors.New("unknown base address")
	}
	delete(handleMap, uintptr(addr))

	e := syscall.CloseHandle(syscall.Handle(handle))
	return os.NewSyscallError("CloseHandle", e)
}
