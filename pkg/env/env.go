package env

import "os"

// Get returns the value of the given environment variable or a fallback.
func Get(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
