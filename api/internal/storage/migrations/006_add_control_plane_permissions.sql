CREATE TABLE user_permissions (
    user_id INTEGER NOT NULL,
    permission TEXT NOT NULL,
    PRIMARY KEY (user_id, permission),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
