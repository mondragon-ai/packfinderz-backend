package instance

import "os"

// GetID returns the worker instance identifier or a default value.
func GetID() string {
	if id := os.Getenv("WORKER_ID"); id != "" {
		return id
	}
	return "worker-0"
}
