package overproto

import (
	"errors"
	"net"
	"sync"
	"time"

	"overproto-go/core"
	"overproto-go/optimize"
	"overproto-go/transport"
)

// RecvCallback - функция обратного вызова для обработки входящих пакетов
type RecvCallback func(streamID uint32, opcode uint8, data []byte, ctx interface{})

var (
	// config - глобальная конфигурация
	config *core.Config
	// initialized - флаг инициализации
	initialized bool
	// recvCallback - callback функция для приёма пакетов
	recvCallback RecvCallback
	// recvCtx - контекст для callback
	recvCtx interface{}
	// mu - мьютекс для thread-safety
	mu sync.RWMutex
)

// Init инициализирует библиотеку
// Thread-safe
// Если cfg == nil, используются значения по умолчанию
func Init(cfg *core.Config) error {
	mu.Lock()
	defer mu.Unlock()

	if initialized {
		return errors.New("already initialized")
	}

	if cfg == nil {
		config = core.NewConfig()
	} else {
		config = cfg
	}

	initialized = true
	return nil
}

// Shutdown завершает работу библиотеки
// Освобождает все ресурсы
// Thread-safe
func Shutdown() {
	mu.Lock()
	defer mu.Unlock()

	if !initialized {
		return
	}

	// Очищаем ключ шифрования
	optimize.ClearEncryptionKey()

	initialized = false
	config = nil
	recvCallback = nil
	recvCtx = nil
}

// SetHandler устанавливает callback функцию для приёма пакетов
// Thread-safe
func SetHandler(callback RecvCallback, ctx interface{}) {
	mu.Lock()
	defer mu.Unlock()
	recvCallback = callback
	recvCtx = ctx
}

// Send отправляет пакет данных
// Удобная функция-обёртка для создания и отправки пакета
// Автоматически применяет компрессию и шифрование если нужно
// conn может быть net.Conn (TCP) или *net.UDPConn (UDP)
func Send(conn interface{}, streamID uint32, opcode, proto uint8, data []byte, flags uint8) (int, error) {
	mu.RLock()
	if !initialized {
		mu.RUnlock()
		return 0, errors.New("not initialized")
	}
	mu.RUnlock()

	// Проверка длины payload (максимум 65535 байт)
	if len(data) > 65535 {
		return 0, errors.New("payload too large (max 65535 bytes)")
	}

	payload := make([]byte, len(data))
	copy(payload, data)

	// 1. Автоматическая компрессия
	// Если размер >= 512 байт и флаг компрессии не установлен
	if len(payload) >= int(core.CompressThreshold) && (flags&core.FlagCompressed) == 0 {
		compressed, err := optimize.Compress(payload)
		if err == nil {
			// Компрессия успешна
			payload = compressed
			flags |= core.FlagCompressed
		}
		// Если компрессия неэффективна, продолжаем без неё
	}

	// 2. Шифрование
	// Если флаг шифрования установлен
	if (flags & core.FlagEncrypted) != 0 {
		if !optimize.IsEncryptionEnabled() {
			return 0, errors.New("encryption enabled but key not set")
		}

		encrypted, iv, err := optimize.Encrypt(payload)
		if err != nil {
			return 0, err
		}

		// Формат: [IV 12 bytes] [Encrypted data] [Tag 16 bytes]
		// Но Encrypt возвращает только encrypted с tag, а IV отдельно
		// Нужно объединить IV и encrypted
		finalEncrypted := make([]byte, len(iv)+len(encrypted))
		copy(finalEncrypted[0:len(iv)], iv)
		copy(finalEncrypted[len(iv):], encrypted)
		payload = finalEncrypted
	}

	// 3. Создание заголовка
	hdr := core.NewPacketHeader()
	hdr.StreamID = streamID
	hdr.Opcode = opcode
	hdr.Proto = proto
	hdr.Flags = flags
	hdr.PayloadLen = uint16(len(payload))
	hdr.Timestamp = uint32(time.Now().Unix())
	hdr.Seq = 0 // TODO: управление sequence numbers

	// 4. Отправка через выбранный транспорт
	switch proto {
	case core.ProtoTCP:
		tcpConn, ok := conn.(net.Conn)
		if !ok {
			return 0, errors.New("invalid connection type for TCP")
		}
		return transport.TCPSend(tcpConn, hdr, payload)

	case core.ProtoUDP:
		udpConn, ok := conn.(*net.UDPConn)
		if !ok {
			return 0, errors.New("invalid connection type for UDP")
		}

		// Проверяем флаг надёжности
		if (flags & core.FlagReliable) != 0 {
			// TODO: использовать reliable transport
			// Пока отправляем через обычный UDP
			return transport.UDPSend(udpConn, hdr, payload, nil)
		}

		return transport.UDPSend(udpConn, hdr, payload, nil)

	default:
		return 0, errors.New("unsupported protocol")
	}
}

// TCPListen создаёт TCP сервер на указанном порту
func TCPListen(port uint16) (net.Listener, error) {
	return transport.TCPListen(port)
}

// TCPAccept принимает TCP соединение
func TCPAccept(listener net.Listener) (net.Conn, error) {
	return transport.TCPAccept(listener)
}

// TCPConnect подключается к TCP серверу
func TCPConnect(host string, port uint16) (net.Conn, error) {
	return transport.TCPConnect(host, port)
}

// TCPRecv принимает пакет через TCP
func TCPRecv(conn *transport.TCPConnection) (*core.PacketHeader, []byte, error) {
	return transport.TCPRecv(conn)
}

// NewTCPConnection создаёт новое TCP соединение с state machine
func NewTCPConnection(conn net.Conn) *transport.TCPConnection {
	return transport.NewTCPConnection(conn)
}

// UDPBind создаёт UDP сокет с привязкой к порту
func UDPBind(port uint16) (*net.UDPConn, error) {
	return transport.UDPBind(port)
}

// UDPConnect создаёт UDP сокет с подключением к удалённому адресу
func UDPConnect(host string, port uint16) (*net.UDPConn, error) {
	return transport.UDPConnect(host, port)
}

// UDPRecv принимает пакет через UDP
func UDPRecv(conn *net.UDPConn) (*core.PacketHeader, []byte, *net.UDPAddr, error) {
	return transport.UDPRecv(conn)
}

// SetEncryptionKey устанавливает ключ шифрования
func SetEncryptionKey(key [32]byte) error {
	return optimize.SetEncryptionKey(key)
}

// IsEncryptionEnabled проверяет, установлен ли ключ шифрования
func IsEncryptionEnabled() bool {
	return optimize.IsEncryptionEnabled()
}

// NewConfig создаёт новую конфигурацию
func NewConfig() *core.Config {
	return core.NewConfig()
}

// Экспортируем константы для удобства
const (
	FlagFragment   = core.FlagFragment
	FlagCompressed = core.FlagCompressed
	FlagEncrypted  = core.FlagEncrypted
	FlagReliable   = core.FlagReliable
	FlagACK        = core.FlagACK

	OpData    = core.OpData
	OpControl = core.OpControl
	OpACK     = core.OpACK
	OpPing    = core.OpPing
	OpPong    = core.OpPong

	ProtoTCP  = core.ProtoTCP
	ProtoUDP  = core.ProtoUDP
	ProtoHTTP = core.ProtoHTTP
)

