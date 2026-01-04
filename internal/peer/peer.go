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

	n, err := conn.Write(to_send)
	if err != nil {
		return nil, err
	} else if n != 68 {
		return nil, fmt.Errorf("was not able to send all 68 bytes of handshake")
	}

	received := make([]byte, 68)
	n, err = conn.Read(received)
	if err != nil {
		return nil, err
	} else if n != 68 {
		return nil, fmt.Errorf("did not receive all 68 bytes of handshake")
	}

	// validate response is the mirror of our own
	if received[0] != 19 || string(received[1:20]) != "BitTorrent protocol" {
		return nil, fmt.Errorf("invalid fixed header in response")
	}

	if !bytes.Equal(received[28:48], metadata.InfoHash[:]) {
		return nil, fmt.Errorf("invalid info hash in response")
	}

	if peer.Id != "" && string(received[48:68]) != peer.Id {
		return nil, fmt.Errorf("invalid peer ID in response")
	}

	return conn, nil
}

func Message() {

}

func RequestSegment() {

}
