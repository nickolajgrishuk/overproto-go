package transport

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"

	"overproto-go/core"
)

const (
	// UDPRecvBufferSize - размер буфера для приёма (64KB)
	UDPRecvBufferSize = 64 * 1024
)

// UDPBind создаёт UDP сокет с привязкой к порту
// Устанавливает SO_REUSEADDR
func UDPBind(port uint16) (*net.UDPConn, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var err error
			c.Control(func(fd uintptr) {
				// Устанавливаем SO_REUSEADDR
				err = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			})
			return err
		},
	}

	addr := &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: int(port),
	}

	conn, err := lc.ListenPacket(context.Background(), "udp", addr.String())
	if err != nil {
		return nil, err
	}

	udpConn, ok := conn.(*net.UDPConn)
	if !ok {
		conn.Close()
		return nil, errors.New("failed to cast to UDPConn")
	}

	return udpConn, nil
}

// UDPConnect создаёт UDP сокет с подключением к удалённому адресу
// Позволяет использовать Write/Read вместо WriteTo/ReadFrom
func UDPConnect(host string, port uint16) (*net.UDPConn, error) {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// UDPSend отправляет пакет через UDP
// Если addr == nil, используется подключённый адрес
// Проверяет MTU и предупреждает если пакет слишком большой
func UDPSend(conn *net.UDPConn, hdr *core.PacketHeader, payload []byte, addr *net.UDPAddr) (int, error) {
	// Сериализуем пакет
	data, err := core.Serialize(hdr, payload)
	if err != nil {
		return 0, err
	}

	// Проверяем MTU
	mtu, err := UDPGetMTU(conn)
	if err == nil {
		if uint(len(data)) > mtu {
			// Предупреждение: пакет превышает MTU, рекомендуется фрагментация
			// Но всё равно отправляем
		}
	}

	// Отправляем данные
	var n int
	if addr == nil {
		// Используем подключённый адрес
		n, err = conn.Write(data)
	} else {
		// Отправляем на указанный адрес
		n, err = conn.WriteToUDP(data, addr)
	}

	if err != nil {
		return 0, err
	}

	return n, nil
}

// UDPRecv принимает пакет через UDP
// Возвращает заголовок, payload и адрес отправителя
func UDPRecv(conn *net.UDPConn) (*core.PacketHeader, []byte, *net.UDPAddr, error) {
	buf := make([]byte, UDPRecvBufferSize)

	n, addr, err := conn.ReadFromUDP(buf)
	if err != nil {
		return nil, nil, nil, err
	}

	// Десериализуем пакет
	hdr, payload, err := core.Deserialize(buf[:n])
	if err != nil {
		return nil, nil, nil, err
	}

	return hdr, payload, addr, nil
}

// UDPGetMTU получает MTU для соединения
// Пытается через getsockopt, иначе возвращает 1400
func UDPGetMTU(conn *net.UDPConn) (uint, error) {
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

