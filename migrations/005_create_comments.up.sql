CREATE TABLE comments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id         UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    author_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id       UUID REFERENCES comments(id) ON DELETE CASCADE,
    text_content    TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_comments_one_per_post ON comments(post_id, author_id) WHERE parent_id IS NULL;
CREATE UNIQUE INDEX idx_comments_one_reply ON comments(parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX idx_comments_post ON comments(post_id);
