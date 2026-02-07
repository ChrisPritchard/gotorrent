package outfiles

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/chrispritchard/gorrent/internal/bitfields"
)

// Test setup helper (updated to use real bitfields)
func setupBitfieldTest(t *testing.T, fileSizes []int, pieceLength int, totalLength int) (*OutFileManager, func()) {
	t.Helper()
	tempDir := t.TempDir()

	indices := make([]file_indices, len(fileSizes))
	files := make([]*os.File, len(fileSizes))

	var offset int
	for i, size := range fileSizes {
		path := filepath.Join(tempDir, fmt.Sprintf("file%d.bin", i))
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}

		// Write some initial data
		initialData := make([]byte, size)
		for j := range initialData {
			initialData[j] = byte((offset + j) % 256)
		}

		_, err = f.Write(initialData)
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

	// Calculate hashes for test pieces
	numPieces := (totalLength + pieceLength - 1) / pieceLength
	hashes := make([]string, numPieces)

	for i := 0; i < numPieces; i++ {
		pieceStart := i * pieceLength
		pieceEnd := min(pieceStart+pieceLength, totalLength)
		pieceSize := pieceEnd - pieceStart

		// Create test piece data
		pieceData := make([]byte, pieceSize)
		for j := 0; j < pieceSize; j++ {
			pieceData[j] = byte((pieceStart + j) % 256)
		}

		hash := sha1.Sum(pieceData)
		hashes[i] = string(hash[:])
	}

	cleanup := func() {
		for _, f := range files {
			f.Close()
		}
	}

	return &OutFileManager{
		piece_length: pieceLength,
		total_length: totalLength,
		indices:      indices,
		files:        files,
		hashes:       hashes,
	}, cleanup
}

// Helper to count set bits in bitfield
func countSetBits(bf bitfields.BitField) int {
	count := 0
	// Calculate max pieces based on bitfield size
	maxPieces := len(bf.Data) * 8
	for i := 0; i < maxPieces; i++ {
		if bf.Get(i) {
			count++
		}
	}
	return count
}

func TestGetDataRange_Basic(t *testing.T) {
	ofm, cleanup := setupBitfieldTest(t, []int{50, 50}, 32, 100)
	defer cleanup()

	// Test reading from middle of first file
	data, err := ofm.get_data_range(10, 30)
	if err != nil {
		t.Fatalf("get_data_range failed: %v", err)
	}

	expectedLen := 20
	if len(data) != expectedLen {
		t.Errorf("Expected data length %d, got %d", expectedLen, len(data))
	}

	// Verify content
	for i := 0; i < 20; i++ {
		expected := byte(10 + i)
		if data[i] != expected {
			t.Errorf("Data[%d] = %d, expected %d", i, data[i], expected)
		}
	}
}

func TestBitfield_AllPiecesComplete(t *testing.T) {
	// Create a simple 100-byte torrent with 32-byte pieces
	fileSizes := []int{100}
	pieceLength := 32
	totalLength := 100

	ofm, cleanup := setupBitfieldTest(t, fileSizes, pieceLength, totalLength)
	defer cleanup()

	bitfield, err := ofm.Bitfield()
	if err != nil {
		t.Fatalf("Bitfield() failed: %v", err)
	}

	// Calculate expected number of pieces
	expectedPieces := (totalLength + pieceLength - 1) / pieceLength
	if expectedPieces != 4 { // 100/32 = 3.125 → 4 pieces
		t.Fatalf("Test setup wrong: expected 4 pieces, got %d", expectedPieces)
	}

	// All pieces should be marked as complete
	for i := 0; i < expectedPieces; i++ {
		if !bitfield.Get(i) {
			t.Errorf("Piece %d should be complete but isn't", i)
		}
	}

	setBits := countSetBits(*bitfield)
	if setBits != expectedPieces {
		t.Errorf("Expected %d complete pieces, got %d", expectedPieces, setBits)
	}
}

func TestBitfield_NoPiecesComplete(t *testing.T) {
	fileSizes := []int{100}
	pieceLength := 32
	totalLength := 100

	ofm, cleanup := setupBitfieldTest(t, fileSizes, pieceLength, totalLength)
	defer cleanup()

	// Corrupt the data in the files (write different data than expected)
	for _, file := range ofm.files {
		file.Seek(0, io.SeekStart)
		corruptData := make([]byte, 100)
		for i := range corruptData {
			corruptData[i] = 0xFF // Wrong data
		}
		file.Write(corruptData)
		file.Seek(0, io.SeekStart)
	}

	bitfield, err := ofm.Bitfield()
	if err != nil {
		t.Fatalf("Bitfield() failed: %v", err)
	}

	// No pieces should be complete
	setBits := countSetBits(*bitfield)
	if setBits != 0 {
		t.Errorf("Expected 0 complete pieces with corrupted data, got %d", setBits)
	}
}

func TestBitfield_SomePiecesComplete(t *testing.T) {
	fileSizes := []int{100}
	pieceLength := 32
	totalLength := 100

	ofm, cleanup := setupBitfieldTest(t, fileSizes, pieceLength, totalLength)
	defer cleanup()

	// Corrupt only the first piece (bytes 0-31)
	ofm.files[0].Seek(0, io.SeekStart)
	corruptData := make([]byte, 32)
	for i := range corruptData {
		corruptData[i] = 0xFF
	}
	ofm.files[0].Write(corruptData)
	ofm.files[0].Seek(0, io.SeekStart)

	bitfield, err := ofm.Bitfield()
	if err != nil {
		t.Fatalf("Bitfield() failed: %v", err)
	}

	expectedPieces := 4
	expectedComplete := 3 // Pieces 1, 2, 3 should be complete

	setBits := countSetBits(*bitfield)
	if setBits != expectedComplete {
		t.Errorf("Expected %d complete pieces, got %d", expectedComplete, setBits)
	}

	// Piece 0 should not be complete
	if bitfield.Get(0) {
		t.Error("Piece 0 should not be complete (corrupted)")
	}

	// Pieces 1-3 should be complete
	for i := 1; i < expectedPieces; i++ {
		if !bitfield.Get(i) {
			t.Errorf("Piece %d should be complete but isn't", i)
		}
	}
}

func TestBitfield_MultipleFiles(t *testing.T) {
	// Torrent with 2 files, pieces crossing boundaries
	fileSizes := []int{40, 60}
	pieceLength := 32
	totalLength := 100

	ofm, cleanup := setupBitfieldTest(t, fileSizes, pieceLength, totalLength)
	defer cleanup()

	bitfield, err := ofm.Bitfield()
	if err != nil {
		t.Fatalf("Bitfield() failed: %v", err)
	}

	expectedPieces := 4
	setBits := countSetBits(*bitfield)
	if setBits != expectedPieces {
		t.Errorf("Expected all %d pieces complete, got %d", expectedPieces, setBits)
	}
}

func TestBitfield_LastPartialPiece(t *testing.T) {
	// Test case where last piece is smaller than piece_length
	fileSizes := []int{70}
	pieceLength := 32
	totalLength := 70

	ofm, cleanup := setupBitfieldTest(t, fileSizes, pieceLength, totalLength)
	defer cleanup()

	bitfield, err := ofm.Bitfield()
	if err != nil {
		t.Fatalf("Bitfield() failed: %v", err)
	}

	expectedPieces := 3 // 70/32 = 2.1875 → 3 pieces
	setBits := countSetBits(*bitfield)
	if setBits != expectedPieces {
		t.Errorf("Expected %d complete pieces, got %d", expectedPieces, setBits)
	}

	// Last piece (piece 2) should be complete despite being only 6 bytes
	if !bitfield.Get(2) {
		t.Error("Last partial piece should be marked complete")
	}
}

func TestBitfield_EmptyTorrent(t *testing.T) {
	// Edge case: empty torrent
	ofm := &OutFileManager{
		piece_length: 0,
		total_length: 0,
		indices:      []file_indices{},
		files:        []*os.File{},
		hashes:       []string{},
	}

	bitfield, err := ofm.Bitfield()
	if err != nil {
		t.Fatalf("Bitfield() failed for empty torrent: %v", err)
	}

	// Empty torrent should have 0 pieces
	if len(bitfield.Data) != 0 {
		t.Errorf("Empty torrent bitfield should have empty Data, got length %d", len(bitfield.Data))
	}
}

func TestBitfield_ReadErrorHandling(t *testing.T) {
	fileSizes := []int{100}
	pieceLength := 32
	totalLength := 100

	ofm, cleanup := setupBitfieldTest(t, fileSizes, pieceLength, totalLength)
	defer cleanup()

	// Close the file to cause read errors
	ofm.files[0].Close()

	// Should still return a bitfield (with no pieces set due to errors)
	bitfield, err := ofm.Bitfield()
	if err != nil {
		t.Fatalf("Bitfield() should handle read errors gracefully, got error: %v", err)
	}

	setBits := countSetBits(*bitfield)
	if setBits != 0 {
		t.Errorf("Bitfield should have 0 pieces when files can't be read, got %d", setBits)
	}
}

func TestBitfield_CompareWithWriteData(t *testing.T) {
	// Test integration between WriteData and Bitfield
	fileSizes := []int{100}
	pieceLength := 32
	totalLength := 100

	ofm, cleanup := setupBitfieldTest(t, fileSizes, pieceLength, totalLength)
	defer cleanup()

	// First, verify all pieces are complete
	bitfield1, err := ofm.Bitfield()
	if err != nil {
		t.Fatal(err)
	}

	setBits1 := countSetBits(*bitfield1)
	if setBits1 != 4 {
		t.Errorf("Initially should have 4 complete pieces, got %d", setBits1)
	}

	// Write some corrupted data to piece 1
	corruptData := make([]byte, 32)
	for i := range corruptData {
		corruptData[i] = 0xAA
	}
	err = ofm.WriteData(0, 32, corruptData) // Overwrite piece 1
	if err != nil {
		t.Fatal(err)
	}

	// Now piece 1 should not be complete
	bitfield2, err := ofm.Bitfield()
	if err != nil {
		t.Fatal(err)
	}

	if bitfield2.Get(1) {
		t.Error("Piece 1 should not be complete after corrupt write")
	}

	// Write correct data back
	correctData := make([]byte, 32)
	for i := range correctData {
		correctData[i] = byte(32 + i)
	}
	err = ofm.WriteData(0, 32, correctData)
	if err != nil {
		t.Fatal(err)
	}

	// Now all pieces should be complete again
	bitfield3, err := ofm.Bitfield()
	if err != nil {
		t.Fatal(err)
	}

	setBits3 := countSetBits(*bitfield3)
	if setBits3 != 4 {
		t.Errorf("After fixing data, should have 4 complete pieces, got %d", setBits3)
	}
}

func TestBitfield_EdgeCaseLargeBitfield(t *testing.T) {
	// Test with many pieces to ensure bitfield indexing works correctly
	fileSizes := []int{10000} // 10KB file
	pieceLength := 256        // 256-byte pieces
	totalLength := 10000

	ofm, cleanup := setupBitfieldTest(t, fileSizes, pieceLength, totalLength)
	defer cleanup()

	bitfield, err := ofm.Bitfield()
	if err != nil {
		t.Fatalf("Bitfield() failed: %v", err)
	}

	// Calculate expected pieces
	expectedPieces := (totalLength + pieceLength - 1) / pieceLength
	expectedBytes := (expectedPieces + 7) / 8

	// Check bitfield size
	if len(bitfield.Data) != expectedBytes {
		t.Errorf("Bitfield should have %d bytes for %d pieces, got %d bytes",
			expectedBytes, expectedPieces, len(bitfield.Data))
	}

	// All pieces should be complete
	setBits := countSetBits(*bitfield)
	if setBits != expectedPieces {
		t.Errorf("Expected %d complete pieces, got %d", expectedPieces, setBits)
	}
}

func TestBitfield_PieceBoundaryCases(t *testing.T) {
	// Test pieces that exactly align with file boundaries
	tests := []struct {
		name         string
		fileSizes    []int
		pieceLength  int
		totalLength  int
		expectPieces int
	}{
		{
			name:         "exact piece boundaries",
			fileSizes:    []int{64, 64},
			pieceLength:  64,
			totalLength:  128,
			expectPieces: 2,
		},
		{
			name:         "uneven boundaries",
			fileSizes:    []int{30, 40, 30},
			pieceLength:  32,
			totalLength:  100,
			expectPieces: 4,
		},
		{
			name:         "single byte pieces",
			fileSizes:    []int{10},
			pieceLength:  1,
			totalLength:  10,
			expectPieces: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ofm, cleanup := setupBitfieldTest(t, tt.fileSizes, tt.pieceLength, tt.totalLength)
			defer cleanup()

			bitfield, err := ofm.Bitfield()
			if err != nil {
				t.Fatalf("Bitfield() failed: %v", err)
			}

			setBits := countSetBits(*bitfield)
			if setBits != tt.expectPieces {
				t.Errorf("Expected %d complete pieces, got %d", tt.expectPieces, setBits)
			}
		})
	}
}
