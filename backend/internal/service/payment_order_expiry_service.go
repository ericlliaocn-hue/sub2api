package service

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

const expiryCheckTimeout = 30 * time.Second

const (
	// paymentOrderExpiryLeaderLockKey gates the periodic reconcile + expiry sweep so
	// that only one instance issues the upstream payment-provider calls per cycle.
	paymentOrderExpiryLeaderLockKey = "payment:order:expiry:leader"
	// paymentOrderExpiryLeaderLockTTL must exceed the combined reconcile + expiry
	// timeouts (2 * expiryCheckTimeout) so the lock never expires mid-run.
	paymentOrderExpiryLeaderLockTTL = 3 * time.Minute
)

// PaymentOrderExpiryService periodically expires timed-out payment orders.
type PaymentOrderExpiryService struct {
	paymentSvc *PaymentService
	interval   time.Duration
	stopCh     chan struct{}
	stopOnce   sync.Once
	wg         sync.WaitGroup

	lockCache    LeaderLockCache
	db           *sql.DB
	instanceID   string
	billingCache *BillingCacheService
	authCache    APIKeyAuthCacheInvalidator
}

func (s *PaymentOrderExpiryService) SetBonusExpiryInvalidators(billingCache *BillingCacheService, authCache APIKeyAuthCacheInvalidator) {
	if s == nil {
		return
	}
	s.billingCache = billingCache
	s.authCache = authCache
}

func NewPaymentOrderExpiryService(paymentSvc *PaymentService, interval time.Duration) *PaymentOrderExpiryService {
	return &PaymentOrderExpiryService{
		paymentSvc: paymentSvc,
		interval:   interval,
		stopCh:     make(chan struct{}),
		instanceID: uuid.NewString(),
	}
}

// SetLeaderLock injects the leader-lock cache and DB used to elect a single
// instance for the periodic reconcile/expiry sweep. When both are nil the job
// runs ungated (single-instance / test behavior).
func (s *PaymentOrderExpiryService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

func (s *PaymentOrderExpiryService) Start() {
	if s == nil || s.paymentSvc == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *PaymentOrderExpiryService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *PaymentOrderExpiryService) runOnce() {
	// Multi-instance guard: only the leader reconciles/expires orders per cycle,
	// avoiding N× upstream payment-provider API calls and update races.
	lockCtx, lockCancel := context.WithTimeout(context.Background(), 2*time.Second)
	release, ok := tryAcquireSingletonLeaderLock(lockCtx, s.lockCache, s.db, paymentOrderExpiryLeaderLockKey, s.instanceID, paymentOrderExpiryLeaderLockTTL)
	lockCancel()
	if !ok {
		return
	}
	defer release()

	reconcileCtx, cancel := context.WithTimeout(context.Background(), expiryCheckTimeout)
	recovered, err := s.paymentSvc.ReconcilePendingWxpayOrders(reconcileCtx)
	cancel()
	if err != nil {
		slog.Warn("[PaymentOrderExpiry] failed to reconcile pending wxpay orders", "error", err)
	} else if recovered > 0 {
		slog.Info("[PaymentOrderExpiry] reconciled paid wxpay orders", "count", recovered)
	}

	expireCtx, cancel := context.WithTimeout(context.Background(), expiryCheckTimeout)
	defer cancel()
	expired, err := s.paymentSvc.ExpireTimedOutOrders(expireCtx)
	if err != nil {
		slog.Error("[PaymentOrderExpiry] failed to expire orders", "error", err)
	} else if expired > 0 {
		slog.Info("[PaymentOrderExpiry] expired timed-out orders", "count", expired)
	}

	bonusCtx, bonusCancel := context.WithTimeout(context.Background(), expiryCheckTimeout)
	expiredUsers, expiredAmount, bonusErr := s.paymentSvc.ExpireDueRechargeBonuses(bonusCtx, 500)
	bonusCancel()
	if len(expiredUsers) > 0 {
		cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 15*time.Second)
		for _, userID := range expiredUsers {
			if s.authCache != nil {
				s.authCache.InvalidateAuthCacheByUserID(cacheCtx, userID)
			}
			if s.billingCache != nil {
				if cacheErr := s.billingCache.InvalidateUserBalance(cacheCtx, userID); cacheErr != nil {
					slog.Warn("[PaymentOrderExpiry] failed to invalidate expired bonus balance cache", "user_id", userID, "error", cacheErr)
				}
			}
		}
		cacheCancel()
	}
	if bonusErr != nil {
		slog.Error("[PaymentOrderExpiry] failed to expire bonus balances", "error", bonusErr)
	} else if len(expiredUsers) > 0 {
		slog.Info("[PaymentOrderExpiry] expired bonus balances", "users", len(expiredUsers), "amount", expiredAmount)
	}
}
