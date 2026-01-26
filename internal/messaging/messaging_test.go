package messaging

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"
)

func TestSendMessage(t *testing.T) {
	// Create a pipe to simulate a connection
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	type args struct {
		kind PeerMessageType
		data []byte
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "send choke (no payload)",
			args: args{
				kind: MSG_CHOKE,
				data: []byte{},
			},
			wantErr: false,
		},
		{
			name: "send piece (with payload)",
			args: args{
				kind: MSG_PIECE,
				data: []byte{0, 0, 0, 1, 0, 0, 0, 0, 1, 2, 3}, // index, begin, block data
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a goroutine to read what SendMessage writes to the pipe
			errChan := make(chan error, 1)
			go func() {
				// 1. Read Length (4 bytes)
				lenBuf := make([]byte, 4)
				if _, err := io.ReadFull(server, lenBuf); err != nil {
					errChan <- err
					return
				}
				length := binary.BigEndian.Uint32(lenBuf)

				// 2. Verify Length = len(data) + 1 (for type byte)
				if length != uint32(len(tt.args.data)+1) {
					t.Errorf("expected length %d, got %d", len(tt.args.data)+1, length)
				}

				// 3. Read Payload (Type + Data)
				payload := make([]byte, length)
				if _, err := io.ReadFull(server, payload); err != nil {
					errChan <- err
					return
				}

				// 4. Verify Type
				if PeerMessageType(payload[0]) != tt.args.kind {
					t.Errorf("expected kind %d, got %d", tt.args.kind, payload[0])
				}

				// 5. Verify Data
				if !bytes.Equal(payload[1:], tt.args.data) {
					t.Errorf("expected data %v, got %v", tt.args.data, payload[1:])
				}
				errChan <- nil
			}()

			if err := SendMessage(client, tt.args.kind, tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Wait for the verifier to finish
			if err := <-errChan; err != nil {
				t.Errorf("verification failed: %v", err)
			}
		})
	}
}

func TestReceiveMessage(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	t.Run("receive valid message", func(t *testing.T) {
		expectedKind := MSG_PIECE
		expectedData := []byte{0xDE, 0xAD, 0xBE, 0xEF}

		go func() {
			// Construct a valid message: Length (4 bytes) + Type (1 byte) + Data
			length := uint32(1 + len(expectedData))
			buf := make([]byte, 4+length)
			binary.BigEndian.PutUint32(buf[0:4], length)
			buf[4] = byte(expectedKind)
			copy(buf[5:], expectedData)
			server.Write(buf)
		}()

		received, err := ReceiveMessage(client)
		if err != nil {
			t.Fatalf("ReceiveMessage() unexpected error: %v", err)
		}
		if received.Kind != expectedKind {
			t.Errorf("ReceiveMessage() kind = %v, want %v", received.Kind, expectedKind)
		}
		if !bytes.Equal(received.Data, expectedData) {
			t.Errorf("ReceiveMessage() data = %v, want %v", received.Data, expectedData)
		}
	})
}
