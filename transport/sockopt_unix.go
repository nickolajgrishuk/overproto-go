//go:build !windows

package transport

import "syscall"

// setSockoptInt устанавливает опцию сокета для Unix-подобных систем
func setSockoptInt(fd uintptr, level, opt, value int) error {
	return syscall.SetsockoptInt(int(fd), level, opt, value)
}

