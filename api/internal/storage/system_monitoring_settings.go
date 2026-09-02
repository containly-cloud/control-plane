package storage

import (
	"context"
	"fmt"

	"api/internal/system"
)

func (s *Store) GetSystemMonitoringSettings(ctx context.Context) (system.MonitoringSettings, error) {
	settings := system.DefaultMonitoringSettings()
	err := s.db.QueryRowContext(ctx, `SELECT enabled, interval_seconds, retention_days FROM system_monitoring_settings WHERE id=1`).Scan(
		&settings.Enabled,
		&settings.IntervalSeconds,
		&settings.RetentionDays,
	)
	if err != nil {
		return system.MonitoringSettings{}, fmt.Errorf("get system monitoring settings: %w", err)
	}
	return settings, nil
}

func (s *Store) SetSystemMonitoringSettings(ctx context.Context, settings system.MonitoringSettings) error {
	if !system.ValidMonitoringSettings(settings) {
		return fmt.Errorf("invalid system monitoring settings")
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE system_monitoring_settings SET enabled=?, interval_seconds=?, retention_days=? WHERE id=1`,
		settings.Enabled,
		settings.IntervalSeconds,
		settings.RetentionDays,
	)
	if err != nil {
		return fmt.Errorf("set system monitoring settings: %w", err)
	}
	return nil
}

// SystemMetricsStorageBytes returns the approximate bytes occupied by metric
// values. SQLite may additionally reserve reusable page space.
func (s *Store) SystemMetricsStorageBytes(ctx context.Context) (uint64, error) {
	var bytes uint64
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(
			LENGTH(CAST(captured_at AS BLOB)) +
			LENGTH(CAST(cpu_usage_percent AS BLOB)) +
			LENGTH(CAST(memory_total_bytes AS BLOB)) +
			LENGTH(CAST(memory_used_bytes AS BLOB))
		), 0)
		FROM system_metrics`).Scan(&bytes)
	if err != nil {
		return 0, fmt.Errorf("measure system metrics: %w", err)
	}
	return bytes, nil
}

// ClearSystemMetrics removes saved metrics and vacuums SQLite so released
// pages are returned to the filesystem instead of only being reused.
func (s *Store) ClearSystemMetrics(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM system_metrics`); err != nil {
		return fmt.Errorf("clear system metrics: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `VACUUM`); err != nil {
		return fmt.Errorf("vacuum system metrics: %w", err)
	}
	return nil
}
