package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nickolajgrishuk/overproto-go"
)

func main() {
	var (
		host     = flag.String("host", "127.0.0.1", "Server host")
		port     = flag.Uint("port", 8080, "Server port")
		reliable = flag.Bool("reliable", true, "Use reliable UDP")
	)
	flag.Parse()

	// Инициализация библиотеки
	cfg := overproto.NewConfig()
	err := overproto.Init(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}
	defer overproto.Shutdown()

	log.Printf("Connecting to UDP server %s:%d (reliable=%v)...", *host, *port, *reliable)

	// Подключение к серверу
	if *port > 65535 {
		log.Fatalf("Port %d exceeds maximum value 65535", *port)
	}
	conn, err := overproto.UDPConnect(*host, uint16(*port))
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
				data := []byte(fmt.Sprintf("UDP message #%d from client", messageNum))

				flags := uint8(0)
				if *reliable {
					flags |= overproto.FlagReliable
				}

				sent, err := overproto.Send(
					conn,
					1,                    // streamID
					overproto.OpData,      // opcode
					overproto.ProtoUDP,    // протокол
					data,                 // данные
					flags,                // флаги
				)
				if err != nil {
					log.Printf("Failed to send: %v", err)
					return
				}

				log.Printf("Sent %d bytes: %s (reliable=%v)", sent, string(data), *reliable)

			case <-sigChan:
				return
			}
		}
	}()

	// Горутина для приёма данных
	go func() {
		for {
			hdr, payload, addr, err := overproto.UDPRecv(conn)
			if err != nil {
				log.Printf("Failed to receive: %v", err)
				return
			}

			log.Printf("Received from %s: streamID=%d, opcode=%d, payloadLen=%d, data=%s",
				addr, hdr.StreamID, hdr.Opcode, hdr.PayloadLen, string(payload))
		}
	}()

	// Ожидание сигнала завершения
	<-sigChan
	log.Println("Shutting down...")
}

