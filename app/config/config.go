// Package config reads settings from the environment (and .env when present).
package config

import (
	"errors"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL   string
	Port          string
	UserAgent     string
	CrawlDelay    time.Duration
	DetailLimit   int
	KTCMaxAgeDays int
	CrawlCron     string
	// LINE Login channel of the LIFF app (empty = LINE login off). Must be under the same
	// LINE provider as the Messaging API channel so user IDs match.
	LINELoginChannelID string
}

// Load reads .env (if any) and the environment.
func Load() Config {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("[config] .env: %v", err)
	}
	return Config{
		DatabaseURL:        env("DATABASE_URL", "postgres://localhost:5432/kardee_development?sslmode=disable"),
		Port:               env("PORT", "8090"),
		UserAgent:          env("CRAWL_USER_AGENT", "Mozilla/5.0 (compatible; KardeeBot/0.1; +https://kardee.app)"),
		CrawlDelay:         time.Duration(envFloat("CRAWL_DELAY_SECONDS", 1.5) * float64(time.Second)),
		DetailLimit:        envInt("CRAWL_DETAIL_LIMIT", 150),
		KTCMaxAgeDays:      envInt("KTC_MAX_AGE_DAYS", 180),
		CrawlCron:          env("CRAWL_CRON", "0 6,18 * * *"),
		LINELoginChannelID: env("LINE_LOGIN_CHANNEL_ID", ""),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if n, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return n
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if f, err := strconv.ParseFloat(os.Getenv(key), 64); err == nil {
		return f
	}
	return def
}

// Thai is Asia/Bangkok; banks publish dates in Thai time.
var Thai = func() *time.Location {
	if loc, err := time.LoadLocation("Asia/Bangkok"); err == nil {
		return loc
	}
	return time.FixedZone("ICT", 7*60*60)
}()
