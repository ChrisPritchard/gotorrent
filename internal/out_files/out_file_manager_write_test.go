package outfiles

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// Helper to create temp files
func setupTestFiles(t *testing.T, fileSizes []int) (*OutFileManager, func()) {
	t.Helper()
	tempDir := t.TempDir()

	indices := make([]file_indices, len(fileSizes))
	files := make([]*os.File, len(fileSizes))

	var offset int
	for i, size := range fileSizes {
		path := filepath.Join(tempDir, fmt.Sprintf("file%d.txt", i))
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}

		// Pre-fill with zeros
		_, err = f.Write(make([]byte, size))
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.Seek(0, io.SeekStart)
		if err != nil {
			t.Fatal(err)
		}

		indices[i] = file_indices{
			start_offset: offset,
			end_offset:   offset + size,
			file_length:  size,
		}
		files[i] = f
		offset += size
	}

	cleanup := func() {
		for _, f := range files {
			f.Close()
		}
	}

	return &OutFileManager{
		piece_length: 16384, // 16KB standard piece size
		indices:      indices,
		files:        files,
	}, cleanup
}

// Helper to read file content
func readFileContent(f *os.File) ([]byte, error) {
	_, err := f.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(f)
}

func TestWritePiece_SingleFile(t *testing.T) {
	ofm, cleanup := setupTestFiles(t, []int{16384}) // Exactly one piece size
	defer cleanup()

	// Write a full piece to the file
	data := make([]byte, 16384)
	for i := range data {
		data[i] = byte(i % 256)
	}

	err := ofm.WritePiece(0, data)
	if err != nil {
		t.Fatalf("WritePiece failed: %v", err)
	}

	content, err := readFileContent(ofm.files[0])
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, data) {
		t.Errorf("File content mismatch. Expected full piece data")
	}
}

func TestWritePiece_ExactFileBoundary(t *testing.T) {
	// Create two files that together equal exactly one piece
	ofm, cleanup := setupTestFiles(t, []int{8192, 8192}) // 8KB + 8KB = 16KB (one piece)
	defer cleanup()

	// Write a full piece that spans both files
	data := make([]byte, 16384)
	for i := range data {
		data[i] = byte(i % 256)
	}

	err := ofm.WritePiece(0, data)
	if err != nil {
		t.Fatalf("WritePiece failed: %v", err)
	}

	// Check first file (first 8192 bytes)
	content1, _ := readFileContent(ofm.files[0])
	if !bytes.Equal(content1, data[:8192]) {
		t.Errorf("File 1 content mismatch")
	}

	// Check second file (next 8192 bytes)
	content2, _ := readFileContent(ofm.files[1])
	if !bytes.Equal(content2, data[8192:]) {
		t.Errorf("File 2 content mismatch")
	}
}

func TestWritePiece_SpanningMultipleFiles(t *testing.T) {
	// Create three files that together span a piece
	ofm, cleanup := setupTestFiles(t, []int{5000, 5000, 6384}) // Total = 16384 (one piece)
	defer cleanup()

	// Write a full piece spanning all 3 files
	data := make([]byte, 16384)
	for i := range data {
		data[i] = byte(i % 256)
	}

	err := ofm.WritePiece(0, data)
	if err != nil {
		t.Fatalf("WritePiece failed: %v", err)
	}

	// Verify each file got the correct portion
	content1, _ := readFileContent(ofm.files[0])
	if !bytes.Equal(content1, data[:5000]) {
		t.Errorf("File 1 content mismatch")
	}

	content2, _ := readFileContent(ofm.files[1])
	if !bytes.Equal(content2, data[5000:10000]) {
		t.Errorf("File 2 content mismatch")
	}

	content3, _ := readFileContent(ofm.files[2])
	if !bytes.Equal(content3, data[10000:]) {
		t.Errorf("File 3 content mismatch")
	}
}

func TestWritePiece_PartialPieceAtEnd(t *testing.T) {
	// Test writing a partial piece (last piece of torrent)
	ofm, cleanup := setupTestFiles(t, []int{10000}) // File smaller than piece size
	defer cleanup()

	// Write data that's smaller than piece size (simulating last piece)
	data := make([]byte, 10000)
	for i := range data {
		data[i] = byte(i % 256)
	}

	err := ofm.WritePiece(0, data)
	if err != nil {
		t.Fatalf("WritePiece failed: %v", err)
	}

	content, err := readFileContent(ofm.files[0])
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(content, data) {
		t.Errorf("File content mismatch for partial piece")
	}
}

func TestWritePiece_MultiplePieces(t *testing.T) {
	// Create a file large enough for multiple pieces
	ofm, cleanup := setupTestFiles(t, []int{32768}) // 32KB = 2 pieces
	defer cleanup()

	// Write first piece
	data1 := make([]byte, 16384)
	for i := range data1 {
		data1[i] = byte(i % 256)
	}

	err := ofm.WritePiece(0, data1)
	if err != nil {
		t.Fatalf("WritePiece failed for piece 0: %v", err)
	}

	// Write second piece
	data2 := make([]byte, 16384)
	for i := range data2 {
		data2[i] = byte((i + 100) % 256) // Different pattern
	}

	err = ofm.WritePiece(1, data2)
	if err != nil {
		t.Fatalf("WritePiece failed for piece 1: %v", err)
	}

	content, err := readFileContent(ofm.files[0])
	if err != nil {
		t.Fatal(err)
	}

	// Check combined content
	expected := append(data1, data2...)
	if !bytes.Equal(content, expected) {
		t.Errorf("File content mismatch for multiple pieces")
	}
}

func TestWritePiece_NoOverlap(t *testing.T) {
	ofm, cleanup := setupTestFiles(t, []int{100})
	defer cleanup()

	// Write piece 1 (starts at offset 16384, which is beyond our 100-byte file)
	data := make([]byte, 16384)
	for i := range data {
		data[i] = byte(i % 256)
	}

	err := ofm.WritePiece(1, data)
	if err != nil {
		t.Fatalf("WritePiece failed: %v", err)
	}

	// File should remain unchanged (all zeros)
	content, _ := readFileContent(ofm.files[0])
	for i, b := range content {
		if b != 0 {
			t.Errorf("File should be all zeros, but got %v at offset %d", b, i)
		}
	}
}

func TestWritePiece_EmptyData(t *testing.T) {
	ofm, cleanup := setupTestFiles(t, []int{100})
	defer cleanup()

	// Write empty data
	err := ofm.WritePiece(0, []byte{})
	if err != nil {
		t.Fatalf("WritePiece failed with empty data: %v", err)
	}

	content, _ := readFileContent(ofm.files[0])
	for i, b := range content {
		if b != 0 {
			t.Errorf("File should be all zeros, but got %v at offset %d", b, i)
		}
	}
}

func TestWritePiece_PieceBeyondFileRange(t *testing.T) {
	ofm, cleanup := setupTestFiles(t, []int{100})
	defer cleanup()

	// Write a piece that starts way beyond the file
	data := make([]byte, 16384)
	for i := range data {
		data[i] = byte(i % 256)
	}

	// Piece 10 starts at offset 163840, far beyond our 100-byte file
	err := ofm.WritePiece(10, data)
	if err != nil {
		t.Fatalf("WritePiece failed: %v", err)
	}

	// File should remain unchanged
	content, _ := readFileContent(ofm.files[0])
	for i, b := range content {
		if b != 0 {
			t.Errorf("File should be all zeros, but got %v at offset %d", b, i)
		}
	}
}

func TestWritePiece_OverwriteData(t *testing.T) {
	ofm, cleanup := setupTestFiles(t, []int{16384})
	defer cleanup()

	// Write first piece
	data1 := make([]byte, 16384)
	for i := range data1 {
		data1[i] = byte(i % 256)
	}

	err := ofm.WritePiece(0, data1)
	if err != nil {
		t.Fatalf("WritePiece failed for first write: %v", err)
	}

	// Overwrite with different data
	data2 := make([]byte, 16384)
	for i := range data2 {
		data2[i] = byte((i + 100) % 256)
	}

	err = ofm.WritePiece(0, data2)
	if err != nil {
		t.Fatalf("WritePiece failed for overwrite: %v", err)
	}

	content, err := readFileContent(ofm.files[0])
	if err != nil {
		t.Fatal(err)
	}

	// Should have the second data, not the first
	if !bytes.Equal(content, data2) {
		t.Errorf("File should contain overwritten data")
	}
	if bytes.Equal(content, data1) {
		t.Errorf("File should not contain original data after overwrite")
	}
}
