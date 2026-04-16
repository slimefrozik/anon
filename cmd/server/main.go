package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/slimefrozik/anon/internal/bot"
	"github.com/slimefrozik/anon/internal/config"
	"github.com/slimefrozik/anon/internal/db"
	pgxredis "github.com/slimefrozik/anon/internal/redis"
	"github.com/slimefrozik/anon/internal/handler"
	"github.com/slimefrozik/anon/internal/middleware"
	"github.com/slimefrozik/anon/internal/service"
	"github.com/slimefrozik/anon/internal/worker"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	rdb, err := pgxredis.NewClient(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("failed to connect to redis: %v", err)
	}
	defer rdb.Close()

	feedSvc := service.NewFeedService(pool, rdb, cfg.S3Endpoint)
	reactionSvc := service.NewReactionService(pool)
	commentSvc := service.NewCommentService(pool)
	abuseSvc := service.NewAbuseService()
	lifecycleSvc := service.NewPostLifecycleService(pool, rdb)

	sessionHandler := handler.NewSessionHandler(pool, rdb, cfg)
	postHandler := handler.NewPostHandler(pool, rdb, feedSvc, abuseSvc, cfg.S3Endpoint)
	reactionHandler := handler.NewReactionHandler(reactionSvc)
	commentHandler := handler.NewCommentHandler(commentSvc, abuseSvc)
	notificationHandler := handler.NewNotificationHandler(commentSvc, pool)

	decayWorker := worker.NewDecayWorker(lifecycleSvc, 15*time.Minute)
	feedWorker := worker.NewFeedPrecomputeWorker(feedSvc, rdb, 30*time.Second)

	go decayWorker.Start(ctx)
	go feedWorker.Start(ctx)

	var tgBot *bot.Bot
	if cfg.TelegramBotToken != "" {
		tgBot, err = bot.New(
			cfg.TelegramBotToken,
			pool,
			feedSvc,
			reactionSvc,
			commentSvc,
			abuseSvc,
			cfg.S3Endpoint,
		)
		if err != nil {
			log.Fatalf("failed to create telegram bot: %v", err)
		}
		go tgBot.Start()
	}

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	router.POST("/sessions", sessionHandler.Create)

	auth := router.Group("/")
	auth.Use(middleware.Auth(pool))
	{
		posts := auth.Group("/posts")
		{
			posts.POST("/",
				middleware.RateLimit(rdb, 1, time.Hour, "post_rate"),
				postHandler.Create,
			)
			posts.GET("/:id", postHandler.GetPost)
			posts.POST("/:id/react",
				middleware.RateLimit(rdb, 30, time.Hour, "reaction_rate"),
				reactionHandler.React,
			)
			posts.POST("/:id/comments",
				middleware.RateLimit(rdb, 10, time.Hour, "comment_rate"),
				commentHandler.Create,
			)
			posts.GET("/:id/comments", commentHandler.GetByPost)
		}

		auth.POST("/comments/:id/reply", commentHandler.Reply)

		auth.GET("/feed", postHandler.GetFeed)

		notifications := auth.Group("/notifications")
		{
			notifications.GET("/", notificationHandler.List)
			notifications.GET("/:id/context", notificationHandler.GetContext)
			notifications.POST("/:id/read", notificationHandler.MarkRead)
		}
	}

	srv := &http.Server{
		Addr:    ":" + cfg.ServerPort,
		Handler: router,
	}

	go func() {
		log.Printf("server starting on :%s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")

	decayWorker.Stop()
	feedWorker.Stop()
	if tgBot != nil {
		tgBot.Stop()
	}
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}

	log.Println("server exited")
}
