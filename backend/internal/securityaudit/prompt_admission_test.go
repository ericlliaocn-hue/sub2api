package securityaudit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPromptAdmissionLimiterBoundsWaitersReleasesAndCleansEntries(t *testing.T) {
	limiter := newPromptAdmissionLimiter(1, 1, time.Second)
	firstRelease, acquired := limiter.Acquire(context.Background(), "user:1")
	require.True(t, acquired)

	waiterDone := make(chan bool, 1)
	go func() {
		release, ok := limiter.Acquire(context.Background(), "user:1")
		if ok {
			release()
		}
		waiterDone <- ok
	}()
	require.Eventually(t, func() bool {
		limiter.mu.Lock()
		defer limiter.mu.Unlock()
		return limiter.entries["user:1"] != nil && limiter.entries["user:1"].waiting == 1
	}, time.Second, 5*time.Millisecond)

	started := time.Now()
	thirdRelease, thirdAcquired := limiter.Acquire(context.Background(), "user:1")
	require.False(t, thirdAcquired)
	require.Nil(t, thirdRelease)
	require.Less(t, time.Since(started), 100*time.Millisecond)

	firstRelease()
	firstRelease()
	require.True(t, <-waiterDone)
	require.Eventually(t, func() bool {
		limiter.mu.Lock()
		defer limiter.mu.Unlock()
		return len(limiter.entries) == 0
	}, time.Second, 5*time.Millisecond)
}

func TestPromptAdmissionLimiterIsolatesIdentitiesAndTimesOutCleanly(t *testing.T) {
	limiter := newPromptAdmissionLimiter(1, 1, 30*time.Millisecond)
	release, acquired := limiter.Acquire(context.Background(), "user:1")
	require.True(t, acquired)

	otherRelease, otherAcquired := limiter.Acquire(context.Background(), "user:2")
	require.True(t, otherAcquired)
	otherRelease()

	started := time.Now()
	timedRelease, timedAcquired := limiter.Acquire(context.Background(), "user:1")
	require.False(t, timedAcquired)
	require.Nil(t, timedRelease)
	require.GreaterOrEqual(t, time.Since(started), 20*time.Millisecond)

	release()
	limiter.mu.Lock()
	require.Empty(t, limiter.entries)
	limiter.mu.Unlock()
}

func TestPromptAdmissionKeyPrefersUserThenAPIKey(t *testing.T) {
	require.Equal(t, "user:7", promptAdmissionKey(Request{UserID: 7, APIKeyID: 9}))
	require.Equal(t, "api_key:9", promptAdmissionKey(Request{APIKeyID: 9}))
	require.Equal(t, "anonymous", promptAdmissionKey(Request{}))
}
