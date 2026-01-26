package main

import (
	"context"
	"encoding/binary"
	"fmt"
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

	local_field := bitfields.CreateBlankBitfield(len(torrent.Pieces))
	ctx := context.Background()
	defer ctx.Done()

	received_channel := make(chan messaging.Received)
	error_channel := make(chan error)

	peers := connect_to_peers(torrent, tracker_response, local_field)
	if len(peers) == 0 {
		return fmt.Errorf("failed to connect to a peer")
	}

	for _, p := range peers {
		p.StartReceiving(ctx, received_channel, error_channel)
		defer p.Close()
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
	go start_requesting_pieces(ctx, peers, partials, pipeline)

	finished_pieces := 0
	for {
		select {
		case received := <-received_channel:
			if received.Kind == messaging.MSG_PIECE {
				index := binary.BigEndian.Uint32(received.Data[0:4])
				begin := binary.BigEndian.Uint32(received.Data[4:8])
				piece := received.Data[8:]

				partials[index].Set(int(begin), piece)
				fmt.Printf("piece %d block received\n", index)
				if partials[index].Valid() {
					partials[index].WritePiece(out_file)
					fmt.Printf("piece %d finished\n", index)
					finished_pieces++
					if finished_pieces == len(torrent.Pieces) {
						fmt.Println("done")
						return nil
					}
				}
			} else {
				fmt.Printf("received an unhandled kind: %d\n", received.Kind)
			}
			pipeline <- 1
		case err := <-error_channel:
			return err
		}
	}
}

func start_requesting_pieces(ctx context.Context, peers []peer.PeerHandler, partials []peer.PartialPiece, pipeline <-chan int) error {

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		for i, p := range partials {
			for j := range p.Length() {
				select {
				case <-ctx.Done():
					return nil
				case <-pipeline:
					err := peers[0].RequestPieceBlock(i, j*peer.BLOCK_SIZE, p.BlockSize(j))
					fmt.Printf("requested block %d of piece %d\n", j, i)
					if err != nil {
						return err
					}
				}
			}
		}
		return nil
	})

	return g.Wait()
}

func connect_to_peers(metadata torrent.TorrentMetadata, tracker_response tracker.TrackerResponse, local_bitfield bitfields.BitField) []peer.PeerHandler {
	ops := make([]util.Op[peer.PeerHandler], len(tracker_response.Peers))
	for i, p := range tracker_response.Peers {
		local_p := p
		ops[i] = func() (peer.PeerHandler, error) {
			return peer.ConnectToPeer(local_p, metadata.InfoHash[:], tracker_response.LocalID, local_bitfield)
		}
	}

	conns, _ := util.Concurrent(ops, 20)
	return conns
}
