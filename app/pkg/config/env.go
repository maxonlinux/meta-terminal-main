package config

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	DataDir          string
	CoreURL          string
	MultiplexerURL   string
	SyncInterval     time.Duration
	Port             string
	RegistryTimeout  time.Duration
	AdminToken       string
	NatsURL          string
	NatsToken        string
	NatsPriceSubject string
	OtpDisabled      bool
	SiteName         string
	SmsAuthToken     string
	SmtpHost         string
	SmtpPort         int
	SmtpUser         string
	SmtpPassword     string
	SmtpFrom         string
	SmtpSkipVerify   bool
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
	OutboxSegmentSize int64
	SnowflakeNode     int64
	LogLevel          string
	LogFormat         string
}

var (
	cfg  Config
	once sync.Once
)

func Load() Config {
	once.Do(func() {
		cfg = Config{
			DataDir:           envString("DATA_DIR", "data"),
			CoreURL:           envString("CORE_URL", "http://localhost:3030/api"),
			MultiplexerURL:    envString("MULTIPLEXER_URL", "http://localhost:3333/proxy/multiplexer/prices"),
			SyncInterval:      envDuration("SYNC_INTERVAL", time.Minute),
			Port:              envString("PORT", "8080"),
			RegistryTimeout:   envDuration("REGISTRY_TIMEOUT", 30*time.Second),
			AdminToken:        envString("ADMIN_TOKEN", ""),
			NatsURL:           envString("NATS_URL", "nats://localhost:4222"),
			NatsToken:         envString("NATS_TOKEN", ""),
			NatsPriceSubject:  envString("NATS_PRICE_SUBJECT", "prices.*"),
			OtpDisabled:       envBool("OTP_DISABLED", false),
			SiteName:          envString("SITE_NAME", "Terminal"),
			SmsAuthToken:      envString("SMS_AUTH_TOKEN", ""),
			SmtpHost:          envString("SMTP_HOST", ""),
			SmtpPort:          envInt("SMTP_PORT", 25),
			SmtpUser:          envString("SMTP_USER", ""),
			SmtpPassword:      envString("SMTP_PASSWORD", ""),
			SmtpFrom:          envString("SMTP_FROM", ""),
			SmtpSkipVerify:    envBool("SMTP_SKIP_VERIFY", true),
			JwtSecret:         envString("JWT_SECRET", ""),
			JwtTokenExpiry:    envDuration("JWT_TOKEN_EXPIRY", 24*time.Hour),
			JwtCookieName:     envString("JWT_COOKIE_NAME", "token"),
			JwtCookiePath:     envString("JWT_COOKIE_PATH", "/"),
			JwtCookieMaxAge:   envInt("JWT_COOKIE_MAX_AGE", 86400),
			AdminAuthSecret:   envString("ADMIN_AUTH_SECRET", ""),
			AdminCookieName:   envString("ADMIN_COOKIE_NAME", "admin_token"),
			AdminCookiePath:   envString("ADMIN_COOKIE_PATH", "/"),
			AdminCookieMaxAge: envInt("ADMIN_COOKIE_MAX_AGE", 7*86400),
			OutboxSegmentSize: envInt64("OUTBOX_SEGMENT_SIZE", 16<<20),
			SnowflakeNode:     envInt64("SNOWFLAKE_NODE", 0),
			LogLevel:          envString("LOG_LEVEL", "info"),
			LogFormat:         envString("LOG_FORMAT", "text"),
		}

		cfg.CoreURL = strings.TrimRight(cfg.CoreURL, "/")
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

// envInt64 returns an int64 env value or the fallback.
func envInt64(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
