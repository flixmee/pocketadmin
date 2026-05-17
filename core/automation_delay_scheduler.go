package core

import (
	"sync/atomic"
	"time"
)

func startAutomationDelayScheduler(app App) {
	if app == nil {
		return
	}

	stopAutomationDelayScheduler(app)

	stop := make(chan struct{})
	app.Store().Set(StoreKeyAutomationDelaySchedulerStop, stop)

	var running atomic.Bool
	run := func() {
		if !running.CompareAndSwap(false, true) {
			return
		}
		defer running.Store(false)

		if err := app.ResumeExpiredAutomationWorkflowStates(); err != nil {
			app.Logger().Warn("Failed to resume expired automation workflow states", "error", err)
		}
	}

	go func() {
		ticker := time.NewTicker(automationDelaySchedulerInterval)
		defer ticker.Stop()

		run()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

func stopAutomationDelayScheduler(app App) {
	if app == nil {
		return
	}

	stop, ok := app.Store().Get(StoreKeyAutomationDelaySchedulerStop).(chan struct{})
	if !ok || stop == nil {
		return
	}

	app.Store().Remove(StoreKeyAutomationDelaySchedulerStop)
	close(stop)
}
