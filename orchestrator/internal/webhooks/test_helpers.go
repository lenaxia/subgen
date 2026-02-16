package webhooks

import (
	"context"

	"github.com/mccloud/subgen/orchestrator/internal/skip"
)

// MockQueue implements QueueInterface for testing
type MockQueue struct {
	EnqueuedTasks []interface{}
}

func (mq *MockQueue) Enqueue(task interface{}) error {
	mq.EnqueuedTasks = append(mq.EnqueuedTasks, task)
	return nil
}

// MockSkipChecker implements skip.Checker for testing
type MockSkipChecker struct {
	ShouldSkip bool
	SkipReason skip.SkipReason
}

func (msc *MockSkipChecker) Check(ctx context.Context, filePath string) (*skip.CheckResult, error) {
	return &skip.CheckResult{
		ShouldSkip: msc.ShouldSkip,
		Reason:     msc.SkipReason,
		Details:    "mock skip check",
	}, nil
}

func (msc *MockSkipChecker) GetConfig() *skip.Config {
	return nil
}
