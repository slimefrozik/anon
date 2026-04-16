package config

import "os"

type Config struct {
	DatabaseURL      string
	RedisAddr        string
	RedisPassword    string
	RedisDB          int
	ServerPort       string
	SessionTTL       string
	MediaStoragePath string
	S3Endpoint       string
	S3Bucket         string
	S3AccessKey      string
	S3SecretKey      string
	TelegramBotToken string
	AdminPassword    string
}

func Load() *Config {
	return &Config{
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://anon:anon@localhost:5432/anon?sslmode=disable"),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		RedisDB:          0,
		ServerPort:       getEnv("SERVER_PORT", "8080"),
		SessionTTL:       getEnv("SESSION_TTL", "720h"),
		MediaStoragePath: getEnv("MEDIA_STORAGE_PATH", "./media"),
		S3Endpoint:       getEnv("S3_ENDPOINT", ""),
		S3Bucket:         getEnv("S3_BUCKET", "anon-media"),
		S3AccessKey:      getEnv("S3_ACCESS_KEY", ""),
		S3SecretKey:      getEnv("S3_SECRET_KEY", ""),
		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		AdminPassword:    getEnv("ADMIN_PASSWORD", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
