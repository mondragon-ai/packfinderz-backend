package instance

import "os"

func GetID() string {
	if id := os.Getenv("WORKER_ID"); id != "" {
		return id
	}
	return "worker-0"
}
