package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/chrispritchard/gotorrent/internal/bitfields"
	"github.com/chrispritchard/gotorrent/internal/messaging"
	"github.com/chrispritchard/gotorrent/internal/peer"
	"github.com/chrispritchard/gotorrent/internal/torrent"
	"github.com/chrispritchard/gotorrent/internal/tracker"
	"github.com/chrispritchard/gotorrent/internal/util"
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

	orig_hash := "c5255beede8949d1ac8da7b4ae80d3c46fe2ccb8"
	f, _ := os.ReadFile("ScreenToGif.exe")
	new_hash := sha1.Sum(f)
	if hex.EncodeToString(new_hash[:]) == orig_hash {
		fmt.Println("success! file integrity checked and matches")
	} else {
		fmt.Printf("failure! file hash doesnt match original")
		os.Exit(1)
	}
}

func try_download(torrent_file_path string) error {
	metadata, err := parse_torrent(torrent_file_path)
	tracker_info, err := tracker.CallTracker(metadata)
	if err != nil {
		return fmt.Errorf("failed to register with tracker: %v", err)
	}

	local_field, err := get_local_bit_field(metadata)
	if err != nil {
		return fmt.Errorf("failed to register with tracker: %v", err)
	}

	out_file, err := establish_outfile(metadata)
	if err != nil {
		return fmt.Errorf("failed to create/read out_file: %v", err)
	}
	defer out_file.Close()

	return start_state_machine(metadata, tracker_info, local_field, out_file)
}

func parse_torrent(torrent_file_path string) (torrent.TorrentMetadata, error) {
	var nil_result torrent.TorrentMetadata
	d, err := os.ReadFile(torrent_file_path)
	if err != nil {
		return nil_result, fmt.Errorf("unable to read file at path %s: %v", torrent_file_path, err)
	}

	torrent, err := torrent.ParseTorrentFile(d)
	if err != nil {
		return nil_result, fmt.Errorf("unable to parse torrent file: %v", err)
	}

	return torrent, nil
}

func get_local_bit_field(metadata torrent.TorrentMetadata) (bitfields.BitField, error) {
	return bitfields.CreateBlankBitfield(len(metadata.Pieces)), nil // TODO: evaluate existing file
}

func establish_outfile(metadata torrent.TorrentMetadata) (*os.File, error) {
	out_file, err := os.Create(metadata.Name) // assuming a single file with no directory info
	if err != nil {
		return nil, err
	}

	err = out_file.Truncate(int64(metadata.Length)) // create full size file
	if err != nil {
		out_file.Close()
		return nil, err
	}

	return out_file, nil
}

func start_state_machine(metadata torrent.TorrentMetadata, tracker_info tracker.TrackerResponse, local_field bitfields.BitField, out_file *os.File) error {
	ctx := context.Background()
	defer ctx.Done()

	received_channel := make(chan messaging.Received)
	error_channel := make(chan error)

	peers := connect_to_peers(metadata, tracker_info, local_field)
	if len(peers) == 0 {
		return fmt.Errorf("failed to connect to a peer")
	}

	pipeline := make(chan int, 5) // rate-limiting requests to received
	for range 5 {
		pipeline <- 1
	}

	for _, p := range peers {
		p.StartReceiving(ctx, received_channel, error_channel)
		defer p.Close()
	}

	requests := peer.CreateEmptyRequestMap()
	partials := peer.CreatePartialPieces(metadata.Pieces, metadata.PieceLength, metadata.Length)
	start_requesting_pieces(ctx, peers, partials, &requests, error_channel, pipeline)

	keep_alive := time.NewTicker(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	finished_pieces := 0
	for {
		select {
		case <-ticker.C:
			print_status(partials, &requests)
		case <-keep_alive.C:
			for _, p := range peers {
				p.SendKeepAlive()
			}
		case received := <-received_channel:
			piece_finished, err := handle_received(received, &requests, peers, partials, out_file)
			if err != nil {
				return err
			}
			if piece_finished {
				finished_pieces++
				if finished_pieces == len(partials) {
					fmt.Println("done")
					print_status(partials, &requests)
					return nil
				}
			}
			pipeline <- 1
		case err := <-error_channel:
			return err
		}
	}
}

func handle_received(received messaging.Received, requests *peer.RequestMap, peers []*peer.PeerHandler, partials []*peer.PartialPiece, out_file *os.File) (piece_finished bool, err error) {
	piece_finished = false
	if received.Kind == messaging.MSG_PIECE {
		index, begin, piece := received.AsPiece()
		requests.Delete(index, begin)
		for i := range peers {
			err = peers[i].CancelRequest(index, begin, len(piece))
			return
		}

		partials[index].Set(int(begin), piece)
		fmt.Printf("piece %d block offset %d received\n", index, begin)

		if partials[index].Valid() {
			partials[index].WritePiece(out_file)
			piece_finished = true
			fmt.Printf("piece %d finished\n", index)

			for i := range peers {
				err = peers[i].SendHave(index)
				return
			}
		}
	} else {
		fmt.Printf("received an unhandled kind: %d\n", received.Kind)
	}
	return
}

func print_status(partials []*peer.PartialPiece, requests *peer.RequestMap) {
	// for i, p := range partials {
	// 	if !p.Done && !p.Valid() {
	// 		fmt.Printf("partial %d is invalid\n", i)
	// 		fmt.Printf("\tmissing: %v\n", p.Missing())
	// 	}
	// }
	fmt.Printf("requested:\n")
	for k, v := range requests.Pieces() {
		var indices strings.Builder
		for _, k2 := range v {
			indices.WriteString(strconv.Itoa(k2+1) + " ")
		}
		fmt.Printf("\t%d: %s\n", k, indices.String())
	}
	fmt.Println()
}

func start_requesting_pieces(ctx context.Context, peers []*peer.PeerHandler, partials []*peer.PartialPiece, requests *peer.RequestMap, error_channel chan<- error, pipeline <-chan int) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-pipeline:
				piece_index := rand.IntN(len(partials))
				partial := partials[piece_index]
				if partial.Done {
					continue
				}
				valid_peers := []*peer.PeerHandler{}
				for i := range peers {
					if peers[i].HasPiece(piece_index) {
						valid_peers = append(valid_peers, peers[i])
					}
				}
				if len(valid_peers) == 0 {
					error_channel <- fmt.Errorf("no peer has piece %d", piece_index)
					return
				}
				peer_index := rand.IntN(len(valid_peers))
				valid_peer := valid_peers[peer_index]
				block_index := partial.Missing()[0]
				block_offset := block_index * peer.BLOCK_SIZE
				block_size := partial.BlockSize(block_index)

				err := valid_peer.RequestPieceBlock(piece_index, block_offset, block_size)
				if err != nil {
					error_channel <- err
					return
				}

				requests.Set(piece_index, block_offset)
				fmt.Printf("requested block %d/%d (offset %d) of piece %d from peer %s\n", block_index+1, partial.Length(), block_offset, piece_index, valid_peer.Id)
			}
		}
	}()
}

func connect_to_peers(metadata torrent.TorrentMetadata, tracker_response tracker.TrackerResponse, local_bitfield bitfields.BitField) []*peer.PeerHandler {
	ops := make([]util.Op[*peer.PeerHandler], len(tracker_response.Peers))
	for i, p := range tracker_response.Peers {
		local_p := p
		ops[i] = func() (*peer.PeerHandler, error) {
			return peer.ConnectToPeer(local_p, metadata.InfoHash[:], tracker_response.LocalID, local_bitfield)
		}
	}

	conns, _ := util.Concurrent(ops, 20)
	return conns
}
