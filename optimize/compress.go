package optimize

import (
	"bytes"
	"compress/zlib"
	"errors"
	"io"

	"github.com/nickolajgrishuk/overproto-go/core"
)

// Compress сжимает данные через zlib deflate
// Если сжатие неэффективно (размер увеличился), возвращает ошибку
// Использует уровень компрессии 6
func Compress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	var buf bytes.Buffer

	// Создаём writer с уровнем компрессии 6
	writer, err := zlib.NewWriterLevel(&buf, core.CompressLevel)
	if err != nil {
		return nil, err
	}

	// Записываем данные
	_, err = writer.Write(data)
	if err != nil {
		_ = writer.Close()
		return nil, err
	}

	// Закрываем writer (важно для завершения сжатия)
	err = writer.Close()
	if err != nil {
		return nil, err
	}

	compressed := buf.Bytes()

	// Проверяем эффективность: если размер не уменьшился, возвращаем ошибку
	if len(compressed) >= len(data) {
		return nil, errors.New("compression not effective")
	}

	return compressed, nil
}

// Decompress распаковывает данные через zlib inflate
// Автоматически определяет размер буфера
func Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}

	// Создаём reader
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	// Начальный размер буфера: input_len * 3
	bufSize := len(data) * 3
	if bufSize < 1024 {
		bufSize = 1024
	}

	var result bytes.Buffer
	result.Grow(bufSize)

	// Распаковываем данные с защитой от decompression bomb
	// Максимальный размер декомпрессированных данных: 10MB
	const maxDecompressedSize = 10 * 1024 * 1024
	limitedReader := io.LimitReader(reader, maxDecompressedSize)
	_, err = io.Copy(&result, limitedReader)
	if err != nil {
		return nil, err
	}
	
	// Проверяем, не превышен ли лимит
	if result.Len() >= maxDecompressedSize {
		return nil, errors.New("decompressed data too large (potential decompression bomb)")
	}

	return result.Bytes(), nil
}

// ShouldCompress проверяет, нужна ли компрессия для данных указанного размера
func ShouldCompress(size uint) bool {
	return size >= core.CompressThreshold
}

