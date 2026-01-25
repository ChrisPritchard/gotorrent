package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"

	"github.com/chrispritchard/gotorrent/internal/bitfields"
	"github.com/chrispritchard/gotorrent/internal/messaging"
	"github.com/chrispritchard/gotorrent/internal/peer"
	"github.com/chrispritchard/gotorrent/internal/torrent"
	"github.com/chrispritchard/gotorrent/internal/tracker"
	"github.com/chrispritchard/gotorrent/internal/util"
	"golang.org/x/sync/errgroup"
)

func main() {
	file := "c:\\users\\chris\\onedrive\\desktop\\test.torrent"
	if _, err := os.Stat("ScreenToGif.exe"); err == nil {
		os.Remove("ScreenToGif.exe")
	}

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

	conns := connect_to_peers(torrent, tracker_response)
	if len(conns) == 0 {
		return fmt.Errorf("failed to connect to a peer")
	}

	for _, c := range conns {
		defer c.Close()
	}

	// working with a single conn

	local_field := bitfields.CreateBlankBitfield(len(torrent.Pieces))
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

	partials := peer.CreatePartialPieces(torrent.Pieces, torrent.PieceLength, torrent.Length)

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

	g, ctx := errgroup.WithContext(context.Background())

	g.Go(func() error {
		for i, p := range partials {
			for j := range p.Length() {
				select {
				case <-ctx.Done():
					return nil
				case <-pipeline:
					err = request_piece_block(conns[0], i, j*peer.BLOCK_SIZE, p.BlockSize(j))
					fmt.Printf("requested block %d of piece %d\n", j, i)
					if err != nil {
						return err
					}
				}
			}
		}
		return nil
	})

	g.Go(func() error {
		finished_pieces := 0
		for {
			select {
			case <-ctx.Done():
				return nil
			default:
				kind, buffer, err := messaging.ReceiveMessage(conns[0])
				if err != nil {
					return err
				}

				if kind == messaging.MSG_PIECE {
					index := binary.BigEndian.Uint32(buffer[0:4])
					begin := binary.BigEndian.Uint32(buffer[4:8])
					piece := buffer[8:]

					partials[index].Set(int(begin), piece)
					fmt.Printf("piece %d block received\n", index)
					if partials[index].Valid() {
						partials[index].WritePiece(out_file)
						fmt.Printf("piece %d finished\n", index)
						finished_pieces++
						if finished_pieces == len(torrent.Pieces) {
							return nil
						}
					}
				} else {
					fmt.Printf("received an unhandled kind: %d\n", kind)
				}
				pipeline <- 1
			}
		}
	})

	if err = g.Wait(); err != nil {
		return err
	}
	fmt.Println("done")

	return nil
}

func connect_to_peers(metadata torrent.TorrentMetadata, tracker_response tracker.TrackerResponse) []net.Conn {
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

func request_piece_block(conn net.Conn, index, begin, length int) error {
	to_send := make([]byte, 12)
	binary.BigEndian.PutUint32(to_send[:4], uint32(index))
	binary.BigEndian.PutUint32(to_send[4:8], uint32(begin))
	binary.BigEndian.PutUint32(to_send[8:], uint32(length))
	return messaging.SendMessage(conn, messaging.MSG_REQUEST, to_send)
}
