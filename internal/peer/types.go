package peer

import (
	"crypto/sha1"
	"fmt"
	"io"
	"math"
	"net"
	"os"

	"github.com/chrispritchard/gotorrent/internal/tracker"
)

type PeerDetails struct {
	tracker.PeerInfo

	Conn net.Conn

	Chocked    bool
	Interested bool

	Has      BitField
	Requests map[int]struct{}
}

func NewPeerCommunication(p tracker.PeerInfo, conn net.Conn) PeerDetails {
	return PeerDetails{
		PeerInfo:   p,
		Conn:       conn,
		Chocked:    true,
		Interested: false,
		Has:        BitField{},
		Requests:   make(map[int]struct{}),
	}
}

type PeerManager struct {
	Have BitField

	Peers      []PeerDetails
	Requesting map[int]struct{}
}

var block_size = 1 << 14

type PartialPiece struct {
	hash   []byte
	offset int64
	blocks []bool
	data   []byte
}

func CreatePartialPiece(hash []byte, offset, full_length int64) PartialPiece {
	return PartialPiece{
		hash:   hash,
		offset: offset,
		blocks: make([]bool, int(math.Ceil(float64(full_length)/float64(block_size)))),
		data:   make([]byte, full_length),
	}
}

func (pp *PartialPiece) Set(block_index int, data []byte) error {
	if block_index < 0 || block_index >= len(pp.blocks) {
		return fmt.Errorf("invalid block index, out of range")
	}
	if len(data) > block_size {
		return fmt.Errorf("data is too large for a single block")
	}
	pp.blocks[block_index] = true
	target := pp.data[block_index*block_size:]
	if len(target) < len(data) {
		return fmt.Errorf("data is too large for the target location") // should only be possible for the last block if truncated
	}
	copy(pp.data[block_index*block_size:], data)
	return nil
}

func (pp *PartialPiece) Valid() bool {
	for _, b := range pp.blocks {
		if !b {
			return false
		}
	}
	hash := sha1.Sum(pp.data)
	return hash == [20]byte(pp.hash)
}

func (pp *PartialPiece) WritePiece(file *os.File) error {
	if !pp.Valid() {
		return fmt.Errorf("piece is not valid")
	}
	_, err := file.Seek(pp.offset, io.SeekStart)
	if err != nil {
		return err
	}
	_, err = file.Write(pp.data)
	return err
}
