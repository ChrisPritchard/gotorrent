package main

import (
	"reflect"
	"testing"
)

func TestParseStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		want     any
		want_rem []byte
		want_err bool
	}{
		{
			name:     "basic parse",
			input:    []byte("4:spam"),
			want:     "spam",
			want_rem: []byte{},
			want_err: false,
		},

		{
			name:     "remainder returned",
			input:    []byte("4:spamtest"),
			want:     "spam",
			want_rem: []byte("test"),
			want_err: false,
		},

		{
			name:     "longer parse",
			input:    []byte("10:abcdefghij"),
			want:     "abcdefghij",
			want_rem: []byte{},
			want_err: false,
		},

		{
			name:     "bad length",
			input:    []byte("02:aa"),
			want:     nil,
			want_rem: nil,
			want_err: true,
		},

		{
			name:     "wrong length",
			input:    []byte("2:a"),
			want:     nil,
			want_rem: nil,
			want_err: true,
		},

		{
			name:     "invalid header",
			input:    []byte("4aspam"),
			want:     nil,
			want_rem: nil,
			want_err: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rem, err := decode_from_bencoded(tt.input)
			if (err != nil) != tt.want_err {
				t.Errorf("parse() error = %v, wantErr %v", err, tt.want_err)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parse() = %v, want %v", got, tt.want)
			}

			if !reflect.DeepEqual(rem, tt.want_rem) {
				t.Errorf("parse() = %v, want remainder %v", rem, tt.want_rem)
			}
		})
	}
}

func TestParseIntegers(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		want     any
		want_rem []byte
		want_err bool
	}{
		{
			name:     "basic parse",
			input:    []byte("i1e"),
			want:     1,
			want_rem: []byte{},
			want_err: false,
		},

		{
			name:     "remainder returned",
			input:    []byte("i2etest"),
			want:     2,
			want_rem: []byte("test"),
			want_err: false,
		},

		{
			name:     "longer parse",
			input:    []byte("i33e"),
			want:     33,
			want_rem: []byte{},
			want_err: false,
		},

		{
			name:     "negative parse",
			input:    []byte("i-1e"),
			want:     -1,
			want_rem: []byte{},
			want_err: false,
		},

		{
			name:     "longer negative parse",
			input:    []byte("i-44e"),
			want:     -44,
			want_rem: []byte{},
			want_err: false,
		},

		{
			name:     "bad start",
			input:    []byte("i02e"),
			want:     nil,
			want_rem: nil,
			want_err: true,
		},

		{
			name:     "invalid negative zero",
			input:    []byte("i-0e"),
			want:     nil,
			want_rem: nil,
			want_err: true,
		},

		{
			name:     "no number",
			input:    []byte("ie"),
			want:     nil,
			want_rem: nil,
			want_err: true,
		},

		{
			name:     "no cap",
			input:    []byte("i4"),
			want:     nil,
			want_rem: nil,
			want_err: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rem, err := decode_from_bencoded(tt.input)
			if (err != nil) != tt.want_err {
				t.Errorf("parse() error = %v, wantErr %v", err, tt.want_err)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parse() = %v, want %v", got, tt.want)
			}

			if !reflect.DeepEqual(rem, tt.want_rem) {
				t.Errorf("parse() = %v, want remainder %v", rem, tt.want_rem)
			}
		})
	}
}

func TestParseLists(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		want     any
		want_rem []byte
		want_err bool
	}{
		{
			name:     "empty list",
			input:    []byte("le"),
			want:     []any{},
			want_rem: []byte{},
			want_err: false,
		},

		{
			name:     "list with one integer",
			input:    []byte("li1ee"),
			want:     []any{1},
			want_rem: []byte{},
			want_err: false,
		},

		{
			name:     "list with two integers",
			input:    []byte("li1ei3ee"),
			want:     []any{1, 3},
			want_rem: []byte{},
			want_err: false,
		},

		{
			name:     "list with three mixed",
			input:    []byte("l4:spam3:busi1ee"),
			want:     []any{"spam", "bus", 1},
			want_rem: []byte{},
			want_err: false,
		},

		{
			name:     "empty list remainder",
			input:    []byte("letest"),
			want:     []any{},
			want_rem: []byte("test"),
			want_err: false,
		},

		{
			name:     "list with entries remainder",
			input:    []byte("li1eetest"),
			want:     []any{1},
			want_rem: []byte("test"),
			want_err: false,
		},

		{
			name:     "bad list entry",
			input:    []byte("li-0ee"),
			want:     nil,
			want_rem: nil,
			want_err: true,
		},

		{
			name:     "missing list cap",
			input:    []byte("li0e"),
			want:     nil,
			want_rem: nil,
			want_err: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rem, err := decode_from_bencoded(tt.input)
			if (err != nil) != tt.want_err {
				t.Errorf("parse() error = %v, wantErr %v", err, tt.want_err)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parse() = %v, want %v", got, tt.want)
			}

			if !reflect.DeepEqual(rem, tt.want_rem) {
				t.Errorf("parse() = %v, want remainder %v", rem, tt.want_rem)
			}
		})
	}
}

func TestParseDicts(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		want     any
		want_rem []byte
		want_err bool
	}{
		{
			name:     "empty dict",
			input:    []byte("de"),
			want:     map[string]any{},
			want_rem: []byte{},
			want_err: false,
		},

		{
			name:     "empty dict with remainder",
			input:    []byte("detest"),
			want:     map[string]any{},
			want_rem: []byte("test"),
			want_err: false,
		},

		{
			name:  "dict with one value",
			input: []byte("d4:testi1ee"),
			want: map[string]any{
				"test": 1,
			},
			want_rem: []byte{},
			want_err: false,
		},

		{
			name:  "dict with one value and remainder",
			input: []byte("d4:testi1eetest"),
			want: map[string]any{
				"test": 1,
			},
			want_rem: []byte("test"),
			want_err: false,
		},

		{
			name:  "dict with mixed values",
			input: []byte("d4:testi1e4:spam4:eggs4:listli1ei2ei3eee"),
			want: map[string]any{
				"test": 1,
				"spam": "eggs",
				"list": []any{1, 2, 3},
			},
			want_rem: []byte{},
			want_err: false,
		},

		{
			name:     "dict with an invalid key",
			input:    []byte("di2ei1e4:spam4:eggse"),
			want:     nil,
			want_rem: nil,
			want_err: true,
		},

		{
			name:     "dict missing a value",
			input:    []byte("d4:testi1e4:spam4:eggs4:liste"),
			want:     nil,
			want_rem: nil,
			want_err: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rem, err := decode_from_bencoded(tt.input)
			if (err != nil) != tt.want_err {
				t.Errorf("parse() error = %v, wantErr %v", err, tt.want_err)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parse() = %v, want %v", got, tt.want)
			}

			if !reflect.DeepEqual(rem, tt.want_rem) {
				t.Errorf("parse() = %v, want remainder %v", rem, tt.want_rem)
			}
		})
	}
}
