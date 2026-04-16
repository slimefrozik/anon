package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slimefrozik/anon/internal/model"
)

type CommentService struct {
	pool *pgxpool.Pool
}

func NewCommentService(pool *pgxpool.Pool) *CommentService {
	return &CommentService{pool: pool}
}

func (s *CommentService) Create(ctx context.Context, postID, userID, text string) (*model.Comment, error) {
	var postStatus int
	err := s.pool.QueryRow(ctx,
		`SELECT status FROM posts WHERE id = $1`, postID,
	).Scan(&postStatus)
	if err != nil {
		return nil, fmt.Errorf("post not found")
	}
	if postStatus != 0 {
		return nil, fmt.Errorf("post expired")
	}

	var existing int
	err = s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM comments WHERE post_id = $1 AND author_id = $2 AND parent_id IS NULL`,
		postID, userID,
	).Scan(&existing)
	if err != nil {
		return nil, err
	}
	if existing > 0 {
		return nil, fmt.Errorf("already commented on this post")
	}

	var c model.Comment
	err = s.pool.QueryRow(ctx,
		`INSERT INTO comments (post_id, author_id, text_content) VALUES ($1, $2, $3)
		 RETURNING id, post_id, author_id, parent_id, text_content, created_at`,
		postID, userID, text,
	).Scan(&c.ID, &c.PostID, &c.AuthorID, &c.ParentID, &c.TextContent, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert comment: %w", err)
	}

	return &c, nil
}

func (s *CommentService) Reply(ctx context.Context, commentID, userID, text string) (*model.Comment, error) {
	var parent model.Comment
	err := s.pool.QueryRow(ctx,
		`SELECT id, post_id, author_id, parent_id FROM comments WHERE id = $1`,
		commentID,
	).Scan(&parent.ID, &parent.PostID, &parent.AuthorID, &parent.ParentID)
	if err != nil {
		return nil, fmt.Errorf("comment not found")
	}

	if parent.ParentID != nil {
		return nil, fmt.Errorf("cannot reply to a reply")
	}

	var postAuthorID string
	err = s.pool.QueryRow(ctx,
		`SELECT author_id FROM posts WHERE id = $1`, parent.PostID,
	).Scan(&postAuthorID)
	if err != nil {
		return nil, fmt.Errorf("post not found")
	}

	if userID != postAuthorID {
		return nil, fmt.Errorf("only the post author can reply to comments")
	}

	var replyCount int
	err = s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM comments WHERE parent_id = $1`, commentID,
	).Scan(&replyCount)
	if err != nil {
		return nil, err
	}
	if replyCount > 0 {
		return nil, fmt.Errorf("comment already has a reply")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var c model.Comment
	err = tx.QueryRow(ctx,
		`INSERT INTO comments (post_id, author_id, parent_id, text_content) VALUES ($1, $2, $3, $4)
		 RETURNING id, post_id, author_id, parent_id, text_content, created_at`,
		parent.PostID, userID, commentID, text,
	).Scan(&c.ID, &c.PostID, &c.AuthorID, &c.ParentID, &c.TextContent, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert reply: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO notifications (user_id, type, comment_id, post_id) VALUES ($1, 0, $2, $3)`,
		parent.AuthorID, commentID, parent.PostID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert notification: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &c, nil
}

func (s *CommentService) GetByPost(ctx context.Context, postID, userID string) ([]model.Comment, error) {
	var postAuthorID string
	err := s.pool.QueryRow(ctx,
		`SELECT author_id FROM posts WHERE id = $1`, postID,
	).Scan(&postAuthorID)
	if err != nil {
		return nil, fmt.Errorf("post not found")
	}

	if userID != postAuthorID {
		return nil, fmt.Errorf("only post author can see comments")
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, post_id, author_id, parent_id, text_content, created_at
		 FROM comments WHERE post_id = $1 ORDER BY created_at ASC`,
		postID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []model.Comment
	for rows.Next() {
		var c model.Comment
		if err := rows.Scan(&c.ID, &c.PostID, &c.AuthorID, &c.ParentID, &c.TextContent, &c.CreatedAt); err != nil {
			continue
		}
		comments = append(comments, c)
	}
	return comments, nil
}

func (s *CommentService) GetNotificationContext(ctx context.Context, notificationID, userID string) (*model.NotificationContext, error) {
	var n model.Notification
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, type, comment_id, post_id, read, created_at
		 FROM notifications WHERE id = $1 AND user_id = $2`,
		notificationID, userID,
	).Scan(&n.ID, &n.UserID, &n.Type, &n.CommentID, &n.PostID, &n.Read, &n.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("notification not found")
	}

	var originalComment model.Comment
	err = s.pool.QueryRow(ctx,
		`SELECT id, post_id, author_id, parent_id, text_content, created_at
		 FROM comments WHERE id = $1`,
		n.CommentID,
	).Scan(&originalComment.ID, &originalComment.PostID, &originalComment.AuthorID, &originalComment.ParentID, &originalComment.TextContent, &originalComment.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("original comment not found")
	}

	var reply model.Comment
	err = s.pool.QueryRow(ctx,
		`SELECT id, post_id, author_id, parent_id, text_content, created_at
		 FROM comments WHERE parent_id = $1 LIMIT 1`,
		n.CommentID,
	).Scan(&reply.ID, &reply.PostID, &reply.AuthorID, &reply.ParentID, &reply.TextContent, &reply.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("reply not found")
	}

	var post model.Post
	err = s.pool.QueryRow(ctx,
		`SELECT id, author_id, content_type, text_content, media_key, created_at, expires_at, health, impression_cap, impressions, status
		 FROM posts WHERE id = $1`,
		n.PostID,
	).Scan(&post.ID, &post.AuthorID, &post.ContentType, &post.TextContent, &post.MediaKey, &post.CreatedAt, &post.ExpiresAt, &post.Health, &post.ImpressionCap, &post.Impressions, &post.Status)
	if err != nil {
		return nil, fmt.Errorf("post not found")
	}

	_, _ = s.pool.Exec(ctx,
		`UPDATE notifications SET read = true WHERE id = $1`,
		notificationID,
	)

	return &model.NotificationContext{
		YourComment: originalComment.TextContent,
		Reply:       reply.TextContent,
		Post:        model.NewPostResponse(post, ""),
	}, nil
}
