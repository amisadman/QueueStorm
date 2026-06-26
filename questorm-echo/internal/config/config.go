package config

import (
	"bufio"
	"os"
	"strings"
)

// Config holds the application configuration loaded from environment variables.
type Config struct {
	Port               string
	GeminiAPIKey       string
	GeminiAPIKeyBackup string
	GroqAPIKey         string
}

// loadEnvFile reads a .env file and sets environment variables if they are not already set.
// It searches the current directory and walks up to parent directories to ensure the .env is found.
func loadEnvFile() {
	dir := "."
	// Search up to 3 parent levels (e.g. if running from cmd/server/)
	for i := 0; i < 4; i++ {
		path := dir + "/.env"
		file, err := os.Open(path)
		if err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				parts := strings.SplitN(line, "=", 2)
				if len(parts) != 2 {
					continue
				}

				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])

				// Strip surrounding quotes if present
				val = strings.Trim(val, `"'`)

				// Only set if not already set in the environment
				if os.Getenv(key) == "" {
					os.Setenv(key, val)
				}
			}
			return // Found and loaded successfully
		}
		dir = dir + "/.."
	}
}

// LoadConfig reads configuration from the environment, loading from .env if present.
func LoadConfig() *Config {
	loadEnvFile()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		Port:               port,
		GeminiAPIKey:       os.Getenv("GEMINI_API_KEY"),
		GeminiAPIKeyBackup: os.Getenv("GEMINI_API_KEY_BACKUP"),
		GroqAPIKey:         os.Getenv("GROQ_API_KEY"),
	}
}
