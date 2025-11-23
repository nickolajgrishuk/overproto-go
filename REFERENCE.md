# OverProto Go API Reference

Complete reference documentation for the OverProto Go library public API.

## Table of Contents

- [Initialization](#initialization)
- [Configuration](#configuration)
- [Sending Data](#sending-data)
- [TCP Functions](#tcp-functions)
- [UDP Functions](#udp-functions)
- [Encryption](#encryption)
- [Types](#types)
- [Constants](#constants)
- [Packet Format](#packet-format)

---

## Initialization

### `Init(cfg *Config) error`

Initializes the OverProto library. Must be called before using any other library functions.

**Parameters:**
- `cfg *Config` - Configuration structure. If `nil`, default values are used.

**Returns:**
- `error` - Returns an error if the library is already initialized or if initialization fails.

**Thread Safety:** Thread-safe. Uses internal mutex for synchronization.

**Example:**
```go
cfg := overproto.NewConfig()
err := overproto.Init(cfg)
if err != nil {
    log.Fatal(err)
}
```

---

### `Shutdown()`

Shuts down the library and releases all resources. Clears encryption keys from memory and resets internal state.

**Thread Safety:** Thread-safe.

**Example:**
```go
defer overproto.Shutdown()
```

---

### `SetHandler(callback RecvCallback, ctx interface{})`

Sets a callback function for handling incoming packets. The callback is invoked automatically when packets are received.

**Parameters:**
- `callback RecvCallback` - Function to be called when a packet is received.
- `ctx interface{}` - Optional context passed to the callback function.

**Thread Safety:** Thread-safe.

**Callback Signature:**
```go
type RecvCallback func(streamID uint32, opcode uint8, data []byte, ctx interface{})
```

**Parameters:**
- `streamID uint32` - Stream identifier for multiplexing.
- `opcode uint8` - Operation code (OpData, OpControl, OpACK, OpPing, OpPong).
- `data []byte` - Packet payload data.
- `ctx interface{}` - Context passed to SetHandler.

**Example:**
```go
overproto.SetHandler(func(streamID uint32, opcode uint8, data []byte, ctx interface{}) {
    fmt.Printf("Received %d bytes on stream %d, opcode: %d\n", len(data), streamID, opcode)
}, nil)
```

---

## Configuration

### `NewConfig() *Config`

Creates a new configuration structure with default values.

**Returns:**
- `*Config` - Configuration structure with default values:
  - `TCPPort: 8080`
  - `UDPPort: 8080`
  - `MTU: 1400`
  - `NonBlocking: false`

**Example:**
```go
cfg := overproto.NewConfig()
cfg.TCPPort = 9000
err := overproto.Init(cfg)
```

---

### `Config` Type

Configuration structure for library initialization.

**Fields:**
- `TCPPort uint16` - Default TCP port for server/listener.
- `UDPPort uint16` - Default UDP port for server/listener.
- `MTU uint` - Maximum Transmission Unit for fragmentation (default: 1400).
- `NonBlocking bool` - Enable non-blocking socket mode (not currently used).

---

## Sending Data

### `Send(conn interface{}, streamID uint32, opcode, proto uint8, data []byte, flags uint8) (int, error)`

Sends a data packet through the specified connection. Automatically applies compression (if payload size >= 512 bytes) and encryption (if flag is set).

**Parameters:**
- `conn interface{}` - Connection object:
  - For TCP: `net.Conn`
  - For UDP: `*net.UDPConn`
- `streamID uint32` - Stream identifier for multiplexing (allows multiple logical streams over one connection).
- `opcode uint8` - Operation code (see [Constants](#constants) section).
- `proto uint8` - Protocol type (ProtoTCP, ProtoUDP, or ProtoHTTP).
- `data []byte` - Payload data to send (maximum 65535 bytes).
- `flags uint8` - Packet flags (see [Constants](#constants) section).

**Returns:**
- `int` - Number of bytes sent (including header, payload, and CRC32).
- `error` - Error if sending fails, library not initialized, or invalid parameters.

**Automatic Features:**
- **Compression:** Automatically compresses payload if size >= 512 bytes and compression flag is not already set.
- **Encryption:** Encrypts payload if `FlagEncrypted` is set (requires encryption key to be set via `SetEncryptionKey`).

**Thread Safety:** Thread-safe (uses read lock).

**Example:**
```go
data := []byte("Hello, OverProto!")
sent, err := overproto.Send(
    conn,
    1,                          // streamID
    overproto.OpData,           // opcode
    overproto.ProtoTCP,         // protocol
    data,                       // data
    overproto.FlagReliable,     // flags
)
if err != nil {
    log.Fatal(err)
}
log.Printf("Sent %d bytes", sent)
```

---

## TCP Functions

### `TCPListen(port uint16) (net.Listener, error)`

Creates a TCP server listener on the specified port. Sets `SO_REUSEADDR` socket option.

**Parameters:**
- `port uint16` - Port number to listen on.

**Returns:**
- `net.Listener` - TCP listener object.
- `error` - Error if binding fails.

**Example:**
```go
listener, err := overproto.TCPListen(8080)
if err != nil {
    log.Fatal(err)
}
defer listener.Close()
```

---

### `TCPAccept(listener net.Listener) (net.Conn, error)`

Accepts a TCP connection from the listener. This is a convenience wrapper around `listener.Accept()`.

**Parameters:**
- `listener net.Listener` - TCP listener returned by `TCPListen`.

**Returns:**
- `net.Conn` - Accepted TCP connection.
- `error` - Error if accept fails.

**Example:**
```go
conn, err := overproto.TCPAccept(listener)
if err != nil {
    log.Printf("Accept error: %v", err)
    continue
}
```

---

### `TCPConnect(host string, port uint16) (net.Conn, error)`

Connects to a TCP server at the specified host and port. Uses a 10-second connection timeout.

**Parameters:**
- `host string` - Server hostname or IP address.
- `port uint16` - Server port number.

**Returns:**
- `net.Conn` - TCP connection object.
- `error` - Error if connection fails.

**Example:**
```go
conn, err := overproto.TCPConnect("127.0.0.1", 8080)
if err != nil {
    log.Fatal(err)
}
defer conn.Close()
```

---

### `NewTCPConnection(conn net.Conn) *TCPConnection`

Creates a new TCP connection wrapper with a state machine for receiving packets. This is required for using `TCPRecv`.

**Parameters:**
- `conn net.Conn` - TCP connection from `TCPConnect` or `TCPAccept`.

**Returns:**
- `*TCPConnection` - TCP connection wrapper with receive state machine.

**Example:**
```go
tcpConn := overproto.NewTCPConnection(conn)
```

---

### `TCPRecv(conn *TCPConnection) (*PacketHeader, []byte, error)`

Receives a complete packet through a TCP connection. Uses a state machine to handle partial reads and ensures complete packet reception.

**Parameters:**
- `conn *TCPConnection` - TCP connection wrapper from `NewTCPConnection`.

**Returns:**
- `*PacketHeader` - Packet header containing metadata.
- `[]byte` - Packet payload data.
- `error` - Error if receive fails (including `io.EOF` on connection close).

**Note:** This function can be called multiple times to read a complete packet. The state machine handles partial reads automatically.

**Example:**
```go
tcpConn := overproto.NewTCPConnection(conn)
for {
    hdr, payload, err := overproto.TCPRecv(tcpConn)
    if err != nil {
        if err == io.EOF {
            log.Println("Connection closed")
        } else {
            log.Printf("Receive error: %v", err)
        }
        return
    }
    
    log.Printf("Received: streamID=%d, opcode=%d, payloadLen=%d",
        hdr.StreamID, hdr.Opcode, hdr.PayloadLen)
}
```

---

## UDP Functions

### `UDPBind(port uint16) (*net.UDPConn, error)`

Creates a UDP socket bound to the specified port. Sets `SO_REUSEADDR` socket option.

**Parameters:**
- `port uint16` - Port number to bind to.

**Returns:**
- `*net.UDPConn` - UDP connection object.
- `error` - Error if binding fails.

**Example:**
```go
conn, err := overproto.UDPBind(8080)
if err != nil {
    log.Fatal(err)
}
defer conn.Close()
```

---

### `UDPConnect(host string, port uint16) (*net.UDPConn, error)`

Creates a UDP socket connected to a remote address. Allows using `Write`/`Read` instead of `WriteToUDP`/`ReadFromUDP`.

**Parameters:**
- `host string` - Remote hostname or IP address.
- `port uint16` - Remote port number.

**Returns:**
- `*net.UDPConn` - Connected UDP connection object.
- `error` - Error if connection fails.

**Example:**
```go
conn, err := overproto.UDPConnect("127.0.0.1", 8080)
if err != nil {
    log.Fatal(err)
}
defer conn.Close()
```

---

### `UDPRecv(conn *net.UDPConn) (*PacketHeader, []byte, *net.UDPAddr, error)`

Receives a packet through UDP. Returns the packet header, payload, and sender address.

**Parameters:**
- `conn *net.UDPConn` - UDP connection from `UDPBind` or `UDPConnect`.

**Returns:**
- `*PacketHeader` - Packet header containing metadata.
- `[]byte` - Packet payload data.
- `*net.UDPAddr` - Address of the packet sender.
- `error` - Error if receive fails.

**Example:**
```go
for {
    hdr, payload, addr, err := overproto.UDPRecv(conn)
    if err != nil {
        log.Printf("Receive error: %v", err)
        continue
    }
    
    log.Printf("Received from %s: streamID=%d, opcode=%d, payloadLen=%d",
        addr.String(), hdr.StreamID, hdr.Opcode, hdr.PayloadLen)
}
```

---

## Encryption

### `SetEncryptionKey(key [32]byte) error`

Sets the global encryption key for AES-256-GCM encryption. The key must be exactly 32 bytes.

**Parameters:**
- `key [32]byte` - 32-byte encryption key for AES-256.

**Returns:**
- `error` - Error if key size is invalid.

**Thread Safety:** Thread-safe.

**Security Note:** The key is stored in memory and cleared when `Shutdown()` is called.

**Example:**
```go
var key [32]byte
_, err := rand.Read(key[:])
if err != nil {
    log.Fatal(err)
}

err = overproto.SetEncryptionKey(key)
if err != nil {
    log.Fatal(err)
}
```

---

### `IsEncryptionEnabled() bool`

Checks if an encryption key is currently set.

**Returns:**
- `bool` - `true` if encryption key is set, `false` otherwise.

**Thread Safety:** Thread-safe.

**Example:**
```go
if overproto.IsEncryptionEnabled() {
    log.Println("Encryption is enabled")
}
```

---

## Types

### `RecvCallback`

Callback function type for handling incoming packets.

**Signature:**
```go
type RecvCallback func(streamID uint32, opcode uint8, data []byte, ctx interface{})
```

**Parameters:**
- `streamID uint32` - Stream identifier.
- `opcode uint8` - Operation code.
- `data []byte` - Packet payload.
- `ctx interface{}` - Context passed to `SetHandler`.

---

### `TCPConnection`

TCP connection wrapper with state machine for packet reception.

**Note:** This type is returned by `NewTCPConnection` and used with `TCPRecv`. It maintains internal state for handling partial packet reads.

**Fields:** (Internal - not directly accessible)

---

### `PacketHeader`

Packet header structure containing packet metadata. This type is defined in the `core` package but is accessible through the public API.

**Fields:**
- `Magic uint16` - Protocol magic number (0xABCD).
- `Version uint8` - Protocol version (0x01).
- `Flags uint8` - Packet flags (see [Constants](#constants)).
- `Opcode uint8` - Operation code (see [Constants](#constants)).
- `Proto uint8` - Protocol type (see [Constants](#constants)).
- `StreamID uint32` - Stream identifier for multiplexing.
- `Seq uint32` - Sequence number for reliable delivery.
- `FragID uint16` - Fragment ID (0-based) for fragmented packets.
- `TotalFrags uint16` - Total number of fragments.
- `PayloadLen uint16` - Length of payload in bytes (0-65535).
- `Timestamp uint32` - Unix timestamp.
- `CRC32 uint32` - CRC32 checksum (computed, not stored in header).

**Note:** All multi-byte fields are transmitted in network byte order (big-endian).

---

## Constants

### Packet Flags

Flags that can be combined using bitwise OR (`|`).

- `FlagFragment = 0x01` - Packet is a fragment of a larger packet.
- `FlagCompressed = 0x02` - Payload is compressed using zlib.
- `FlagEncrypted = 0x04` - Payload is encrypted using AES-256-GCM.
- `FlagReliable = 0x08` - Reliable delivery required (for UDP).
- `FlagACK = 0x10` - Packet is an ACK acknowledgment.

**Example:**
```go
flags := overproto.FlagCompressed | overproto.FlagEncrypted
```

---

### Operation Opcodes

Operation codes indicating the type of packet.

- `OpData = 0x01` - Data packet.
- `OpControl = 0x02` - Control packet.
- `OpACK = 0x03` - Acknowledgment packet.
- `OpPing = 0x04` - Ping packet.
- `OpPong = 0x05` - Pong packet.

---

### Protocol Types

Protocol type constants.

- `ProtoTCP = 0x01` - TCP protocol.
- `ProtoUDP = 0x02` - UDP protocol.
- `ProtoHTTP = 0x03` - HTTP protocol (reserved for future use).

---

## Packet Format

An OverProto packet consists of three parts:

1. **Header** (24 bytes) - Contains packet metadata:
   - Magic number and version
   - Flags and opcode
   - Stream ID and sequence number
   - Fragment information
   - Payload length
   - Timestamp

2. **Payload** (0-65535 bytes) - The actual data:
   - May be compressed (if `FlagCompressed` is set)
   - May be encrypted (if `FlagEncrypted` is set)
   - For encrypted packets: `[IV 12 bytes] [Encrypted data] [Tag 16 bytes]`

3. **CRC32** (4 bytes) - Checksum for header + payload integrity verification

**Serialization:**
- All multi-byte fields in the header are transmitted in network byte order (big-endian).
- CRC32 is computed over the header (with CRC32 field set to 0) + payload.
- The CRC32 value is appended at the end of the packet.

**Total Packet Size:**
- Minimum: 28 bytes (24 header + 0 payload + 4 CRC32)
- Maximum: 65563 bytes (24 header + 65535 payload + 4 CRC32)

---

## Error Handling

Common errors returned by library functions:

- `"not initialized"` - Library not initialized via `Init()`.
- `"already initialized"` - Attempt to initialize library twice.
- `"payload too large (max 65535 bytes)"` - Payload exceeds maximum size.
- `"encryption enabled but key not set"` - `FlagEncrypted` set but no encryption key configured.
- `"invalid connection type for TCP"` - Wrong connection type passed to `Send()` for TCP.
- `"invalid connection type for UDP"` - Wrong connection type passed to `Send()` for UDP.
- `"unsupported protocol"` - Invalid protocol type specified.
- `"CRC32 mismatch"` - Packet integrity check failed.
- `"invalid magic number"` - Packet header validation failed.
- `"invalid version"` - Protocol version mismatch.

---

## Thread Safety

The OverProto library is designed to be thread-safe:

- **Initialization functions** (`Init`, `Shutdown`, `SetHandler`) use internal mutexes.
- **Send function** uses read locks for thread-safe access.
- **Encryption functions** use read/write locks for key access.
- **Connection objects** (`TCPConnection`) maintain their own mutexes for state machine operations.

**Best Practices:**
- Initialize the library once at application startup.
- Each goroutine can safely use its own connection objects.
- Multiple goroutines can safely call `Send()` concurrently.
- Use separate `TCPConnection` objects for each goroutine receiving packets.

---

## Performance Considerations

1. **Automatic Compression:**
   - Compression is applied automatically for payloads >= 512 bytes.
   - Compression uses zlib level 6.
   - If compression is not effective (size doesn't decrease), the original data is sent.

2. **Encryption Overhead:**
   - Encrypted packets include 12-byte IV and 16-byte authentication tag.
   - Total overhead: 28 bytes per encrypted packet.

3. **TCP Receive State Machine:**
   - `TCPRecv` uses a state machine to handle partial reads efficiently.
   - Buffer size: 64 KB (automatically expanded if needed).

4. **UDP Fragmentation:**
   - Large UDP packets should be fragmented manually or use the fragmentation API.
   - Default MTU: 1400 bytes.

---

## Examples

See the `examples/` directory for complete working examples:

- `examples/tcp-client/` - TCP client example
- `examples/tcp-server/` - TCP server example
- `examples/udp-client/` - UDP client example
- `examples/udp-server/` - UDP server example
- `examples/encrypted/` - Encryption usage example

