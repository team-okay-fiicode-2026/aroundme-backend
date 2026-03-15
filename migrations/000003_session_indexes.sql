-- Speeds up cleanup queries that scan for expired or revoked sessions.
CREATE INDEX IF NOT EXISTS sessions_refresh_expires_at_idx
    ON sessions (refresh_expires_at);

CREATE INDEX IF NOT EXISTS sessions_access_expires_at_idx
    ON sessions (access_expires_at);
