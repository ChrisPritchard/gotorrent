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
	defer r.mutex.Unlock()
	if e, ok := r.data[piece]; ok {
		e[offset] = struct{}{}
	} else {
		r.data[piece] = map[int]struct{}{}
		r.data[piece][offset] = struct{}{}
	}
}

func (r *RequestMap) Has(piece, offset int) bool {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	e, ok := r.data[piece]
	if !ok {
		return false
	}
	_, ok = e[offset]
	return ok
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
