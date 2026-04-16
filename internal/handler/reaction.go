package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/slimefrozik/anon/internal/middleware"
	"github.com/slimefrozik/anon/internal/model"
	"github.com/slimefrozik/anon/internal/service"
)

type ReactionHandler struct {
	reactionService *service.ReactionService
}

func NewReactionHandler(svc *service.ReactionService) *ReactionHandler {
	return &ReactionHandler{reactionService: svc}
}

func (h *ReactionHandler) React(c *gin.Context) {
	postID := c.Param("id")
	userID := middleware.GetUserID(c)

	var req model.ReactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reactionType, err := service.ParseReactionType(req.ReactionType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.reactionService.Process(c.Request.Context(), postID, userID, reactionType); err != nil {
		status := http.StatusBadRequest
		if err.Error() == "already reacted" {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "recorded"})
}
