package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slimefrozik/anon/internal/middleware"
	"github.com/slimefrozik/anon/internal/model"
	"github.com/slimefrozik/anon/internal/service"
)

type NotificationHandler struct {
	commentService *service.CommentService
	pool           *pgxpool.Pool
}

func NewNotificationHandler(commentSvc *service.CommentService, pool *pgxpool.Pool) *NotificationHandler {
	return &NotificationHandler{commentService: commentSvc, pool: pool}
}

func (h *NotificationHandler) List(c *gin.Context) {
	userID := middleware.GetUserID(c)

	rows, err := h.pool.Query(c.Request.Context(),
		`SELECT id, user_id, type, comment_id, post_id, read, created_at
		 FROM notifications
		 WHERE user_id = $1 AND read = false
		 ORDER BY created_at DESC
		 LIMIT 50`,
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch notifications"})
		return
	}
	defer rows.Close()

	notifications := make([]model.Notification, 0)
	for rows.Next() {
		var n model.Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.CommentID, &n.PostID, &n.Read, &n.CreatedAt); err != nil {
			continue
		}
		notifications = append(notifications, n)
	}

	c.JSON(http.StatusOK, gin.H{"notifications": notifications})
}

func (h *NotificationHandler) GetContext(c *gin.Context) {
	notificationID := c.Param("id")
	userID := middleware.GetUserID(c)

	ctx, err := h.commentService.GetNotificationContext(c.Request.Context(), notificationID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ctx)
}

func (h *NotificationHandler) MarkRead(c *gin.Context) {
	notificationID := c.Param("id")
	userID := middleware.GetUserID(c)

	_, err := h.pool.Exec(c.Request.Context(),
		`UPDATE notifications SET read = true WHERE id = $1 AND user_id = $2`,
		notificationID, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mark read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "read"})
}
