package controllers

import (
	"testing"

	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
)

func TestCreateProductRequestToCreateInput(t *testing.T) {
	req := createProductRequest{
		SKU:        "test-sku",
		Title:      "Test Product",
		Category:   "flower",
		Feelings:   []string{"relaxed"},
		Flavors:    []string{"earthy"},
		Usage:      []string{"stress_relief"},
		Unit:       "unit",
		MOQ:        1,
		PriceCents: 100,
		Inventory: createInventoryRequest{
			AvailableQty: 5,
		},
	}

	input, err := req.toCreateInput()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if input.Category != enums.ProductCategoryFlower {
		t.Fatalf("expected category flower, got %s", input.Category)
	}
	if !input.IsActive {
		t.Fatalf("expected default is_active true")
	}
	if input.IsFeatured {
		t.Fatalf("expected default is_featured false")
	}
	if input.Inventory.AvailableQty != 5 {
		t.Fatalf("expected inventory available 5, got %d", input.Inventory.AvailableQty)
	}
}

func TestCreateProductRequestInvalidCategory(t *testing.T) {
	req := createProductRequest{
		SKU:        "test-sku",
		Title:      "Test Product",
		Category:   "invalid",
		Feelings:   []string{"relaxed"},
		Flavors:    []string{"earthy"},
		Usage:      []string{"stress_relief"},
		Unit:       "unit",
		MOQ:        1,
		PriceCents: 100,
		Inventory: createInventoryRequest{
			AvailableQty: 5,
		},
	}

	if _, err := req.toCreateInput(); err == nil {
		t.Fatal("expected validation error for category")
	}
}
