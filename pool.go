package pool

import (
	"context"
	"io"
	"log"
	"os"
	"runtime"
	"sync"
)

type Callback interface {
	Worker(ctx context.Context) error
}

type Option func(*Pool)
type ErrorHandler func(error) error

type Pool struct {
	ctx    context.Context
	stoper context.CancelFunc

	job chan Callback
	err chan error

	ErrHandler ErrorHandler
}

var stdLogger = log.New(os.Stdout, "", log.LstdFlags).Writer()
var stdErrorHandler = func(err error) error {
	return err
}

// New created pool
func New(ctx context.Context, capacity uint64, options ...Option) *Pool {
	ctx, cancel := context.WithCancel(ctx)
	pool := &Pool{
		ctx:        ctx,
		stoper:     cancel,
		job:        make(chan Callback, capacity),
		err:        make(chan error, capacity),
		ErrHandler: stdErrorHandler,
	}
	for _, exec := range options {
		exec(pool)
	}
	return pool
}

// AddLogger replaces the logging source
func AddLogger(writer io.Writer) Option {
	return func(*Pool) {
		stdLogger = writer
	}
}

func (p *Pool) Add(f Callback) {
	p.job <- f
}

func (p *Pool) Close() error {
	close(p.job)
	close(p.err)
	return nil
}

func (p *Pool) Stop() {
	p.stoper()
}

func (p *Pool) Work() error {
	defer Close(p)
	wg := &sync.WaitGroup{}
	defer wg.Wait()
	for {
		select {
		case <-p.ctx.Done():
			return p.ctx.Err()
		case err := <-p.err:
			return p.ErrHandler(err)
		case job := <-p.job:
			wg.Add(1)
			go func(g *sync.WaitGroup) {
				runtime.Gosched()
				defer g.Done()
				if err := job.Worker(p.ctx); err != nil {
					p.err <- err
					p.Add(job)
				}
			}(wg)
		}
	}
}
