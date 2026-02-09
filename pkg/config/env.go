package config

import (
	"os"
	"strconv"
	"sync"
	"time"
)

type Config struct {
	DataDir         string
	AssetsURL       string
	MultiplexerURL  string
	SyncInterval    time.Duration
	Port            string
	RegistryTimeout time.Duration
	AdminToken      string
	OtpDisabled     bool
	// JWT config controls user session signing and cookies.
	JwtSecret       string
	JwtTokenExpiry  time.Duration
	JwtCookieName   string
	JwtCookiePath   string
	JwtCookieMaxAge int
	// Admin auth config controls admin session signing and cookies.
	AdminAuthSecret   string
	AdminCookieName   string
	AdminCookiePath   string
	AdminCookieMaxAge int
}

var (
	cfg  Config
	once sync.Once
)

func Load() Config {
	once.Do(func() {
		cfg = Config{
			DataDir:           envString("DATA_DIR", "data"),
			AssetsURL:         envString("ASSETS_URL", "http://localhost:3333/proxy/core/assets"),
			MultiplexerURL:    envString("MULTIPLEXER_URL", "http://localhost:3333/proxy/multiplexer/prices"),
			SyncInterval:      envDuration("SYNC_INTERVAL", time.Minute),
			Port:              envString("PORT", "8080"),
			RegistryTimeout:   envDuration("REGISTRY_TIMEOUT", 30*time.Second),
			AdminToken:        envString("ADMIN_TOKEN", ""),
			OtpDisabled:       envBool("OTP_DISABLED", false),
			JwtSecret:         envString("JWT_SECRET", ""),
			JwtTokenExpiry:    envDuration("JWT_TOKEN_EXPIRY", 24*time.Hour),
			JwtCookieName:     envString("JWT_COOKIE_NAME", "token"),
			JwtCookiePath:     envString("JWT_COOKIE_PATH", "/"),
			JwtCookieMaxAge:   envInt("JWT_COOKIE_MAX_AGE", 86400),
			AdminAuthSecret:   envString("ADMIN_AUTH_SECRET", ""),
			AdminCookieName:   envString("ADMIN_COOKIE_NAME", "admin_token"),
			AdminCookiePath:   envString("ADMIN_COOKIE_PATH", "/"),
			AdminCookieMaxAge: envInt("ADMIN_COOKIE_MAX_AGE", 7*86400),
		}
	})
	return cfg
}

func envString(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	switch value {
	case "1", "true", "TRUE", "True", "yes", "YES", "Yes":
		return true
	case "0", "false", "FALSE", "False", "no", "NO", "No":
		return false
	default:
		return fallback
	}
}

// envInt returns an integer env value or the fallback.
func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
