package workerpool

import (
	"sync"
	"testing"
	"time"
	"fmt"
)

func TestWorkerPool(t *testing.T) {
	wp := NewDefaultWorkerPool()
	wg := &sync.WaitGroup{}
	workRequests := 25
	wg.Add(workRequests)
	errChan := make(chan error, workRequests)
	for i := 0; i < workRequests; i++ {
		go func(workerId int) {
			defer wg.Done()
			err := wp.RequestWork(func() {
				t.Logf("worker %v is sleeping", workerId)
				time.Sleep(2 * time.Second)
				t.Logf("worker %v is done", workerId)
			})
			errChan <- err	
		}(i)
	}
	wg.Wait()
	for i := 0; i < workRequests; i++ {
		err := <-errChan
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Log("All workers finished")
}

func SomeKindOfWork(id int, name, dest string) error {
	if name == "fail" {
		return fmt.Errorf("fail will fail")
	}
	return nil
}

func TestOneCall(t *testing.T) {
	wp := NewDefaultWorkerPool()
	var err error
	workRequest := func() {
		err = SomeKindOfWork(5, "machine-image-name", "whatever")
	}

	wrErr  := wp.RequestWork(workRequest)
	if wrErr != nil || err != nil {
		t.Fatal(err)
	}
	workRequestToFail := func() {
		err = SomeKindOfWork(10, "fail", "whatever")
	}
	wrErr = wp.RequestWork(workRequestToFail)
	if wrErr != nil {
		t.Fatal(err)
	}
	if err == nil {
		t.Fatal("expected work request to fail, but it didn't")
	}
}