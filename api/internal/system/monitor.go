package system

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// MetricStore is the small persistence surface used by the background monitor.
type MetricStore interface {
	SaveSystemMetric(ctx context.Context, capturedAt time.Time, cpuUsage float64, memoryTotal, memoryUsed uint64) error
	PruneSystemMetrics(ctx context.Context, before time.Time) error
}

type OverviewCollector interface {
	Collect(context.Context) (Overview, error)
}

// Monitor keeps the latest state in memory for efficient live reads and stores
// CPU and memory samples locally for historical monitoring.
type Monitor struct {
	collector OverviewCollector
	storage   MetricStore
	logger    *slog.Logger

	mu        sync.RWMutex
	current   Overview
	hasValue  bool
	lastPrune time.Time
	started   bool
	enabled   bool
	context   context.Context
	cancel    context.CancelFunc
}

func NewMonitor(collector OverviewCollector, storage MetricStore, logger *slog.Logger) *Monitor {
	return &Monitor{collector: collector, storage: storage, logger: logger}
}

// Start binds the monitor to the service lifetime. Configure can then start,
// stop, or reschedule collection without restarting the HTTP server.
func (m *Monitor) Start(ctx context.Context, settings MonitoringSettings) {
	m.mu.Lock()
	m.started = true
	m.context = ctx
	m.mu.Unlock()
	m.Configure(settings)
}

// Configure atomically replaces the active collection schedule. A disabled
// configuration cancels the worker, preventing new metric rows from being
// persisted. Re-enabling performs a fresh capture immediately.
func (m *Monitor) Configure(settings MonitoringSettings) {
	if !ValidMonitoringSettings(settings) {
		m.logger.Error("invalid system monitoring settings")
		return
	}
	m.mu.Lock()
	previousCancel := m.cancel
	m.cancel = nil
	m.enabled = settings.Enabled
	m.lastPrune = time.Time{}
	if !m.started || !settings.Enabled {
		m.mu.Unlock()
		if previousCancel != nil {
			previousCancel()
		}
		return
	}
	ctx, cancel := context.WithCancel(m.context)
	m.cancel = cancel
	m.mu.Unlock()
	if previousCancel != nil {
		previousCancel()
	}
	go func() {
		m.capture(ctx, settings.RetentionDays)
		ticker := time.NewTicker(time.Duration(settings.IntervalSeconds) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.capture(ctx, settings.RetentionDays)
			}
		}
	}()
}

func (m *Monitor) Overview(ctx context.Context) (Overview, error) {
	m.mu.RLock()
	if m.hasValue && m.enabled {
		overview := m.current
		m.mu.RUnlock()
		return overview, nil
	}
	m.mu.RUnlock()

	// With background collection disabled, always serve a fresh live reading
	// without persisting it. This keeps refreshes current while metric storage
	// remains untouched.
	overview, err := m.collector.Collect(ctx)
	if err != nil {
		return Overview{}, err
	}
	m.mu.Lock()
	m.current = overview
	m.hasValue = true
	m.mu.Unlock()
	return overview, nil
}

func (m *Monitor) capture(ctx context.Context, retentionDays int) {
	if ctx.Err() != nil {
		return
	}
	overview, err := m.collector.Collect(ctx)
	if err != nil {
		m.logger.Error("collect system metrics", "error", err)
		return
	}
	if ctx.Err() != nil {
		return
	}
	m.mu.Lock()
	m.current = overview
	m.hasValue = true
	m.mu.Unlock()
	if err := m.storage.SaveSystemMetric(ctx, overview.CapturedAt, overview.CPU.UsagePercent, overview.Memory.TotalBytes, overview.Memory.UsedBytes); err != nil {
		m.logger.Error("save system metrics", "error", err)
	}
	m.prune(ctx, overview.CapturedAt, retentionDays)
}

func (m *Monitor) prune(ctx context.Context, now time.Time, retentionDays int) {
	m.mu.Lock()
	if now.Sub(m.lastPrune) < time.Hour {
		m.mu.Unlock()
		return
	}
	m.lastPrune = now
	m.mu.Unlock()
	if err := m.storage.PruneSystemMetrics(ctx, now.Add(-time.Duration(retentionDays)*24*time.Hour)); err != nil {
		m.logger.Error("prune system metrics", "error", err)
	}
}
