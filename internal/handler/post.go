package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/slimefrozik/anon/internal/middleware"
	"github.com/slimefrozik/anon/internal/model"
	"github.com/slimefrozik/anon/internal/service"
)

type PostHandler struct {
	pool         *pgxpool.Pool
	rdb          *redis.Client
	feedService  *service.FeedService
	abuseService *service.AbuseService
	mediaBaseURL string
}

func NewPostHandler(pool *pgxpool.Pool, rdb *redis.Client, feedSvc *service.FeedService, abuseSvc *service.AbuseService, mediaBaseURL string) *PostHandler {
	return &PostHandler{pool: pool, rdb: rdb, feedService: feedSvc, abuseService: abuseSvc, mediaBaseURL: mediaBaseURL}
}

func (h *PostHandler) Create(c *gin.Context) {
	var req model.CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.abuseService.ValidatePost(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)

	contentType := model.ContentTypeText
	if req.ContentType == "image" {
		contentType = model.ContentTypeImage
	}

	shadowBanned := h.abuseService.ShouldShadowBan(req.TextContent)

	postID := uuid.New().String()
	now := time.Now()
	expiresAt := now.Add(service.DefaultPostTTL)
	health := 1.0
	status := 0

	if shadowBanned {
		health = 0
		status = 1
	}

	_, err = h.pool.Exec(c.Request.Context(),
		`INSERT INTO posts (id, author_id, content_type, text_content, expires_at, health, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		postID, userID, contentType, req.TextContent, expiresAt, health, status,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create post"})
		return
	}

	resp := model.NewPostResponse(model.Post{
		ID:          postID,
		ContentType: contentType,
		TextContent: req.TextContent,
		CreatedAt:   now,
		ExpiresAt:   expiresAt,
	}, h.mediaBaseURL)

	c.JSON(http.StatusCreated, resp)
}

func (h *PostHandler) GetFeed(c *gin.Context) {
	userID := middleware.GetUserID(c)

	posts, err := h.feedService.Generate(c.Request.Context(), userID, 20)
	if err != nil {
		fmt.Println("feed error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate feed"})
		return
	}

	if posts == nil {
		posts = []model.PostResponse{}
	}

	c.JSON(http.StatusOK, gin.H{"posts": posts})
}

func (h *PostHandler) GetPost(c *gin.Context) {
	postID := c.Param("id")

	var p model.Post
	err := h.pool.QueryRow(c.Request.Context(),
		`SELECT id, author_id, content_type, text_content, media_key, created_at, expires_at, health, impression_cap, impressions, status
		 FROM posts WHERE id = $1 AND status = 0`,
		postID,
	).Scan(&p.ID, &p.AuthorID, &p.ContentType, &p.TextContent, &p.MediaKey,
		&p.CreatedAt, &p.ExpiresAt, &p.Health, &p.ImpressionCap, &p.Impressions, &p.Status)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return
	}

	c.JSON(http.StatusOK, model.NewPostResponse(p, h.mediaBaseURL))
}
