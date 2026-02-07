package outfiles

import (
	"crypto/sha1"
	"io"
	"os"
	"path/filepath"

	"github.com/chrispritchard/gorrent/internal/bitfields"
	. "github.com/chrispritchard/gorrent/internal/torrent_files"
)

type OutFileManager struct {
	files                      []*os.File
	indices                    []file_indices
	hashes                     []string
	piece_length, total_length int
}

type file_indices struct {
	start_offset, end_offset, file_length int
}

func CreateOutFileManager(metadata TorrentMetadata, base_dir string) (*OutFileManager, error) {
	var files []TorrentFile
	if len(metadata.Files) > 0 {
		files = metadata.Files
	} else {
		files = []TorrentFile{{
			Path:   []string{metadata.Name},
			Length: metadata.Length,
		}}
	}

	out_files := []*os.File{}
	indices := []file_indices{}
	total_length := 0

	offset := 0
	for _, fm := range files {
		f, err := create_file(base_dir, fm.Path, int64(fm.Length))
		if err != nil {
			return nil, err
		}
		out_files = append(out_files, f)

		indices = append(indices, file_indices{offset, offset + fm.Length, fm.Length})
		offset += fm.Length
		total_length += fm.Length
	}

	return &OutFileManager{out_files, indices, metadata.Pieces, metadata.PieceLength, total_length}, nil
}

func (ofm *OutFileManager) Close() {
	for _, f := range ofm.files {
		f.Close()
	}
}

func (ofm *OutFileManager) WriteData(piece, offset int, data []byte) error {
	data_start := (piece * ofm.piece_length) + offset
	data_end := data_start + len(data)

	for i, fi := range ofm.indices {
		if fi.start_offset >= data_end || fi.end_offset <= data_start {
			continue
		}

		file := ofm.files[i]

		overlap_start := max(data_start, fi.start_offset)
		overlap_end := min(data_end, fi.end_offset)
		overlap_len := overlap_end - overlap_start

		if overlap_len <= 0 {
			continue
		}

		read_start := overlap_start - data_start
		write_start := overlap_start - fi.start_offset

		_, err := file.Seek(int64(write_start), io.SeekStart)
		if err != nil {
			return err
		}
		_, err = file.Write(data[read_start : read_start+overlap_len])
		if err != nil {
			return err
		}
	}

	return nil
}

func (ofm *OutFileManager) Bitfield() (*bitfields.BitField, error) {
	bitfield := bitfields.CreateBlankBitfield(len(ofm.hashes))

	for i, h := range ofm.hashes {

		piece_start := i * ofm.piece_length
		piece_end := min(piece_start+ofm.piece_length, ofm.total_length)

		if piece_end == piece_start {
			continue
		}

		piece_data, err := ofm.get_data_range(piece_start, piece_end)
		if err != nil {
			continue
		}

		hash := sha1.Sum(piece_data)
		if string(hash[:]) == h {
			bitfield.Set(uint(i))
		}
	}

	return &bitfield, nil
}

func (ofm *OutFileManager) get_data_range(data_start, data_end int) ([]byte, error) {
	data := make([]byte, data_start-data_end)

	for i, fi := range ofm.indices {
		if fi.start_offset >= data_end || fi.end_offset <= data_start {
			continue
		}

		file := ofm.files[i]

		overlap_start := max(data_start, fi.start_offset)
		overlap_end := min(data_end, fi.end_offset)
		overlap_len := overlap_end - overlap_start

		if overlap_len <= 0 {
			continue
		}

		read_start := overlap_start - data_start
		write_start := overlap_start - fi.start_offset

		_, err := file.Seek(int64(write_start), io.SeekStart)
		if err != nil {
			return nil, err
		}
		segment := make([]byte, overlap_len)
		_, err = io.ReadFull(file, segment)
		if err != nil {
			return nil, err
		}
		copy(data[read_start:read_start+overlap_len], segment)
	}

	return data, nil
}

func create_file(base_dir string, path []string, length int64) (*os.File, error) {
	path = append([]string{base_dir}, path...)
	full_path := filepath.Join(path...)
	dir := filepath.Dir(full_path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	out_file, err := os.Create(full_path)
	if err != nil {
		return nil, err
	}

	err = out_file.Truncate(length) // create full size file
	if err != nil {
		out_file.Close()
		return nil, err
	}

	return out_file, nil
}
