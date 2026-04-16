package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/slimefrozik/anon/internal/config"
)

type SessionHandler struct {
	pool       *pgxpool.Pool
	rdb        *redis.Client
	sessionTTL time.Duration
}

func NewSessionHandler(pool *pgxpool.Pool, rdb *redis.Client, cfg *config.Config) *SessionHandler {
	ttl, _ := time.ParseDuration(cfg.SessionTTL)
	if ttl == 0 {
		ttl = 720 * time.Hour
	}
	return &SessionHandler{pool: pool, rdb: rdb, sessionTTL: ttl}
}

type SessionResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

func (h *SessionHandler) Create(c *gin.Context) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}
	token := hex.EncodeToString(tokenBytes)
	hash := sha256.Sum256([]byte(token))

	userID := uuid.New().String()

	_, err := h.pool.Exec(c.Request.Context(),
		`INSERT INTO users (id) VALUES ($1) ON CONFLICT DO NOTHING`,
		userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	expiresAt := time.Now().Add(h.sessionTTL)
	sessionID := uuid.New().String()

	_, err = h.pool.Exec(c.Request.Context(),
		`INSERT INTO sessions (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`,
		sessionID, userID, hash[:], expiresAt,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}

	_, _ = h.pool.Exec(c.Request.Context(),
		`INSERT INTO user_influence (user_id) VALUES ($1) ON CONFLICT DO NOTHING`,
		userID,
	)

	h.rdb.Set(c.Request.Context(), "session:"+hex.EncodeToString(hash[:]), userID, h.sessionTTL)

	c.JSON(http.StatusCreated, SessionResponse{
		Token:     token,
		ExpiresAt: expiresAt.Format(time.RFC3339),
	})
}
