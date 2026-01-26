package messaging

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

type Received struct {
	Kind PeerMessageType
	Data []byte
}

var nil_received Received

func SendMessage(conn net.Conn, kind PeerMessageType, data []byte) error {
	length := len(data) + 1           // 1 for the message type
	to_send := make([]byte, 4+length) // first four bytes are where we put the length
	binary.BigEndian.PutUint32(to_send, uint32(length))

	to_send[4] = byte(kind)
	copy(to_send[5:], data)

	n, err := conn.Write(to_send)
	if err != nil {
		return err
	}
	if n != len(to_send) {
		return fmt.Errorf("unable to send full message: tried to send %d but actually sent %d", len(to_send), n)
	}
	return err
}

func ReceiveMessage(conn net.Conn) (Received, error) {
	reader := bufio.NewReader(conn)

	length_buffer := make([]byte, 4)
	var length uint32
	for {
		n, err := reader.Read(length_buffer)
		if err != nil {
			return nil_received, err
		}
		if n == 0 {
			// keep_alive received
			continue
		}
		if n == 4 {
			length = binary.BigEndian.Uint32(length_buffer)
			break
		}
		return nil_received, fmt.Errorf("unrecognised bytes received, length %d: %x", n, length_buffer[:n])
	}

	received := make([]byte, length)
	n, err := io.ReadFull(reader, received)
	if err != nil {
		return nil_received, err
	}
	if n != int(length) {
		return nil_received, fmt.Errorf("was expecting %d bytes, but only received %d", length, n)
	}

	kind := PeerMessageType(received[0])
	if kind > MSG_CANCEL || kind < MSG_CHOKE {
		return nil_received, fmt.Errorf("invalid message type received: %d", kind)
	}

	data := received[1:]
	return Received{kind, data}, nil
}
