package bitfields

import (
	"fmt"
	"math"
	"strings"
)

type BitField struct {
	Data   []byte
	Length int
}

func CreateBlankBitfield(length int) BitField {
	b := int(math.Ceil(float64(length) / 8))
	return NewBitfield(make([]byte, b), length)
}

func NewBitfield(data []byte, length int) BitField {
	return BitField{data, length}
}

func (bf *BitField) Set(index uint) error {
	b := index / 8
	if b >= uint(len(bf.Data)) {
		return fmt.Errorf("index is out of range of valid bitfield values")
	}
	m := index % 8
	n := 1
	n = n << (7 - m)
	bf.Data[b] |= byte(n)
	return nil
}

func (bf *BitField) Get(index int) bool {
	if index < 0 {
		return false
	}
	b := index / 8
	if b >= len(bf.Data) {
		return false
	}
	m := index % 8
	n := 1
	n = n << (7 - m)
	res := bf.Data[b] & byte(n)
	return res != 0
}

func (bf *BitField) BitString() string {
	var s strings.Builder
	for i := range bf.Length {
		if bf.Get(i) {
			s.WriteString("1")
		} else {
			s.WriteString("0")
		}
	}
	return s.String()
}

func (bf *BitField) Incomplete() bool {
	for i := range bf.Length {
		if !bf.Get(i) {
			return true
		}
	}
	return false
}
