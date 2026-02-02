package router

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/internal/analytics/types"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/payloads"
	"github.com/google/uuid"
)

func TestRouterUnsupportedEvent(t *testing.T) {
	router := newTestRouter(t, nil)
	env := types.Envelope{
		EventType: enums.AnalyticsEventType("unsupported"),
		Payload:   []byte(`{"foo":"bar"}`),
	}
	err := router.Handle(context.Background(), env)
	if !errors.Is(err, ErrUnsupportedEventType) {
		t.Fatalf("expected unsupported error, got %v", err)
	}
}

func TestRouterRoutesToHandler(t *testing.T) {
	handler := &stubHandler{}
	router := newTestRouter(t, map[enums.AnalyticsEventType]Handler{
		enums.AnalyticsEventOrderCreated: handler,
	})
	payload := payloads.OrderCreatedEvent{
		CheckoutGroupID: uuidFromString(t, "00000000-0000-0000-0000-000000000001"),
		VendorOrderIDs:  []uuid.UUID{uuidFromString(t, "00000000-0000-0000-0000-000000000002")},
	}
	data, _ := json.Marshal(payload)
	env := types.Envelope{
		EventType: enums.AnalyticsEventOrderCreated,
		Payload:   data,
	}
	if err := router.Handle(context.Background(), env); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handler.called {
		t.Fatalf("handler not invoked")
	}
}

func newTestRouter(t *testing.T, overrides map[enums.AnalyticsEventType]Handler) *Router {
	t.Helper()
	writer := &stubWriter{}
	router, err := NewRouter(writer, logger.New(logger.Options{ServiceName: "router-test"}), overrides)
	if err != nil {
		t.Fatalf("construct router: %v", err)
	}
	return router
}

type stubHandler struct {
	called bool
}

func (s *stubHandler) Handle(ctx context.Context, envelope types.Envelope, payload any) error {
	s.called = true
	return nil
}

type stubWriter struct{}

func (stubWriter) InsertMarketplace(ctx context.Context, row types.MarketplaceEventRow) error {
	return nil
}
func (stubWriter) InsertAdFact(ctx context.Context, row types.AdEventFactRow) error { return nil }

func uuidFromString(t *testing.T, value string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(value)
	if err != nil {
		t.Fatalf("parse uuid: %v", err)
	}
	return id
}
