CREATE TABLE user_influence (
    user_id             UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    suppress_weight     REAL NOT NULL DEFAULT 1.0,
    suppress_count_7d   INT NOT NULL DEFAULT 0,
    total_reactions_7d  INT NOT NULL DEFAULT 0,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
