// Package config loads Tessera's runtime configuration from environment
// variables into a single typed struct, so the rest of the application never
// reaches for os.Getenv directly. Load after any .env file has been read into
// the process environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for the Tessera server and agent.
type Config struct {
	// HTTP server
	Port           string
	AllowedOrigins []string // CORS allowlist; exact origins, no wildcard

	// AI agent transport (Option A): Hermes relays the Anthropic Messages API.
	// Tessera owns the tool-calling loop. When Hermes is unset or failing, the
	// agent falls back to the Anthropic API directly.
	HermesBaseURL   string
	HermesToken     string
	AnthropicAPIKey string
	Model           string

	// Data sources
	OSOAPIKey   string
	GitHubToken string

	// Limits & guards
	RateLimitRPS       float64       // per-IP token-bucket refill rate (req/sec)
	RateLimitBurst     int           // per-IP burst size
	AgentMaxIterations int           // max tool-use rounds per agent run
	AgentDailyBudget   int           // global cap on agent runs per day (cost guard); 0 = unlimited
	RequestTimeout     time.Duration // budget for a single agent run / upstream fetch
	CacheTTL           time.Duration // in-memory cache TTL for upstream data
}

// Defaults applied when an environment variable is unset or invalid.
const (
	defaultPort               = "8080"
	defaultModel              = "claude-opus-4-8"
	defaultAllowedOrigins     = "http://localhost:3000"
	defaultRateLimitRPS       = 1.0
	defaultRateLimitBurst     = 5
	defaultAgentMaxIterations = 12
	defaultAgentDailyBudget   = 0
	defaultRequestTimeout     = 90 * time.Second
	defaultCacheTTL           = 10 * time.Minute
)

// Load reads configuration from the process environment, applying defaults.
func Load() *Config {
	return &Config{
		Port:               getEnv("PORT", defaultPort),
		AllowedOrigins:     splitCSV(getEnv("ALLOWED_ORIGINS", defaultAllowedOrigins)),
		HermesBaseURL:      strings.TrimRight(os.Getenv("HERMES_BASE_URL"), "/"),
		HermesToken:        os.Getenv("HERMES_TOKEN"),
		AnthropicAPIKey:    os.Getenv("ANTHROPIC_API_KEY"),
		Model:              getEnv("TESSERA_MODEL", defaultModel),
		OSOAPIKey:          os.Getenv("OSO_API_KEY"),
		GitHubToken:        os.Getenv("GITHUB_TOKEN"),
		RateLimitRPS:       getEnvFloat("RATE_LIMIT_RPS", defaultRateLimitRPS),
		RateLimitBurst:     getEnvInt("RATE_LIMIT_BURST", defaultRateLimitBurst),
		AgentMaxIterations: getEnvInt("AGENT_MAX_ITERATIONS", defaultAgentMaxIterations),
		AgentDailyBudget:   getEnvInt("AGENT_DAILY_BUDGET", defaultAgentDailyBudget),
		RequestTimeout:     getEnvDuration("REQUEST_TIMEOUT", defaultRequestTimeout),
		CacheTTL:           getEnvDuration("CACHE_TTL", defaultCacheTTL),
	}
}

// HasAgentBackend reports whether at least one AI backend is configured.
func (c *Config) HasAgentBackend() bool {
	return c.HermesBaseURL != "" || c.AnthropicAPIKey != ""
}

// Validate returns an error if the configuration is unusable.
func (c *Config) Validate() error {
	if !c.HasAgentBackend() {
		return fmt.Errorf("no AI backend configured: set HERMES_BASE_URL (relay) or ANTHROPIC_API_KEY (fallback)")
	}
	return nil
}

// --- env helpers ---

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
