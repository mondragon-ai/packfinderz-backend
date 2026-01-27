package controllers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/angelmondragon/packfinderz-backend/api/middleware"
	"github.com/angelmondragon/packfinderz-backend/internal/notifications"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
)

type testNotificationsService struct {
	markReadFn    func(ctx context.Context, storeID, notificationID uuid.UUID) error
	markAllReadFn func(ctx context.Context, storeID uuid.UUID) (int64, error)
	listFn        func(ctx context.Context, params notifications.ListParams) (*notifications.ListResult, error)
}

func (s *testNotificationsService) List(ctx context.Context, params notifications.ListParams) (*notifications.ListResult, error) {
	if s.listFn != nil {
		return s.listFn(ctx, params)
	}
	return nil, nil
}

func (s *testNotificationsService) MarkRead(ctx context.Context, storeID, notificationID uuid.UUID) error {
	if s.markReadFn != nil {
		return s.markReadFn(ctx, storeID, notificationID)
	}
	return nil
}

func (s *testNotificationsService) MarkAllRead(ctx context.Context, storeID uuid.UUID) (int64, error) {
	if s.markAllReadFn != nil {
		return s.markAllReadFn(ctx, storeID)
	}
	return 0, nil
}

func TestMarkNotificationReadSuccess(t *testing.T) {
	storeID := uuid.New()
	notificationID := uuid.New()
	called := false
	svc := &testNotificationsService{
		markReadFn: func(ctx context.Context, sid, nid uuid.UUID) error {
			called = true
			if sid != storeID {
				t.Fatalf("unexpected store %s", sid)
			}
			if nid != notificationID {
				t.Fatalf("unexpected notification %s", nid)
			}
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/"+notificationID.String()+"/read", nil)
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("notificationId", notificationID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))

	resp := httptest.NewRecorder()
	handler := MarkNotificationRead(svc, logger.New(logger.Options{ServiceName: "test", Output: io.Discard}))
	handler(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", resp.Code)
	}
	if !called {
		t.Fatal("expected service called")
	}
	var envelope struct {
		Data map[string]bool `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !envelope.Data["read"] {
		t.Fatal("response missing read flag")
	}
}

func TestMarkNotificationReadMissingStore(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/"+uuid.NewString()+"/read", nil)
	req = addRouteParam(req, "notificationId", uuid.NewString())
	resp := httptest.NewRecorder()
	handler := MarkNotificationRead(&testNotificationsService{}, logger.New(logger.Options{ServiceName: "test", Output: io.Discard}))
	handler(resp, req)
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d", resp.Code)
	}
}

func TestMarkNotificationReadInvalidStore(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/"+uuid.NewString()+"/read", nil)
	req = req.WithContext(middleware.WithStoreID(req.Context(), "bad"))
	req = addRouteParam(req, "notificationId", uuid.NewString())
	resp := httptest.NewRecorder()
	handler := MarkNotificationRead(&testNotificationsService{}, logger.New(logger.Options{ServiceName: "test", Output: io.Discard}))
	handler(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", resp.Code)
	}
}

func TestMarkNotificationReadInvalidID(t *testing.T) {
	storeID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/invalid/read", nil)
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))
	req = addRouteParam(req, "notificationId", "invalid")
	resp := httptest.NewRecorder()
	handler := MarkNotificationRead(&testNotificationsService{}, logger.New(logger.Options{ServiceName: "test", Output: io.Discard}))
	handler(resp, req)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", resp.Code)
	}
}

func TestMarkAllNotificationsReadSuccess(t *testing.T) {
	storeID := uuid.New()
	called := false
	svc := &testNotificationsService{
		markAllReadFn: func(ctx context.Context, sid uuid.UUID) (int64, error) {
			called = true
			if sid != storeID {
				t.Fatalf("unexpected store %s", sid)
			}
			return 5, nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/read-all", nil)
	req = req.WithContext(middleware.WithStoreID(req.Context(), storeID.String()))
	resp := httptest.NewRecorder()
	handler := MarkAllNotificationsRead(svc, logger.New(logger.Options{ServiceName: "test", Output: io.Discard}))
	handler(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", resp.Code)
	}
	if !called {
		t.Fatal("expected service called")
	}
	var envelope struct {
		Data map[string]float64 `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if envelope.Data["updated"] != 5 {
		t.Fatalf("expected updated=5 got %v", envelope.Data["updated"])
	}
}

func addRouteParam(req *http.Request, key, value string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}
