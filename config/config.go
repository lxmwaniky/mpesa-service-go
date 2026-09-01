package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port                 string
	DatabaseURL          string
	MpesaEnv             string
	MpesaConsumerKey     string
	MpesaConsumerSecret  string
	MpesaPasskey         string
	MpesaShortcode       string
	MpesaPartyB          string
	MpesaTransactionType string
	MpesaCallbackURL     string
	AppAPIKey            string
}

func LoadEnv(filenames ...string) error {
	filename := ".env"
	if len(filenames) > 0 {
		filename = filenames[0]
	}
	file, err := os.Open(filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to open env file: %w", err)
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
		_ = os.Setenv(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to scan env file: %w", err)
	}
	return nil
}

func LoadConfig() (*Config, error) {
	dbUser := getEnv("DB_USER", "mpesa_user")
	dbPassword := getEnv("DB_PASSWORD", "mpesa_password")
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbName := getEnv("DB_NAME", "mpesa_db")

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbPort, dbName)

	mpesaPartyB := getEnv("MPESA_PARTY_B", "")
	if mpesaPartyB == "" {
		mpesaPartyB = getEnv("MPESA_SHORTCODE", "")
	}

	cfg := &Config{
		Port:                 getEnv("PORT", "8080"),
		DatabaseURL:          getEnv("DATABASE_URL", dbURL),
		MpesaEnv:             getEnv("MPESA_ENV", "sandbox"),
		MpesaConsumerKey:     getEnv("MPESA_CONSUMER_KEY", ""),
		MpesaConsumerSecret:  getEnv("MPESA_CONSUMER_SECRET", ""),
		MpesaPasskey:         getEnv("MPESA_PASSKEY", ""),
		MpesaShortcode:       getEnv("MPESA_SHORTCODE", ""),
		MpesaPartyB:          mpesaPartyB,
		MpesaTransactionType: getEnv("MPESA_TRANSACTION_TYPE", "CustomerPayBillOnline"),
		MpesaCallbackURL:     getEnv("MPESA_CALLBACK_URL", ""),
		AppAPIKey:            getEnv("API_KEY", ""),
	}

	if cfg.MpesaEnv == "production" {
		if cfg.MpesaConsumerKey == "" {
			return nil, fmt.Errorf("MPESA_CONSUMER_KEY must be set in production")
		}
		if cfg.MpesaConsumerSecret == "" {
			return nil, fmt.Errorf("MPESA_CONSUMER_SECRET must be set in production")
		}
		if cfg.MpesaPasskey == "" {
			return nil, fmt.Errorf("MPESA_PASSKEY must be set in production")
		}
		if cfg.MpesaShortcode == "" {
			return nil, fmt.Errorf("MPESA_SHORTCODE must be set in production")
		}
		if cfg.MpesaCallbackURL == "" {
			return nil, fmt.Errorf("MPESA_CALLBACK_URL must be set in production")
		}
		if cfg.AppAPIKey == "" {
			return nil, fmt.Errorf("API_KEY must be set in production")
		}
	}

	if cfg.MpesaCallbackURL != "" {
		if !strings.HasPrefix(cfg.MpesaCallbackURL, "https://") {
			return nil, fmt.Errorf("MPESA_CALLBACK_URL must use HTTPS protocol")
		}
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}
