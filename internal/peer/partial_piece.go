package peer

import (
	"crypto/sha1"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
)

var BLOCK_SIZE = 1 << 14

type PartialPiece struct {
	hash        string
	offset      int
	blocks      []bool
	block_sizes []int
	data        []byte
	Done        bool
	mutex       sync.Mutex
}

func CreatePartialPieces(hashes []string, piece_length, total_pieces_length int) []*PartialPiece {
	total_pieces := len(hashes)
	result := make([]*PartialPiece, total_pieces)
	last_size := total_pieces_length % piece_length
	for i := range total_pieces {
		length := piece_length
		if i == len(hashes)-1 && last_size != 0 {
			length = last_size
		}
		result[i] = new_partial_piece(hashes[i], i*piece_length, length)
	}
	return result
}

func new_partial_piece(hash string, offset, full_length int) *PartialPiece {
	block_count := int(math.Ceil(float64(full_length) / float64(BLOCK_SIZE)))
	last_size := full_length % BLOCK_SIZE
	sizes := make([]int, block_count)
	for i := range sizes {
		if i == len(sizes)-1 && last_size != 0 {
			sizes[i] = last_size
		} else {
			sizes[i] = BLOCK_SIZE
		}
	}
	return &PartialPiece{
		hash:        hash,
		offset:      offset,
		blocks:      make([]bool, block_count),
		block_sizes: sizes,
		data:        make([]byte, full_length),
		Done:        false,
		mutex:       sync.Mutex{},
	}
}

// Length returns the number of blocks in this piece, filled or otherwise
func (pp *PartialPiece) Length() int {
	return len(pp.blocks)
}

func (pp *PartialPiece) BlockSize(index int) int {
	return pp.block_sizes[index]
}

func (pp *PartialPiece) Set(offset int, data []byte) error {
	pp.mutex.Lock()
	defer pp.mutex.Unlock()
	block_index := offset / BLOCK_SIZE
	if block_index < 0 || block_index >= len(pp.blocks) {
		return fmt.Errorf("invalid block index, out of range")
	}
	if len(data) > BLOCK_SIZE {
		return fmt.Errorf("data is too large for a single block")
	}
	pp.blocks[block_index] = true
	target := pp.data[block_index*BLOCK_SIZE:]
	if len(target) < len(data) {
		return fmt.Errorf("data is too large for the target location") // should only be possible for the last block if truncated
	}
	copy(pp.data[block_index*BLOCK_SIZE:], data)
	return nil
}

func (pp *PartialPiece) Valid() bool {
	pp.mutex.Lock()
	defer pp.mutex.Unlock()
	for _, b := range pp.blocks {
		if !b {
			return false
		}
	}
	hash := sha1.Sum(pp.data)
	return string(hash[:]) == pp.hash
}

// Missing returns the index of missing blocks
func (pp *PartialPiece) Missing() []int {
	pp.mutex.Lock()
	defer pp.mutex.Unlock()
	missing := []int{}
	for i, b := range pp.blocks {
		if !b {
			missing = append(missing, i)
		}
	}
	return missing
}

func (pp *PartialPiece) WritePiece(file *os.File) error {
	if !pp.Valid() {
		return fmt.Errorf("piece is not valid")
	}
	pp.mutex.Lock()
	defer pp.mutex.Unlock()
	_, err := file.Seek(int64(pp.offset), io.SeekStart)
	if err != nil {
		return err
	}
	_, err = file.Write(pp.data)
	pp.Done = true
	clear(pp.data)
	return err
}
