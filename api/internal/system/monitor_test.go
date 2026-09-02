package system

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
)

type collectorStub struct {
	mu    sync.Mutex
	calls int
	seen  chan struct{}
}

func (c *collectorStub) Collect(context.Context) (Overview, error) {
	c.mu.Lock()
	c.calls++
	c.mu.Unlock()
	select {
	case c.seen <- struct{}{}:
	default:
	}
	return Overview{CapturedAt: time.Now()}, nil
}

type metricStoreStub struct{ saveCalls int }

func (s *metricStoreStub) SaveSystemMetric(context.Context, time.Time, float64, uint64, uint64) error {
	s.saveCalls++
	return nil
}
func (*metricStoreStub) PruneSystemMetrics(context.Context, time.Time) error { return nil }

func TestMonitorCanBeStoppedAndStartedWithoutRestart(t *testing.T) {
	collector := &collectorStub{seen: make(chan struct{}, 2)}
	monitor := NewMonitor(collector, &metricStoreStub{}, slog.New(slog.NewTextHandler(testWriter{t}, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor.Start(ctx, MonitoringSettings{Enabled: false, IntervalSeconds: 5, RetentionDays: 30})
	select {
	case <-collector.seen:
		t.Fatal("disabled monitoring collected a metric")
	case <-time.After(20 * time.Millisecond):
	}

	monitor.Configure(MonitoringSettings{Enabled: true, IntervalSeconds: 5, RetentionDays: 30})
	select {
	case <-collector.seen:
	case <-time.After(time.Second):
		t.Fatal("enabled monitoring did not collect a metric")
	}

	monitor.Configure(MonitoringSettings{Enabled: false, IntervalSeconds: 5, RetentionDays: 30})
	collector.mu.Lock()
	calls := collector.calls
	collector.mu.Unlock()
	if calls != 1 {
		t.Errorf("collect calls = %d, want 1", calls)
	}
}

func TestDisabledMonitorServesLiveOverviewWithoutSavingMetrics(t *testing.T) {
	collector := &collectorStub{seen: make(chan struct{}, 1)}
	store := &metricStoreStub{}
	monitor := NewMonitor(collector, store, slog.New(slog.NewTextHandler(testWriter{t}, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	monitor.Start(ctx, MonitoringSettings{Enabled: false, IntervalSeconds: 5, RetentionDays: 30})

	overview, err := monitor.Overview(context.Background())

	if err != nil || overview.CapturedAt.IsZero() {
		t.Fatalf("Overview() = %+v, %v", overview, err)
	}
	if store.saveCalls != 0 {
		t.Errorf("SaveSystemMetric calls = %d, want 0", store.saveCalls)
	}
	if _, err := monitor.Overview(context.Background()); err != nil {
		t.Fatalf("second Overview() error = %v", err)
	}
	collector.mu.Lock()
	calls := collector.calls
	collector.mu.Unlock()
	if calls != 2 {
		t.Errorf("disabled Overview collection calls = %d, want 2", calls)
	}
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Log(string(p))
	return len(p), nil
}
