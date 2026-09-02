package storage

import (
	"api/internal/system"
	"context"
	"database/sql"
	"errors"
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

func (s *Store) ListSystemMetrics(ctx context.Context, from, to time.Time, granularity system.MetricGranularity) ([]system.MetricSample, error) {
	bucketFormat := map[system.MetricGranularity]string{
		system.MetricGranularitySecond: "%Y-%m-%dT%H:%M:%SZ",
		system.MetricGranularityMinute: "%Y-%m-%dT%H:%M:00Z",
		system.MetricGranularityHour:   "%Y-%m-%dT%H:00:00Z",
	}[granularity]
	if bucketFormat == "" {
		return nil, fmt.Errorf("invalid metric granularity")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT strftime(?, captured_at) AS bucket,
		       AVG(cpu_usage_percent), AVG(memory_total_bytes), AVG(memory_used_bytes)
		FROM system_metrics
		WHERE captured_at >= ? AND captured_at < ?
		GROUP BY bucket
		ORDER BY bucket`,
		bucketFormat,
		from.UTC().Format(time.RFC3339Nano),
		to.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("list system metrics: %w", err)
	}
	defer rows.Close()
	metrics := []system.MetricSample{}
	for rows.Next() {
		var sample system.MetricSample
		var capturedAt string
		var memoryTotal, memoryUsed float64
		if err := rows.Scan(&capturedAt, &sample.CPUUsagePercent, &memoryTotal, &memoryUsed); err != nil {
			return nil, fmt.Errorf("scan system metric: %w", err)
		}
		sample.CapturedAt, err = time.Parse(time.RFC3339, capturedAt)
		if err != nil {
			return nil, fmt.Errorf("parse system metric timestamp: %w", err)
		}
		sample.MemoryTotalBytes = uint64(memoryTotal)
		sample.MemoryUsedBytes = uint64(memoryUsed)
		metrics = append(metrics, sample)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate system metrics: %w", err)
	}
	return metrics, nil
}

func (s *Store) LatestSystemMetric(ctx context.Context) (system.MetricSample, bool, error) {
	var sample system.MetricSample
	var capturedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT captured_at, cpu_usage_percent, memory_total_bytes, memory_used_bytes
		FROM system_metrics
		ORDER BY captured_at DESC
		LIMIT 1`).Scan(&capturedAt, &sample.CPUUsagePercent, &sample.MemoryTotalBytes, &sample.MemoryUsedBytes)
	if errors.Is(err, sql.ErrNoRows) {
		return system.MetricSample{}, false, nil
	}
	if err != nil {
		return system.MetricSample{}, false, fmt.Errorf("get latest system metric: %w", err)
	}
	sample.CapturedAt, err = time.Parse(time.RFC3339Nano, capturedAt)
	if err != nil {
		return system.MetricSample{}, false, fmt.Errorf("parse latest system metric timestamp: %w", err)
	}
	return sample, true, nil
}

func (s *Store) OldestSystemMetric(ctx context.Context) (time.Time, bool, error) {
	var capturedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT captured_at
		FROM system_metrics
		ORDER BY captured_at ASC
		LIMIT 1`).Scan(&capturedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, fmt.Errorf("get oldest system metric: %w", err)
	}
	oldest, err := time.Parse(time.RFC3339Nano, capturedAt)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("parse oldest system metric timestamp: %w", err)
	}
	return oldest, true, nil
}

// PruneSystemMetrics bounds the local database and its backups while retaining
// enough raw samples for short-term operational troubleshooting.
func (s *Store) PruneSystemMetrics(ctx context.Context, before time.Time) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM system_metrics WHERE captured_at < ?`, before.UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("prune system metrics: %w", err)
	}
	return nil
}
