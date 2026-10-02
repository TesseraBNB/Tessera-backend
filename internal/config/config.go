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

	// AI agent transport (Option A): Tessera owns the tool-calling loop and
	// speaks only the Anthropic Messages API. Backends are tried in order:
	// Hermes (relay), the provider at AnthropicBaseURL (the Anthropic API when
	// unset, or any Messages-compatible provider), then an optional second
	// Messages-compatible provider. The fallback carries its own model id
	// because providers name the same model differently.
	HermesBaseURL    string
	HermesToken      string
	AnthropicBaseURL string
	AnthropicAPIKey  string
	Model            string
	FallbackBaseURL  string
	FallbackAPIKey   string
	FallbackModel    string // defaults to Model

	// Data sources
	OSOAPIKey   string
	GitHubToken string

	// Verdict notary: BNB Attestation Service (BAS) on BNB Smart Chain.
	// Notarisation is off when NotaryPrivateKey is empty.
	NotaryPrivateKey  string
	NotaryRPCURL      string
	NotaryBAS         string // BAS (EAS) contract
	NotaryChainID     int64
	NotaryDailyBudget int    // attestations per day; 0 = unlimited
	PublicURL         string // this API's public base URL, used in attested report links

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
	defaultAllowedOrigins     = "http://localhost:3000,https://tessera-bnb.vercel.app" // deployed UI calls the visitor's localhost backend
	defaultRateLimitRPS       = 1.0
	defaultRateLimitBurst     = 5
	defaultAgentMaxIterations = 12
	defaultAgentDailyBudget   = 0
	defaultRequestTimeout     = 90 * time.Second
	defaultCacheTTL           = 10 * time.Minute
	defaultNotaryRPCURL       = "https://bsc-testnet-rpc.publicnode.com"
	defaultNotaryBAS          = "0x6c2270298b1e6046898a322acB3Cbad6F99f7CBD" // BAS on BSC testnet
	defaultNotaryChainID      = 97
	defaultNotaryDailyBudget  = 50
)

// Load reads configuration from the process environment, applying defaults.
func Load() *Config {
	model := getEnv("TESSERA_MODEL", defaultModel)
	port := getEnv("PORT", defaultPort)
	return &Config{
		Port:               port,
		AllowedOrigins:     splitCSV(getEnv("ALLOWED_ORIGINS", defaultAllowedOrigins)),
		HermesBaseURL:      strings.TrimRight(os.Getenv("HERMES_BASE_URL"), "/"),
		HermesToken:        os.Getenv("HERMES_TOKEN"),
		AnthropicBaseURL:   strings.TrimRight(os.Getenv("ANTHROPIC_BASE_URL"), "/"),
		AnthropicAPIKey:    os.Getenv("ANTHROPIC_API_KEY"),
		Model:              model,
		FallbackBaseURL:    strings.TrimRight(os.Getenv("FALLBACK_BASE_URL"), "/"),
		FallbackAPIKey:     os.Getenv("FALLBACK_API_KEY"),
		FallbackModel:      getEnv("FALLBACK_MODEL", model),
		OSOAPIKey:          os.Getenv("OSO_API_KEY"),
		GitHubToken:        os.Getenv("GITHUB_TOKEN"),
		NotaryPrivateKey:   os.Getenv("NOTARY_PRIVATE_KEY"),
		NotaryRPCURL:       getEnv("NOTARY_RPC_URL", defaultNotaryRPCURL),
		NotaryBAS:          getEnv("NOTARY_BAS_CONTRACT", defaultNotaryBAS),
		NotaryChainID:      int64(getEnvInt("NOTARY_CHAIN_ID", defaultNotaryChainID)),
		NotaryDailyBudget:  getEnvInt("NOTARY_DAILY_BUDGET", defaultNotaryDailyBudget),
		PublicURL:          publicURL(port),
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
	return c.HermesBaseURL != "" || c.AnthropicAPIKey != "" || (c.FallbackBaseURL != "" && c.FallbackAPIKey != "")
}

// Validate returns an error if the configuration is unusable.
func (c *Config) Validate() error {
	if !c.HasAgentBackend() {
		return fmt.Errorf("no AI backend configured: set HERMES_BASE_URL (relay), ANTHROPIC_API_KEY (+ ANTHROPIC_BASE_URL), or FALLBACK_BASE_URL + FALLBACK_API_KEY")
	}
	return nil
}

// --- env helpers ---

// publicURL is PUBLIC_URL, else the domain Railway assigns the service, else
// the local listener.
func publicURL(port string) string {
	if u := strings.TrimRight(os.Getenv("PUBLIC_URL"), "/"); u != "" {
		return u
	}
	if d := os.Getenv("RAILWAY_PUBLIC_DOMAIN"); d != "" {
		return "https://" + d
	}
	return "http://localhost:" + port
}

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
