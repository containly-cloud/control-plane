package storage

import (
	"context"
	"fmt"
	"time"
)

func (s *Store) SaveSystemMetric(ctx context.Context, capturedAt time.Time, cpuUsage float64, memoryTotal, memoryUsed uint64) error {
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO system_metrics (captured_at, cpu_usage_percent, memory_total_bytes, memory_used_bytes)
		VALUES (?, ?, ?, ?)`,
		capturedAt.UTC().Format(time.RFC3339Nano), cpuUsage, memoryTotal, memoryUsed,
	); err != nil {
		return fmt.Errorf("save system metric: %w", err)
	}
	return nil
}

// PruneSystemMetrics bounds the local database and its backups while retaining
// enough raw samples for short-term operational troubleshooting.
func (s *Store) PruneSystemMetrics(ctx context.Context, before time.Time) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM system_metrics WHERE captured_at < ?`, before.UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("prune system metrics: %w", err)
	}
	return nil
}
