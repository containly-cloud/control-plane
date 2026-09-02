CREATE TABLE system_monitoring_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    interval_seconds INTEGER NOT NULL DEFAULT 5 CHECK (interval_seconds BETWEEN 5 AND 86400)
);

INSERT INTO system_monitoring_settings (id, enabled, interval_seconds) VALUES (1, 1, 5);
