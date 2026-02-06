package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/chrispritchard/gotorrent/internal/bitfields"
	"github.com/chrispritchard/gotorrent/internal/downloading"
	"github.com/chrispritchard/gotorrent/internal/messaging"
	"github.com/chrispritchard/gotorrent/internal/peer"
	. "github.com/chrispritchard/gotorrent/internal/torrent_files"
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

func parse_torrent(torrent_file_path string) (TorrentMetadata, error) {
	var nil_result TorrentMetadata
	d, err := os.ReadFile(torrent_file_path)
	if err != nil {
		return nil_result, fmt.Errorf("unable to read file at path %s: %v", torrent_file_path, err)
	}

	torrent, err := ParseTorrentFile(d)
	if err != nil {
		return nil_result, fmt.Errorf("unable to parse torrent file: %v", err)
	}

	return torrent, nil
}

func get_local_bit_field(metadata TorrentMetadata) (bitfields.BitField, error) {
	return bitfields.CreateBlankBitfield(len(metadata.Pieces)), nil // TODO: evaluate existing file
}

func establish_outfile(metadata TorrentMetadata) (*os.File, error) {
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

func start_state_machine(metadata TorrentMetadata, tracker_info tracker.TrackerResponse, local_field bitfields.BitField, out_file *os.File) error {
	ctx := context.Background()
	defer ctx.Done()

	received_channel := make(chan messaging.Received)
	error_channel := make(chan error)
	finished_channel := make(chan int, 1)

	peers := connect_to_peers(metadata, tracker_info, local_field)
	if len(peers) == 0 {
		return fmt.Errorf("failed to connect to a peer")
	}

	for _, p := range peers {
		p.StartReceiving(ctx, received_channel, error_channel)
		defer p.Close()
	}

	download_state := downloading.NewDownloadState(metadata, peers)
	download_state.StartRequestingPieces(ctx, error_channel)

	keep_alive := time.NewTicker(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-finished_channel:
			return nil // complete!
		// case <-ticker.C:
		// 	print_status(partials, &requests)
		case <-keep_alive.C:
			for _, p := range peers {
				p.SendKeepAlive()
			}
		case received := <-received_channel:
			if received.Kind == messaging.MSG_PIECE {
				index, begin, piece := received.AsPiece()
				err := download_state.ReceiveBlock(index, begin, piece, out_file, finished_channel)
				if err != nil {
					return err
				}
				// TODO: check if all done
			} else {
				fmt.Printf("received an unhandled kind: %d\n", received.Kind)
			}
		case err := <-error_channel:
			return err
		}
	}
}

// func print_status(partials []*peer.PartialPiece, requests *peer.RequestMap) {
// 	for i, p := range partials {
// 		if !p.Done && !p.Valid() {
// 			fmt.Printf("partial %d is invalid\n", i)
// 			fmt.Printf("\tmissing: %v\n", p.Missing())
// 		}
// 	}
// 	fmt.Printf("requested:\n")
// 	for k, v := range requests.Pieces() {
// 		var indices strings.Builder
// 		for _, k2 := range v {
// 			indices.WriteString(strconv.Itoa(k2+1) + " ")
// 		}
// 		fmt.Printf("\t%d: %s\n", k, indices.String())
// 	}
// 	fmt.Println()
// }

func connect_to_peers(metadata TorrentMetadata, tracker_response tracker.TrackerResponse, local_bitfield bitfields.BitField) []*peer.PeerHandler {
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
