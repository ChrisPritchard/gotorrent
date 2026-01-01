package main

import "fmt"

func query_tracker(metadata TorrentMetadata, id [20]byte, ip string, port int, tracker_url string) {
	keys := fmt.Sprintf("info_hash=%s&peer_id=%s&ip=%s&port=%d&uploaded=%d&downloaded=%d&left=%d&event=%s")
}
