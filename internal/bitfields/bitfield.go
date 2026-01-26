package bitfields

import (
	"fmt"
	"math"
)

type BitField struct {
	Data []byte
}

func CreateBlankBitfield(length int) BitField {
	b := int(math.Ceil(float64(length) / 8))
	return NewBitfield(make([]byte, b))
}

func NewBitfield(data []byte) BitField {
	return BitField{data}
}

func (bf *BitField) Set(index uint) error {
	b := index / 8
	if b >= uint(len(bf.Data)) {
		return fmt.Errorf("index is out of range of valid bitfield values")
	}
	m := index % 8
	n := 1
	n = n << m
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
	n = n << m
	res := bf.Data[b] & byte(n)
	return res != 0
}
