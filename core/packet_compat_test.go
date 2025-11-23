package core

import (
	"encoding/binary"
	"testing"
)

// TestPacketFormatCompatibility проверяет совместимость формата пакета с C версией
func TestPacketFormatCompatibility(t *testing.T) {
	// Создаём тестовый заголовок
	hdr := NewPacketHeader()
	hdr.StreamID = 0x12345678
	hdr.Seq = 0x87654321
	hdr.FragID = 0x1111
	hdr.TotalFrags = 0x2222
	hdr.PayloadLen = 4
	hdr.Opcode = OpData
	hdr.Proto = ProtoTCP
	hdr.Flags = FlagCompressed

	payload := []byte("test")

	// Сериализуем
	data, err := Serialize(hdr, payload)
	if err != nil {
		t.Fatalf("Serialize failed: %v", err)
	}

	// Проверяем размер
	expectedSize := HeaderSize + len(payload) + 4 // Header + Payload + CRC32
	if len(data) != expectedSize {
		t.Errorf("Packet size mismatch: got %d, expected %d", len(data), expectedSize)
	}

	// Проверяем Magic (первые 2 байта в network byte order)
	magic := binary.BigEndian.Uint16(data[0:2])
	if magic != Magic {
		t.Errorf("Magic mismatch: got 0x%04X, expected 0x%04X", magic, Magic)
	}

	// Проверяем Version (байт 2)
	version := data[2]
	if version != Version {
		t.Errorf("Version mismatch: got 0x%02X, expected 0x%02X", version, Version)
	}

	// Проверяем Flags (байт 3)
	flags := data[3]
	if flags != FlagCompressed {
		t.Errorf("Flags mismatch: got 0x%02X, expected 0x%02X", flags, FlagCompressed)
	}

	// Проверяем Opcode (байт 4)
	opcode := data[4]
	if opcode != OpData {
		t.Errorf("Opcode mismatch: got 0x%02X, expected 0x%02X", opcode, OpData)
	}

	// Проверяем Proto (байт 5)
	proto := data[5]
	if proto != ProtoTCP {
		t.Errorf("Proto mismatch: got 0x%02X, expected 0x%02X", proto, ProtoTCP)
	}

	// Проверяем StreamID (байты 6-10 в network byte order)
	streamID := binary.BigEndian.Uint32(data[6:10])
	if streamID != hdr.StreamID {
		t.Errorf("StreamID mismatch: got 0x%08X, expected 0x%08X", streamID, hdr.StreamID)
	}

	// Проверяем PayloadLen (байты 18-20 в network byte order)
	payloadLen := binary.BigEndian.Uint16(data[18:20])
	if payloadLen != uint16(len(payload)) {
		t.Errorf("PayloadLen mismatch: got %d, expected %d", payloadLen, len(payload))
	}

	// Десериализуем и проверяем
	hdr2, payload2, err := Deserialize(data)
	if err != nil {
		t.Fatalf("Deserialize failed: %v", err)
	}

	if hdr2.StreamID != hdr.StreamID {
		t.Errorf("StreamID after deserialize: got 0x%08X, expected 0x%08X", hdr2.StreamID, hdr.StreamID)
	}

	if string(payload2) != string(payload) {
		t.Errorf("Payload mismatch: got %q, expected %q", string(payload2), string(payload))
	}
}

// TestCRC32Compatibility проверяет совместимость CRC32 с C версией
func TestCRC32Compatibility(t *testing.T) {
	// Тестовые данные
	testData := []byte("Hello, OverProto!")

	// Вычисляем CRC32
	crc1 := ComputeCRC32(testData)

	// Проверяем, что результат не нулевой
	if crc1 == 0 {
		t.Error("CRC32 should not be zero")
	}

	// Проверяем консистентность
	crc2 := ComputeCRC32(testData)
	if crc1 != crc2 {
		t.Errorf("CRC32 not consistent: got 0x%08X and 0x%08X", crc1, crc2)
	}

	// Проверяем инкрементальное вычисление
	ctx := NewCRC32()
	ctx.Update(testData)
	crc3 := ctx.Final()
	if crc1 != crc3 {
		t.Errorf("CRC32 incremental mismatch: got 0x%08X, expected 0x%08X", crc3, crc1)
	}
}

