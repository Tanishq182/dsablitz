package configs

import "os"

const defaultHTTPAddr = ":8080"

type Config struct {
	HTTPAddr string
}

func Load() Config {
	return Config{
		HTTPAddr: getEnv("HTTP_ADDR", defaultHTTPAddr),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
