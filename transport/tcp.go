package transport

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"syscall"
	"time"

	"overproto-go/core"
)

// TCPRecvState - состояние state machine для приёма TCP пакетов
type TCPRecvState int

const (
	// StateIdle - начальное состояние
	StateIdle TCPRecvState = iota
	// StateReadingHeader - чтение заголовка
	StateReadingHeader
	// StateReadingPayload - чтение payload
	StateReadingPayload
	// StateReadingCRC - чтение CRC32
	StateReadingCRC
	// StateReady - пакет готов
	StateReady
)

// TCPConnection - TCP соединение с state machine для приёма
type TCPConnection struct {
	fd            net.Conn
	recvState     TCPRecvState
	recvBuffer    []byte
	recvBytesRead uint
	mu            sync.Mutex
}

const (
	// TCPRecvBufferSize - размер буфера для приёма (64KB)
	TCPRecvBufferSize = 64 * 1024
	// TCPBacklog - backlog для listen
	TCPBacklog = 10
)

// TCPListen создаёт TCP сервер на указанном порту
// Устанавливает SO_REUSEADDR
func TCPListen(port uint16) (net.Listener, error) {
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

	addr := &net.TCPAddr{
		IP:   net.IPv4zero,
		Port: int(port),
	}

	return lc.Listen(context.Background(), "tcp", addr.String())
}

// TCPAccept принимает соединение
func TCPAccept(listener net.Listener) (net.Conn, error) {
	return listener.Accept()
}

// TCPConnect подключается к TCP серверу
func TCPConnect(host string, port uint16) (net.Conn, error) {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	return net.DialTimeout("tcp", addr, 10*time.Second)
}

// NewTCPConnection создаёт новое TCP соединение с state machine
func NewTCPConnection(conn net.Conn) *TCPConnection {
	return &TCPConnection{
		fd:            conn,
		recvState:     StateIdle,
		recvBuffer:    make([]byte, TCPRecvBufferSize),
		recvBytesRead: 0,
	}
}

// readExact читает точное количество байт (гарантированное чтение)
func (conn *TCPConnection) readExact(buf []byte) error {
	totalRead := 0
	for totalRead < len(buf) {
		n, err := conn.fd.Read(buf[totalRead:])
		if err != nil {
			if err == io.EOF {
				return io.EOF
			}
			return err
		}
		if n == 0 {
			return io.EOF
		}
		totalRead += n
	}
	return nil
}

// TCPRecv принимает пакет через TCP
// Использует state machine для чтения по частям
// Может быть вызвана несколько раз для чтения полного пакета
func TCPRecv(conn *TCPConnection) (*core.PacketHeader, []byte, error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	for {
		switch conn.recvState {
		case StateIdle:
			// Начинаем чтение заголовка
			conn.recvBuffer = make([]byte, core.HeaderSize)
			conn.recvBytesRead = 0
			conn.recvState = StateReadingHeader

		case StateReadingHeader:
			// Читаем заголовок (24 байта)
			remaining := core.HeaderSize - int(conn.recvBytesRead)
			if remaining > 0 {
				err := conn.readExact(conn.recvBuffer[int(conn.recvBytesRead):core.HeaderSize])
				if err != nil {
					conn.recvState = StateIdle
					return nil, nil, err
				}
				conn.recvBytesRead = core.HeaderSize
			}

			// Извлекаем payload_len из заголовка
			payloadLen := uint16(conn.recvBuffer[18])<<8 | uint16(conn.recvBuffer[19])
			totalSize := core.HeaderSize + int(payloadLen) + 4 // Header + Payload + CRC32

			// Расширяем буфер если нужно
			if totalSize > len(conn.recvBuffer) {
				newBuf := make([]byte, totalSize)
				copy(newBuf, conn.recvBuffer[:core.HeaderSize])
				conn.recvBuffer = newBuf
			}

			conn.recvState = StateReadingPayload
			conn.recvBytesRead = core.HeaderSize

		case StateReadingPayload:
			// Читаем payload
			payloadLen := uint16(conn.recvBuffer[18])<<8 | uint16(conn.recvBuffer[19])
			payloadStart := core.HeaderSize
			payloadEnd := payloadStart + int(payloadLen)

			remaining := payloadEnd - int(conn.recvBytesRead)
			if remaining > 0 {
				err := conn.readExact(conn.recvBuffer[int(conn.recvBytesRead):payloadEnd])
				if err != nil {
					conn.recvState = StateIdle
					return nil, nil, err
				}
				conn.recvBytesRead = uint(payloadEnd)
			}

			conn.recvState = StateReadingCRC

		case StateReadingCRC:
			// Читаем CRC32 (4 байта)
			payloadLen := uint16(conn.recvBuffer[18])<<8 | uint16(conn.recvBuffer[19])
			crcStart := core.HeaderSize + int(payloadLen)
			crcEnd := crcStart + 4

			remaining := crcEnd - int(conn.recvBytesRead)
			if remaining > 0 {
				err := conn.readExact(conn.recvBuffer[int(conn.recvBytesRead):crcEnd])
				if err != nil {
					conn.recvState = StateIdle
					return nil, nil, err
				}
				conn.recvBytesRead = uint(crcEnd)
			}

			conn.recvState = StateReady

		case StateReady:
			// Десериализуем пакет
			packetData := conn.recvBuffer[:conn.recvBytesRead]
			hdr, payload, err := core.Deserialize(packetData)
			if err != nil {
				conn.recvState = StateIdle
				return nil, nil, err
			}

			// Сбрасываем состояние
			conn.recvState = StateIdle
			conn.recvBytesRead = 0

			return hdr, payload, nil
		}
	}
}

// TCPSend отправляет пакет через TCP
// Сериализует пакет и отправляет целиком
func TCPSend(conn net.Conn, hdr *core.PacketHeader, payload []byte) (int, error) {
	// Сериализуем пакет
	data, err := core.Serialize(hdr, payload)
	if err != nil {
		return 0, err
	}

	// Отправляем данные
	n, err := conn.Write(data)
	if err != nil {
		return 0, err
	}

	return n, nil
}

// TCPClose закрывает TCP соединение
func TCPClose(conn net.Conn) error {
	return conn.Close()
}

