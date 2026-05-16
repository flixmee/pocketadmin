package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	StoreKeyAutomationPolicyConfig  = "pbAppAutomationPolicyConfig"
	storeKeyAutomationPolicyContext = "pbAppAutomationPolicyContext"
)

type AutomationPolicyConfig struct {
	MaxDepth          int
	MaxRunsPerMinute  int
	MaxConcurrentRuns int
	Cooldown          time.Duration
	DedupeWindow      time.Duration
}

type automationPolicyDecision struct {
	Allowed             bool   `json:"allowed"`
	Reason              string `json:"reason,omitempty"`
	MaxDepth            int    `json:"maxDepth,omitempty"`
	Depth               int    `json:"depth"`
	MaxRunsPerMinute    int    `json:"maxRunsPerMinute,omitempty"`
	RunsPerMinute       int    `json:"runsPerMinute,omitempty"`
	MaxConcurrentRuns   int    `json:"maxConcurrentRuns,omitempty"`
	ConcurrentRuns      int    `json:"concurrentRuns,omitempty"`
	CooldownSeconds     int64  `json:"cooldownSeconds,omitempty"`
	DedupeWindowSeconds int64  `json:"dedupeWindowSeconds,omitempty"`
	DedupeKey           string `json:"dedupeKey,omitempty"`
}

type automationPolicyContext struct {
	RunId string
	Depth int
}

func defaultAutomationPolicyConfig() AutomationPolicyConfig {
	return AutomationPolicyConfig{
		MaxDepth:          5,
		MaxRunsPerMinute:  100,
		MaxConcurrentRuns: 10,
	}
}

func getAutomationPolicyConfig(app App) AutomationPolicyConfig {
	config, ok := app.Store().Get(StoreKeyAutomationPolicyConfig).(AutomationPolicyConfig)
	if !ok {
		return defaultAutomationPolicyConfig()
	}

	defaults := defaultAutomationPolicyConfig()
	if config.MaxDepth < 0 {
		config.MaxDepth = defaults.MaxDepth
	}
	if config.MaxRunsPerMinute <= 0 {
		config.MaxRunsPerMinute = defaults.MaxRunsPerMinute
	}
	if config.MaxConcurrentRuns <= 0 {
		config.MaxConcurrentRuns = defaults.MaxConcurrentRuns
	}

	return config
}

func evaluateAutomationPolicy(app App, automation *Automation, payload automationTriggerPayload, run *AutomationRun) (automationPolicyDecision, error) {
	config := getAutomationPolicyConfig(app)
	decision := automationPolicyDecision{
		Allowed:             true,
		Depth:               payload.Depth,
		MaxDepth:            config.MaxDepth,
		MaxRunsPerMinute:    config.MaxRunsPerMinute,
		MaxConcurrentRuns:   config.MaxConcurrentRuns,
		CooldownSeconds:     int64(config.Cooldown.Seconds()),
		DedupeWindowSeconds: int64(config.DedupeWindow.Seconds()),
		DedupeKey:           run.DedupeKey(),
	}

	if payload.Depth > config.MaxDepth {
		return denyAutomationPolicy(decision, "max_depth_exceeded")
	}

	concurrentRuns, err := countAutomationRunsSince(app, automation.Id, time.Time{}, []string{AutomationRunStatusRunning})
	if err != nil {
		return decision, err
	}
	decision.ConcurrentRuns = concurrentRuns
	if concurrentRuns >= config.MaxConcurrentRuns {
		return denyAutomationPolicy(decision, "max_concurrent_runs_exceeded")
	}

	recentRuns, err := countAutomationRunsSince(app, automation.Id, time.Now().Add(-time.Minute), nil)
	if err != nil {
		return decision, err
	}
	decision.RunsPerMinute = recentRuns
	if recentRuns >= config.MaxRunsPerMinute {
		return denyAutomationPolicy(decision, "max_runs_per_minute_exceeded")
	}

	if config.Cooldown > 0 {
		cooldownRuns, err := countAutomationRunsSince(app, automation.Id, time.Now().Add(-config.Cooldown), nil)
		if err != nil {
			return decision, err
		}
		if cooldownRuns > 0 {
			return denyAutomationPolicy(decision, "cooldown_active")
		}
	}

	if config.DedupeWindow > 0 && strings.TrimSpace(run.DedupeKey()) != "" {
		duplicateRuns, err := countAutomationRunsSince(app, automation.Id, time.Now().Add(-config.DedupeWindow), nil, dbx.HashExp{"dedupeKey": run.DedupeKey()})
		if err != nil {
			return decision, err
		}
		if duplicateRuns > 0 {
			return denyAutomationPolicy(decision, "duplicate_dedupe_key")
		}
	}

	return decision, nil
}

func denyAutomationPolicy(decision automationPolicyDecision, reason string) (automationPolicyDecision, error) {
	decision.Allowed = false
	decision.Reason = reason
	return decision, fmt.Errorf("automation policy rejected run: %s", reason)
}

func countAutomationRunsSince(app App, automationID string, since time.Time, statuses []string, extra ...dbx.Expression) (int, error) {
	query := app.RecordQuery(CollectionNameAutomationRuns).
		AndWhere(dbx.HashExp{"automationRef": automationID})

	if !since.IsZero() {
		sinceDate, err := types.ParseDateTime(since)
		if err != nil {
			return 0, err
		}
		query.AndWhere(dbx.NewExp("[[created]] >= {:created}", dbx.Params{"created": sinceDate.String()}))
	}
	if len(statuses) > 0 {
		values := make([]any, len(statuses))
		for i, status := range statuses {
			values[i] = status
		}
		query.AndWhere(dbx.In("status", values...))
	}
	for _, exp := range extra {
		query.AndWhere(exp)
	}

	total := 0
	if err := query.Select("count(*)").Row(&total); err != nil {
		return 0, err
	}

	return total, nil
}

func inheritAutomationPolicyContext(app App, payload *automationTriggerPayload) {
	if payload == nil {
		return
	}

	ctx, ok := app.Store().Get(storeKeyAutomationPolicyContext).(automationPolicyContext)
	if !ok || ctx.RunId == "" {
		return
	}

	payload.ParentRunId = ctx.RunId
	payload.Depth = ctx.Depth + 1
}

func setCurrentAutomationPolicyContext(app App, run *AutomationRun) {
	if run == nil || run.Id == "" {
		return
	}

	app.Store().Set(storeKeyAutomationPolicyContext, automationPolicyContext{
		RunId: run.Id,
		Depth: run.Depth(),
	})
}

func clearCurrentAutomationPolicyContext(app App, runId string) {
	ctx, ok := app.Store().Get(storeKeyAutomationPolicyContext).(automationPolicyContext)
	if !ok || ctx.RunId != runId {
		return
	}

	app.Store().Remove(storeKeyAutomationPolicyContext)
}
