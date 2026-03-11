package taskqueue

import (
	"context"
	"testing"
	"time"
)

func TestManagerStartCompletesTask(t *testing.T) {
	t.Parallel()

	manager := New(nil)
	task := manager.Start("library.scan", "user-1", map[string]any{"scope": "library"}, func(ctx context.Context, handle *Handle) (any, error) {
		handle.SetMessage("Scanning library")
		handle.SetProgress(1, 2)
		handle.Increment(2)
		return map[string]any{"message": "done"}, nil
	})

	deadline := time.Now().Add(2 * time.Second)
	for {
		current, ok := manager.Get(task.ID)
		if !ok {
			t.Fatalf("task %s disappeared", task.ID)
		}
		if current.Status == StatusCompleted {
			if current.Percent != 100 {
				t.Fatalf("expected 100 percent, got %v", current.Percent)
			}
			if current.Total != 2 || current.Completed != 2 {
				t.Fatalf("unexpected progress %+v", current)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("task did not complete in time: %+v", current)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestManagerSummaryCountsFailedTasks(t *testing.T) {
	t.Parallel()

	manager := New(nil)
	task := manager.Start("media.autotag_batch", "user-2", nil, func(ctx context.Context, handle *Handle) (any, error) {
		handle.SetMessage("Batch failed")
		return nil, context.Canceled
	})

	deadline := time.Now().Add(2 * time.Second)
	for {
		current, _ := manager.Get(task.ID)
		if current.Status == StatusFailed {
			summary := manager.Summary()
			if summary.Failed != 1 {
				t.Fatalf("expected one failed task, got %+v", summary)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("task did not fail in time")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
