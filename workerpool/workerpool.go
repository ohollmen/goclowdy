package workerpool

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Pool struct {
	cfg *Config
	workChan chan func()
	semaphore chan struct{} //update to any on later version
}

func NewDefaultWorkerPool() (p *Pool) {
	p, _ = NewWorkerPool(NewDefaultConfig())
	return
}

func NewWorkerPool(cfg *Config) (p *Pool, err error) {
	if cfg == nil {
		err = fmt.Errorf("Must provide a worker pool config")
	}
	p = &Pool{
		cfg: cfg,
	}
	p.Init()
	return
}

func (p *Pool) Init() error {
	if p.cfg == nil {
		return fmt.Errorf("Worker pool not yet configured, first call 'NewWorkerPool'")
	}

	p.workChan = make(chan func())
	p.semaphore = make(chan struct{}, p.cfg.WorkerLimit)
	go p.startWorkRequestChan()
	return nil
}

func (p *Pool) RequestWork(work func()) (err error) {
	ctx := context.Background()
	if p.cfg.WorkerTimeoutSeconds != 0 {
	  var cancel context.CancelFunc
	  ctx, cancel = context.WithTimeout(context.Background(), time.Duration(p.cfg.WorkerTimeoutSeconds) *time.Second)
	  defer cancel()
	}
	wg := &sync.WaitGroup{}
	wg.Add(1)
	p.workChan <- func(ctx context.Context, wg *sync.WaitGroup) func() {
		return func() {
			defer wg.Done()
			select {
				case <-ctx.Done():
					err = ctx.Err()
					return
				default:
					work()
					return
			}
		}
	}(ctx, wg)
	wg.Wait()
	return
}

func (p *Pool) startWorkRequestChan() {
	for work := range p.workChan {
		p.issueWork(work)
	}
}

func (p *Pool) issueWork(work func()) {
	p.semaphore <- struct{}{} // acquire lock
	go func() {
		work()
		<-p.semaphore // release lock
	}()
}
