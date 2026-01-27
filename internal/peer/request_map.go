package peer

import (
	"sync"
)

type RequestMap struct {
	data  map[int]map[int]struct{}
	mutex sync.Mutex
}

func CreateEmptyRequestMap() RequestMap {
	return RequestMap{make(map[int]map[int]struct{}), sync.Mutex{}}
}

func (r *RequestMap) Set(piece, offset int) {
	r.mutex.Lock()
	if e, ok := r.data[piece]; ok {
		e[offset] = struct{}{}
	} else {
		r.data[piece] = map[int]struct{}{}
		r.data[piece][offset] = struct{}{}
	}
	r.mutex.Unlock()
}

func (r *RequestMap) Clear(piece, offset int) {
	r.mutex.Lock()
	p, ok := r.data[piece]
	if ok {
		delete(p, offset)
		if len(p) == 0 {
			delete(r.data, piece)
		}
	}
	r.mutex.Unlock()
}

func (r *RequestMap) Pieces() map[int][]int {
	result := map[int][]int{}
	for k, v := range r.data {
		var indices []int
		for k2 := range v {
			indices = append(indices, k2)
		}
		result[k] = indices
	}
	return result
}
