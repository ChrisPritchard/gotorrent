package tracker

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/chrispritchard/gotorrent/internal/bencode"
	. "github.com/chrispritchard/gotorrent/internal/torrent_files"
)

func escape(data []byte) string {
	return url.QueryEscape(string(data))
}

func CallTracker(metadata TorrentMetadata) (TrackerResponse, error) {
	id := make([]byte, 20)
	_, err := rand.Read(id)
	if err != nil {
		return nil_resp, err
	}

	port := 6881

	keys := fmt.Sprintf("info_hash=%s&peer_id=%s&port=%d&uploaded=%d&downloaded=%d&left=%d&event=%s&compact=1",
		escape(metadata.InfoHash[:]), escape(id), port, 0, 0, metadata.Length, "started")

	url := metadata.Announcers[0] + "?" + keys
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil_resp, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil_resp, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil_resp, err
	}

	peers, interval, err := parse_tracker_response(body)
	return TrackerResponse{
		LocalID:   id,
		LocalPort: uint16(port),
		Peers:     peers,
		Interval:  interval,
	}, nil
}

func parse_tracker_response(data []byte) ([]PeerInfo, int, error) {
	decoded, _, err := bencode.Decode(data)
	if err != nil {
		return nil, 0, err
	}
	root, ok := decoded.(map[string]any)
	if !ok {
		return nil, 0, fmt.Errorf("invalid tracker response - not a bencode dict")
	}

	failure, err := bencode.Get[string](root, "failure reason")
	if err == nil {
		return nil, 0, fmt.Errorf("tracker returned failure: %s", failure)
	}

	interval, err := bencode.Get[int](root, "interval")
	if err != nil {
		return nil, 0, fmt.Errorf("invalid tracker response - missing interval")
	}

	peers_compact, err1 := bencode.Get[string](root, "peers")
	peers_full, err2 := bencode.Get[[]any](root, "peers")
	if err1 != nil && err2 != nil {
		return nil, 0, fmt.Errorf("invalid tracker response - missing peers")
	}

	var peers []PeerInfo
	if err1 == nil {
		peers, err = parse_compact_peers(peers_compact)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid tracker response - invalid compact peer response: %v", err)
		}
	} else {
		peers, err = parse_full_peers(peers_full)
		if err != nil {
			return nil, 0, fmt.Errorf("invalid tracker response - invalid peer response: %v", err)
		}
	}

	return peers, interval, nil
}

func parse_full_peers(peers_full []any) ([]PeerInfo, error) {
	result := []PeerInfo{}

	for _, p := range peers_full {
		peer, ok := p.(map[string]any)
		if !ok {
			return nil_info, fmt.Errorf(" %v is not a dict", p)
		}

		port, err := bencode.Get[int](peer, "port")
		if err != nil {
			return nil_info, fmt.Errorf("missing port on a peer")
		}

		ip, err := bencode.Get[string](peer, "ip")
		if err != nil {
			return nil_info, fmt.Errorf(" missing ip on a peer")
		}

		id, err := bencode.Get[string](peer, "peer id")
		if err != nil {
			return nil_info, fmt.Errorf("missing peer id on a peer")
		}

		result = append(result, PeerInfo{
			Id:   id,
			IP:   ip,
			Port: uint16(port),
		})
	}

	return result, nil
}

func parse_compact_peers(peers_compact string) ([]PeerInfo, error) {
	if len(peers_compact)%6 != 0 {
		return nil_info, fmt.Errorf("size isnt a multiple of 6")
	}
	result := []PeerInfo{}

	for i := 0; i < len(peers_compact); i += 6 {
		ip_bytes := peers_compact[i : i+4]
		ip := net.IPv4(ip_bytes[0], ip_bytes[1], ip_bytes[2], ip_bytes[3]).String()

		port_bytes := []byte(peers_compact[i+4 : i+6])
		port := binary.BigEndian.Uint16(port_bytes)

		result = append(result, PeerInfo{IP: ip, Port: port})
	}

	return result, nil
}
