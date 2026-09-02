CREATE TABLE control_plane_audit (
    id INTEGER PRIMARY KEY,
    actor_user_id INTEGER NOT NULL,
    action TEXT NOT NULL,
    target TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (actor_user_id) REFERENCES users(id) ON DELETE RESTRICT
);

CREATE INDEX control_plane_audit_by_time ON control_plane_audit(created_at DESC);
