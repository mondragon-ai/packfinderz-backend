package errors

import (
	stdErrors "errors"
	"net/http"
	"testing"
)

func TestMetadataForKnownCodes(t *testing.T) {
	tests := []struct {
		code      Code
		status    int
		publicMsg string
		retryable bool
		detailsOK bool
	}{
		{code: CodeValidation, status: http.StatusBadRequest, publicMsg: "validation failed", detailsOK: true},
		{code: CodeUnauthorized, status: http.StatusUnauthorized, publicMsg: "authentication required"},
		{code: CodeForbidden, status: http.StatusForbidden, publicMsg: "access denied"},
		{code: CodeNotFound, status: http.StatusNotFound, publicMsg: "resource not found"},
		{code: CodeConflict, status: http.StatusConflict, publicMsg: "conflict detected"},
		{code: CodeStateConflict, status: http.StatusUnprocessableEntity, publicMsg: "state transition disallowed", detailsOK: true},
		{code: CodeInternal, status: http.StatusInternalServerError, publicMsg: "internal server error", retryable: true},
		{code: CodeDependency, status: http.StatusServiceUnavailable, publicMsg: "dependency unavailable", retryable: true, detailsOK: true},
	}

	for _, tt := range tests {
		meta := MetadataFor(tt.code)
		if meta.HTTPStatus != tt.status {
			t.Fatalf("code %s expected status %d got %d", tt.code, tt.status, meta.HTTPStatus)
		}
		if meta.PublicMessage != tt.publicMsg {
			t.Fatalf("code %s expected public message %q got %q", tt.code, tt.publicMsg, meta.PublicMessage)
		}
		if meta.Retryable != tt.retryable {
			t.Fatalf("code %s expected retryable %v got %v", tt.code, tt.retryable, meta.Retryable)
		}
		if meta.DetailsAllowed != tt.detailsOK {
			t.Fatalf("code %s expected details allowed %v got %v", tt.code, tt.detailsOK, meta.DetailsAllowed)
		}
	}
}

func TestMetadataForUnknownCodeDefaultsToInternal(t *testing.T) {
	meta := MetadataFor("SOMETHING_UNKNOWN")
	if meta.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("expected internal status, got %d", meta.HTTPStatus)
	}
}

func TestErrorConstructors(t *testing.T) {
	base := New(CodeValidation, "missing foo")
	if base.Code() != CodeValidation {
		t.Fatalf("expected validation code, got %s", base.Code())
	}
	if base.Message() != "missing foo" {
		t.Fatalf("unexpected message %q", base.Message())
	}
	if base.Details() != nil {
		t.Fatalf("details should be nil by default")
	}

	detail := map[string]any{"field": "foo"}
	base.WithDetails(detail)
	if base.Details() == nil {
		t.Fatalf("details should be preserved")
	}

	cause := stdErrors.New("boom")
	wrapped := Wrap(CodeConflict, cause, "ctx")
	if !stdErrors.Is(wrapped, cause) {
		t.Fatalf("Wrap did not preserve cause")
	}
	if wrapped.Code() != CodeConflict {
		t.Fatalf("unexpected code %s", wrapped.Code())
	}
}

func TestAsReturnsTypedError(t *testing.T) {
	err := New(CodeForbidden, "no entry")
	if got := As(err); got == nil || got.Code() != CodeForbidden {
		t.Fatalf("As failed to return typed error")
	}
	if As(nil) != nil {
		t.Fatalf("As(nil) should return nil")
	}
}
