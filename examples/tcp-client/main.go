package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"overproto-go"
)

func main() {
	var (
		host = flag.String("host", "127.0.0.1", "Server host")
		port = flag.Uint("port", 8080, "Server port")
	)
	flag.Parse()

	// Инициализация библиотеки
	cfg := overproto.NewConfig()
	err := overproto.Init(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}
	defer overproto.Shutdown()

	log.Printf("Connecting to %s:%d...", *host, *port)

	// Подключение к серверу
	conn, err := overproto.TCPConnect(*host, uint16(*port))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	log.Println("Connected successfully!")

	// Обработка сигналов для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Горутина для отправки данных
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		messageNum := 0
		for {
			select {
			case <-ticker.C:
				messageNum++
				data := []byte(fmt.Sprintf("Message #%d from TCP client", messageNum))

				sent, err := overproto.Send(
					conn,
					1,                  // streamID
					overproto.OpData,   // opcode
					overproto.ProtoTCP, // протокол
					data,               // данные
					0,                  // флаги
				)
				if err != nil {
					log.Printf("Failed to send: %v", err)
					return
				}

				log.Printf("Sent %d bytes: %s", sent, string(data))

			case <-sigChan:
				return
			}
		}
	}()

	// Горутина для приёма данных
	tcpConn := overproto.NewTCPConnection(conn)
	go func() {
		for {
			hdr, payload, err := overproto.TCPRecv(tcpConn)
			if err != nil {
				// EOF означает нормальное закрытие соединения сервером
				if err == io.EOF {
					log.Println("Server closed connection")
				} else {
					log.Printf("Failed to receive: %v", err)
				}
				return
			}

			log.Printf("Received: streamID=%d, opcode=%d, payloadLen=%d, data=%s",
				hdr.StreamID, hdr.Opcode, hdr.PayloadLen, string(payload))
		}
	}()

	// Ожидание сигнала завершения
	<-sigChan
	log.Println("Shutting down...")
}
