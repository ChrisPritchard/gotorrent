package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"

	"github.com/chrispritchard/gotorrent/internal/peer"
	"github.com/chrispritchard/gotorrent/internal/torrent"
	"github.com/chrispritchard/gotorrent/internal/tracker"
	"github.com/chrispritchard/gotorrent/internal/util"
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

	conns := ConnectToPeers(torrent, tracker_response)
	if len(conns) == 0 {
		return fmt.Errorf("failed to connect to a peer")
	}

	for _, c := range conns {
		defer c.Close()
	}

	// a. create local bitfield, and wrap each conn with the remote bitfield
	// also track in flight messages per peer, so we can cancel them if received else where? if requesting from more than one peer at a time
	// b. start sending requests, allocating to each peer
	// c. continuosly recieve from each peer
	// could just forward this to the central manager, as the data. but if we were to handle 'have' requests or choke requests that would need to be peer peer
	// d. channel track received pieces, update local bitfield
	// e. channel to send requests to peers.
	// should just be kind and data. maybe just the entire message

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

	pipeline := make(chan int, 5) // concurrent requests
	for range 5 {
		pipeline <- 1
	}
	errmsg := make(chan error)
	valid := make(chan bool)
	received := 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pieces_per_block := torrent.PieceLength / (1 << 14)

	go func() {
		for i := range len(torrent.Pieces) {
			for j := range pieces_per_block {
				select {
				case <-ctx.Done():
					return
				case <-pipeline:
					err = peer.RequestPiecePart(conns[0], uint(i), uint(j), 1<<14)
					if err != nil {
						errmsg <- err
					}
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				var n int
				buffer := make([]byte, 1<<14+5)
				for {
					n, err = conns[0].Read(buffer)
					if err != nil {
						errmsg <- err
					}
					if n == 0 {
						continue
					}
					break
				}
				length := binary.BigEndian.Uint32(buffer[0:4]) - 4 // exclude length?
				if length != uint32(n) {
					errmsg <- fmt.Errorf("received less bytes (%d) than length specified (%d). first 12 bytes: %x", n, length, buffer[0:min(n, 12)])
					return
				}
				kind := buffer[4]
				if int(kind) == 7 {
					index := binary.BigEndian.Uint32(buffer[5:9])
					start := binary.BigEndian.Uint32(buffer[9:13])
					piece := buffer[13:n]

					partials[index].Set(int(start), piece)
					fmt.Printf("piece %d block received\n", index)
					if partials[index].Valid() {
						partials[index].WritePiece(out_file)
						valid <- true
						fmt.Printf("piece %d finished\n", index)
					}

					pipeline <- 1
				}
			}
		}
	}()

	select {
	case e := <-errmsg:
		return e
	case <-valid:
		received++
		if received == len(torrent.Pieces) {
			fmt.Println("done")
			return nil
		}
	}

	return nil
}

func ConnectToPeers(metadata torrent.TorrentMetadata, tracker_response tracker.TrackerResponse) []net.Conn {
	ops := make([]util.Op[net.Conn], len(tracker_response.Peers))
	for i, p := range tracker_response.Peers {
		local_p := p
		ops[i] = func() (net.Conn, error) {
			return peer.Handshake(metadata, tracker_response, local_p)
		}
	}

	conns, _ := util.Concurrent(ops, 20)
	return conns
}
