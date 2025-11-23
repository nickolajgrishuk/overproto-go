package core

import (
	"encoding/binary"
	"errors"
	"time"
)

// PacketHeader - заголовок пакета OverProto (24 байта)
// Все multi-byte поля должны быть в network byte order (big-endian) при сериализации
type PacketHeader struct {
	Magic      uint16 // 0xABCD - уникальная сигнатура
	Version    uint8  // 0x01
	Flags      uint8  // Флаги: FRAG|COMP|ENC|RELIABLE|ACK
	Opcode     uint8  // Тип операции: OP_DATA, OP_CONTROL, OP_ACK, OP_PING, OP_PONG
	Proto      uint8  // Тип протокола: OP_PROTO_TCP, OP_PROTO_UDP, OP_PROTO_HTTP
	StreamID   uint32 // ID потока для мультиплексирования
	Seq        uint32 // Порядковый номер пакета
	FragID     uint16 // ID фрагмента (0-based)
	TotalFrags uint16 // Всего фрагментов
	PayloadLen uint16 // Длина payload
	Timestamp  uint32 // Unix timestamp
	CRC32      uint32 // CRC32 (вычисляется, но хранится в заголовке)
}

// ValidateHeader проверяет Magic и Version заголовка
func ValidateHeader(hdr *PacketHeader) error {
	if hdr.Magic != Magic {
		return errors.New("invalid magic number")
	}
	if hdr.Version != Version {
		return errors.New("invalid version")
	}
	return nil
}

// Serialize сериализует пакет в буфер
// Возвращает: [Header 24 bytes] [Payload] [CRC32 4 bytes]
func Serialize(hdr *PacketHeader, payload []byte) ([]byte, error) {
	// Проверка длины payload
	if len(payload) > 65535 {
		return nil, errors.New("payload too large (max 65535 bytes)")
	}

	// Создаём буфер для заголовка
	headerBuf := make([]byte, HeaderSize)
	if len(headerBuf) < HeaderSize {
		return nil, errors.New("header buffer too small")
	}

	// Заполняем заголовок в network byte order (big-endian)
	binary.BigEndian.PutUint16(headerBuf[0:2], hdr.Magic)
	if len(headerBuf) > 2 {
		headerBuf[2] = hdr.Version
	}
	if len(headerBuf) > 3 {
		headerBuf[3] = hdr.Flags
	}
	if len(headerBuf) > 4 {
		headerBuf[4] = hdr.Opcode
	}
	if len(headerBuf) > 5 {
		headerBuf[5] = hdr.Proto
	}
	binary.BigEndian.PutUint32(headerBuf[6:10], hdr.StreamID)
	binary.BigEndian.PutUint32(headerBuf[10:14], hdr.Seq)
	binary.BigEndian.PutUint16(headerBuf[14:16], hdr.FragID)
	binary.BigEndian.PutUint16(headerBuf[16:18], hdr.TotalFrags)
	binary.BigEndian.PutUint16(headerBuf[18:20], hdr.PayloadLen)
	// В C версии поле crc32 в заголовке обнуляется перед копированием в буфер
	// Поэтому в отправленном пакете это поле всегда равно 0
	// В Go версии мы используем Timestamp для этой позиции, но при отправке оно должно быть 0
	binary.BigEndian.PutUint32(headerBuf[20:24], 0) // Обнуляем поле CRC32 (как в C версии: hdr_net.crc32 = 0)

	// Вычисляем CRC32 для (Header + Payload)
	// CRC32 вычисляется для заголовка (где поле CRC32 = 0) + payload
	crcCtx := NewCRC32()
	crcCtx.Update(headerBuf)
	crcCtx.Update(payload)
	crc32Value := crcCtx.Final()

	// В C версии заголовок копируется в буфер с обнуленным полем crc32
	// Поэтому не восстанавливаем Timestamp - поле crc32 должно остаться 0 в отправленном пакете

	// Создаём итоговый буфер
	result := make([]byte, HeaderSize+len(payload)+4)
	copy(result[0:HeaderSize], headerBuf)
	copy(result[HeaderSize:HeaderSize+len(payload)], payload)
	binary.BigEndian.PutUint32(result[HeaderSize+len(payload):], crc32Value)

	return result, nil
}

// Deserialize десериализует пакет из буфера
// Проверяет Magic, Version и CRC32
// Возвращает заголовок, payload и ошибку
func Deserialize(data []byte) (*PacketHeader, []byte, error) {
	// Проверяем минимальный размер (Header + CRC32)
	if len(data) < HeaderSize+4 {
		return nil, nil, errors.New("data too short for packet")
	}

	// Читаем заголовок
	hdr := &PacketHeader{}
	hdr.Magic = binary.BigEndian.Uint16(data[0:2])
	hdr.Version = data[2]
	hdr.Flags = data[3]
	hdr.Opcode = data[4]
	hdr.Proto = data[5]
	hdr.StreamID = binary.BigEndian.Uint32(data[6:10])
	hdr.Seq = binary.BigEndian.Uint32(data[10:14])
	hdr.FragID = binary.BigEndian.Uint16(data[14:16])
	hdr.TotalFrags = binary.BigEndian.Uint16(data[16:18])
	hdr.PayloadLen = binary.BigEndian.Uint16(data[18:20])
	hdr.Timestamp = binary.BigEndian.Uint32(data[20:24])
	// Поле CRC32 в заголовке не используется для хранения CRC32, оно для других целей
	// CRC32 хранится в конце пакета

	// Проверяем Magic и Version
	if err := ValidateHeader(hdr); err != nil {
		return nil, nil, err
	}

	// Читаем payload
	payloadStart := HeaderSize
	payloadEnd := payloadStart + int(hdr.PayloadLen)
	if payloadEnd > len(data)-4 {
		return nil, nil, errors.New("payload length exceeds available data")
	}

	payload := make([]byte, hdr.PayloadLen)
	if hdr.PayloadLen > 0 {
		copy(payload, data[payloadStart:payloadEnd])
	}

	// Читаем CRC32 из конца пакета
	crc32Received := binary.BigEndian.Uint32(data[len(data)-4:])

	// Вычисляем CRC32 для (Header + Payload)
	// В C версии при десериализации CRC32 вычисляется для заголовка из буфера напрямую
	// В отправленном пакете поле crc32 уже равно 0 (было обнулено при сериализации)
	// Поэтому вычисляем CRC32 для заголовка из буфера напрямую (как в C версии)
	crcCtx := NewCRC32()
	crcCtx.Update(data[0:HeaderSize]) // Заголовок из буфера (где crc32 уже = 0)
	crcCtx.Update(payload)
	crc32Computed := crcCtx.Final()

	// Проверяем CRC32
	if crc32Received != crc32Computed {
		return nil, nil, errors.New("CRC32 mismatch")
	}

	return hdr, payload, nil
}

// NewPacketHeader создаёт новый заголовок пакета с заполненными полями по умолчанию
func NewPacketHeader() *PacketHeader {
	unixTime := time.Now().Unix()
	timestamp, _ := SafeInt64ToUint32(unixTime) // В худшем случае будет 0, что приемлемо
	return &PacketHeader{
		Magic:     Magic,
		Version:   Version,
		Timestamp: timestamp,
	}
}

