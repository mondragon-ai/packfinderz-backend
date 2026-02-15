package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

var defaultCORSOrigins = []string{
	"http://localhost:3000",                                           // local dev
	"https://packfinderz-62265cad6213.herokuapp.com",                  // backend API
	"https://pack-finderz.vercel.app",                                 // Vercel domain
	"https://pack-finderz-db5jp8k0h-mondragonais-projects.vercel.app", // Vercel deployment URL
}

// CORS returns middleware that applies the API's allowed origin policy.
func CORS() func(http.Handler) http.Handler {
	return cors.New(cors.Options{
		AllowedOrigins:   defaultCORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-PF-Token", "Idempotency-Key", "X-Requested-With"},
		ExposedHeaders:   []string{"X-PF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	}).Handler
}
