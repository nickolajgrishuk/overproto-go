//go:build !linux

package transport

import (
	"net"

	"github.com/nickolajgrishuk/overproto-go/core"
)

// getMTU получает MTU для соединения на не-Linux платформах
// На macOS и Windows IP_MTU недоступен, возвращаем значение по умолчанию
func getMTU(conn *net.UDPConn) (uint, error) {
	// На не-Linux платформах IP_MTU недоступен
	// Возвращаем значение по умолчанию
	return core.FragMTUDefault, nil
}
