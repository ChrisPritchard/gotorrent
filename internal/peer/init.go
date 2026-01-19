package peer

import (
	"bytes"
	"fmt"
	"net"
	"time"

	"github.com/chrispritchard/gotorrent/internal/torrent"
	"github.com/chrispritchard/gotorrent/internal/tracker"
)

var conn_timeout = 5 * time.Second

func Handshake(metadata torrent.TorrentMetadata, tracker_response tracker.TrackerResponse, peer tracker.PeerInfo) (net.Conn, error) {
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
	copy(to_send[28:48], metadata.InfoHash[:])

	// peer id
	copy(to_send[48:68], tracker_response.LocalID)

	// send to peer
	n, err := conn.Write(to_send)
	if err != nil {
		return nil, err
	} else if n != 68 {
		return nil, fmt.Errorf("was not able to send all 68 bytes of handshake")
	}

	// recieve their response
	received := make([]byte, 68)
	n, err = conn.Read(received)
	if err != nil {
		return nil, err
	} else if n != 68 {
		return nil, fmt.Errorf("did not receive all 68 bytes of handshake")
	}

	// validate response is the mirror of our own
	// fixed header
	if received[0] != 19 || string(received[1:20]) != "BitTorrent protocol" {
		return nil, fmt.Errorf("invalid fixed header in response")
	}

	// info hash
	if !bytes.Equal(received[28:48], metadata.InfoHash[:]) {
		return nil, fmt.Errorf("invalid info hash in response")
	}

	// their peer id (should match what we have for them, if we have it - we dont in the compact version of the tracker response)
	if peer.Id != "" && string(received[48:68]) != peer.Id {
		return nil, fmt.Errorf("invalid peer ID in response")
	}

	return conn, nil
}

func ExchangeBitfields(conn net.Conn, local BitField) (remote BitField, err error) {
	err = send_message(conn, bitfield, local.data)
	if err != nil {
		return
	}

	kind, data, err := receive_message(conn)
	if err != nil {
		return
	}
	if kind != bitfield {
		err = fmt.Errorf("expected a bitfield response message from peer, got %d", kind)
		return
	}

	remote = BitField{data}
	if local.Length() != remote.Length() {
		err = fmt.Errorf("remote bitfield has a different length (%d) than local bitfield (%d)", remote.Length(), local.Length())
	}

	return
}

func SendInterested(conn net.Conn) (err error) {
	err = send_message(conn, interested, []byte{})
	if err != nil {
		return
	}

	kind, _, err := receive_message(conn)
	if err != nil {
		return
	}
	if kind != unchoke {
		err = fmt.Errorf("expected a unchoke response message from peer, got %d", kind)
		return
	}

	return
}
