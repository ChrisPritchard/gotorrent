package downloading

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	outfiles "github.com/chrispritchard/gorrent/internal/out_files"
	"github.com/chrispritchard/gorrent/internal/peer"
	"github.com/chrispritchard/gorrent/internal/torrent_files"
)

var PAUSE_BETWEEN_REQUESTS = 1 * time.Millisecond
var REQUEST_MAX_AGE = 3 * time.Second

type DownloadState struct {
	requests  RequestMap
	partials  []*PartialPiece
	complete  int
	peers     []*peer.PeerHandler
	out_files *outfiles.OutFileManager
	verbose   bool
	mutex     sync.Mutex
}

func NewDownloadState(metadata torrent_files.TorrentMetadata, peers []*peer.PeerHandler, out_file_manager *outfiles.OutFileManager, verbose bool) *DownloadState {
	return &DownloadState{
		requests:  CreateEmptyRequestMap(REQUEST_MAX_AGE),
		partials:  CreatePartialPieces(metadata),
		complete:  0,
		peers:     peers,
		out_files: out_file_manager,
		verbose:   verbose,
		mutex:     sync.Mutex{},
	}
}

func (ds *DownloadState) run_in_lock(action func() error) error {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()
	return action()
}

func (ds *DownloadState) CompletedPieces() int {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()
	return ds.complete
}

func (ds *DownloadState) ReceiveBlock(index, begin int, piece []byte) (finished bool, err error) {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	ds.requests.Delete(index, begin)
	for _, p := range ds.peers {
		err := p.CancelRequest(index, begin, len(piece))
		if err != nil {
			return false, err
		}
	}

	partial := ds.partials[index]

	partial.Set(int(begin), piece)
	if ds.verbose {
		fmt.Printf("piece %d block offset %d received\n", index, begin)
	}

	if !partial.Valid() {
		return false, nil
	}

	err = partial.Conclude(index, ds.out_files)
	if err != nil {
		return false, err
	}
	if ds.verbose {
		fmt.Printf("piece %d finished\n", index)
	}

	for _, p := range ds.peers {
		err := p.SendHave(index)
		if err != nil {
			return false, err
		}
	}

	ds.complete++
	if ds.complete == len(ds.partials) {
		return true, nil
	}

	return false, nil
}

func (ds *DownloadState) StartRequestingPieces(ctx context.Context, error_channel chan<- error) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(PAUSE_BETWEEN_REQUESTS):
				err := ds.run_in_lock(func() error {
					possible_indices := []int{}
					for i, p := range ds.partials {
						if !p.Done {
							possible_indices = append(possible_indices, i)
						}
					}
					if len(possible_indices) == 0 {
						return nil
					}

					piece_index := possible_indices[rand.IntN(len(possible_indices))]
					partial := ds.partials[piece_index]

					valid_peers := []*peer.PeerHandler{}
					for _, p := range ds.peers {
						if p.HasPiece(piece_index) {
							valid_peers = append(valid_peers, p)
						}
					}

					if len(valid_peers) == 0 {
						return fmt.Errorf("no peer has piece %d", piece_index)
					}

					peer_index := rand.IntN(len(valid_peers))
					valid_peer := valid_peers[peer_index]
					block_index := partial.Missing()[0]
					block_offset := block_index * BLOCK_SIZE
					block_size := partial.BlockSize(block_index)

					err := valid_peer.RequestPieceBlock(piece_index, block_offset, block_size)
					if err != nil {
						return err
					}

					ds.requests.Set(piece_index, block_offset)
					if ds.verbose {
						fmt.Printf("requested block %d/%d (offset %d) of piece %d from peer %s\n", block_index+1, partial.Length(), block_offset, piece_index, valid_peer.Id)
					}
					return nil
				})
				if err != nil {
					error_channel <- err
				}
			}
		}
	}()
}
