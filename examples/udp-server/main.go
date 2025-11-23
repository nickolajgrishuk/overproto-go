package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"overproto-go"
)

func main() {
	var (
		port = flag.Uint("port", 8080, "Server port")
	)
	flag.Parse()

	// Инициализация библиотеки
	cfg := overproto.NewConfig()
	cfg.UDPPort = uint16(*port)
	err := overproto.Init(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}
	defer overproto.Shutdown()

	// Создание UDP сервера
	conn, err := overproto.UDPBind(uint16(*port))
	if err != nil {
		log.Fatalf("Failed to bind: %v", err)
	}
	defer conn.Close()

	log.Printf("UDP server listening on :%d", *port)

	// Обработка сигналов для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Горутина для отправки периодических сообщений
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		messageNum := 0
		for {
			select {
			case <-ticker.C:
				messageNum++
				// Отправка broadcast сообщения (требует сохранения адресов клиентов)
				log.Printf("Server heartbeat #%d", messageNum)
			case <-sigChan:
				return
			}
		}
	}()

	// Приём пакетов
	for {
		select {
		case <-sigChan:
			log.Println("Shutting down server...")
			return

		default:
			// Устанавливаем timeout для ReadFromUDP
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))

			hdr, payload, addr, err := overproto.UDPRecv(conn)
			if err != nil {
				// Проверяем, это timeout или реальная ошибка
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				log.Printf("Receive error: %v", err)
				continue
			}

			log.Printf("Received from %s: streamID=%d, opcode=%d, payloadLen=%d, data=%s",
				addr, hdr.StreamID, hdr.Opcode, hdr.PayloadLen, string(payload))

			// Эхо-ответ
			echoData := []byte(fmt.Sprintf("Echo: %s", string(payload)))
			flags := hdr.Flags & overproto.FlagReliable // Сохраняем флаг надёжности

			_, err = overproto.Send(
				conn,
				hdr.StreamID,        // тот же streamID
				overproto.OpData,    // opcode
				overproto.ProtoUDP,  // протокол
				echoData,            // данные
				flags,               // флаги
			)
			if err != nil {
				log.Printf("Failed to send echo: %v", err)
			} else {
				log.Printf("Sent echo to %s", addr)
			}
		}
	}
}

