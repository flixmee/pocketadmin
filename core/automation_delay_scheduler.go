package core

import (
	"time"

	"github.com/pocketbase/pocketbase/tools/types"
)

type automationDelayScheduler struct {
	stop chan struct{}
	wake chan struct{}
	done chan struct{}
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
	app.Store().Remove(StoreKeyAutomationDelaySchedulerStop)

	switch scheduler := value.(type) {
	case *automationDelayScheduler:
		if scheduler != nil && scheduler.stop != nil {
			close(scheduler.stop)
			if scheduler.done != nil {
				<-scheduler.done
			}
		}
	case chan struct{}:
		if scheduler != nil {
			close(scheduler)
		}
	}
}

func nextAutomationDelaySchedulerWait(app App) (time.Duration, bool) {
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

	wait := next.Sub(now)
	if wait <= 0 {
		return automationDelaySchedulerInterval, true
	}

	return wait, true
}
