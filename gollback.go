package gollback

import (
	"context"
	"sync"
)

// AsyncFunc represents asynchronous function
type AsyncFunc func(ctx context.Context) (interface{}, error)

// Gollback provides set of utility methods to easily manage asynchronous functions
type Gollback interface {
	// Race method returns a response as soon as one of the callbacks in an iterable resolves with the value that is not an error,
	// otherwise last error is returned
	Race(fns ...AsyncFunc) (interface{}, error)
	// All method returns when all of the callbacks passed as an iterable have finished,
	// returned responses and errors are ordered according to callback order
	All(fns ...AsyncFunc) ([]interface{}, []error)
}

type gollback struct {
	gollbacks []AsyncFunc
	ctx       context.Context
	cancel    context.CancelFunc
}

type response struct {
	res   interface{}
	err   error
	index int
}

func (p *gollback) Race(fns ...AsyncFunc) (interface{}, error) {
	out := make(chan *response, 1)

	for i, fn := range fns {
		go func(index int, f AsyncFunc) {
			for {
				select {
				case <-p.ctx.Done():
					return
				default:
					var r response
					r.res, r.err = f(p.ctx)

					if p.ctx.Err() != nil {
						return
					}

					if r.err == nil || index == len(fns)-1 {
						p.cancel()
						out <- &r
					}
					return
				}
			}
		}(i, fn)
	}

	r := <-out

	return r.res, r.err
}

func (p *gollback) All(fns ...AsyncFunc) ([]interface{}, []error) {
	rs := make([]interface{}, len(fns))
	errs := make([]error, len(fns))

	var wg sync.WaitGroup
	wg.Add(len(fns))

	for i, fn := range fns {
		go func(index int, f AsyncFunc) {
			defer wg.Done()

			for {
				select {
				case <-p.ctx.Done():
					return
				default:
					var r response
					r.res, r.err = f(p.ctx)

					if p.ctx.Err() != nil {
						return
					}

					rs[index] = r.res
					errs[index] = r.err

					return
				}
			}
		}(i, fn)
	}

	wg.Wait()
	p.cancel()

	return rs, errs
}

// New creates new gollback
func New(ctx context.Context) Gollback {
	if ctx == nil {
		ctx = context.Background()
	}

	ctxWithCancel, cancel := context.WithCancel(ctx)

	return &gollback{
		ctx:    ctxWithCancel,
		cancel: cancel,
	}
}
