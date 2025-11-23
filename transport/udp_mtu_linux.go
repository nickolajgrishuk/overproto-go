//go:build linux

package transport

import (
	"net"
	"syscall"

	"github.com/nickolajgrishuk/overproto-go/core"
)

// getMTU получает MTU для соединения на Linux
func getMTU(conn *net.UDPConn) (uint, error) {
	// Пытаемся получить MTU через syscall
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return core.FragMTUDefault, nil
	}

	var mtu int
	var getErr error
	err = rawConn.Control(func(fd uintptr) {
		mtu, getErr = syscall.GetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_MTU)
	})

	if err != nil || getErr != nil {
		// Если не удалось получить, возвращаем значение по умолчанию
		return core.FragMTUDefault, nil
	}

	if mtu <= 0 {
		return core.FragMTUDefault, nil
	}

	return uint(mtu), nil
}

