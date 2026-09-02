CREATE TABLE users_next (
    id INTEGER PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (role IN ('root', 'member'))
);

INSERT INTO users_next (id, username, role, password_hash, created_at, updated_at)
SELECT id, username, role, password_hash, created_at, updated_at FROM users;

DROP TABLE users;
ALTER TABLE users_next RENAME TO users;
CREATE UNIQUE INDEX one_root_user ON users(role) WHERE role = 'root';
