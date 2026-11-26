package planchange

import (
	"context"
	"errors"
	"sync"
	"time"

	"v/internal/logger"
)

// Scheduler processes due changes and retries committed purchases whose
// runtime provisioning failed. All durable operations are safe to repeat.
type Scheduler struct {
	service           *Service
	interval          time.Duration
	logger            logger.Logger
	reconcilePayments func(context.Context) error
	maintenanceTasks  []func(context.Context) error
	mu                sync.Mutex
	cancel            context.CancelFunc
	done              chan struct{}
}

func (s *Scheduler) WithPaymentReconciliation(run func(context.Context) error) *Scheduler {
	s.reconcilePayments = run
	return s
}

// WithMaintenanceTasks appends periodic commercial housekeeping tasks (e.g. trial expiry, commission settlement).
func (s *Scheduler) WithMaintenanceTasks(tasks ...func(context.Context) error) *Scheduler {
	for _, task := range tasks {
		if task != nil {
			s.maintenanceTasks = append(s.maintenanceTasks, task)
		}
	}
	return s
}

func NewScheduler(service *Service, interval time.Duration, log logger.Logger) *Scheduler {
	if interval <= 0 {
		interval = time.Minute
	}
	return &Scheduler{service: service, interval: interval, logger: log}
}

func (s *Scheduler) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel, s.done = cancel, make(chan struct{})
	go func(done chan struct{}) {
		defer close(done)
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			if err := s.RunOnce(ctx); err != nil && ctx.Err() == nil {
				s.logger.Error("Commercial reconciliation failed", logger.Err(err))
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}(s.done)
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel == nil {
		return
	}
	s.cancel()
	<-s.done
	s.cancel = nil
}

func (s *Scheduler) RunOnce(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	_, downgradeErr := s.service.ProcessDueDowngrades(ctx)
	var fulfillmentErr, paymentErr, couponErr, expiredOrderErr error
	if s.service.orderService != nil {
		fulfillmentErr = s.service.orderService.RetryPendingFulfillments(ctx)
		couponErr = s.service.orderService.ReleaseExpiredCouponOrders(ctx)
		expiredOrderErr = s.service.orderService.CancelExpiredOrders(ctx)
	}
	if s.reconcilePayments != nil {
		paymentErr = s.reconcilePayments(ctx)
	}
	errs := []error{downgradeErr, fulfillmentErr, paymentErr, couponErr, expiredOrderErr}
	for _, task := range s.maintenanceTasks {
		if err := task(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
