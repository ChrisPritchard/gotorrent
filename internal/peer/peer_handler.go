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
	Id       string
	bitfield BitField
	conn     net.Conn
	requests RequestMap
}

func ConnectToPeer(peer tracker.PeerInfo, info_hash, local_id []byte, local_bitfield BitField) (*PeerHandler, error) {
	conn, err := handshake(info_hash, local_id, peer)
	if err != nil {
		return nil, err
	}

	field, err := exchange_bitfields(conn, local_bitfield)
	if err != nil {
		return nil, err
	}

	err = send_interested(conn)
	if err != nil {
		return nil, err
	}

	err = receive_unchoked(conn)
	if err != nil {
		return nil, err
	}

	handler := PeerHandler{peer.Id, field, conn, CreateEmptyRequestMap()}
	return &handler, nil
}

func (p *PeerHandler) HasPiece(index int) bool {
	return p.bitfield.Get(index)
}

func (p *PeerHandler) CancelRequest(index, begin, length int) error {
	if p.requests.Has(index, begin) {
		p.requests.Delete(index, begin)
		to_send := make([]byte, 12)
		binary.BigEndian.PutUint32(to_send[:4], uint32(index))
		binary.BigEndian.PutUint32(to_send[4:8], uint32(begin))
		binary.BigEndian.PutUint32(to_send[8:], uint32(length))
		return messaging.SendMessage(p.conn, messaging.MSG_CANCEL, to_send)
	}
	return nil
}

func (p *PeerHandler) RequestPieceBlock(index, begin, length int) error {
	if !p.HasPiece(index) {
		return fmt.Errorf("peer %s does not have the requested piece with index %d", p.Id, index)
	}
	p.requests.Set(index, begin) // for later cancellation

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
				} else if received.Kind == messaging.MSG_PIECE {
					index, begin, _ := received.AsPiece()
					p.requests.Delete(index, begin)
				} else {
					received_channnel <- received
				}
			}
		}
	}()
}

func (p *PeerHandler) SendHave(piece_index int) error {
	to_send := make([]byte, 4)
	binary.BigEndian.PutUint32(to_send, uint32(piece_index))
	return messaging.SendMessage(p.conn, messaging.MSG_HAVE, to_send)
}

func (p *PeerHandler) SendKeepAlive() error {
	_, err := p.conn.Write([]byte{})
	return err
}

func (p *PeerHandler) Close() error {
	return p.conn.Close()
}
