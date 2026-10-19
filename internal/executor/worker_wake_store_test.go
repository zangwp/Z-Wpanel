package executor

import (
	"context"
	"testing"
	"time"
)

func drainWorkerWake(wake <-chan struct{}) {
	for {
		select {
		case <-wake:
		default:
			return
		}
	}
}

func TestCommittedInventoryEnqueueWakesAnIdleWorker(t *testing.T) {
	store, siteID := newWPInventoryStoreTest(t)
	drainWorkerWake(wpInventoryWake)
	t.Cleanup(func() { drainWorkerWake(wpInventoryWake) })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	poll := newWorkerIdlePoll(time.Second, wpInventoryWake)
	poll.next = poll.maximum
	woken := make(chan bool, 1)
	go func() { woken <- poll.wait(ctx) }()
	now := time.Now().UTC()
	id, created, err := store.enqueue(ctx, siteID, wpInventoryTriggerManual, now, now)
	if err != nil || !created {
		t.Fatalf("enqueue=%q created=%t err=%v", id, created, err)
	}
	select {
	case ok := <-woken:
		if !ok {
			t.Fatal("enqueue did not wake the idle worker before its fallback poll")
		}
	case <-ctx.Done():
		t.Fatal("enqueue left the worker asleep")
	}
	if _, err := store.getJob(ctx, id); err != nil {
		t.Fatalf("wake preceded a committed durable job: %v", err)
	}
}

func TestSealedUpdatePlanWakesAnIdleWorker(t *testing.T) {
	store, siteID := newWPUpdateStoreTest(t)
	drainWorkerWake(wpUpdateWake)
	t.Cleanup(func() { drainWorkerWake(wpUpdateWake) })
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	poll := newWorkerIdlePoll(time.Second, wpUpdateWake)
	poll.next = poll.maximum
	woken := make(chan bool, 1)
	go func() { woken <- poll.wait(ctx) }()
	task := createAndSealUpdateTask(t, store, siteID, time.Now().UTC())
	select {
	case ok := <-woken:
		if !ok {
			t.Fatal("sealed plan did not wake the idle worker before its fallback poll")
		}
	case <-ctx.Done():
		t.Fatal("sealed plan left the worker asleep")
	}
	got, err := store.getTask(ctx, task.ID)
	if err != nil || got.Status != wpUpdateQueued {
		t.Fatalf("wake did not refer to a committed queued plan: %+v %v", got, err)
	}
}
