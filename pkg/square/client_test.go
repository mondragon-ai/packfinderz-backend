package square

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	sq "github.com/square/square-go-sdk"
	sqcore "github.com/square/square-go-sdk/core"

	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
)

func TestEnsureIdempotencyKey(t *testing.T) {
	c := &Client{}
	// Provided key should be used verbatim.
	if got := c.ensureIdempotencyKey("pref", "custom-key"); got != "custom-key" {
		t.Fatalf("expected provided key, got %q", got)
	}
	// Empty key should be generated and include prefix.
	if got := c.ensureIdempotencyKey("prefix", ""); !strings.HasPrefix(got, "prefix-") {
		t.Fatalf("generated idempotency key %q missing prefix", got)
	}
}

func TestRedact(t *testing.T) {
	c := &Client{}
	out := c.redact("payment_token", "abc123")
	if out != "[REDACTED]" {
		t.Fatalf("expected redacted value, got %v", out)
	}
	// Non-sensitive keys should be preserved.
	if v := c.redact("status", "ok"); v != "ok" {
		t.Fatalf("unexpected redaction for safe key")
	}
}

func TestDomainCodeForStatus(t *testing.T) {
	tests := []struct {
		status int
		code   pkgerrors.Code
	}{
		{http.StatusUnauthorized, pkgerrors.CodeUnauthorized},
		{http.StatusForbidden, pkgerrors.CodeForbidden},
		{http.StatusNotFound, pkgerrors.CodeNotFound},
		{http.StatusConflict, pkgerrors.CodeConflict},
		{http.StatusTooManyRequests, pkgerrors.CodeRateLimit},
		{http.StatusBadRequest, pkgerrors.CodeValidation},
		{http.StatusUnprocessableEntity, pkgerrors.CodeStateConflict},
		{http.StatusInternalServerError, pkgerrors.CodeDependency},
	}
	for _, tt := range tests {
		if got := domainCodeForStatus(tt.status); got != tt.code {
			t.Fatalf("status %d expected %s got %s", tt.status, tt.code, got)
		}
	}
}

func TestMapSquareError(t *testing.T) {
	c := &Client{}
	table := []struct {
		name     string
		status   int
		payload  string
		wantCode pkgerrors.Code
	}{
		{
			name:     "authentication error",
			status:   http.StatusUnauthorized,
			payload:  `{"errors":[{"category":"AUTHENTICATION_ERROR","code":"UNAUTHORIZED"}]}`,
			wantCode: pkgerrors.CodeUnauthorized,
		},
		{
			name:     "idempotency key reused",
			status:   http.StatusConflict,
			payload:  `{"errors":[{"category":"API_ERROR","code":"IDEMPOTENCY_KEY_REUSED"}]}`,
			wantCode: pkgerrors.CodeIdempotency,
		},
	}
	for _, tt := range table {
		err := sqcore.NewAPIError(tt.status, errors.New(tt.payload))
		mapped := c.mapSquareError(err, "operation")
		if mapped == nil {
			t.Fatalf("%s: expected error", tt.name)
		}
		typed := pkgerrors.As(mapped)
		if typed == nil {
			t.Fatalf("%s: result is not pkgerror", tt.name)
		}
		if typed.Code() != tt.wantCode {
			t.Fatalf("%s: expected code %s, got %s", tt.name, tt.wantCode, typed.Code())
		}
	}
}

func TestExtractSquareErrors(t *testing.T) {
	c := &Client{}
	payload := `{"errors":[{"category":"API_ERROR","code":"BAD_REQUEST","detail":"oops"}]}`
	apiErr := sqcore.NewAPIError(http.StatusBadRequest, errors.New(payload))
	got := c.extractSquareErrors(apiErr)
	if len(got) != 1 {
		t.Fatalf("expected 1 error, got %d", len(got))
	}
	if got[0].GetCode() != sq.ErrorCodeBadRequest {
		t.Fatalf("unexpected error code %s", got[0].GetCode())
	}
}
