package peer

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sync"

	. "github.com/chrispritchard/gorrent/internal/bitfields"
	"github.com/chrispritchard/gorrent/internal/messaging"
	"github.com/chrispritchard/gorrent/internal/tracker"
)

type PeerHandler struct {
	Id       string
	bitfield BitField
	conn     net.Conn
	mutex    sync.Mutex
	requests map[int]map[int]struct{}
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

	handler := PeerHandler{peer.Id, field, conn, sync.Mutex{}, map[int]map[int]struct{}{}}
	return &handler, nil
}

func (p *PeerHandler) delete_request(index, begin int) bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if blocks, exists := p.requests[index]; exists {
		if _, exists = blocks[begin]; exists {
			delete(blocks, begin)
			if len(blocks) == 0 {
				delete(p.requests, index)
			}
			return true
		}
	}
	return false
}

func (p *PeerHandler) set_request(index, begin int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if blocks, exists := p.requests[index]; exists {
		blocks[begin] = struct{}{}
	} else {
		p.requests[index] = map[int]struct{}{
			begin: {},
		}
	}
}

func (p *PeerHandler) HasPiece(index int) bool {
	return p.bitfield.Get(index)
}

func (p *PeerHandler) CancelRequest(index, begin, length int) error {
	if p.delete_request(index, begin) {
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
	p.set_request(index, begin) // for later cancellation

	to_send := make([]byte, 12)
	binary.BigEndian.PutUint32(to_send[:4], uint32(index))
	binary.BigEndian.PutUint32(to_send[4:8], uint32(begin))
	binary.BigEndian.PutUint32(to_send[8:], uint32(length))
	return messaging.SendMessage(p.conn, messaging.MSG_REQUEST, to_send)
}

func (p *PeerHandler) StartReceiving(ctx context.Context, received_channel chan<- messaging.Received, error_channel chan<- error) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				received, err := messaging.ReceiveMessage(p.conn)
				if err != nil {
					error_channel <- err
					continue
				}

				if received.Kind == messaging.MSG_PIECE {
					index, begin, _ := received.AsPiece()
					p.delete_request(index, begin)
				}

				received_channel <- received
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
	_, err := p.conn.Write([]byte{0, 0, 0, 0})
	return err
}

func (p *PeerHandler) Close() error {
	return p.conn.Close()
}
