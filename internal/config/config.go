package config

import (
	"errors"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort               string
	DatabaseURL           string
	CORSOrigin            string
	Env                   string
	MigrationsDir         string
	UploadsDir            string
	AccessTokenTTLMinutes int
	RefreshTokenTTLHours  int
	AllowDevSocialAuth    bool
	ExpoPushAccessToken   string
	AnthropicAPIKey       string
	SQSQueueURL           string
	NotificationQueueURL  string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	env := getEnv("ENV", "development")

	cfg := Config{
		AppPort:               getEnv("APP_PORT", "8080"),
		DatabaseURL:           getEnv("DATABASE_URL", ""),
		CORSOrigin:            getEnv("CORS_ORIGIN", "http://localhost:8081,http://localhost:19006,http://localhost:3000"),
		Env:                   env,
		MigrationsDir:         getEnv("MIGRATIONS_DIR", "migrations"),
		UploadsDir:            getEnv("UPLOADS_DIR", "uploads"),
		AccessTokenTTLMinutes: getEnvInt("ACCESS_TOKEN_TTL_MINUTES", 15),
		RefreshTokenTTLHours:  getEnvInt("REFRESH_TOKEN_TTL_HOURS", 24*30),
		AllowDevSocialAuth:    getEnvBool("ALLOW_DEV_SOCIAL_AUTH", false),
		ExpoPushAccessToken:   getEnv("EXPO_PUSH_ACCESS_TOKEN", ""),
		AnthropicAPIKey:       getEnv("ANTHROPIC_API_KEY", ""),
		SQSQueueURL:           getEnv("SQS_QUEUE_URL", ""),
		NotificationQueueURL:  getEnv("NOTIFICATION_QUEUE_URL", ""),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}
