package pool

import (
	"context"
	"io"
	"log"
	"os"
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

	ch  chan Callback
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
		ch:         make(chan Callback, capacity),
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
	p.ch <- f
}

func (p *Pool) Close() error {
	close(p.ch)
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
		case err := <-p.getError():
			return p.ErrHandler(err)
		case job := <-p.getJob():
			wg.Add(1)
			go func(g *sync.WaitGroup) {
				defer g.Done()
				if err := job.Worker(p.ctx); err != nil {
					p.setError(err)
					p.Add(job)
				}
			}(wg)
		}
	}
}

func (p *Pool) getJob() <-chan Callback {
	return p.ch
}

func (p *Pool) getError() <-chan error {
	return p.err
}

func (p *Pool) setError(err error) {
	p.err <- err
}
