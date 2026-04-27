package config

import (
	"os"
	"strconv"
)

// Config holds all runtime configuration for Forge.
type Config struct {
	Port           int
	DatabasePath   string
	DockerSocket   string
	APIToken       string
	MaxWorkers     int
	KubeconfigPath string
	DataDir        string
}

// Load reads configuration from environment variables, falling back to defaults.
func Load() *Config {
	return &Config{
		Port:           getEnvInt("FORGE_PORT", 8080),
		DatabasePath:   getEnv("FORGE_DB_PATH", "./data/forge.db"),
		DockerSocket:   getEnv("FORGE_DOCKER_SOCKET", "unix:///var/run/docker.sock"),
		APIToken:       getEnv("FORGE_API_TOKEN", "forge-secret"),
		MaxWorkers:     getEnvInt("FORGE_MAX_WORKERS", 5),
		KubeconfigPath: getEnv("FORGE_KUBECONFIG", ""),
		DataDir:        getEnv("FORGE_DATA_DIR", "./data"),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}
