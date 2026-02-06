package peer

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"time"

	. "github.com/chrispritchard/gorrent/internal/bitfields"
	. "github.com/chrispritchard/gorrent/internal/messaging"
	"github.com/chrispritchard/gorrent/internal/tracker"
)

var conn_timeout = 5 * time.Second

func handshake(info_hash, local_id []byte, peer tracker.PeerInfo) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", peer.IP, peer.Port), conn_timeout)
	if err != nil {
		return nil, err
	}

	to_send := make([]byte, 68) // fixed header, fixed bytes, info hash, peer id

	// fixed header
	to_send[0] = 19 // length of following string
	copy(to_send[1:20], []byte("BitTorrent protocol"))

	// fixed bytes
	copy(to_send[20:28], []byte{0, 0, 0, 0, 0, 0, 0, 0})

	// info hash
	copy(to_send[28:48], info_hash)

	// peer id
	copy(to_send[48:68], local_id)

	// send to peer
	n, err := conn.Write(to_send)
	if err != nil {
		return nil, err
	} else if n != 68 {
		return nil, fmt.Errorf("was not able to send all 68 bytes of handshake")
	}

	// recieve their response
	received := make([]byte, 68)
	_, err = io.ReadFull(conn, received)
	if err != nil {
		return nil, err
	}

	// validate response is the mirror of our own
	// fixed header
	if received[0] != 19 || string(received[1:20]) != "BitTorrent protocol" {
		return nil, fmt.Errorf("invalid fixed header in response")
	}

	// info hash
	if !bytes.Equal(received[28:48], info_hash) {
		return nil, fmt.Errorf("invalid info hash in response")
	}

	// their peer id (should match what we have for them, if we have it - we dont in the compact version of the tracker response)
	if peer.Id != "" && string(received[48:68]) != peer.Id {
		return nil, fmt.Errorf("invalid peer ID in response")
	}

	return conn, nil
}

func exchange_bitfields(conn net.Conn, local BitField) (remote BitField, err error) {
	err = SendMessage(conn, MSG_BITFIELD, local.Data)
	if err != nil {
		return
	}

	received, err := ReceiveMessage(conn)
	if err != nil {
		return
	}
	if received.Kind != MSG_BITFIELD {
		err = fmt.Errorf("expected a bitfield response message from peer, got %d", received.Kind)
		return
	}

	remote = BitField{Data: received.Data}
	if len(local.Data) != len(remote.Data) {
		err = fmt.Errorf("remote bitfield has a different length (%d) than local bitfield (%d)", len(remote.Data), len(local.Data))
	}

	return
}

func send_interested(conn net.Conn) error {
	return SendMessage(conn, MSG_INTERESTED, []byte{})
}

func receive_unchoked(conn net.Conn) error {
	received, err := ReceiveMessage(conn)
	if err != nil {
		return err
	}

	if received.Kind != MSG_UNCHOKE {
		err = fmt.Errorf("expected a unchoke response message from peer, got %d", received.Kind)
		return err
	}

	return nil
}
