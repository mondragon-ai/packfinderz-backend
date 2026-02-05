package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/internal/squarecustomers"
	"github.com/angelmondragon/packfinderz-backend/pkg/logger"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
)

func TestAdminSquareCustomerEnsureSuccess(t *testing.T) {
	svc := &stubControllerSquareCustomerService{result: "cust-abc"}
	store := &stubControllerSquareCustomerStore{}
	handler := AdminSquareCustomerEnsure(svc, store, logger.New(logger.Options{ServiceName: "test"}))

	payload := adminSquareCustomerRequest{
		StoreID:     uuid.New(),
		FirstName:   "Jamie",
		LastName:    "Rivera",
		Email:       "jamie@example.com",
		Phone:       controllerPtrString("+15550000000"),
		CompanyName: "NewCo",
		Address: types.Address{
			Line1:      "123 Main St",
			City:       "Oklahoma City",
			State:      "OK",
			PostalCode: "73102",
			Country:    "US",
		},
	}
	reqBody, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/square/customers", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rec.Code)
	}
	var envelope struct {
		Data map[string]string `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data["square_customer_id"] != "cust-abc" {
		t.Fatalf("unexpected customer id: %v", envelope.Data)
	}
	if store.lastID == nil || *store.lastID != "cust-abc" {
		t.Fatalf("store not updated with customer id")
	}
}

type stubControllerSquareCustomerService struct {
	input  squarecustomers.Input
	result string
	err    error
}

func (s *stubControllerSquareCustomerService) EnsureCustomer(ctx context.Context, input squarecustomers.Input) (string, error) {
	s.input = input
	if s.err != nil {
		return "", s.err
	}
	return s.result, nil
}

type stubControllerSquareCustomerStore struct {
	lastID *string
}

func (s *stubControllerSquareCustomerStore) UpdateSquareCustomerID(ctx context.Context, storeID uuid.UUID, customerID *string) error {
	s.lastID = customerID
	return nil
}

func controllerPtrString(value string) *string {
	return &value
}
