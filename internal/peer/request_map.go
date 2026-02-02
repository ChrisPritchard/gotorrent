package peer

import (
	"sync"
	"time"
)

type RequestMap struct {
	data          map[int]map[int]time.Time
	max_piece_age time.Duration
	mutex         sync.Mutex
}

func CreateEmptyRequestMap(max_piece_age time.Duration) RequestMap {
	return RequestMap{make(map[int]map[int]time.Time), max_piece_age, sync.Mutex{}}
}

func (r *RequestMap) Set(piece, offset int) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if e, ok := r.data[piece]; ok {
		e[offset] = time.Now()
	} else {
		r.data[piece] = map[int]time.Time{}
		r.data[piece][offset] = time.Now()
	}
}

func (r *RequestMap) Has(piece, offset int) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	e, ok := r.data[piece]
	if !ok {
		return false
	}
	created, ok := e[offset]
	return ok && time.Since(created) < r.max_piece_age
}

func (r *RequestMap) Delete(piece, offset int) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	p, ok := r.data[piece]
	if ok {
		delete(p, offset)
		if len(p) == 0 {
			delete(r.data, piece)
		}
	}
}

func (r *RequestMap) Pieces() map[int][]int {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	result := map[int][]int{}
	for k, v := range r.data {
		var indices []int
		for k2, created := range v {
			if time.Since(created) < r.max_piece_age {
				indices = append(indices, k2)
			} else {
				delete(v, k2)
			}
		}
		if len(indices) == 0 {
			delete(r.data, k)
		} else {
			result[k] = indices
		}
	}
	return result
}
