package checkout

import (
	"testing"

	"github.com/google/uuid"

	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
)

func TestValidateMOQ_NoViolations(t *testing.T) {
	items := []MOQValidationInput{
		{
			ProductID:   uuid.New(),
			ProductName: "Inclusive Product",
			MOQ:         1,
			Quantity:    0,
		},
		{
			ProductID:   uuid.New(),
			ProductName: "Satisfied Product",
			MOQ:         2,
			Quantity:    2,
		},
	}
	if err := ValidateMOQ(items); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateMOQ_Violations(t *testing.T) {
	violationItems := []MOQValidationInput{
		{
			ProductID:   uuid.New(),
			ProductName: "Shortfall Product",
			MOQ:         5,
			Quantity:    3,
		},
		{
			ProductID:   uuid.New(),
			ProductName: "Barely Above Zero Product",
			MOQ:         2,
			Quantity:    1,
		},
	}
	err := ValidateMOQ(violationItems)
	if err == nil {
		t.Fatal("expected error for MOQ violation")
	}
	typed := pkgerrors.As(err)
	if typed == nil {
		t.Fatalf("expected pkgerrors.Error, got %T", err)
	}
	if typed.Code() != pkgerrors.CodeStateConflict {
		t.Fatalf("expected code %s, got %s", pkgerrors.CodeStateConflict, typed.Code())
	}
	details, ok := typed.Details().(map[string]any)
	if !ok {
		t.Fatalf("expected details map, got %T", typed.Details())
	}
	rawViolations, ok := details["violations"].([]MOQViolationDetail)
	if !ok {
		t.Fatalf("expected violations slice, got %T", details["violations"])
	}
	if len(rawViolations) != len(violationItems) {
		t.Fatalf("expected %d violations, got %d", len(violationItems), len(rawViolations))
	}
	for i, violation := range rawViolations {
		input := violationItems[i]
		if violation.ProductID != input.ProductID {
			t.Fatalf("expected product id %s, got %s", input.ProductID, violation.ProductID)
		}
		if violation.ProductName != input.ProductName {
			t.Fatalf("expected product name %q, got %q", input.ProductName, violation.ProductName)
		}
		if violation.RequiredQty != input.MOQ {
			t.Fatalf("expected required qty %d, got %d", input.MOQ, violation.RequiredQty)
		}
		if violation.RequestedQty != input.Quantity {
			t.Fatalf("expected requested qty %d, got %d", input.Quantity, violation.RequestedQty)
		}
	}
}
