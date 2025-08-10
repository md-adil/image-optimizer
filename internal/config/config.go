package config

import (
	"os"
	"strconv"
)

func EnvInt(name string, fallback int) int {
	if s := os.Getenv(name); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			return v
		}
	}
	return fallback
}
