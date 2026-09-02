CREATE TABLE system_metrics (
    id INTEGER PRIMARY KEY,
    captured_at TEXT NOT NULL,
    cpu_usage_percent REAL NOT NULL,
    memory_total_bytes INTEGER NOT NULL,
    memory_used_bytes INTEGER NOT NULL
);

CREATE INDEX system_metrics_by_capture ON system_metrics(captured_at);
