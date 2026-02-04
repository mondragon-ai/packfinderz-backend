package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/subscriptions"
	squarewebhook "github.com/angelmondragon/packfinderz-backend/internal/webhooks/square"
	"github.com/google/uuid"
)

func TestSquareWebhook_SuccessAndIdempotent(t *testing.T) {
	payload := buildSquareEvent(t, "subscription.created")
	header := buildSquareSignature(payload, "secret")
	service := &fakeSquareWebhookService{}
	store := newInMemoryStore()
	guard, err := squarewebhook.NewIdempotencyGuard(store, time.Minute, "square-webhook")
	if err != nil {
		t.Fatalf("guard setup: %v", err)
	}
	handler := SquareWebhook(service, &fakeSigningClient{secret: "secret"}, guard, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/square", bytes.NewReader(payload))
	req.Header.Set("Square-Signature", header)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if service.calls != 1 {
		t.Fatalf("expected service called once, got %d", service.calls)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/square", bytes.NewReader(payload))
	req2.Header.Set("Square-Signature", header)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 on duplicate, got %d", rec2.Code)
	}
	if service.calls != 1 {
		t.Fatalf("duplicate should not increment calls, got %d", service.calls)
	}
}

func TestSquareWebhook_InvalidSignature(t *testing.T) {
	payload := buildSquareEvent(t, "subscription.updated")
	service := &fakeSquareWebhookService{}
	store := newInMemoryStore()
	guard, err := squarewebhook.NewIdempotencyGuard(store, time.Minute, "square-webhook")
	if err != nil {
		t.Fatalf("guard setup: %v", err)
	}
	handler := SquareWebhook(service, &fakeSigningClient{secret: "secret"}, guard, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/square", bytes.NewReader(payload))
	req.Header.Set("Square-Signature", "invalid")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for invalid signature, got %d", rec.Code)
	}
	if service.calls != 0 {
		t.Fatalf("service should not be invoked on invalid signature")
	}
}

func buildSquareEvent(t *testing.T, eventType string) []byte {
	subscription := &subscriptions.SquareSubscription{
		ID:     "sub_" + uuid.NewString(),
		Status: "ACTIVE",
		Metadata: map[string]string{
			"store_id":                 uuid.NewString(),
			"square_customer_id":       "cust",
			"square_payment_method_id": "pm",
		},
		Items: &subscriptions.SquareSubscriptionItemList{
			Data: []*subscriptions.SquareSubscriptionItem{
				{CurrentPeriodStart: 1, CurrentPeriodEnd: 2, Price: &subscriptions.SquareSubscriptionPrice{ID: "price_1"}},
			},
		},
	}
	event := &squarewebhook.SquareWebhookEvent{
		EventID: "evt_" + uuid.NewString(),
		Type:    eventType,
		Data: squarewebhook.SquareWebhookData{
			ID: eventType + "_" + uuid.NewString(),
			Object: squarewebhook.SquareWebhookObject{
				Subscription: subscription,
			},
		},
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	return payload
}

func buildSquareSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

type fakeSquareWebhookService struct {
	calls int
}

func (f *fakeSquareWebhookService) HandleEvent(ctx context.Context, event *squarewebhook.SquareWebhookEvent) error {
	f.calls++
	return nil
}

type fakeSigningClient struct {
	secret string
}

func (c *fakeSigningClient) SigningSecret() string {
	return c.secret
}

type inMemoryStore struct {
	mu   sync.Mutex
	data map[string]string
}

func newInMemoryStore() *inMemoryStore {
	return &inMemoryStore{data: make(map[string]string)}
}

func (s *inMemoryStore) Get(ctx context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data[key], nil
}

func (s *inMemoryStore) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[key]; exists {
		return false, nil
	}
	s.data[key] = fmt.Sprintf("%v", value)
	return true, nil
}

func (s *inMemoryStore) IdempotencyKey(scope, id string) string {
	return fmt.Sprintf("pf:idempotency:%s:%s", scope, id)
}

func (s *inMemoryStore) Del(ctx context.Context, keys ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, key := range keys {
		delete(s.data, key)
	}
	return nil
}
