package transport

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/nickolajgrishuk/overproto-go/core"
)

const (
	// WindowSize - размер sliding window (32 пакета)
	WindowSize = 32
	// InitialRTT - начальный RTT в миллисекундах
	InitialRTT = 100
	// InitialCwnd - начальный congestion window
	InitialCwnd = 4
	// MaxCwnd - максимальный congestion window
	MaxCwnd = 32
	// MaxRetries - максимум попыток ретрансмиссии
	MaxRetries = 5
	// FastRetransmitThreshold - порог для Fast Retransmit (дубликаты ACK)
	FastRetransmitThreshold = 3
)

// PacketState - состояние пакета в окне
type PacketState int

const (
	// StateEmpty - слот пуст
	StateEmpty PacketState = iota
	// StateSent - пакет отправлен
	StateSent
	// StateACKed - пакет подтверждён
	StateACKed
	// StateRetransmit - пакет требует ретрансмиссии
	StateRetransmit
)

// WindowSlot - слот в sliding window
type WindowSlot struct {
	Header     *core.PacketHeader
	Data       []byte
	Serialized []byte
	State      PacketState
	SentAt     time.Time
	RetryCount uint32
}

// RTTStats - статистика RTT
type RTTStats struct {
	SRTT        uint32 // Smoothed RTT в миллисекундах
	RTTVar      uint32 // RTT variance
	RTO         uint32 // Retransmission timeout
	SamplesCount int
}

// ReliableContext - контекст надёжной передачи через UDP
type ReliableContext struct {
	conn *net.UDPConn
	addr *net.UDPAddr

	// Sliding window для отправки
	sendWindow [WindowSize]WindowSlot
	sendBase   uint32
	nextSeq    uint32
	windowSize uint32

	// Receive window
	recvBase   uint32
	recvWindow [WindowSize]bool // Bitmap полученных пакетов

	// RTT
	rtt RTTStats

	// Congestion control
	cwnd        uint32
	ssthresh    uint32
	dupACKCount uint32
	lastACKSeq  uint32
	inSlowStart bool

	mu sync.Mutex
}

// NewReliableContext инициализирует контекст надёжной передачи
func NewReliableContext(conn *net.UDPConn, addr *net.UDPAddr) (*ReliableContext, error) {
	ctx := &ReliableContext{
		conn:        conn,
		addr:        addr,
		sendBase:    0,
		nextSeq:     0,
		windowSize:  WindowSize,
		recvBase:    0,
		cwnd:        InitialCwnd,
		ssthresh:    MaxCwnd,
		inSlowStart: true,
	}

	// Инициализируем RTT статистику
	ctx.rtt.SRTT = InitialRTT
	ctx.rtt.RTTVar = InitialRTT / 2
	ctx.rtt.RTO = ctx.rtt.SRTT + 4*ctx.rtt.RTTVar

	return ctx, nil
}

// getWindowIndex возвращает индекс в окне для sequence number
func (ctx *ReliableContext) getWindowIndex(seq uint32) uint32 {
	return seq % WindowSize
}

// isInSendWindow проверяет, находится ли sequence number в окне отправки
func (ctx *ReliableContext) isInSendWindow(seq uint32) bool {
	if ctx.sendBase <= seq && seq < ctx.sendBase+ctx.windowSize {
		return true
	}
	// Обработка переполнения (для uint32 максимальное значение 0xFFFFFFFF)
	if ctx.sendBase+ctx.windowSize < ctx.sendBase {
		return seq >= ctx.sendBase || seq < ctx.sendBase+ctx.windowSize
	}
	return false
}

// isInRecvWindow проверяет, находится ли sequence number в окне приёма
func (ctx *ReliableContext) isInRecvWindow(seq uint32) bool {
	if ctx.recvBase <= seq && seq < ctx.recvBase+ctx.windowSize {
		return true
	}
	// Обработка переполнения (для uint32 максимальное значение 0xFFFFFFFF)
	if ctx.recvBase+ctx.windowSize < ctx.recvBase {
		return seq >= ctx.recvBase || seq < ctx.recvBase+ctx.windowSize
	}
	return false
}

// Send отправляет пакет с надёжностью
// Добавляет в sliding window
// Устанавливает sequence number и флаг FlagReliable
func (ctx *ReliableContext) Send(hdr *core.PacketHeader, payload []byte) error {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	// Проверяем, есть ли место в окне (с учётом congestion window)
	availableSlots := ctx.windowSize - (ctx.nextSeq - ctx.sendBase)
	if availableSlots > ctx.windowSize {
		availableSlots = ctx.windowSize
	}

	if availableSlots == 0 || availableSlots > ctx.cwnd {
		availableSlots = ctx.cwnd
	}

	if ctx.nextSeq-ctx.sendBase >= availableSlots {
		return errors.New("send window full")
	}

	// Присваиваем sequence number
	seq := ctx.nextSeq
	ctx.nextSeq++

	// Создаём копию заголовка
	pktHdr := *hdr
	pktHdr.Seq = seq
	pktHdr.Flags |= core.FlagReliable

	// Сериализуем пакет
	serialized, err := core.Serialize(&pktHdr, payload)
	if err != nil {
		return err
	}

	// Сохраняем в окне
	idx := ctx.getWindowIndex(seq)
	ctx.sendWindow[idx] = WindowSlot{
		Header:     &pktHdr,
		Data:       payload,
		Serialized: serialized,
		State:      StateSent,
		SentAt:     time.Now(),
		RetryCount: 0,
	}

	// Отправляем пакет
	_, err = ctx.conn.WriteToUDP(serialized, ctx.addr)
	if err != nil {
		return err
	}

	return nil
}

// Recv принимает пакет с надёжностью
// Отправляет ACK
// Обрабатывает дубликаты
func (ctx *ReliableContext) Recv() (*core.PacketHeader, []byte, error) {
	// Принимаем пакет через UDP
	hdr, payload, addr, err := UDPRecv(ctx.conn)
	if err != nil {
		return nil, nil, err
	}

	// Проверяем адрес
	if addr.String() != ctx.addr.String() {
		// Игнорируем пакеты от других адресов
		return nil, nil, errors.New("packet from wrong address")
	}

	// Проверяем флаг надёжности
	if hdr.Flags&core.FlagReliable == 0 {
		// Не надёжный пакет - возвращаем как есть
		return hdr, payload, nil
	}

	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	seq := hdr.Seq

	// Проверяем, находится ли sequence number в окне приёма
	if !ctx.isInRecvWindow(seq) {
		// Вне окна - отправляем ACK и игнорируем
		ctx.sendACK(seq)
		return nil, nil, errors.New("sequence number out of receive window")
	}

	// Вычисляем индекс в окне
	idx := ctx.getWindowIndex(seq)

	// Проверяем, не получен ли уже этот пакет (дубликат)
	if ctx.recvWindow[idx] {
		// Дубликат - отправляем ACK и игнорируем
		ctx.sendACK(seq)
		return nil, nil, errors.New("duplicate packet")
	}

	// Сохраняем пакет
	ctx.recvWindow[idx] = true

	// Если это ожидаемый пакет (recvBase), сдвигаем окно
	if seq == ctx.recvBase {
		// Сдвигаем окно вперёд
		for ctx.recvWindow[ctx.getWindowIndex(ctx.recvBase)] {
			ctx.recvWindow[ctx.getWindowIndex(ctx.recvBase)] = false
			ctx.recvBase++
		}
	}

	// Отправляем ACK
	ctx.sendACK(seq)

	return hdr, payload, nil
}

// sendACK отправляет ACK пакет
func (ctx *ReliableContext) sendACK(ackSeq uint32) {
	ackHdr := core.NewPacketHeader()
	ackHdr.Opcode = core.OpACK
	ackHdr.Flags = core.FlagACK | core.FlagReliable
	ackHdr.Seq = ackSeq

	// Отправляем ACK (не ждём подтверждения для ACK)
	serialized, err := core.Serialize(ackHdr, nil)
	if err != nil {
		return
	}

	ctx.conn.WriteToUDP(serialized, ctx.addr)
}

// ProcessACK обрабатывает входящий ACK
// Обновляет sliding window
// Обновляет RTT статистику
// Управляет congestion control
func (ctx *ReliableContext) ProcessACK(ackSeq uint32) error {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	// Проверяем, находится ли ACK в окне отправки
	if !ctx.isInSendWindow(ackSeq) {
		// Вне окна - игнорируем
		return nil
	}

	idx := ctx.getWindowIndex(ackSeq)
	slot := &ctx.sendWindow[idx]

	// Проверяем состояние слота
	if slot.State == StateEmpty || slot.State == StateACKed {
		// Уже обработан или пуст
		return nil
	}

	// Проверяем, является ли это дубликатом ACK
	if ackSeq == ctx.lastACKSeq {
		ctx.dupACKCount++
		if ctx.dupACKCount == FastRetransmitThreshold {
			// Fast Retransmit
			if slot.State == StateSent {
				slot.State = StateRetransmit
				// Ретранслируем немедленно
				ctx.conn.WriteToUDP(slot.Serialized, ctx.addr)
			}
		}
		return nil
	}

	// Новый ACK
	ctx.dupACKCount = 0
	ctx.lastACKSeq = ackSeq

	// Обновляем RTT статистику (только для первого ACK, не для ретрансмиссий)
	if slot.RetryCount == 0 && slot.State == StateSent {
		rtt := uint32(time.Since(slot.SentAt).Milliseconds())
		ctx.updateRTT(rtt)
	}

	// Помечаем пакет как подтверждённый
	slot.State = StateACKed

	// Обновляем congestion window
	ctx.updateCongestionWindow()

	// Сдвигаем окно отправки, если возможно
	for ctx.sendBase < ctx.nextSeq {
		baseIdx := ctx.getWindowIndex(ctx.sendBase)
		if ctx.sendWindow[baseIdx].State == StateACKed {
			ctx.sendWindow[baseIdx] = WindowSlot{} // Очищаем слот
			ctx.sendBase++
		} else {
			break
		}
	}

	return nil
}

// updateRTT обновляет RTT статистику (Karn's algorithm)
func (ctx *ReliableContext) updateRTT(rtt uint32) {
	if ctx.rtt.SamplesCount == 0 {
		// Первый образец
		ctx.rtt.SRTT = rtt
		ctx.rtt.RTTVar = rtt / 2
	} else {
		// Обновляем SRTT и RTTVar
		delta := rtt
		if delta > ctx.rtt.SRTT {
			delta -= ctx.rtt.SRTT
		} else {
			delta = ctx.rtt.SRTT - delta
		}
		ctx.rtt.RTTVar = (3*ctx.rtt.RTTVar + delta) / 4
		ctx.rtt.SRTT = (7*ctx.rtt.SRTT + rtt) / 8
	}

	ctx.rtt.RTO = ctx.rtt.SRTT + 4*ctx.rtt.RTTVar
	ctx.rtt.SamplesCount++
}

// updateCongestionWindow обновляет congestion window
func (ctx *ReliableContext) updateCongestionWindow() {
	if ctx.inSlowStart {
		// Slow Start: экспоненциальный рост
		ctx.cwnd++
		if ctx.cwnd >= ctx.ssthresh {
			ctx.inSlowStart = false
		}
		if ctx.cwnd > MaxCwnd {
			ctx.cwnd = MaxCwnd
		}
	} else {
		// Congestion Avoidance: линейный рост
		ctx.cwnd += 1 / ctx.cwnd // Упрощённая версия
		if ctx.cwnd > MaxCwnd {
			ctx.cwnd = MaxCwnd
		}
	}
}

// ProcessTimeouts обрабатывает таймеры
// Ретранслирует пакеты при timeout
// Возвращает количество ретранслированных пакетов
func (ctx *ReliableContext) ProcessTimeouts() (int, error) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	retransmitted := 0
	now := time.Now()

	// Проверяем все пакеты в окне отправки
	for i := uint32(0); i < ctx.windowSize; i++ {
		seq := ctx.sendBase + i
		if seq >= ctx.nextSeq {
			break
		}

		idx := ctx.getWindowIndex(seq)
		slot := &ctx.sendWindow[idx]

		if slot.State == StateEmpty || slot.State == StateACKed {
			continue
		}

		// Проверяем timeout
		elapsed := uint32(now.Sub(slot.SentAt).Milliseconds())
		if elapsed > ctx.rtt.RTO {
			// Timeout
			if slot.RetryCount >= MaxRetries {
				// Превышен лимит попыток - удаляем из окна
				slot.State = StateEmpty
				continue
			}

			// Ретранслируем пакет
			slot.RetryCount++
			slot.SentAt = now
			slot.State = StateRetransmit

			// Применяем exponential backoff
			backoffRTO := ctx.rtt.RTO
			for j := uint32(0); j < slot.RetryCount; j++ {
				backoffRTO *= 2
			}

			// Уменьшаем congestion window
			ctx.ssthresh = ctx.cwnd / 2
			if ctx.ssthresh < 2 {
				ctx.ssthresh = 2
			}
			ctx.cwnd = InitialCwnd
			ctx.inSlowStart = true

			// Отправляем пакет
			_, err := ctx.conn.WriteToUDP(slot.Serialized, ctx.addr)
			if err != nil {
				return retransmitted, err
			}

			retransmitted++
		}
	}

	return retransmitted, nil
}

