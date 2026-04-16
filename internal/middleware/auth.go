package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ctxKey string

const UserIDKey ctxKey = "user_id"

func Auth(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		if token == header {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token format"})
			return
		}

		hash := sha256.Sum256([]byte(token))

		var userID string
		err := pool.QueryRow(c.Request.Context(),
			`SELECT user_id FROM sessions WHERE token_hash = $1 AND expires_at > now()`,
			hash[:],
		).Scan(&userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired session"})
			return
		}

		c.Set(string(UserIDKey), userID)
		c.Next()
	}
}

func GetUserID(c *gin.Context) string {
	v, exists := c.Get(string(UserIDKey))
	if !exists {
		return ""
	}
	return v.(string)
}

func RequireSelfOrAuthor(pool *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := GetUserID(c)
		postID := c.Param("id")

		var authorID string
		err := pool.QueryRow(c.Request.Context(),
			`SELECT author_id FROM posts WHERE id = $1`,
			postID,
		).Scan(&authorID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}

		if subtle.ConstantTimeCompare([]byte(userID), []byte(authorID)) != 1 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "not authorized"})
			return
		}

		c.Next()
	}
}
