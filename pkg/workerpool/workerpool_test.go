package workerpool_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"gitlab.com/yakshaving.art/hurrdurr/pkg/workerpool"
)

func TestWorkerPool(t *testing.T) {
	wp := workerpool.New(50)

	wg := &sync.WaitGroup{}
	wg.Add(2)

	x := func(i int) func() {
		return func() {
			time.Sleep(time.Millisecond)
			fmt.Printf("%d\n", i)
		}
	}

	go func() {
		wp.Do(func() { fmt.Printf("start sending jobs\n") })
		wg.Done()

		for i := 0; i < 100; i++ {
			wp.Do(x(i))
		}

		wp.Do(func() { fmt.Printf("done sending jobs\n") })
	}()
	go func() {

		wp.Do(func() { fmt.Printf("start sending other jobs\n") })
		wg.Done()

		for i := 100; i < 200; i++ {
			wp.Do(x(i))
		}

		wp.Do(func() { fmt.Printf("done sending other jobs\n") })
	}()

	wg.Wait() // wait for both to start

	fmt.Println("Waiting for the job queue to end")

	wp.Wait()

	fmt.Println("Done Waiting")

	// t.Fail() // This is only to see the result, as the only validation I'm doing is visual
}
