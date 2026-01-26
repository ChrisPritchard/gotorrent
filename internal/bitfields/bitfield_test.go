package bitfields

import (
	"reflect"
	"testing"
)

func TestBitfieldSet(t *testing.T) {
	tests := []struct {
		name     string
		state    []byte
		index    uint
		want     []byte
		want_err bool
	}{
		{
			name:     "single byte set",
			state:    []byte{0},
			index:    4,
			want:     []byte{0b00010000},
			want_err: false,
		},

		{
			name:     "single byte reset",
			state:    []byte{0b00010000},
			index:    4,
			want:     []byte{0b00010000},
			want_err: false,
		},

		{
			name:     "single byte invalid 1",
			state:    []byte{0},
			index:    8,
			want:     []byte{},
			want_err: true,
		},

		{
			name:     "single byte invalid 2",
			state:    []byte{0},
			index:    453,
			want:     []byte{},
			want_err: true,
		},

		{
			name:     "multi byte set",
			state:    []byte{0, 0, 0},
			index:    12,
			want:     []byte{0, 0b00010000, 0},
			want_err: false,
		},

		{
			name:     "multi byte invalid",
			state:    []byte{0, 0, 0},
			index:    153,
			want:     []byte{},
			want_err: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			bf := BitField{Data: tt.state}

			err := bf.Set(tt.index)
			if (err != nil) != tt.want_err {
				t.Errorf("parse() error = %v, wantErr %v", err, tt.want_err)
				return
			}

			if err == nil && !reflect.DeepEqual(bf.Data, tt.want) {
				t.Errorf("parse() = %v, want %v", bf.Data, tt.want)
			}
		})
	}
}

func TestBitfieldGet(t *testing.T) {
	tests := []struct {
		name  string
		state []byte
		index int
		want  bool
	}{
		{
			name:  "single byte get true",
			state: []byte{0b00010000},
			index: 4,
			want:  true,
		},

		{
			name:  "single byte get false",
			state: []byte{0},
			index: 4,
			want:  false,
		},

		{
			name:  "single byte invalid 1",
			state: []byte{0},
			index: 8,
			want:  false,
		},

		{
			name:  "single byte invalid 2",
			state: []byte{0},
			index: 453,
			want:  false,
		},

		{
			name:  "multi byte get true",
			state: []byte{0, 0b00010000, 0},
			index: 12,
			want:  true,
		},

		{
			name:  "multi byte get false",
			state: []byte{0, 0, 0},
			index: 12,
			want:  false,
		},

		{
			name:  "multi byte invalid",
			state: []byte{0, 0, 0},
			index: 153,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			bf := BitField{Data: tt.state}

			r := bf.Get(tt.index)

			if r != tt.want {
				t.Errorf("parse() = %v, want %v", bf.Data, tt.want)
			}
		})
	}
}
