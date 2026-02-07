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

func TestWriteData_SingleFile(t *testing.T) {
	ofm, cleanup := setupTestFiles(t, []int{100})
	defer cleanup()

	// Write to middle of file
	data := []byte("HELLO")
	err := ofm.WriteData(0, 10, data)
	if err != nil {
		t.Fatalf("WriteData failed: %v", err)
	}

	content, err := readFileContent(ofm.files[0])
	if err != nil {
		t.Fatal(err)
	}

	expected := make([]byte, 100)
	copy(expected[10:15], []byte("HELLO"))

	if !bytes.Equal(content, expected) {
		t.Errorf("File content mismatch. Got: %s, Expected zeros with HELLO at offset 10", string(content[10:15]))
	}
}

func TestWriteData_ExactFileBoundary(t *testing.T) {
	ofm, cleanup := setupTestFiles(t, []int{50, 50})
	defer cleanup()

	// Write exactly at boundary between files
	data := []byte("TEST")
	err := ofm.WriteData(0, 48, data) // 48-52 spans both files
	if err != nil {
		t.Fatalf("WriteData failed: %v", err)
	}

	// Check first file
	content1, _ := readFileContent(ofm.files[0])
	if string(content1[48:50]) != "TE" {
		t.Errorf("File 1 got %s, expected TE", string(content1[48:50]))
	}

	// Check second file
	content2, _ := readFileContent(ofm.files[1])
	if string(content2[0:2]) != "ST" {
		t.Errorf("File 2 got %s, expected ST", string(content2[0:2]))
	}
}

func TestWriteData_SpanningMultipleFiles(t *testing.T) {
	ofm, cleanup := setupTestFiles(t, []int{30, 30, 30})
	defer cleanup()

	// Write data spanning all 3 files
	data := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ012345")
	err := ofm.WriteData(0, 20, data) // Writes from offset 20 to 56
	if err != nil {
		t.Fatalf("WriteData failed: %v", err)
	}

	// Verify each file
	tests := []struct {
		fileIdx  int
		offset   int
		expected string
	}{
		{0, 20, "ABCDEFGHIJ"},
		{1, 0, "KLMNOPQRSTUVWXYZ012345"},
		{2, 0, "6"}, // Just the '6' from "0123456..." overflow
	}

	for _, tt := range tests {
		content, _ := readFileContent(ofm.files[tt.fileIdx])
		start := tt.offset
		end := start + len(tt.expected)
		actual := string(content[start:end])
		if actual != tt.expected {
			t.Errorf("File %d offset %d: got %q, expected %q",
				tt.fileIdx, tt.offset, actual, tt.expected)
		}
	}
}

func TestWriteData_DataCompletelyInsideFile(t *testing.T) {
	ofm, cleanup := setupTestFiles(t, []int{100, 100})
	defer cleanup()

	// Write data completely inside second file
	data := []byte("INSIDE")
	err := ofm.WriteData(0, 120, data) // Should only write to second file at offset 20
	if err != nil {
		t.Fatalf("WriteData failed: %v", err)
	}

	// First file should be unchanged
	content1, _ := readFileContent(ofm.files[0])
	for i, b := range content1 {
		if b != 0 {
			t.Errorf("File 1 should be all zeros, but got %v at offset %d", b, i)
		}
	}

	// Second file should have data at offset 20
	content2, _ := readFileContent(ofm.files[1])
	if string(content2[20:26]) != "INSIDE" {
		t.Errorf("File 2 got %s, expected INSIDE", string(content2[20:26]))
	}
}

func TestWriteData_DataOverlapsFileStart(t *testing.T) {
	ofm, cleanup := setupTestFiles(t, []int{50, 50})
	defer cleanup()

	// Data starts before file and ends inside it
	data := []byte("PREHELLO")
	err := ofm.WriteData(0, 45, data) // Data from offset 45-53
	if err != nil {
		t.Fatalf("WriteData failed: %v", err)
	}

	content1, _ := readFileContent(ofm.files[0])
	if string(content1[45:50]) != "PREHE" {
		t.Errorf("File 1 got %s, expected PREHE", string(content1[45:50]))
	}

	content2, _ := readFileContent(ofm.files[1])
	if string(content2[0:3]) != "LLO" {
		t.Errorf("File 2 got %s, expected LLO", string(content2[0:3]))
	}
}

func TestWriteData_DataOverlapsFileEnd(t *testing.T) {
	ofm, cleanup := setupTestFiles(t, []int{50, 50})
	defer cleanup()

	// Data starts inside file and ends after it
	data := []byte("WORLDPOST")
	err := ofm.WriteData(0, 47, data) // Data from offset 47-56
	if err != nil {
		t.Fatalf("WriteData failed: %v", err)
	}

	content1, _ := readFileContent(ofm.files[0])
	if string(content1[47:50]) != "WOR" {
		t.Errorf("File 1 got %s, expected WOR", string(content1[47:50]))
	}

	content2, _ := readFileContent(ofm.files[1])
	if string(content2[0:6]) != "LDPOST" {
		t.Errorf("File 2 got %s, expected LDPOST", string(content2[0:6]))
	}
}

func TestWriteData_NoOverlap(t *testing.T) {
	ofm, cleanup := setupTestFiles(t, []int{10, 10, 10})
	defer cleanup()

	// Write completely outside file ranges
	data := []byte("TEST")
	err := ofm.WriteData(0, 35, data) // Offset 35-39, files only go to 30
	if err != nil {
		t.Fatalf("WriteData failed: %v", err)
	}

	// All files should remain unchanged
	for i, file := range ofm.files {
		content, _ := readFileContent(file)
		for j, b := range content {
			if b != 0 {
				t.Errorf("File %d should be all zeros, but got %v at offset %d", i, b, j)
			}
		}
	}
}

func TestWriteData_ExactFileSize(t *testing.T) {
	ofm, cleanup := setupTestFiles(t, []int{20})
	defer cleanup()

	// Write exactly filling the file
	data := []byte("0123456789ABCDEFGHIJ")
	if len(data) != 20 {
		t.Fatalf("Test data should be 20 bytes")
	}

	err := ofm.WriteData(0, 0, data)
	if err != nil {
		t.Fatalf("WriteData failed: %v", err)
	}

	content, _ := readFileContent(ofm.files[0])
	if string(content) != string(data) {
		t.Errorf("File content mismatch. Got: %s, Expected: %s", string(content), string(data))
	}
}

func TestWriteData_MultiplePieces(t *testing.T) {
	ofm, cleanup := setupTestFiles(t, []int{100})
	defer cleanup()

	// Test with piece > 0
	data := []byte("PIECE1")
	err := ofm.WriteData(2, 10, data) // Piece 2, offset 10 = absolute offset (2*16384)+10 = 32778
	if err != nil {
		t.Fatalf("WriteData failed: %v", err)
	}

	// Since our test file is only 100 bytes, and we're writing at offset 32778,
	// this should be a no-op (no overlap)
	content, _ := readFileContent(ofm.files[0])
	for i, b := range content {
		if b != 0 {
			t.Errorf("File should be all zeros, but got %v at offset %d", b, i)
		}
	}
}

func TestWriteData_EmptyData(t *testing.T) {
	ofm, cleanup := setupTestFiles(t, []int{100})
	defer cleanup()

	// Write empty data
	err := ofm.WriteData(0, 10, []byte{})
	if err != nil {
		t.Fatalf("WriteData failed with empty data: %v", err)
	}

	content, _ := readFileContent(ofm.files[0])
	for i, b := range content {
		if b != 0 {
			t.Errorf("File should be all zeros, but got %v at offset %d", b, i)
		}
	}
}

func TestWriteData_LargeDataAcrossBoundary(t *testing.T) {
	ofm, cleanup := setupTestFiles(t, []int{1000, 1000})
	defer cleanup()

	// Write 1500 bytes spanning both files
	data := make([]byte, 1500)
	for i := range data {
		data[i] = byte(i % 256)
	}

	err := ofm.WriteData(0, 500, data)
	if err != nil {
		t.Fatalf("WriteData failed: %v", err)
	}

	// Verify first file (bytes 500-999)
	content1, _ := readFileContent(ofm.files[0])
	for i := 500; i < 1000; i++ {
		if content1[i] != byte((i-500)%256) {
			t.Errorf("File 1 mismatch at offset %d: got %d, expected %d",
				i, content1[i], byte((i-500)%256))
			break
		}
	}

	// Verify second file (bytes 0-499 of data, which is bytes 1000-1499 absolute)
	content2, _ := readFileContent(ofm.files[1])
	for i := 0; i < 500; i++ {
		if content2[i] != byte((i+500)%256) {
			t.Errorf("File 2 mismatch at offset %d: got %d, expected %d",
				i, content2[i], byte((i+500)%256))
			break
		}
	}
}
