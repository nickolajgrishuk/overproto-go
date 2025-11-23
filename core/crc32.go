package core

// CRC32Context - контекст для вычисления CRC32
type CRC32Context struct {
	crc uint32
}

var (
	// crc32Table - таблица lookup для быстрого вычисления CRC32 IEEE 802.3
	// Полином: 0xEDB88320 (reversed для IEEE 802.3)
	crc32Table [256]uint32
	// crc32TableInit - флаг инициализации таблицы
	crc32TableInit bool
)

// initCRC32Table инициализирует таблицу lookup для CRC32
func initCRC32Table() {
	if crc32TableInit {
		return
	}

	// Полином для IEEE 802.3 (reversed): 0xEDB88320
	poly := uint32(0xEDB88320)

	for i := 0; i < 256; i++ {
		crc := uint32(i) //nolint:gosec // i всегда в диапазоне 0-255, переполнение невозможно
		for j := 0; j < 8; j++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ poly
			} else {
				crc >>= 1
			}
		}
		crc32Table[i] = crc
	}

	crc32TableInit = true
}

// NewCRC32 создаёт новый контекст для вычисления CRC32
// Начальное значение: 0xFFFFFFFF
func NewCRC32() *CRC32Context {
	initCRC32Table()
	return &CRC32Context{
		crc: 0xFFFFFFFF,
	}
}

// Update обновляет CRC32 новыми данными
func (ctx *CRC32Context) Update(data []byte) {
	for _, b := range data {
		ctx.crc = (ctx.crc >> 8) ^ crc32Table[(ctx.crc^uint32(b))&0xFF]
	}
}

// Final возвращает итоговое значение CRC32 с XOR 0xFFFFFFFF
func (ctx *CRC32Context) Final() uint32 {
	return ctx.crc ^ 0xFFFFFFFF
}

// ComputeCRC32 вычисляет CRC32 для блока данных
// Удобная функция для одноразового вычисления
func ComputeCRC32(data []byte) uint32 {
	ctx := NewCRC32()
	ctx.Update(data)
	return ctx.Final()
}

