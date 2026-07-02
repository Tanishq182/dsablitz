package configs

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

const (
	defaultEnv  = "development"
	defaultPort = "8080"
)

type Config struct {
	Port        string
	Env         string
	DatabaseURL string
	RedisURL    string
	JWTSecret   string
	HTTPAddr    string
}

func (c Config) IsProduction() bool {
	return c.Env == "production"
}

func Load() Config {
	_ = godotenv.Load(".env", "backend/.env")

	port := getEnv("PORT", defaultPort)

	return Config{
		Port:        port,
		Env:         getEnv("ENV", defaultEnv),
		DatabaseURL: getEnv("DATABASE_URL", ""),
		RedisURL:    getEnv("REDIS_URL", ""),
		JWTSecret:   getEnv("JWT_SECRET", ""),
		HTTPAddr:    formatHTTPAddr(port),
	}
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func formatHTTPAddr(port string) string {
	if strings.HasPrefix(port, ":") {
		return port
	}

	return fmt.Sprintf(":%s", port)
}
