package peer

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"

	. "github.com/chrispritchard/gotorrent/internal/bitfields"
	"github.com/chrispritchard/gotorrent/internal/messaging"
	"github.com/chrispritchard/gotorrent/internal/tracker"
)

type PeerHandler struct {
	id       string
	bitfield BitField
	conn     net.Conn
}

var nil_peer PeerHandler

func ConnectToPeer(peer tracker.PeerInfo, info_hash, local_id []byte, local_bitfield BitField) (PeerHandler, error) {
	conn, err := handshake(info_hash, local_id, peer)
	if err != nil {
		return nil_peer, err
	}

	field, err := exchange_bitfields(conn, local_bitfield)
	if err != nil {
		return nil_peer, err
	}

	err = send_interested(conn)
	if err != nil {
		return nil_peer, err
	}

	handler := PeerHandler{peer.Id, field, conn}
	return handler, nil
}

func (p *PeerHandler) HasPiece(index int) bool {
	return p.bitfield.Get(index)
}

func (p *PeerHandler) RequestPieceBlock(index, begin, length int) error {
	if !p.HasPiece(index) {
		return fmt.Errorf("peer %s does not have the requested piece with index %d", p.id, index)
	}
	to_send := make([]byte, 12)
	binary.BigEndian.PutUint32(to_send[:4], uint32(index))
	binary.BigEndian.PutUint32(to_send[4:8], uint32(begin))
	binary.BigEndian.PutUint32(to_send[8:], uint32(length))
	return messaging.SendMessage(p.conn, messaging.MSG_REQUEST, to_send)
}

func (p *PeerHandler) StartReceiving(ctx context.Context, received_channnel chan<- messaging.Received, error_channel chan<- error) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				received, err := messaging.ReceiveMessage(p.conn)
				if err != nil {
					error_channel <- err
				} else {
					received_channnel <- received
				}
			}
		}
	}()
}

func (p *PeerHandler) Close() error {
	return p.conn.Close()
}
