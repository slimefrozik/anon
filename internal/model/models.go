package model

import "time"

const (
	ContentTypeText = iota
	ContentTypeImage
)

const (
	PostStatusAlive = iota
	PostStatusExpired
	PostStatusSuppressed
)

const (
	ReactionExtend = iota
	ReactionPromote
	ReactionSkip
	ReactionSuppress
)

const (
	NotificationReplyReceived = iota
)

type User struct {
	ID        string    `json:"-"`
	CreatedAt time.Time `json:"-"`
}

type Session struct {
	ID        string    `json:"-"`
	UserID    string    `json:"-"`
	TokenHash []byte    `json:"-"`
	CreatedAt time.Time `json:"-"`
	ExpiresAt time.Time `json:"-"`
}

type Post struct {
	ID            string    `json:"id"`
	AuthorID      string    `json:"-"`
	ContentType   int       `json:"content_type"`
	TextContent   string    `json:"text_content,omitempty"`
	MediaKey      string    `json:"media_url,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	Health        float64   `json:"-"`
	ImpressionCap int       `json:"-"`
	Impressions   int       `json:"-"`
	Status        int       `json:"-"`
}

type PostResponse struct {
	ID          string `json:"id"`
	ContentType string `json:"content_type"`
	TextContent string `json:"text_content,omitempty"`
	MediaURL    string `json:"media_url,omitempty"`
	CreatedAt   string `json:"created_at"`
	ExpiresAt   string `json:"expires_at"`
}

func NewPostResponse(p Post, mediaBaseURL string) PostResponse {
	ct := "text"
	if p.ContentType == ContentTypeImage {
		ct = "image"
	}

	mediaURL := ""
	if p.MediaKey != "" && mediaBaseURL != "" {
		mediaURL = mediaBaseURL + "/" + p.MediaKey
	}

	return PostResponse{
		ID:          p.ID,
		ContentType: ct,
		TextContent: p.TextContent,
		MediaURL:    mediaURL,
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
		ExpiresAt:   p.ExpiresAt.Format(time.RFC3339),
	}
}

type Reaction struct {
	ID           string    `json:"-"`
	PostID       string    `json:"post_id"`
	UserID       string    `json:"-"`
	ReactionType int       `json:"reaction_type"`
	CreatedAt    time.Time `json:"-"`
}

type Comment struct {
	ID          string    `json:"id"`
	PostID      string    `json:"post_id"`
	AuthorID    string    `json:"-"`
	ParentID    *string   `json:"parent_id,omitempty"`
	TextContent string    `json:"text_content"`
	CreatedAt   time.Time `json:"created_at"`
}

type Notification struct {
	ID         string    `json:"id"`
	UserID     string    `json:"-"`
	Type       int       `json:"type"`
	CommentID  string    `json:"comment_id"`
	PostID     string    `json:"post_id"`
	Read       bool      `json:"read"`
	CreatedAt  time.Time `json:"created_at"`
}

type NotificationContext struct {
	YourComment string       `json:"your_comment"`
	Reply       string       `json:"reply"`
	Post        PostResponse `json:"post"`
}

type UserInfluence struct {
	UserID           string    `json:"-"`
	SuppressWeight   float64   `json:"-"`
	SuppressCount7d  int       `json:"-"`
	TotalReactions7d int       `json:"-"`
	UpdatedAt        time.Time `json:"-"`
}

type CreatePostRequest struct {
	ContentType string `json:"content_type" binding:"required"`
	TextContent string `json:"text_content"`
}

type ReactRequest struct {
	ReactionType string `json:"reaction_type" binding:"required"`
}

type CreateCommentRequest struct {
	TextContent string `json:"text_content" binding:"required,min=1,max=300"`
}

type ReplyCommentRequest struct {
	TextContent string `json:"text_content" binding:"required,min=1,max=300"`
}
