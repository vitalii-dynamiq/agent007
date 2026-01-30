package main

import (
	"bufio"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dynamiq/manus-like/internal/api"
	"github.com/dynamiq/manus-like/internal/config"
)

func main() {
	// Load .env files if they exist (repo root or backend/)
	loadEnvFiles()

	// Load configuration
	cfg := config.Load()

	// Validate required config
	if cfg.LLMAPIKey == "" {
		log.Fatal("LLM_API_KEY is required")
	}
	if cfg.E2BAPIKey == "" {
		log.Fatal("E2B_API_KEY is required")
	}
	if cfg.PipedreamClientID == "" || cfg.PipedreamClientSecret == "" {
		log.Fatal("PIPEDREAM_CLIENT_ID and PIPEDREAM_CLIENT_SECRET are required")
	}
	if strings.TrimSpace(cfg.JWTSecret) == "" || cfg.JWTSecret == "default-secret-change-me" {
		log.Fatal("JWT_SECRET must be set to a secure random value")
	}
	if len(cfg.JWTSecret) < 32 {
		log.Fatal("JWT_SECRET must be at least 32 characters")
	}

	// Create handlers
	handlers, err := api.NewHandlers(cfg)
	if err != nil {
		log.Fatalf("Failed to create handlers: %v", err)
	}

	// Create router
	router := api.NewRouter(handlers)

	// Start server
	addr := ":" + cfg.Port
	log.Printf("Server starting on %s", addr)
	log.Printf("Frontend URL: %s", cfg.FrontendURL)

	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func loadEnvFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		return // File doesn't exist, skip
	}
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
		value := strings.TrimSpace(parts[1])

		// Only set if not already set
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

func loadEnvFiles() {
	cwd, err := os.Getwd()
	if err != nil {
		loadEnvFile(".env")
		loadEnvFile(".env.integrations")
		return
	}

	root := findRepoRoot(cwd)
	paths := []string{
		filepath.Join(cwd, ".env"),
		filepath.Join(cwd, ".env.integrations"),
	}
	if root != "" {
		paths = append(paths,
			filepath.Join(root, "backend", ".env"),
			filepath.Join(root, "backend", ".env.integrations"),
			filepath.Join(root, ".env"),
			filepath.Join(root, ".env.integrations"),
		)
	}

	for _, path := range paths {
		loadEnvFile(path)
	}
}

func findRepoRoot(start string) string {
	dir := start
	for i := 0; i < 6; i++ {
		if hasDir(dir, "backend") && hasDir(dir, "frontend") {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func hasDir(base, name string) bool {
	info, err := os.Stat(filepath.Join(base, name))
	return err == nil && info.IsDir()
}
