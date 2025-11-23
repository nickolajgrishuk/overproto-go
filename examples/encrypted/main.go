package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nickolajgrishuk/overproto-go"
)

func main() {
	var (
		host = flag.String("host", "127.0.0.1", "Server host")
		port = flag.Uint("port", 8080, "Server port")
		mode = flag.String("mode", "client", "Mode: client or server")
	)
	flag.Parse()

	// Инициализация библиотеки
	cfg := overproto.NewConfig()
	err := overproto.Init(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}
	defer overproto.Shutdown()

	// Генерация ключа шифрования (32 байта для AES-256)
	var key [32]byte
	_, err = rand.Read(key[:])
	if err != nil {
		log.Fatalf("Failed to generate key: %v", err)
	}

	// Установка ключа
	err = overproto.SetEncryptionKey(key)
	if err != nil {
		log.Fatalf("Failed to set encryption key: %v", err)
	}

	log.Println("Encryption enabled with AES-256-GCM")

	if *mode == "server" {
		runServer(uint16(*port))
	} else {
		runClient(*host, uint16(*port))
	}
}

func runServer(port uint16) {
	// Создание TCP сервера
	listener, err := overproto.TCPListen(port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	defer listener.Close()

	log.Printf("Encrypted TCP server listening on :%d", port)

	// Обработка сигналов
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Принятие соединений
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Accept error: %v", err)
				return
			}

			log.Printf("Client connected from %s", conn.RemoteAddr())
			go handleEncryptedConnection(conn)
		}
	}()

	<-sigChan
	log.Println("Shutting down server...")
}

func handleEncryptedConnection(conn net.Conn) {
	defer conn.Close()

	tcpConn := overproto.NewTCPConnection(conn)

	for {
		hdr, payload, err := overproto.TCPRecv(tcpConn)
		if err != nil {
			log.Printf("Receive error: %v", err)
			return
		}

		log.Printf("Received encrypted: streamID=%d, opcode=%d, payloadLen=%d, data=%s",
			hdr.StreamID, hdr.Opcode, hdr.PayloadLen, string(payload))

		// Эхо-ответ с шифрованием
		echoData := []byte(fmt.Sprintf("Encrypted echo: %s", string(payload)))
		_, err = overproto.Send(
			conn,
			hdr.StreamID,
			overproto.OpData,
			overproto.ProtoTCP,
			echoData,
			overproto.FlagEncrypted, // флаг шифрования
		)
		if err != nil {
			log.Printf("Failed to send: %v", err)
		}
	}
}

func runClient(host string, port uint16) {
	// Подключение к серверу
	conn, err := overproto.TCPConnect(host, port)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	log.Println("Connected successfully!")

	// Обработка сигналов
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Горутина для отправки зашифрованных данных
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		messageNum := 0
		for {
			select {
			case <-ticker.C:
				messageNum++
				data := []byte(fmt.Sprintf("Encrypted message #%d", messageNum))

				sent, err := overproto.Send(
					conn,
					1,
					overproto.OpData,
					overproto.ProtoTCP,
					data,
					overproto.FlagEncrypted, // флаг шифрования
				)
				if err != nil {
					log.Printf("Failed to send: %v", err)
					return
				}

				log.Printf("Sent %d encrypted bytes: %s", sent, string(data))

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
				log.Printf("Failed to receive: %v", err)
				return
			}

			log.Printf("Received: streamID=%d, opcode=%d, payloadLen=%d, data=%s",
				hdr.StreamID, hdr.Opcode, hdr.PayloadLen, string(payload))
		}
	}()

	<-sigChan
	log.Println("Shutting down...")
}

