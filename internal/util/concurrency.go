package util

import "sync"

// Op represents a function that returns a value and/or an error
type Op[T any] = func() (T, error)

// Concurrent runs the operations specified in multiple goroutines, up to the limit of max_concurrent at the same time
func Concurrent[T any](ops []Op[T], max_concurrent int) ([]T, []error) {
	var result []T
	var errors []error

	var mutex sync.Mutex // ensuring single-time access to slices

	sem := make(chan struct{}, max_concurrent) // basically a queue of open 'chances' - we cant run an op until we can reserve a spot in the queue
	var wg sync.WaitGroup                      // used to wait until all ops are completed - each one adds to this
	wg.Add(len(ops))

	for _, o := range ops {
		go func(o func() (T, error)) {
			defer wg.Done()

			sem <- struct{}{}        // acquire slot
			defer func() { <-sem }() // release slot

			r, e := o()
			mutex.Lock()
			if e != nil {
				errors = append(errors, e)
			} else {
				result = append(result, r)
			}
			mutex.Unlock()
		}(o)
	}

	wg.Wait() // will wait until all done
	return result, errors
}
