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

	stripewebhook "github.com/angelmondragon/packfinderz-backend/internal/webhooks/stripe"
	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v84"
)

func TestStripeWebhook_SuccessAndIdempotent(t *testing.T) {
	payload, header := buildSignedEvent(t)
	service := &fakeStripeWebhookService{}
	store := newInMemoryStore()
	guard, err := stripewebhook.NewIdempotencyGuard(store, time.Minute, "stripe-webhook")
	if err != nil {
		t.Fatalf("guard setup: %v", err)
	}
	handler := StripeWebhook(service, &fakeSigningClient{secret: "whsec_test"}, guard, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", header)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if service.calls != 1 {
		t.Fatalf("expected service called once, got %d", service.calls)
	}

	// Replay the same event
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(payload))
	req2.Header.Set("Stripe-Signature", header)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 on duplicate, got %d (%s)", rec2.Code, rec2.Body.String())
	}
	if service.calls != 1 {
		t.Fatalf("expected duplicate not processed, call count %d", service.calls)
	}
}

func TestStripeWebhook_InvalidSignature(t *testing.T) {
	payload, _ := buildSignedEvent(t)
	service := &fakeStripeWebhookService{}
	store := newInMemoryStore()
	guard, err := stripewebhook.NewIdempotencyGuard(store, time.Minute, "stripe-webhook")
	if err != nil {
		t.Fatalf("guard setup: %v", err)
	}
	handler := StripeWebhook(service, &fakeSigningClient{secret: "whsec_test"}, guard, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/stripe", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", "t=1,v1=invalid")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for invalid signature, got %d", rec.Code)
	}
	if service.calls != 0 {
		t.Fatalf("service should not be invoked on invalid signature")
	}
}

func buildSignedEvent(t *testing.T) ([]byte, string) {
	subscription := &stripe.Subscription{
		ID:     "sub_" + uuid.NewString(),
		Status: stripe.SubscriptionStatusActive,
		Metadata: map[string]string{
			"store_id":                 uuid.NewString(),
			"stripe_customer_id":       "cust",
			"stripe_payment_method_id": "pm",
		},
		Items: &stripe.SubscriptionItemList{
			Data: []*stripe.SubscriptionItem{
				{
					CurrentPeriodStart: 1,
					CurrentPeriodEnd:   2,
					Price: &stripe.Price{
						ID: "price_1",
					},
				},
			},
		},
	}
	rawSub, err := json.Marshal(subscription)
	if err != nil {
		t.Fatalf("marshal subscription: %v", err)
	}
	event := &stripe.Event{
		ID:         "evt_" + uuid.NewString(),
		Type:       stripe.EventTypeCustomerSubscriptionCreated,
		Object:     "event",
		APIVersion: stripe.APIVersion,
		Data: &stripe.EventData{
			Raw: rawSub,
		},
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	header := buildStripeSignatureHeader(payload, "whsec_test", time.Now().Unix())
	return payload, header
}

func buildStripeSignatureHeader(payload []byte, secret string, ts int64) string {
	signedPayload := fmt.Sprintf("%d.%s", ts, payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	return fmt.Sprintf("t=%d,v1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
}

type fakeStripeWebhookService struct {
	calls int
}

func (f *fakeStripeWebhookService) HandleEvent(ctx context.Context, event *stripe.Event) error {
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
	return &inMemoryStore{
		data: make(map[string]string),
	}
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
