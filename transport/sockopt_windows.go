//go:build windows

package transport

import "syscall"

// setSockoptInt устанавливает опцию сокета для Windows
func setSockoptInt(fd uintptr, level, opt, value int) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), level, opt, value)
}

