package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/slimefrozik/anon/internal/middleware"
	"github.com/slimefrozik/anon/internal/model"
	"github.com/slimefrozik/anon/internal/service"
)

type CommentHandler struct {
	commentService *service.CommentService
	abuseService   *service.AbuseService
}

func NewCommentHandler(commentSvc *service.CommentService, abuseSvc *service.AbuseService) *CommentHandler {
	return &CommentHandler{commentService: commentSvc, abuseService: abuseSvc}
}

func (h *CommentHandler) Create(c *gin.Context) {
	postID := c.Param("id")
	userID := middleware.GetUserID(c)

	var req model.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.abuseService.ValidateComment(req.TextContent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	comment, err := h.commentService.Create(c.Request.Context(), postID, userID, req.TextContent)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, comment)
}

func (h *CommentHandler) Reply(c *gin.Context) {
	commentID := c.Param("id")
	userID := middleware.GetUserID(c)

	var req model.ReplyCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.abuseService.ValidateComment(req.TextContent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reply, err := h.commentService.Reply(c.Request.Context(), commentID, userID, req.TextContent)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, reply)
}

func (h *CommentHandler) GetByPost(c *gin.Context) {
	postID := c.Param("id")
	userID := middleware.GetUserID(c)

	comments, err := h.commentService.GetByPost(c.Request.Context(), postID, userID)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	if comments == nil {
		comments = []model.Comment{}
	}

	c.JSON(http.StatusOK, gin.H{"comments": comments})
}
