package peer

import (
	"encoding/binary"
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

func message(conn net.Conn, kind peer_message_type, data []byte) error {
	length := len(data) + 1           // 1 for the message type
	to_send := make([]byte, 4+length) // first four bytes are where we put the length
	binary.BigEndian.PutUint32(to_send, uint32(length))

	to_send[4] = byte(kind)
	copy(to_send[5:], data)

	_, err := conn.Write(to_send)
	return err
}
