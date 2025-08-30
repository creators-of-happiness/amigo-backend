package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port              string
	GinMode           string
	AuthSecret        string
	OTPFixedCode      string
	OTPExpiresMinutes int
	AccessTokenTTLHrs int
}

func FromEnv() Config {
	return Config{
		Port:              getenv("PORT", "8080"),
		GinMode:           getenv("GIN_MODE", "release"),
		AuthSecret:        getenv("AUTH_SECRET", "dev-secret-change-me"),
		OTPFixedCode:      getenv("OTP_FIXED_CODE", "000000"),
		OTPExpiresMinutes: mustAtoi(getenv("OTP_EXPIRES_MINUTES", "5")),
		AccessTokenTTLHrs: mustAtoi(getenv("ACCESS_TOKEN_TTL_HOURS", "24")),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func mustAtoi(s string) int {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	if err != nil {
		return 0
	}
	return v
}
