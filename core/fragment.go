package core

import (
	"errors"
	"sync"
	"time"
)

// FragmentContext - контекст для сборки фрагментов пакета
type FragmentContext struct {
	StreamID          uint32
	Seq               uint32
	TotalFrags        uint16
	ReceivedFrags     uint16
	Fragments         [256][]byte // Буферы для каждого фрагмента
	FragSizes         [256]uint16 // Размеры фрагментов
	CreatedAt         time.Time
	Header            *PacketHeader
	TotalPayloadSize  uint
	ReceivedPayloadSize uint
	mu                sync.Mutex
}

// NewFragmentContext создаёт контекст для сборки фрагментов
func NewFragmentContext(streamID, seq uint32, totalFrags uint16) *FragmentContext {
	return &FragmentContext{
		StreamID:      streamID,
		Seq:           seq,
		TotalFrags:    totalFrags,
		ReceivedFrags: 0,
		CreatedAt:     time.Now(),
	}
}

// AddFragment добавляет фрагмент в контекст
// Возвращает true если все фрагменты собраны
func (ctx *FragmentContext) AddFragment(fragID uint16, hdr *PacketHeader, data []byte) (bool, error) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	// Проверяем валидность fragID
	if fragID >= ctx.TotalFrags {
		return false, errors.New("invalid fragment ID")
	}

	// Проверяем, не получен ли уже этот фрагмент
	if ctx.Fragments[fragID] != nil {
		// Дубликат - игнорируем
		return false, nil
	}

	// Сохраняем фрагмент
	ctx.Fragments[fragID] = make([]byte, len(data))
	copy(ctx.Fragments[fragID], data)
	ctx.FragSizes[fragID] = uint16(len(data))
	ctx.ReceivedFrags++
	ctx.ReceivedPayloadSize += uint(len(data))

	// Сохраняем заголовок из первого фрагмента
	if fragID == 0 {
		ctx.Header = hdr
	}

	// Проверяем, собраны ли все фрагменты
	if ctx.ReceivedFrags == ctx.TotalFrags {
		return true, nil
	}

	return false, nil
}

// Assemble собирает полный пакет из фрагментов
// Вызывается когда все фрагменты получены
func (ctx *FragmentContext) Assemble() (*PacketHeader, []byte, error) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	// Проверяем, что все фрагменты получены
	if ctx.ReceivedFrags != ctx.TotalFrags {
		return nil, nil, errors.New("not all fragments received")
	}

	// Создаём итоговый payload, склеивая фрагменты в правильном порядке
	totalSize := 0
	for i := uint16(0); i < ctx.TotalFrags; i++ {
		if ctx.Fragments[i] == nil {
			return nil, nil, errors.New("missing fragment")
		}
		totalSize += len(ctx.Fragments[i])
	}

	payload := make([]byte, 0, totalSize)
	for i := uint16(0); i < ctx.TotalFrags; i++ {
		payload = append(payload, ctx.Fragments[i]...)
	}

	// Создаём итоговый заголовок на основе первого фрагмента
	// Убираем флаг фрагментации
	finalHeader := *ctx.Header
	finalHeader.Flags &^= FlagFragment
	finalHeader.FragID = 0
	finalHeader.TotalFrags = 0
	finalHeader.PayloadLen = uint16(len(payload))

	return &finalHeader, payload, nil
}

// IsTimeout проверяет, истёк ли timeout (30 секунд)
func (ctx *FragmentContext) IsTimeout() bool {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	return time.Since(ctx.CreatedAt) > time.Duration(FragTimeoutSec)*time.Second
}

// FragmentPacket фрагментирует пакет на части
// Возвращает массив сериализованных фрагментов и их заголовки
// Если фрагментация не нужна, возвращает nil
func FragmentPacket(hdr *PacketHeader, payload []byte, mtu uint) ([][]byte, []*PacketHeader, error) {
	// Вычисляем максимальный размер payload для одного фрагмента
	// max_frag_payload = MTU - HeaderSize - CRC32Size
	maxFragPayload := mtu - HeaderSize - 4
	if maxFragPayload <= 0 {
		return nil, nil, errors.New("MTU too small for fragmentation")
	}

	payloadSize := uint(len(payload))

	// Если payload помещается в один пакет, фрагментация не нужна
	if payloadSize <= maxFragPayload {
		return nil, nil, nil
	}

	// Вычисляем количество фрагментов
	// total_frags = (payload_size + max_frag_payload - 1) / max_frag_payload
	totalFrags := (payloadSize + maxFragPayload - 1) / maxFragPayload

	// Проверяем максимальное количество фрагментов
	if totalFrags > FragMaxFragments {
		return nil, nil, errors.New("too many fragments required")
	}

	// Создаём фрагменты
	fragments := make([][]byte, 0, totalFrags)
	headers := make([]*PacketHeader, 0, totalFrags)

	for i := uint16(0); i < uint16(totalFrags); i++ {
		// Вычисляем смещение и размер для этого фрагмента
		offset := uint(i) * maxFragPayload
		fragSize := maxFragPayload
		if offset+fragSize > payloadSize {
			fragSize = payloadSize - offset
		}

		// Создаём заголовок для фрагмента
		fragHeader := *hdr
		fragHeader.Flags |= FlagFragment
		fragHeader.FragID = i
		fragHeader.TotalFrags = uint16(totalFrags)
		fragHeader.PayloadLen = uint16(fragSize)

		// Извлекаем payload фрагмента
		fragPayload := payload[offset : offset+fragSize]

		// Сериализуем фрагмент
		serialized, err := Serialize(&fragHeader, fragPayload)
		if err != nil {
			return nil, nil, err
		}

		fragments = append(fragments, serialized)
		headers = append(headers, &fragHeader)
	}

	return fragments, headers, nil
}

