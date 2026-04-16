CREATE TABLE telegram_sessions (
    telegram_id  BIGINT PRIMARY KEY,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_tg_sessions_user ON telegram_sessions(user_id);

CREATE TABLE pending_comments (
    user_id     UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    post_id     UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
