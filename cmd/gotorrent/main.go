package main

import (
	"fmt"
	"log"
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

	conns := try_handshake(torrent, tracker_response, 20)
	for _, c := range conns {
		defer c.Close()
	}

	if len(conns) == 0 {
		return fmt.Errorf("failed to connect to a peer")
	}

	// pass conns to a downloader queue

	return nil
}

func try_handshake(metadata torrent.TorrentMetadata, tracker_response tracker.TrackerResponse, maxConcurrent int) []net.Conn {
	var mu sync.Mutex
	var connections []net.Conn
	sem := make(chan struct{}, maxConcurrent) // Semaphore for limiting concurrency
	var wg sync.WaitGroup

	for _, p := range tracker_response.Peers {
		wg.Add(1)
		go func(p tracker.PeerInfo) {
			defer wg.Done()

			sem <- struct{}{}        // Acquire slot
			defer func() { <-sem }() // Release slot

			conn, err := peer.Handshake(metadata, tracker_response, p)
			if err != nil {
				log.Printf("Failed handshake with %s:%d: %v", p.IP, p.Port, err)
				return
			}

			mu.Lock()
			connections = append(connections, conn)
			mu.Unlock()
		}(p)
	}

	wg.Wait()
	log.Printf("Successfully connected to %d/%d peers", len(connections), len(tracker_response.Peers))
	return connections
}
