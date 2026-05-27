package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppAddr     string
	BaseURL     string
	DataDir     string
	StoreKind   string
	DatabaseURL string
	FileTTL     time.Duration
	Threshold   float64

	AdminUser     string
	AdminPassword string

	DeepSeekAPIKey  string
	DeepSeekBaseURL string
	DeepSeekModel   string

	DashScopeAPIKey         string
	DashScopeBaseURL        string
	DashScopeEmbeddingModel string

	OneBotHTTPAPI     string
	OneBotAccessToken string
	QQAllowedIDs      map[string]bool

	ClawbotToken string

	MaxJobInputTokens int

	LogLevel string
	LogJSON  bool
}

func Load() Config {
	dataDir := env("HERMESCLAW_DATA_DIR", "data")
	return Config{
		AppAddr:                 env("HERMESCLAW_ADDR", ":8080"),
		BaseURL:                 strings.TrimRight(env("HERMESCLAW_BASE_URL", "http://localhost:8080"), "/"),
		DataDir:                 filepath.Clean(dataDir),
		StoreKind:               env("HERMESCLAW_STORE", ""),
		DatabaseURL:             os.Getenv("HERMESCLAW_DATABASE_URL"),
		FileTTL:                 envDuration("HERMESCLAW_FILE_TTL", 30*time.Minute),
		Threshold:               envFloat("HERMESCLAW_RAG_THRESHOLD", 0.60),
		AdminUser:               env("HERMESCLAW_ADMIN_USER", "admin"),
		AdminPassword:           env("HERMESCLAW_ADMIN_PASSWORD", "change-me"),
		DeepSeekAPIKey:          os.Getenv("DEEPSEEK_API_KEY"),
		DeepSeekBaseURL:         strings.TrimRight(env("DEEPSEEK_BASE_URL", "https://api.deepseek.com"), "/"),
		DeepSeekModel:           env("DEEPSEEK_MODEL", "deepseek-v4-flash"),
		DashScopeAPIKey:         os.Getenv("DASHSCOPE_API_KEY"),
		DashScopeBaseURL:        strings.TrimRight(env("DASHSCOPE_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"), "/"),
		DashScopeEmbeddingModel: env("DASHSCOPE_EMBEDDING_MODEL", "text-embedding-v4"),
		OneBotHTTPAPI:           strings.TrimRight(os.Getenv("ONEBOT_HTTP_API"), "/"),
		OneBotAccessToken:       os.Getenv("ONEBOT_ACCESS_TOKEN"),
		QQAllowedIDs:            parseAllowed(os.Getenv("QQ_ALLOWED_IDS")),
		ClawbotToken:            os.Getenv("CLAWBOT_TOKEN"),
		MaxJobInputTokens:       envInt("HERMESCLAW_MAX_JOB_INPUT_TOKENS", 32000),
		LogLevel:               env("HERMESCLAW_LOG_LEVEL", "info"),
		LogJSON:                envBool("HERMESCLAW_LOG_JSON", false),
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envFloat(key string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	return raw == "true" || raw == "1" || raw == "yes"
}

func parseAllowed(raw string) map[string]bool {
	allowed := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		id := strings.TrimSpace(part)
		if id != "" {
			allowed[id] = true
		}
	}
	return allowed
}
