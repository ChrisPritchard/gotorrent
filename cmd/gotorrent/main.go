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
		fmt.Printf("unable to download from torrent file: %v", err)
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

	local_field := peer.NewBitfield(len(torrent.Pieces))
	remote_field, err := peer.ExchangeBitfields(conns[0], local_field)
	if err != nil {
		return err
	}

	err = peer.SendInterested(conns[0])
	if err != nil {
		return err
	}

	pipeline := make(chan struct{}, 5)

	go func() {
		for i := range torrent.Pieces {
			for j := 0; j < torrent.PieceLength; j += 1 << 14 {
				pipeline <- struct{}{}
				err := peer.RequestPiecePart(conns[0], uint(i), uint(j), uint(torrent.PieceLength))
				if err != nil {
					panic(err)
				}
			}
		}
	}()

	buffer := make([]byte, 1<<14+5)
	for {
		n, err := conns[0].Read(buffer)
		if err != nil {
			return err
		}
		if n < 5 {
			continue
		}

		kind := buffer[4]
		if int(kind) == 7 {
			index := binary.BigEndian.Uint32(buffer[5:9])
			start := binary.BigEndian.Uint32(buffer[9:13])
			piece := buffer[13:n]
		}
	}

	// peer.RequestPieces()
	// peer.ListenAndRespond()

	fmt.Println(remote_field)

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
