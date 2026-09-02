ALTER TABLE system_monitoring_settings
ADD COLUMN retention_days INTEGER NOT NULL DEFAULT 30 CHECK (retention_days BETWEEN 1 AND 3650);
