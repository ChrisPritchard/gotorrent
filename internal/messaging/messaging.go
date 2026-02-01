package messaging

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

func SendMessage(conn net.Conn, kind PeerMessageType, data []byte) error {
	length := len(data) + 1           // 1 for the message type
	to_send := make([]byte, 4+length) // first four bytes are where we put the length
	binary.BigEndian.PutUint32(to_send, uint32(length))

	to_send[4] = byte(kind)
	copy(to_send[5:], data)

	deadline := time.Now().Add(5 * time.Second)
	conn.SetWriteDeadline(deadline)
	defer conn.SetWriteDeadline(time.Time{})

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

	deadline := time.Now().Add(10 * time.Second)
	conn.SetReadDeadline(deadline)
	defer conn.SetReadDeadline(time.Time{})

	length_buffer := make([]byte, 4)
	var length uint32
	for {
		_, err := io.ReadFull(conn, length_buffer)
		if err != nil {
			return nil_received, err
		}
		length = binary.BigEndian.Uint32(length_buffer)
		if length == 0 {
			continue // keep-alive, keep listening
		} else {
			break
		}
	}

	received := make([]byte, length)
	_, err := io.ReadFull(conn, received)
	if err != nil {
		return nil_received, err
	}

	kind := PeerMessageType(received[0])
	if kind > MSG_CANCEL || kind < MSG_CHOKE {
		return nil_received, fmt.Errorf("invalid message type received: %d", kind)
	}

	data := received[1:]
	return Received{kind, data}, nil
}
