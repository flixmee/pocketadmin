package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/tools/types"
)

type automationDelayScheduler struct {
	stop     chan struct{}
	wake     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

// shutdown stops the scheduler goroutine and waits for it to exit.
//
// It is safe to call concurrently and multiple times (e.g. from
// overlapping terminate events) - every caller blocks until the
// goroutine is done so that the app resources are not released
// while it is still running.
func (s *automationDelayScheduler) shutdown() {
	s.stopOnce.Do(func() {
		close(s.stop)
	})
	<-s.done
}

func startAutomationDelayScheduler(app App) {
	if app == nil {
		return
	}

	stopAutomationDelayScheduler(app)

	scheduler := &automationDelayScheduler{
		stop: make(chan struct{}),
		wake: make(chan struct{}, 1),
		done: make(chan struct{}),
	}
	app.Store().Set(StoreKeyAutomationDelaySchedulerStop, scheduler)

	go func() {
		defer close(scheduler.done)

		for {
			wait, ok := nextAutomationDelaySchedulerWait(app)
			if !ok {
				select {
				case <-scheduler.stop:
					return
				case <-scheduler.wake:
					continue
				}
			}

			timer := time.NewTimer(wait)
			select {
			case <-scheduler.stop:
				timer.Stop()
				return
			case <-scheduler.wake:
				timer.Stop()
				continue
			case <-timer.C:
				continue
			}
		}
	}()
}

func wakeAutomationDelayScheduler(app App) {
	if app == nil {
		return
	}

	scheduler, ok := app.Store().Get(StoreKeyAutomationDelaySchedulerStop).(*automationDelayScheduler)
	if !ok || scheduler == nil {
		return
	}

	select {
	case scheduler.wake <- struct{}{}:
	default:
	}
}

func stopAutomationDelayScheduler(app App) {
	if app == nil {
		return
	}

	value := app.Store().Get(StoreKeyAutomationDelaySchedulerStop)

	switch scheduler := value.(type) {
	case *automationDelayScheduler:
		if scheduler == nil {
			return
		}
		scheduler.shutdown()

		// remove only if it wasn't replaced in the meantime by a newer scheduler
		if current, _ := app.Store().Get(StoreKeyAutomationDelaySchedulerStop).(*automationDelayScheduler); current == scheduler {
			app.Store().Remove(StoreKeyAutomationDelaySchedulerStop)
		}
	case chan struct{}:
		app.Store().Remove(StoreKeyAutomationDelaySchedulerStop)
		if scheduler != nil {
			close(scheduler)
		}
	}
}

func nextAutomationDelaySchedulerWait(app App) (wait time.Duration, ok bool) {
	// the db connections could be closed/reset while the goroutine is
	// still running (e.g. on app re-bootstrap) so retry later
	if !app.IsBootstrapped() {
		return automationDelaySchedulerInterval, true
	}

	// a panic in a background goroutine would crash the entire process
	defer func() {
		if rec := recover(); rec != nil {
			app.Logger().Error("Automation delay scheduler panicked", "error", fmt.Sprint(rec))
			wait, ok = automationDelaySchedulerInterval, true
		}
	}()

	if err := app.ResumeExpiredAutomationWorkflowStates(); err != nil {
		app.Logger().Warn("Failed to resume expired automation workflow states", "error", err)
		return automationDelaySchedulerInterval, true
	}

	now := types.NowDateTime()
	next, err := app.FindNextWaitingWorkflowStateExpiry(now)
	if err != nil {
		app.Logger().Warn("Failed to find next automation workflow state expiry", "error", err)
		return automationDelaySchedulerInterval, true
	}
	if next.IsZero() {
		return 0, false
	}

	wait = next.Sub(now)
	if wait <= 0 {
		return automationDelaySchedulerInterval, true
	}

	return wait, true
}
