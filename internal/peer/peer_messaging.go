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

func keep_alive(conn net.Conn) error {
	to_send := []byte{0, 0, 0, 0}
	_, err := conn.Write(to_send)
	return err
}

func send_message(conn net.Conn, kind peer_message_type, data []byte) error {
	length := len(data) + 1           // 1 for the message type
	to_send := make([]byte, 4+length) // first four bytes are where we put the length
	binary.BigEndian.PutUint32(to_send, uint32(length))

	to_send[4] = byte(kind)
	copy(to_send[5:], data)

	_, err := conn.Write(to_send)
	return err
}

func receive_message(conn net.Conn) (peer_message_type, []byte, error) {
	reader := bufio.NewReader(conn)

	len_bytes, err := reader.Peek(4)
	if err != nil {
		return 0, nil, err
	}

	length := binary.BigEndian.Uint32(len_bytes)
	received := make([]byte, 4+length) // we include the 4 bytes we peeked
	n, err := io.ReadFull(reader, received)
	if err != nil {
		return 0, nil, err
	}
	if n != int(length)+4 {
		return 0, nil, fmt.Errorf("was expecting %d bytes, but only received %d", length+4, n)
	}

	kind := peer_message_type(received[4])
	data := received[5:]
	return kind, data, nil
}
