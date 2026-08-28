package consumers

import (
	"core/common/delay_queue"
	"testing"
)

func TestJobPanicDoesNotKillWorker(t *testing.T) {
	job := delay_queue.Job{ID: "regression"}
	if err := handleJob(func(delay_queue.Job) error { panic("failed") }, job); err == nil {
		t.Fatal("panic was not converted to retryable error")
	}
	called := false
	if err := handleJob(func(delay_queue.Job) error { called = true; return nil }, job); err != nil || !called {
		t.Fatal("subsequent job did not run")
	}
}
