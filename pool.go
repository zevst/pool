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
	ctx context.Context

	job chan Callback
	err chan error

	cyclic     bool
	ErrHandler ErrorHandler
}

var stdLogger = log.New(os.Stdout, "", log.LstdFlags).Writer()
var stdErrorHandler = func(err error) error {
	return err
}

// New created pool
func New(ctx context.Context, capacity uint64, options ...Option) *Pool {
	pool := &Pool{
		ctx:        ctx,
		job:        make(chan Callback, capacity),
		err:        make(chan error, capacity),
		ErrHandler: stdErrorHandler,
	}
	for _, exec := range options {
		exec(pool)
	}
	return pool
}

//Cyclic job returns to pool after work and error handling
func Cyclic() Option {
	return func(p *Pool) {
		p.cyclic = true
	}
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
			go p.process(wg, job)
		}
	}
}

func (p *Pool) process(wg *sync.WaitGroup, job Callback) {
	runtime.Gosched()
	defer wg.Done()
	if err := job.Worker(p.ctx); err != nil {
		p.err <- err
	}
	if p.cyclic {
		p.Add(job)
	}
}
