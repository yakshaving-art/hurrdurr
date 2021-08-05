package workerpool

import "sync"

type WorkerPool struct {
	wg   *sync.WaitGroup
	jobs chan struct{}
}

func New(size int) *WorkerPool {
	return &WorkerPool{
		wg:   &sync.WaitGroup{},
		jobs: make(chan struct{}, size),
	}
}

func (w *WorkerPool) Do(f func()) {
	w.wg.Add(1)

	w.jobs <- struct{}{} // Add one, whenever we can
	go func() {
		defer w.wg.Done()
		f()      // When we add one, run the function
		<-w.jobs // When done, remove the struct from the queue to unblock the next
	}()
}

func (w *WorkerPool) Wait() {
	w.wg.Wait()
}
