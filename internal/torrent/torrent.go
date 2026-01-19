package torrent

import (
	"crypto/sha1"
	"fmt"

	"github.com/chrispritchard/gotorrent/internal/bencode"
)

// Decodes a torrent file into the relevant properties for further downloading

func ParseTorrentFile(file_data []byte) (TorrentMetadata, error) {
	var nil_torrent TorrentMetadata
	hash, err := get_info_hash(file_data)
	if err != nil {
		return nil_torrent, err
	}
	decoded, _, err := bencode.Decode(file_data)
	if err != nil {
		return nil_torrent, err
	}

	root, ok := decoded.(map[string]any)
	if !ok {
		return nil_torrent, fmt.Errorf("invalid torrent: root is not a dict")
	}

	announce, err := bencode.Get[string](root, "announce")
	if err != nil {
		return nil_torrent, fmt.Errorf("invalid torrent: %v", err)
	}
	announcers := []string{announce}

	announce_list, err := bencode.Get[[]any](root, "announce-list")
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

	info, err := bencode.Get[map[string]any](root, "info")
	if err != nil {
		return nil_torrent, fmt.Errorf("invalid torrent: %v", err)
	}

	name, err := bencode.Get[string](info, "name")
	if err != nil {
		return nil_torrent, fmt.Errorf("invalid torrent: %v", err)
	}

	piece_length, err := bencode.Get[int](info, "piece length")
	if err != nil {
		return nil_torrent, fmt.Errorf("invalid torrent: %v", err)
	}

	pieces, err := bencode.Get[string](info, "pieces")
	if err != nil {
		return nil_torrent, fmt.Errorf("invalid torrent: %v", err)
	}
	pieces_parsed := []string{}
	for i := 0; i < len(pieces)/20; i++ {
		pieces_parsed = append(pieces_parsed, pieces[i*20:(i+1)*20])
	}

	length, err := bencode.Get[int](info, "length")
	files, err2 := bencode.Get[[]any](info, "files")
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
			file_length, err := bencode.Get[int](info, "length")
			if err != nil {
				return nil_torrent, fmt.Errorf("invalid torrent: %v", err)
			}
			path, err := bencode.GetStrings(info, "path")
			if err != nil {
				return nil_torrent, fmt.Errorf("invalid torrent: %v", err)
			}
			file_set = append(file_set, TorrentFile{
				Length: file_length,
				Path:   path,
			})
		}
	}

	if length == 0 {
		for _, f := range file_set {
			length += f.Length
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

func get_info_hash(data []byte) ([20]byte, error) {
	data = data[1:]
	var nil_hash [20]byte
	if data[0] == 'e' {
		return nil_hash, fmt.Errorf("no info key found")
	}
	is_key := true
	key := ""
	for {
		n, r, e := bencode.Decode(data)
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
