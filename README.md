# OverProto Go

High-performance library for network data transmission in Go with support for TCP/UDP, compression, encryption, fragmentation, and reliable delivery.

## Key Features

- **Stream Multiplexing** - support for multiple data streams in a single connection
- **Compression** - automatic compression of large packets via zlib (threshold 512 bytes)
- **Encryption** - data protection via AES-256-GCM
- **Fragmentation** - automatic splitting of large packets for UDP
- **Reliable Transmission** - reliable delivery via UDP (Selective Repeat ARQ)
- **Thread-safe API** - safe usage from multiple threads
- **CRC32 Verification** - packet integrity verification

## Installation

```bash
go get github.com/nickolajgrishuk/overproto-go
```

## Quick Start

### TCP Client

```go
package main

import (
	"log"
	"github.com/nickolajgrishuk/overproto-go"
)

func main() {
	// Initialize the library
	cfg := overproto.NewConfig()
	err := overproto.Init(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer overproto.Shutdown()

	// Connect to server
	conn, err := overproto.TCPConnect("127.0.0.1", 8080)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// Send data
	data := []byte("Hello, OverProto!")
	sent, err := overproto.Send(
		conn,
		1,                          // streamID
		overproto.OpData,           // opcode
		overproto.ProtoTCP,         // protocol
		data,                        // data
		0,                          // flags
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Sent %d bytes", sent)
}
```

### TCP Server

```go
package main

import (
	"fmt"
	"log"
	"net"
	"github.com/nickolajgrishuk/overproto-go"
	"github.com/nickolajgrishuk/overproto-go/core"
	"github.com/nickolajgrishuk/overproto-go/transport"
)

func main() {
	// Initialize the library
	cfg := overproto.NewConfig()
	cfg.TCPPort = 8080
	err := overproto.Init(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer overproto.Shutdown()

	// Set handler for incoming packets
	overproto.SetHandler(func(streamID uint32, opcode uint8, data []byte, ctx interface{}) {
		fmt.Printf("Received %d bytes on stream %d, opcode: %d\n", len(data), streamID, opcode)
	}, nil)

	// Create TCP server
	listener, err := overproto.TCPListen(8080)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	log.Println("TCP server listening on :8080")

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}

		// Handle connection in separate goroutine
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	tcpConn := overproto.NewTCPConnection(conn)

	for {
		hdr, payload, err := overproto.TCPRecv(tcpConn)
		if err != nil {
			log.Printf("Receive error: %v", err)
			return
		}

		// Process packet
		log.Printf("Received packet: streamID=%d, opcode=%d, payloadLen=%d",
			hdr.StreamID, hdr.Opcode, hdr.PayloadLen)

		// Call callback via SetHandler
		// (in a real application this is done automatically)
	}
}
```

### UDP Client with Reliable Delivery

```go
package main

import (
	"log"
	"github.com/nickolajgrishuk/overproto-go"
)

func main() {
	// Initialize
	cfg := overproto.NewConfig()
	err := overproto.Init(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer overproto.Shutdown()

	// Connect via UDP
	conn, err := overproto.UDPConnect("127.0.0.1", 8080)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// Send data with reliability
	data := []byte("Reliable UDP message")
	sent, err := overproto.Send(
		conn,
		1,                          // streamID
		overproto.OpData,           // opcode
		overproto.ProtoUDP,         // protocol
		data,                        // data
		overproto.FlagReliable,     // reliability flag
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Sent %d bytes reliably", sent)
}
```

### Using Encryption

```go
package main

import (
	"crypto/rand"
	"log"
	"github.com/nickolajgrishuk/overproto-go"
)

func main() {
	// Initialize
	cfg := overproto.NewConfig()
	err := overproto.Init(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer overproto.Shutdown()

	// Generate encryption key (32 bytes for AES-256)
	var key [32]byte
	_, err = rand.Read(key[:])
	if err != nil {
		log.Fatal(err)
	}

	// Set key
	err = overproto.SetEncryptionKey(key)
	if err != nil {
		log.Fatal(err)
	}

	// Now all packets with FlagEncrypted flag will be encrypted
	conn, err := overproto.TCPConnect("127.0.0.1", 8080)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	data := []byte("Encrypted message")
	sent, err := overproto.Send(
		conn,
		1,
		overproto.OpData,
		overproto.ProtoTCP,
		data,
		overproto.FlagEncrypted, // encryption flag
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Sent %d encrypted bytes", sent)
}
```

## API Documentation

### Initialization

#### `Init(cfg *Config) error`

Initializes the library. If `cfg == nil`, default values are used.

#### `Shutdown()`

Shuts down the library and releases all resources.

#### `SetHandler(callback RecvCallback, ctx interface{})`

Sets a callback function for handling incoming packets.

### Sending Data

#### `Send(conn interface{}, streamID uint32, opcode, proto uint8, data []byte, flags uint8) (int, error)`

Sends a data packet. Automatically applies compression (if size >= 512 bytes) and encryption (if flag is set).

Parameters:
- `conn` - connection (net.Conn for TCP or *net.UDPConn for UDP)
- `streamID` - stream ID for multiplexing
- `opcode` - operation type (OpData, OpControl, OpACK, OpPing, OpPong)
- `proto` - protocol type (ProtoTCP, ProtoUDP, ProtoHTTP)
- `data` - data to send (maximum 65535 bytes)
- `flags` - packet flags (FlagFragment, FlagCompressed, FlagEncrypted, FlagReliable, FlagACK)

### TCP Functions

#### `TCPListen(port uint16) (net.Listener, error)`

Creates a TCP server on the specified port.

#### `TCPConnect(host string, port uint16) (net.Conn, error)`

Connects to a TCP server.

#### `TCPRecv(conn *TCPConnection) (*PacketHeader, []byte, error)`

Receives a packet via TCP connection.

#### `NewTCPConnection(conn net.Conn) *TCPConnection`

Creates a new TCP connection with state machine for receiving.

### UDP Functions

#### `UDPBind(port uint16) (*net.UDPConn, error)`

Creates a UDP socket bound to a port.

#### `UDPConnect(host string, port uint16) (*net.UDPConn, error)`

Creates a UDP socket connected to a remote address.

#### `UDPRecv(conn *net.UDPConn) (*PacketHeader, []byte, *net.UDPAddr, error)`

Receives a packet via UDP.

### Encryption

#### `SetEncryptionKey(key [32]byte) error`

Sets the AES-256 encryption key.

#### `IsEncryptionEnabled() bool`

Checks if the encryption key is set.

## Constants

### Packet Flags

- `FlagFragment` - packet is a fragment
- `FlagCompressed` - payload is compressed via zlib
- `FlagEncrypted` - payload is encrypted via AES-GCM
- `FlagReliable` - reliable delivery required
- `FlagACK` - packet is an ACK acknowledgment

### Operation Opcodes

- `OpData` - data
- `OpControl` - control packet
- `OpACK` - acknowledgment
- `OpPing` - ping
- `OpPong` - pong

### Protocol Type

- `ProtoTCP` - TCP protocol
- `ProtoUDP` - UDP protocol
- `ProtoHTTP` - HTTP protocol

## Packet Format

An OverProto packet consists of:
- Header (24 bytes) - contains packet metadata
- Payload (0-65535 bytes) - data
- CRC32 (4 bytes) - checksum

All multi-byte fields in the header are transmitted in network byte order (big-endian).
