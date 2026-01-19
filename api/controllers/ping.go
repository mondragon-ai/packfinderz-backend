package controllers

import (
	"net/http"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/api/responses"
)

func PublicPing() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		responses.WriteSuccess(w, map[string]string{"scope": "public", "status": "ok"})
	}
}

func PrivatePing() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload := map[string]string{"scope": "private", "status": "ok"}
		if store := middleware.StoreIDFromContext(r.Context()); store != "" {
			payload["store_id"] = store
		}
		responses.WriteSuccess(w, payload)
	}
}

func AdminPing() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload := map[string]string{"scope": "admin", "status": "ok"}
		if store := middleware.StoreIDFromContext(r.Context()); store != "" {
			payload["store_id"] = store
		}
		responses.WriteSuccess(w, payload)
	}
}

func AgentPing() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		payload := map[string]string{"scope": "agent", "status": "ok"}
		if store := middleware.StoreIDFromContext(r.Context()); store != "" {
			payload["store_id"] = store
		}
		responses.WriteSuccess(w, payload)
	}
}
