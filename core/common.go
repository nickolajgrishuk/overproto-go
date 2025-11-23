package core

import "errors"

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

// SafeUint16ToUint16 проверяет, что значение uint помещается в uint16
func SafeUint16ToUint16(v uint) (uint16, error) {
	if v > 65535 {
		return 0, errors.New("value exceeds uint16 max")
	}
	return uint16(v), nil
}

// SafeIntToUint16 проверяет, что значение int помещается в uint16
func SafeIntToUint16(v int) (uint16, error) {
	if v < 0 || v > 65535 {
		return 0, errors.New("value out of uint16 range")
	}
	return uint16(v), nil
}

// SafeInt64ToUint32 проверяет, что значение int64 помещается в uint32
func SafeInt64ToUint32(v int64) (uint32, error) {
	if v < 0 || v > 4294967295 {
		return 0, errors.New("value out of uint32 range")
	}
	return uint32(v), nil
}

// SafeIntToUint проверяет, что значение int помещается в uint
func SafeIntToUint(v int) (uint, error) {
	if v < 0 {
		return 0, errors.New("value is negative")
	}
	return uint(v), nil
}

// SafeUintToUint16 проверяет, что значение uint помещается в uint16
func SafeUintToUint16(v uint) (uint16, error) {
	if v > 65535 {
		return 0, errors.New("value exceeds uint16 max")
	}
	return uint16(v), nil
}

