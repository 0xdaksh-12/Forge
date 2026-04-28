package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
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
	MasterKey      string
}

// Load reads configuration from environment variables, falling back to defaults.
func Load() *Config {
	dataDir := getEnv("FORGE_DATA_DIR", "./data")
	masterKey := loadOrGenerateMasterKey(dataDir)

	return &Config{
		Port:           getEnvInt("FORGE_PORT", 8080),
		DatabasePath:   getEnv("FORGE_DB_PATH", filepath.Join(dataDir, "forge.db")),
		DockerSocket:   getEnv("FORGE_DOCKER_SOCKET", "unix:///var/run/docker.sock"),
		APIToken:       getEnv("FORGE_API_TOKEN", "forge-secret"),
		MaxWorkers:     getEnvInt("FORGE_MAX_WORKERS", 5),
		KubeconfigPath: getEnv("FORGE_KUBECONFIG", ""),
		DataDir:        dataDir,
		MasterKey:      masterKey,
	}
}

func loadOrGenerateMasterKey(dataDir string) string {
	if key := os.Getenv("FORGE_MASTER_KEY"); key != "" {
		return key
	}

	keyFile := filepath.Join(dataDir, ".forge_master_key")
	if data, err := os.ReadFile(keyFile); err == nil && len(data) > 0 {
		return string(data)
	}

	// Generate a new 32-byte (256-bit) master key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(fmt.Sprintf("failed to generate master key: %v", err))
	}
	hexKey := hex.EncodeToString(key)

	// Ensure data directory exists
	os.MkdirAll(dataDir, 0o755)

	// Write to file with restrictive permissions
	if err := os.WriteFile(keyFile, []byte(hexKey), 0o600); err != nil {
		panic(fmt.Sprintf("failed to save master key to %s: %v", keyFile, err))
	}

	fmt.Printf("🔒 Generated new master key and saved to %s\n", keyFile)
	return hexKey
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
