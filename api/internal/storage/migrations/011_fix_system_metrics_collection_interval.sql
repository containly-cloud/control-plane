CREATE TABLE system_monitoring_settings_next (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    retention_days INTEGER NOT NULL DEFAULT 30 CHECK (retention_days BETWEEN 1 AND 3650)
);

INSERT INTO system_monitoring_settings_next (id, enabled, retention_days)
SELECT id, enabled, retention_days FROM system_monitoring_settings;

DROP TABLE system_monitoring_settings;
ALTER TABLE system_monitoring_settings_next RENAME TO system_monitoring_settings;
