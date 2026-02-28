package shared

import (
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL        string
	NatsURL            string
	JWTSecret          string
	JWTExpiry          time.Duration
	RefreshTokenExpiry time.Duration
	APIHost            string
	Env                string
	LogLevel           string
}

func LoadConfig() *Config {
	_ = godotenv.Load()

	jwtExpiry, _ := time.ParseDuration(getEnv("JWT_EXPIRY", "15m"))
	refreshExpiry, _ := time.ParseDuration(getEnv("REFRESH_TOKEN_EXPIRY", "720h"))

	return &Config{
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://yggdrasil:yggdrasil@localhost:5432/yggdrasil?sslmode=disable"),
		NatsURL:            getEnv("NATS_URL", "nats://localhost:4222"),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		JWTExpiry:          jwtExpiry,
		RefreshTokenExpiry: refreshExpiry,
		APIHost:            getEnv("API_HOST", "0.0.0.0:8080"),
		Env:                getEnv("ENV", "development"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
