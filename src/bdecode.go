package main

import (
	"fmt"
	"strconv"
)

// Code to decode a bencoded file, e.g. a torrent file. There are only four datatypes, and its all done around individual bytes (text encoding does not apply here)

func decode_from_bencoded(data []byte) (any, []byte, error) {
	data_len := len(data)

	if data_len == 0 {
		return nil, nil, nil
	}

	switch data[0] {
	case 'i':
		return parse_bencoded_int(data, data_len)
	case 'l':
		return parse_bencoded_list(data, data_len)
	case 'd':
		return parse_bencoded_dict(data, data_len)
	}

	return parse_bencoded_string(data, data_len)
}

func parse_bencoded_dict(data []byte, data_len int) (any, []byte, error) {
	if data_len < 2 {
		return nil, nil, fmt.Errorf("invalid list - should start with 'd' and end with 'e'")
	}
	result := make(map[string]any)
	data = data[1:]
	if data[0] == 'e' {
		return result, data[1:], nil
	}
	is_key := true
	key := ""
	for {
		n, r, e := decode_from_bencoded(data)
		if e != nil {
			return nil, nil, e
		}
		if is_key {
			k, ok := n.(string)
			if !ok {
				return nil, nil, fmt.Errorf("invalid dictionary - keys should be strings")
			}
			key = k
			is_key = false
		} else {
			result[key] = n
			is_key = true
		}
		data = r
		if len(data) == 0 {
			return nil, nil, fmt.Errorf("invalid dictionary - should start with 'd' and end with 'e'")
		}
		if data[0] == 'e' {
			if !is_key {
				return nil, nil, fmt.Errorf("invalid dictionary - an entry is missing a defined value")
			}
			return result, data[1:], nil
		}
	}
}

func parse_bencoded_list(data []byte, data_len int) (any, []byte, error) {
	if data_len < 2 {
		return nil, nil, fmt.Errorf("invalid list - should start with 'l' and end with 'e'")
	}
	result := []any{}
	data = data[1:]
	if data[0] == 'e' {
		return result, data[1:], nil
	}
	for {
		n, r, e := decode_from_bencoded(data)
		if e != nil {
			return nil, nil, e
		}
		result = append(result, n)
		data = r
		if len(data) == 0 {
			return nil, nil, fmt.Errorf("invalid list - should start with 'l' and end with 'e'")
		}
		if data[0] == 'e' {
			return result, data[1:], nil
		}
	}
}

func parse_bencoded_int(data []byte, data_len int) (any, []byte, error) {
	if data_len < 3 || (data[1] == '-' && data_len < 4) {
		return nil, nil, fmt.Errorf("invalid integer - should start with 'i' and end with 'e'")
	}

	s, e := 1, 1
	negative := data[s] == '-'
	if negative {
		s, e = 2, 2
	}

	for data[e] >= '0' && data[e] <= '9' {
		if data_len <= e {
			return nil, nil, fmt.Errorf("invalid integer - should start with 'i' and end with 'e'")
		}
		e++
	}

	if s == e {
		return nil, nil, fmt.Errorf("invalid integer - no number specified")
	} else if data[e] != 'e' {
		return nil, nil, fmt.Errorf("invalid integer - should start with 'i' and end with 'e'")
	} else if data[s] == '0' && (e != s+1 || negative) {
		return nil, nil, fmt.Errorf("invalid integer - cannot start with 0 or be negative 0")
	}

	value, _ := strconv.Atoi(string(data[s:e]))
	if negative {
		value *= -1
	}
	return value, data[e+1:], nil
}

func parse_bencoded_string(data []byte, data_len int) (any, []byte, error) {
	i := 0
	for data[i] >= '0' && data[i] <= '9' {
		if data_len <= i {
			return nil, nil, fmt.Errorf("unrecognised start token")
		}
		i++
	}

	if i == 0 {
		return nil, nil, fmt.Errorf("unrecognised start token")
	} else if data[0] == '0' {
		return nil, nil, fmt.Errorf("invalid string length - starts with 0")
	}

	length, _ := strconv.Atoi(string(data[0:i]))
	if data_len < i+1+length {
		return nil, nil, fmt.Errorf("invalid string length - string len does not match length header")
	}
	if data[i] != ':' {
		return nil, nil, fmt.Errorf("invalid header, missing separator colon")
	}

	s := data[i+1 : i+1+length]
	return string(s), data[i+1+length:], nil
}
