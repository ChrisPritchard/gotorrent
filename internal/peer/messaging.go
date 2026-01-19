package peer

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

type peer_message_type int

const (
	choke peer_message_type = iota
	unchoke
	interested
	not_interested
	have
	bitfield
	request
	piece
	cancel
)

// func keep_alive(conn net.Conn) error {
// 	to_send := []byte{0, 0, 0, 0}
// 	_, err := conn.Write(to_send)
// 	return err
// }

// func keep_alive(conn net.Conn) error {
// 	_, err := conn.Write([]byte{})
// 	return err
// }

func send_message(conn net.Conn, kind peer_message_type, data []byte) error {
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

func receive_message(conn net.Conn) (peer_message_type, []byte, error) {
	reader := bufio.NewReader(conn)

	length_buffer := make([]byte, 4)
	var length uint32
	for {
		n, err := reader.Read(length_buffer)
		if err != nil {
			return 0, nil, err
		}
		if n == 0 {
			// keep_alive received
			continue
		}
		if n == 4 {
			length = binary.BigEndian.Uint32(length_buffer)
			break
		}
		return 0, nil, fmt.Errorf("unrecognised bytes received, length %d: %x", n, length_buffer[:n])
	}

	received := make([]byte, length)
	n, err := io.ReadFull(reader, received)
	if err != nil {
		return 0, nil, err
	}
	if n != int(length) {
		return 0, nil, fmt.Errorf("was expecting %d bytes, but only received %d", length, n)
	}

	kind := peer_message_type(received[0])
	if kind > cancel || kind < choke {
		return 0, nil, fmt.Errorf("invalid message type received: %d", kind)
	}

	data := received[1:]
	return kind, data, nil
}
