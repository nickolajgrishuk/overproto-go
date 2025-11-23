package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"overproto-go"
)

var (
	clients     = make(map[net.Conn]bool)
	clientsMu   sync.RWMutex
	clientCount int
)

func main() {
	var (
		port = flag.Uint("port", 8080, "Server port")
	)
	flag.Parse()

	// Инициализация библиотеки
	cfg := overproto.NewConfig()
	cfg.TCPPort = uint16(*port)
	err := overproto.Init(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}
	defer overproto.Shutdown()

	// Установка обработчика входящих пакетов
	overproto.SetHandler(func(streamID uint32, opcode uint8, data []byte, ctx interface{}) {
		log.Printf("Handler: streamID=%d, opcode=%d, dataLen=%d, data=%s",
			streamID, opcode, len(data), string(data))
	}, nil)

	// Создание TCP сервера
	listener, err := overproto.TCPListen(uint16(*port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer listener.Close()

	log.Printf("TCP server listening on :%d", *port)

	// Обработка сигналов для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Горутина для принятия соединений
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Accept error: %v", err)
				return
			}

			clientsMu.Lock()
			clients[conn] = true
			clientCount++
			clientID := clientCount
			clientsMu.Unlock()

			log.Printf("Client #%d connected from %s", clientID, conn.RemoteAddr())

			// Обработка соединения в отдельной горутине
			go handleClient(conn, clientID)
		}
	}()

	// Ожидание сигнала завершения
	<-sigChan
	log.Println("Shutting down server...")

	// Закрытие всех соединений
	clientsMu.Lock()
	for conn := range clients {
		conn.Close()
	}
	clientsMu.Unlock()

	log.Println("Server stopped")
}

func handleClient(conn net.Conn, clientID int) {
	defer func() {
		clientsMu.Lock()
		delete(clients, conn)
		clientsMu.Unlock()
		conn.Close()
		log.Printf("Client #%d disconnected", clientID)
	}()

	tcpConn := overproto.NewTCPConnection(conn)

	// Горутина для отправки периодических сообщений клиенту
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		messageNum := 0
		for {
			select {
			case <-ticker.C:
				messageNum++
				data := []byte(fmt.Sprintf("Server message #%d to client #%d", messageNum, clientID))

				sent, err := overproto.Send(
					conn,
					1,                    // streamID
					overproto.OpData,     // opcode
					overproto.ProtoTCP,   // протокол
					data,                 // данные
					0,                    // флаги
				)
				if err != nil {
					log.Printf("Failed to send to client #%d: %v", clientID, err)
					return
				}

				log.Printf("Sent %d bytes to client #%d", sent, clientID)
			}
		}
	}()

	// Приём пакетов от клиента
	for {
		hdr, payload, err := overproto.TCPRecv(tcpConn)
		if err != nil {
			// EOF означает нормальное закрытие соединения клиентом
			if err == io.EOF {
				log.Printf("Client #%d disconnected (EOF)", clientID)
			} else {
				log.Printf("Client #%d receive error: %v", clientID, err)
			}
			return
		}

		log.Printf("Client #%d: streamID=%d, opcode=%d, payloadLen=%d, data=%s",
			clientID, hdr.StreamID, hdr.Opcode, hdr.PayloadLen, string(payload))

		// Эхо-ответ
		echoData := []byte(fmt.Sprintf("Echo: %s", string(payload)))
		_, err = overproto.Send(
			conn,
			hdr.StreamID,        // тот же streamID
			overproto.OpData,    // opcode
			overproto.ProtoTCP,  // протокол
			echoData,            // данные
			0,                   // флаги
		)
		if err != nil {
			log.Printf("Failed to send echo to client #%d: %v", clientID, err)
		}
	}
}

