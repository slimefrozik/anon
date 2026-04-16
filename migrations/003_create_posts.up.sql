CREATE TABLE posts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content_type    SMALLINT NOT NULL, -- 0=text, 1=image
    text_content    TEXT,
    media_key       TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ NOT NULL,
    health          REAL NOT NULL DEFAULT 1.0,
    impression_cap  INT NOT NULL DEFAULT 50,
    impressions     INT NOT NULL DEFAULT 0,
    status          SMALLINT NOT NULL DEFAULT 0 -- 0=alive, 1=expired, 2=suppressed
);
CREATE INDEX idx_posts_feed ON posts(status, health DESC) WHERE status = 0;
CREATE INDEX idx_posts_expire ON posts(expires_at) WHERE status = 0;
CREATE INDEX idx_posts_author ON posts(author_id) WHERE status = 0;
