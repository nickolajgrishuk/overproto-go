package optimize

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"sync"
)

const (
	// AESKeySize - размер ключа AES-256 (32 байта)
	AESKeySize = 32
	// AESIVSize - размер IV для AES-GCM (12 байт)
	AESIVSize = 12
	// AESGCMTagSize - размер аутентификационного tag (16 байт)
	AESGCMTagSize = 16
)

var (
	// encryptionKey - глобальный ключ шифрования
	encryptionKey []byte
	// keyMutex - мьютекс для thread-safe доступа к ключу
	keyMutex sync.RWMutex
)

// SetEncryptionKey устанавливает глобальный ключ шифрования
// Thread-safe
func SetEncryptionKey(key [32]byte) error {
	if len(key) != AESKeySize {
		return errors.New("invalid key size")
	}

	keyMutex.Lock()
	defer keyMutex.Unlock()

	// Копируем ключ
	encryptionKey = make([]byte, AESKeySize)
	copy(encryptionKey, key[:])

	return nil
}

// IsEncryptionEnabled проверяет, установлен ли ключ шифрования
func IsEncryptionEnabled() bool {
	keyMutex.RLock()
	defer keyMutex.RUnlock()
	return encryptionKey != nil && len(encryptionKey) == AESKeySize
}

// ClearEncryptionKey очищает ключ из памяти (заполняет нулями)
func ClearEncryptionKey() {
	keyMutex.Lock()
	defer keyMutex.Unlock()

	if encryptionKey != nil {
		// Заполняем нулями для безопасности
		for i := range encryptionKey {
			encryptionKey[i] = 0
		}
		encryptionKey = nil
	}
}

// Encrypt шифрует данные через AES-256-GCM
// Возвращает зашифрованные данные и IV
// IV генерируется случайно для каждого шифрования
// Формат результата: [IV 12 bytes] [Encrypted data] [Tag 16 bytes]
func Encrypt(data []byte) ([]byte, []byte, error) {
	keyMutex.RLock()
	key := encryptionKey
	keyMutex.RUnlock()

	if key == nil || len(key) != AESKeySize {
		return nil, nil, errors.New("encryption key not set")
	}

	if len(data) == 0 {
		return nil, nil, errors.New("empty data")
	}

	// Создаём AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	// Создаём GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	// Генерируем случайный IV (12 байт)
	iv := make([]byte, AESIVSize)
	_, err = rand.Read(iv)
	if err != nil {
		return nil, nil, err
	}

	// Шифруем данные
	// Seal автоматически добавляет tag в конец
	encrypted := gcm.Seal(nil, iv, data, nil)

	// Формат: [IV 12 bytes] [Encrypted data] [Tag 16 bytes]
	// Но encrypted уже содержит tag, поэтому просто возвращаем его
	// IV возвращаем отдельно
	return encrypted, iv, nil
}

// Decrypt расшифровывает данные через AES-256-GCM
// Проверяет аутентификационный tag
// encrypted должен содержать зашифрованные данные с tag в конце
// iv - это IV из начала зашифрованных данных
func Decrypt(encrypted []byte, iv []byte) ([]byte, error) {
	keyMutex.RLock()
	key := encryptionKey
	keyMutex.RUnlock()

	if key == nil || len(key) != AESKeySize {
		return nil, errors.New("encryption key not set")
	}

	if len(encrypted) == 0 {
		return nil, errors.New("empty encrypted data")
	}

	if len(iv) != AESIVSize {
		return nil, errors.New("invalid IV size")
	}

	// Проверяем минимальный размер (должен быть хотя бы tag)
	if len(encrypted) < AESGCMTagSize {
		return nil, errors.New("encrypted data too short")
	}

	// Создаём AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Создаём GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Расшифровываем данные
	// Open автоматически проверяет tag
	decrypted, err := gcm.Open(nil, iv, encrypted, nil)
	if err != nil {
		return nil, err
	}

	return decrypted, nil
}

