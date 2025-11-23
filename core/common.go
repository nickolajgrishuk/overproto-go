package core

// Константы протокола
const (
	// Magic - уникальная сигнатура протокола
	Magic = 0xABCD
	// Version - версия протокола
	Version = 0x01
	// HeaderSize - размер заголовка пакета в байтах
	HeaderSize = 24
	// FragMTUDefault - MTU по умолчанию для фрагментации
	FragMTUDefault = 1400
	// FragMaxFragments - максимальное количество фрагментов на пакет
	FragMaxFragments = 256
	// FragTimeoutSec - timeout сборки фрагментов в секундах
	FragTimeoutSec = 30
	// CompressThreshold - порог размера для автоматической компрессии (байт)
	CompressThreshold = 512
	// CompressLevel - уровень компрессии zlib (1-9)
	CompressLevel = 6
)

// Флаги пакета
const (
	// FlagFragment - пакет является фрагментом
	FlagFragment = 0x01
	// FlagCompressed - payload сжат через zlib
	FlagCompressed = 0x02
	// FlagEncrypted - payload зашифрован через AES-GCM
	FlagEncrypted = 0x04
	// FlagReliable - требуется надёжная доставка
	FlagReliable = 0x08
	// FlagACK - пакет является ACK подтверждением
	FlagACK = 0x10
)

// Opcode операции
const (
	// OpData - данные
	OpData = 0x01
	// OpControl - управляющий пакет
	OpControl = 0x02
	// OpACK - подтверждение
	OpACK = 0x03
	// OpPing - ping
	OpPing = 0x04
	// OpPong - pong
	OpPong = 0x05
)

// Тип протокола
const (
	// ProtoTCP - TCP протокол
	ProtoTCP = 0x01
	// ProtoUDP - UDP протокол
	ProtoUDP = 0x02
	// ProtoHTTP - HTTP протокол
	ProtoHTTP = 0x03
)

// Config - конфигурация библиотеки
type Config struct {
	// TCPPort - TCP порт по умолчанию
	TCPPort uint16
	// UDPPort - UDP порт по умолчанию
	UDPPort uint16
	// MTU - MTU по умолчанию
	MTU uint
	// NonBlocking - non-blocking режим сокетов
	NonBlocking bool
}

// NewConfig создаёт новую конфигурацию с значениями по умолчанию
func NewConfig() *Config {
	return &Config{
		TCPPort: 8080,
		UDPPort: 8080,
		MTU:     1400,
	}
}

