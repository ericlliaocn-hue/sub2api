package securityaudit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPromptCapacitySyncWaitsWithinDeadlineAndReleaseIsIdempotent(t *testing.T) {
	capacity := newPromptCapacity(1, 1, 1, 1)
	firstRelease, acquired := capacity.AcquireSync(context.Background(), "guard")
	require.True(t, acquired)

	waiterDone := make(chan bool, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		release, ok := capacity.AcquireSync(ctx, "guard")
		if ok {
			release()
		}
		waiterDone <- ok
	}()

	select {
	case <-waiterDone:
		t.Fatal("synchronous capacity must wait while the slot is occupied")
	case <-time.After(30 * time.Millisecond):
	}
	firstRelease()
	firstRelease()
	require.True(t, <-waiterDone)
	require.Eventually(t, func() bool {
		capacity.mu.Lock()
		defer capacity.mu.Unlock()
		return capacity.globalActive == 0 && len(capacity.nodes) == 0
	}, time.Second, 5*time.Millisecond)
}

func TestPromptCapacityAsyncIsLowPriorityAndStrictlyBounded(t *testing.T) {
	capacity := newPromptCapacity(2, 2, 1, 1)
	asyncRelease, acquired := capacity.TryAcquireAsync("guard")
	require.True(t, acquired)

	secondAsyncRelease, secondAsyncAcquired := capacity.TryAcquireAsync("guard")
	require.False(t, secondAsyncAcquired)
	require.Nil(t, secondAsyncRelease)

	// The remaining model slot stays available to synchronous traffic.
	syncRelease, syncAcquired := capacity.AcquireSync(context.Background(), "guard")
	require.True(t, syncAcquired)
	syncRelease()
	asyncRelease()
}

func TestPromptCapacityAsyncNeverWaitsBehindSynchronousTraffic(t *testing.T) {
	capacity := newPromptCapacity(1, 1, 1, 1)
	syncRelease, acquired := capacity.AcquireSync(context.Background(), "guard")
	require.True(t, acquired)

	started := time.Now()
	asyncRelease, asyncAcquired := capacity.TryAcquireAsync("guard")
	require.False(t, asyncAcquired)
	require.Nil(t, asyncRelease)
	require.Less(t, time.Since(started), 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	waitRelease, waitAcquired := capacity.AcquireSync(ctx, "guard")
	require.False(t, waitAcquired)
	require.Nil(t, waitRelease)
	syncRelease()
}

func TestPromptCapacityNormalizesEmptyNodeIDForCleanup(t *testing.T) {
	capacity := newPromptCapacity(1, 1, 1, 1)
	release, acquired := capacity.AcquireSync(context.Background(), "")
	require.True(t, acquired)
	release()
	capacity.mu.Lock()
	require.Empty(t, capacity.nodes)
	capacity.mu.Unlock()
}

func TestPromptCapacityLeastInflightOrdersNodesByActiveLoad(t *testing.T) {
	capacity := newPromptCapacity(8, 4, 1, 1)
	firstRelease, firstOK := capacity.AcquireSync(context.Background(), "first")
	require.True(t, firstOK)
	secondRelease, secondOK := capacity.AcquireSync(context.Background(), "first")
	require.True(t, secondOK)
	otherRelease, otherOK := capacity.AcquireSync(context.Background(), "other")
	require.True(t, otherOK)

	ordered := capacity.OrderEndpoints(PromptAuditStrategyLeastInflight, []ActiveEndpoint{
		{ID: "first"}, {ID: "other"},
	})
	require.Equal(t, []string{"other", "first"}, []string{ordered[0].ID, ordered[1].ID})
	otherRelease()
	secondRelease()
	firstRelease()
}

func TestPromptCapacityPriorityKeepsConfiguredOrder(t *testing.T) {
	capacity := newPromptCapacity(4, 2, 1, 1)
	release, acquired := capacity.AcquireSync(context.Background(), "first")
	require.True(t, acquired)
	defer release()

	ordered := capacity.OrderEndpoints(PromptAuditStrategyPriority, []ActiveEndpoint{{ID: "first"}, {ID: "other"}})
	require.Equal(t, []string{"first", "other"}, []string{ordered[0].ID, ordered[1].ID})
}
