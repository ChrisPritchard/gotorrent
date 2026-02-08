package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/chrispritchard/gorrent/internal/bitfields"
	"github.com/chrispritchard/gorrent/internal/downloading"
	"github.com/chrispritchard/gorrent/internal/messaging"
	outfiles "github.com/chrispritchard/gorrent/internal/out_files"
	"github.com/chrispritchard/gorrent/internal/peer"
	"github.com/chrispritchard/gorrent/internal/terminal"
	. "github.com/chrispritchard/gorrent/internal/torrent_files"
	"github.com/chrispritchard/gorrent/internal/tracker"
	"github.com/chrispritchard/gorrent/internal/util"
)

var verbose bool

func main() {
	fmt.Print("\033[38;5;153m") // pale blue
	defer fmt.Print("\033[0m")

	flag.BoolVar(&verbose, "v", false, "enable verbose output")
	flag.Parse()

	if len(flag.Args()) == 0 {
		fmt.Println("Usage: gorrent [options] <torrent-file>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	file := flag.Arg(0)

	err := try_download(file)
	if err != nil {
		fmt.Printf("unable to download via torrent file: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Download complete.")
}

func try_download(torrent_file_path string) error {
	metadata, err := parse_torrent(torrent_file_path)
	tracker_info, err := tracker.CallTracker(metadata)
	if err != nil {
		return fmt.Errorf("failed to register with tracker: %v", err)
	}

	out_files, err := outfiles.CreateOutFileManager(metadata, "")
	if err != nil {
		return fmt.Errorf("failed to establish local files: %v", err)
	}
	defer out_files.Close()

	return start_state_machine(metadata, tracker_info, out_files)
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

func start_state_machine(metadata TorrentMetadata, tracker_info tracker.TrackerResponse, out_file_manager *outfiles.OutFileManager) error {
	ctx := context.Background()
	defer ctx.Done()

	received_channel := make(chan messaging.Received)
	error_channel := make(chan error)

	current_local_field, err := out_file_manager.Bitfield()
	if err != nil {
		return err
	}
	peers := connect_to_peers(metadata, tracker_info, current_local_field)
	if len(peers) == 0 {
		return fmt.Errorf("failed to connect to a peer")
	}
	if verbose {
		fmt.Printf("Connected to %d peers\n", len(peers))
	}

	for _, p := range peers {
		p.StartReceiving(ctx, received_channel, error_channel)
		defer p.Close()
	}

	download_state := downloading.NewDownloadState(metadata, peers, out_file_manager, verbose)
	download_state.StartRequestingPieces(ctx, error_channel)

	keep_alive := time.NewTicker(2 * time.Minute)
	progress_ticker := time.NewTicker(250 * time.Millisecond)
	defer progress_ticker.Stop()

	ba := &terminal.BufferedArea{}
	defer ba.Close()

	for {
		select {
		case <-keep_alive.C:
			for _, p := range peers {
				p.SendKeepAlive()
			}
		case <-progress_ticker.C:
			print_status(ba, metadata, len(peers), download_state.CompletedPieces(), verbose)
		case received := <-received_channel:
			if received.Kind == messaging.MSG_PIECE {
				index, begin, piece := received.AsPiece()
				finished, err := download_state.ReceiveBlock(index, begin, piece)
				if err != nil {
					return err
				}
				if verbose {
					fmt.Printf("Received block: index=%d begin=%d len=%d\n", index, begin, len(piece))
				}
				if finished {
					print_status(ba, metadata, len(peers), download_state.CompletedPieces(), verbose)
					return nil // complete
				}
			} else {
				if verbose {
					fmt.Printf("received an unhandled kind: %d\n", received.Kind)
				}
			}
		case err := <-error_channel:
			return err
		}
	}
}

func connect_to_peers(metadata TorrentMetadata, tracker_response tracker.TrackerResponse, local_bitfield *bitfields.BitField) []*peer.PeerHandler {
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

func print_status(ba *terminal.BufferedArea, metadata TorrentMetadata, connected_peers, finished_pieces int, verbose bool) {
	if verbose {
		return
	}
	total_pieces := len(metadata.Pieces)
	max_width := len(fmt.Sprintf("%d", total_pieces))
	piece_fraction := fmt.Sprintf("%*d/%*d complete", max_width, finished_pieces, max_width, total_pieces)

	prog_bar, _ := terminal.ProgressBar(finished_pieces, total_pieces, 40, piece_fraction)
	ba.Update([]string{
		"name: " + metadata.Name,
		fmt.Sprintf("peers: %d", connected_peers),
		"progress:",
		prog_bar,
	})
}
