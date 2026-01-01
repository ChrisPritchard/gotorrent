package main

import (
	"crypto/sha1"
	"fmt"
)

// Decodes a torrent file into the relevant properties for further downloading

type TorrentMetadata struct {
	Announcers  []string
	InfoHash    [20]byte
	Name        string
	PieceLength int
	Pieces      []string
	Length      int
	Files       []TorrentFile
}

type TorrentFile struct {
	Path   []string
	Length int
}

func parse_torrent_file(file_data []byte) (TorrentMetadata, error) {
	var nil_torrent TorrentMetadata
	hash, err := get_info_hash(file_data)
	if err != nil {
		return nil_torrent, err
	}
	decoded, _, err := decode_from_bencoded(file_data)
	if err != nil {
		return nil_torrent, err
	}

	root, ok := decoded.(map[string]any)
	if !ok {
		return nil_torrent, fmt.Errorf("invalid torrent: root is not a dict")
	}

	announce, err := get_val[string](root, "announce")
	if err != nil {
		return nil_torrent, fmt.Errorf("invalid torrent: %v", err)
	}
	announcers := []string{announce}

	announce_list, err := get_val[[]any](root, "announce-list")
	if err == nil {
		for _, entry := range announce_list {
			sub_list, ok := entry.([]any)
			if !ok {
				return nil_torrent, fmt.Errorf("invalid announce-list entry: %v", entry)
			}
			for _, sub_entry := range sub_list {
				final_entry, ok := sub_entry.(string)
				if !ok {
					return nil_torrent, fmt.Errorf("invalid announce-list entry: %v", entry)
				}
				announcers = append(announcers, final_entry)
			}
		}
	}

	info, err := get_val[map[string]any](root, "info")
	if err != nil {
		return nil_torrent, fmt.Errorf("invalid torrent: %v", err)
	}

	name, err := get_val[string](info, "name")
	if err != nil {
		return nil_torrent, fmt.Errorf("invalid torrent: %v", err)
	}

	piece_length, err := get_val[int](info, "piece length")
	if err != nil {
		return nil_torrent, fmt.Errorf("invalid torrent: %v", err)
	}

	pieces, err := get_val[string](info, "pieces")
	if err != nil {
		return nil_torrent, fmt.Errorf("invalid torrent: %v", err)
	}
	pieces_parsed := []string{}
	for i := 0; i < len(pieces)/20; i += 20 {
		pieces_parsed = append(pieces_parsed, pieces[i*20:(i+1)*20])
	}

	length, err := get_val[int](info, "length")
	files, err2 := get_val[[]any](info, "files")
	if err != nil && err2 != nil {
		return nil_torrent, fmt.Errorf("invalid torrent: invalid files or missing length")
	}
	file_set := []TorrentFile{}
	if err2 == nil {
		for _, file := range files {
			info, ok := file.(map[string]any)
			if !ok {
				return nil_torrent, fmt.Errorf("invalid torrent: file entries are not valid dictionaries")
			}
			file_length, err := get_val[int](info, "length")
			if err != nil {
				return nil_torrent, fmt.Errorf("invalid torrent: %v", err)
			}
			path, err := get_string_list(info, "path")
			if err != nil {
				return nil_torrent, fmt.Errorf("invalid torrent: %v", err)
			}
			file_set = append(file_set, TorrentFile{
				Length: file_length,
				Path:   path,
			})
		}
	}

	return TorrentMetadata{
		Announcers:  announcers,
		InfoHash:    hash,
		Name:        name,
		PieceLength: piece_length,
		Pieces:      pieces_parsed,
		Length:      length,
		Files:       file_set,
	}, nil
}

func get_val[T any](m map[string]any, key string) (T, error) {
	var nilT T
	val, exists := m[key]
	if !exists {
		return nilT, fmt.Errorf("key %s was not in map", key)
	}
	res, ok := val.(T)
	if !ok {
		return nilT, fmt.Errorf("key %s's value was an invalid type: %v", key, val)
	}
	return res, nil
}

func get_string_list(m map[string]any, key string) ([]string, error) {
	list, err := get_val[[]any](m, key)
	if err != nil {
		return nil, err
	}
	results := []string{}
	for _, v := range list {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("a non-string value was in the list: %v", v)
		}
		results = append(results, string(s))
	}
	return results, nil
}

func get_info_hash(data []byte) ([20]byte, error) {
	data = data[1:]
	var nil_hash [20]byte
	if data[0] == 'e' {
		return nil_hash, fmt.Errorf("no info key found")
	}
	is_key := true
	key := ""
	for {
		n, r, e := decode_from_bencoded(data)
		if e != nil {
			return nil_hash, e
		}
		if is_key {
			k, ok := n.(string)
			if !ok {
				return nil_hash, fmt.Errorf("invalid dictionary - keys should be strings")
			}
			key = k
			is_key = false
		} else if key == "info" {
			sub_set := data[:len(data)-len(r)]
			hash := sha1.Sum([]byte(sub_set))
			return hash, nil
		} else {
			is_key = true
		}
		data = r
		if len(data) == 0 {
			return nil_hash, fmt.Errorf("invalid dictionary - should start with 'd' and end with 'e'")
		}
		if data[0] == 'e' {
			if !is_key {
				return nil_hash, fmt.Errorf("invalid dictionary - an entry is missing a defined value")
			}
			return nil_hash, fmt.Errorf("no info key found")
		}
	}
}
