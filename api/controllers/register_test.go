package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/angelmondragon/packfinderz-backend/internal/auth"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
)

type stubRegisterService struct {
	err error
}

func (s stubRegisterService) Register(ctx context.Context, req auth.RegisterRequest) error {
	return s.err
}

type stubAuthService struct {
	resp *auth.LoginResponse
	err  error
}

func (s stubAuthService) Login(ctx context.Context, req auth.LoginRequest) (*auth.LoginResponse, error) {
	return s.resp, s.err
}

func (s stubAuthService) AdminLogin(ctx context.Context, req auth.LoginRequest) (*auth.AdminLoginResponse, error) {
	return nil, s.err
}

func TestAuthRegisterSuccess(t *testing.T) {
	token := "new-token"
	resp := &auth.LoginResponse{
		AccessToken:  token,
		RefreshToken: "refresh",
		Stores: []auth.StoreSummary{
			{ID: uuid.New(), Name: "test store", Type: enums.StoreTypeBuyer},
		},
	}
	handler := AuthRegister(stubRegisterService{}, stubAuthService{resp: resp}, nil)

	body := []byte(`{
		"first_name": "Alice",
		"last_name": "Buyer",
		"email": "alice@example.com",
		"password": "Secret123!",
		"company_name": "PackFinderz Store",
		"store_type": "buyer",
		"address": {
			"line1": "123 Main St",
			"city": "Oklahoma City",
			"state": "OK",
			"postal_code": "73102",
			"country": "US",
			"lat": 35.4676,
			"lng": -97.5164
		},
		"accept_tos": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	respRec := httptest.NewRecorder()

	handler.ServeHTTP(respRec, req)

	if respRec.Code != http.StatusOK {
		t.Fatalf("expected 201 got %d", respRec.Code)
	}
	if got := respRec.Header().Get("X-PF-Token"); got != token {
		t.Fatalf("expected x-pf-token %s got %s", token, got)
	}

	var envelope struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.NewDecoder(respRec.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func TestAuthRegisterPropagatesError(t *testing.T) {
	handler := AuthRegister(stubRegisterService{err: pkgerrors.New(pkgerrors.CodeConflict, "duplicate")}, stubAuthService{}, nil)

	body := []byte(`{
		"first_name": "Alice",
		"last_name": "Buyer",
		"email": "alice@example.com",
		"password": "Secret123!",
		"company_name": "PackFinderz Store",
		"store_type": "buyer",
		"address": {
			"line1": "123 Main St",
			"city": "Oklahoma City",
			"state": "OK",
			"postal_code": "73102",
			"country": "US",
			"lat": 35.4676,
			"lng": -97.5164
		},
		"accept_tos": true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	respRec := httptest.NewRecorder()

	handler.ServeHTTP(respRec, req)

	if respRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 got %d", respRec.Code)
	}
}
