package responses

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
)

func TestWriteSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	WriteSuccess(w, http.StatusOK, map[string]string{"hello": "world"})

	if got := w.Code; got != http.StatusOK {
		t.Fatalf("expected status 200 but got %d", got)
	}

	var body types.SuccessEnvelope
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode success envelope: %v", err)
	}
	if body.Data.(map[string]any)["hello"] != "world" {
		t.Fatalf("unexpected payload %v", body.Data)
	}
}

func TestWriteErrorMapsTypedError(t *testing.T) {
	w := httptest.NewRecorder()
	err := pkgerrors.New(pkgerrors.CodeValidation, "bad input").
		WithDetails(map[string]string{"field": "demo"})
	WriteError(w, err)

	if got := w.Code; got != http.StatusBadRequest {
		t.Fatalf("expected status 400 but got %d", got)
	}

	var body types.ErrorEnvelope
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error envelope: %v", err)
	}
	if body.Error.Code != string(pkgerrors.CodeValidation) {
		t.Fatalf("unexpected code %s", body.Error.Code)
	}
	if body.Error.Details == nil {
		t.Fatalf("expected details in public payload")
	}
}

func TestWriteErrorDefaultsToInternalForUntrustedErrors(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, errors.New("boom"))

	if got := w.Code; got != http.StatusInternalServerError {
		t.Fatalf("expected status 500 but got %d", got)
	}

	var body types.ErrorEnvelope
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode error envelope: %v", err)
	}
	if body.Error.Code != string(pkgerrors.CodeInternal) {
		t.Fatalf("unexpected code %s", body.Error.Code)
	}
	if body.Error.Details != nil {
		t.Fatalf("details should be omitted for internal errors")
	}
}
