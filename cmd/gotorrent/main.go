package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/chrispritchard/gotorrent/internal/peer"
	"github.com/chrispritchard/gotorrent/internal/torrent"
	"github.com/chrispritchard/gotorrent/internal/tracker"
)

func main() {
	file := "c:\\users\\chris\\onedrive\\desktop\\test.torrent"

	err := try_download(file)
	if err != nil {
		fmt.Printf("unable to download via torrent file: %v", err)
		os.Exit(1)
	}
}

func try_download(torrent_file_path string) error {
	d, err := os.ReadFile(torrent_file_path)
	if err != nil {
		return fmt.Errorf("unable to read file at path %s: %v", torrent_file_path, err)
	}

	torrent, err := torrent.ParseTorrentFile(d)
	if err != nil {
		return fmt.Errorf("unable to parse torrent file: %v", err)
	}

	tracker_response, err := tracker.CallTracker(torrent)
	if err != nil {
		return fmt.Errorf("failed to register with tracker: %v", err)
	}

	peer_handshakes := get_handshakes(torrent, tracker_response)
	conns, _ := concurrent(peer_handshakes, 20)
	for _, c := range conns {
		defer c.Close()
	}

	if len(conns) == 0 {
		return fmt.Errorf("failed to connect to a peer")
	}

	// working with a single conn

	local_field := peer.CreateBlankBitfield(len(torrent.Pieces))
	remote_field, err := peer.ExchangeBitfields(conns[0], local_field)
	if err != nil {
		return err
	}
	fmt.Println(remote_field)

	err = peer.SendInterested(conns[0])
	if err != nil {
		return err
	}

	// set up all partial pieces

	partials := make([]peer.PartialPiece, len(torrent.Pieces))
	for i, p := range torrent.Pieces {
		partials[i] = peer.CreatePartialPiece(p, i*torrent.PieceLength, torrent.PieceLength)
	}

	// create full file
	out_file, err := os.Create(torrent.Name) // assuming a single file with no directory info
	if err != nil {
		return err
	}
	defer out_file.Close()

	err = out_file.Truncate(int64(torrent.Length)) // create full size file
	if err != nil {
		return err
	}

	// test request
	err = peer.RequestPiecePart(conns[0], 3, 20, 1<<14)
	if err != nil {
		return err
	}

	// test receive
	var n int
	buffer := make([]byte, 1<<14+5)
	for {
		n, err = conns[0].Read(buffer)
		if err != nil {
			return err
		}
		if n == 0 {
			continue
		}
		break
	}
	len := binary.BigEndian.Uint32(buffer[0:4])
	fmt.Println(len)
	kind := buffer[4]
	if int(kind) == 7 {
		index := binary.BigEndian.Uint32(buffer[5:9])
		start := binary.BigEndian.Uint32(buffer[9:13])
		piece := buffer[13:n]

		partials[index].Set(int(start), piece)
		if partials[index].Valid() {
			partials[index].WritePiece(out_file)
		}
	}

	// continuously request pieces

	// pipeline := make(chan struct{}, 5) // limits the number of concurrent requests

	// go func() {
	// 	for i := range torrent.Pieces {
	// 		for j := 0; j < torrent.PieceLength; j += 1 << 14 {
	// 			pipeline <- struct{}{}
	// 			err := peer.RequestPiecePart(conns[0], uint(i), uint(j), uint(torrent.PieceLength))
	// 			if err != nil {
	// 				panic(err)
	// 			}
	// 		}
	// 	}
	// }()

	// valid_count := 0
	// buffer := make([]byte, 1<<14+5)

	// for {
	// 	n, err := conns[0].Read(buffer)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	if n < 5 {
	// 		continue
	// 	}

	// 	kind := buffer[4]
	// 	if int(kind) == 7 {
	// 		index := binary.BigEndian.Uint32(buffer[5:9])
	// 		start := binary.BigEndian.Uint32(buffer[9:13])
	// 		piece := buffer[13:n]

	// 		partials[index].Set(int(start), piece)
	// 		if partials[index].Valid() {
	// 			partials[index].WritePiece(out_file)
	// 			valid_count++
	// 		}
	// 	}

	// 	if valid_count == len(partials) {
	// 		break
	// 	}
	// }

	fmt.Println("done")

	return nil
}

func get_handshakes(metadata torrent.TorrentMetadata, tracker_response tracker.TrackerResponse) []op[net.Conn] {
	ops := make([]op[net.Conn], len(tracker_response.Peers))
	for i, p := range tracker_response.Peers {
		local_p := p
		ops[i] = func() (net.Conn, error) {
			return peer.Handshake(metadata, tracker_response, local_p)
		}
	}
	return ops
}

type op[T any] = func() (T, error)

func concurrent[T any](ops []op[T], max_concurrent int) ([]T, []error) {
	var result []T
	var errors []error

	var mutex sync.Mutex // ensuring single-time access to slices

	sem := make(chan struct{}, max_concurrent) // basically a queue of open 'chances' - we cant run an op until we can reserve a spot in the queue
	var wg sync.WaitGroup                      // used to wait until all ops are completed - each one adds to this
	wg.Add(len(ops))

	for _, o := range ops {
		go func(o func() (T, error)) {
			defer wg.Done()

			sem <- struct{}{}        // acquire slot
			defer func() { <-sem }() // release slot

			r, e := o()
			mutex.Lock()
			if e != nil {
				errors = append(errors, e)
			} else {
				result = append(result, r)
			}
			mutex.Unlock()
		}(o)
	}

	wg.Wait() // will wait until all done
	return result, errors
}
