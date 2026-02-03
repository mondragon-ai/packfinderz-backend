package checkout

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/cart"
	"github.com/angelmondragon/packfinderz-backend/internal/checkout/reservation"
	"github.com/angelmondragon/packfinderz-backend/internal/memberships"
	"github.com/angelmondragon/packfinderz-backend/internal/orders"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/pagination"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestServiceUsesVendorGroupTotals(t *testing.T) {
	t.Parallel()

	buyerID := uuid.New()
	vendorID := uuid.New()
	invalidVendorID := uuid.New()
	productID := uuid.New()

	cartRecord := &models.CartRecord{
		ID:           uuid.New(),
		BuyerStoreID: buyerID,
		Status:       enums.CartStatusActive,
		Currency:     enums.CurrencyUSD,
		ValidUntil:   time.Now().Add(10 * time.Minute),
		Items: []models.CartItem{
			{
				ID:                    uuid.New(),
				ProductID:             productID,
				VendorStoreID:         vendorID,
				Quantity:              2,
				UnitPriceCents:        1500,
				LineSubtotalCents:     2500,
				Status:                enums.CartItemStatusOK,
				AppliedVolumeDiscount: &types.AppliedVolumeDiscount{Label: "tier 2", AmountCents: 500},
			},
			{
				ID:                uuid.New(),
				ProductID:         uuid.New(),
				VendorStoreID:     invalidVendorID,
				Quantity:          1,
				UnitPriceCents:    1200,
				LineSubtotalCents: 1200,
				Status:            enums.CartItemStatusInvalid,
			},
		},
		VendorGroups: []models.CartVendorGroup{
			{
				VendorStoreID: vendorID,
				Status:        enums.VendorGroupStatusOK,
				SubtotalCents: 3000,
				TotalCents:    2500,
			},
			{
				VendorStoreID: invalidVendorID,
				Status:        enums.VendorGroupStatusInvalid,
				SubtotalCents: 0,
				TotalCents:    0,
			},
		},
		ShippingAddress: &types.Address{Line1: "Old", City: "Broken", State: "OK", PostalCode: "00000", Country: "US"},
	}

	shippingAddress := &types.Address{Line1: "123 Market", City: "Tulsa", State: "OK", PostalCode: "74104", Country: "US"}
	shippingLine := &types.ShippingLine{Code: "express", Title: "Express", PriceCents: 500}

	cartRepo := &stubCartRepo{
		record: cartRecord,
	}

	storeSvc := &stubStoreService{
		records: map[uuid.UUID]*stores.StoreDTO{
			buyerID: {
				ID:          buyerID,
				Type:        enums.StoreTypeBuyer,
				KYCStatus:   enums.KYCStatusVerified,
				Address:     types.Address{State: "OK"},
				CompanyName: "Buyer",
			},
			vendorID: {
				ID:                 vendorID,
				Type:               enums.StoreTypeVendor,
				KYCStatus:          enums.KYCStatusVerified,
				SubscriptionActive: true,
				Address:            types.Address{State: "OK"},
				CompanyName:        "Vendor",
			},
		},
	}

	productLoader := stubProductLoader{
		products: map[uuid.UUID]*models.Product{
			productID: {
				ID:       productID,
				StoreID:  vendorID,
				SKU:      "SKU123",
				Title:    "Test Product",
				Category: enums.ProductCategoryFlower,
				Unit:     enums.ProductUnitGram,
				Strain:   ptrString("Blue Dream"),
			},
		},
	}

	reserver := stubReservationRunner{
		results: map[uuid.UUID]reservation.InventoryReservationResult{},
	}
	for _, item := range cartRecord.Items {
		if item.Status == enums.CartItemStatusOK {
			reserver.results[item.ID] = reservation.InventoryReservationResult{
				CartItemID: item.ID,
				ProductID:  item.ProductID,
				Qty:        item.Quantity,
				Reserved:   true,
			}
		}
	}

	orderRepo := newStubOrdersRepository()
	publisher := &stubOutboxPublisher{}

	service, err := NewService(
		stubTxRunner{},
		cartRepo,
		orderRepo,
		storeSvc,
		productLoader,
		reserver,
		publisher,
		false,
	)
	if err != nil {
		t.Fatalf("build service: %v", err)
	}

	result, err := service.Execute(context.Background(), buyerID, cartRecord.ID, CheckoutInput{
		IdempotencyKey:  "key",
		ShippingAddress: shippingAddress,
		PaymentMethod:   enums.PaymentMethodCash,
		ShippingLine:    shippingLine,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if cartRepo.updated == nil {
		t.Fatalf("expected cart update")
	}
	if cartRepo.updated.Status != enums.CartStatusConverted {
		t.Fatalf("cart status not converted: %s", cartRepo.updated.Status)
	}
	if cartRepo.updated.PaymentMethod == nil || *cartRepo.updated.PaymentMethod != enums.PaymentMethodCash {
		t.Fatalf("cart payment method missing")
	}
	if cartRepo.updated.ShippingLine == nil || cartRepo.updated.ShippingLine.Code != "express" {
		t.Fatalf("cart shipping line not updated")
	}

	if len(result.VendorOrders) != 1 {
		t.Fatalf("expected 1 vendor order, got %d", len(result.VendorOrders))
	}

	order := result.VendorOrders[0]
	if order.ShippingAddress == nil || order.ShippingAddress.Line1 != "123 Market" {
		t.Fatalf("vendor order missing shipping address")
	}
	if order.VendorStoreID != vendorID {
		t.Fatalf("unexpected vendor: %s", order.VendorStoreID)
	}
	if order.SubtotalCents != 3000 {
		t.Fatalf("subtotal mismatch: got %d", order.SubtotalCents)
	}
	if order.TotalCents != 2500 {
		t.Fatalf("total mismatch: got %d", order.TotalCents)
	}
	if order.DiscountsCents != 500 {
		t.Fatalf("discount mismatch: got %d", order.DiscountsCents)
	}
	if order.BalanceDueCents != 2500 {
		t.Fatalf("balance due mismatch: %d", order.BalanceDueCents)
	}

	if len(order.Items) != 1 {
		t.Fatalf("expected 1 line item, got %d", len(order.Items))
	}
	item := order.Items[0]
	if item.TotalCents != 2500 {
		t.Fatalf("line total mismatch: %d", item.TotalCents)
	}
	if item.DiscountCents != 500 {
		t.Fatalf("line discount mismatch: %d", item.DiscountCents)
	}
	if item.LineSubtotalCents != 2500 {
		t.Fatalf("line subtotal mismatch: %d", item.LineSubtotalCents)
	}

	if len(orderRepo.vendorOrders) != 1 {
		t.Fatalf("unexpected vendor order count in repo: %d", len(orderRepo.vendorOrders))
	}
	intent, ok := orderRepo.paymentIntents[order.ID]
	if !ok {
		t.Fatalf("payment intent missing for order %s", order.ID)
	}
	if intent.AmountCents != order.TotalCents {
		t.Fatalf("payment intent amount mismatch (%d vs %d)", intent.AmountCents, order.TotalCents)
	}
	if intent.Method != enums.PaymentMethodCash {
		t.Fatalf("payment intent method mismatch: %s", intent.Method)
	}
	if intent.Status != enums.PaymentStatusUnpaid {
		t.Fatalf("payment intent status changed: %s", intent.Status)
	}
	if len(result.CartVendorGroups) != len(cartRecord.VendorGroups) {
		t.Fatalf("expected %d vendor groups in response, got %d", len(cartRecord.VendorGroups), len(result.CartVendorGroups))
	}
}

func TestServiceAllowsACHWhenFlagEnabled(t *testing.T) {
	t.Parallel()

	buyerID := uuid.New()
	vendorID := uuid.New()
	productID := uuid.New()

	cartRecord := &models.CartRecord{
		ID:           uuid.New(),
		BuyerStoreID: buyerID,
		Status:       enums.CartStatusActive,
		Currency:     enums.CurrencyUSD,
		ValidUntil:   time.Now().Add(10 * time.Minute),
		Items: []models.CartItem{
			{
				ID:                uuid.New(),
				ProductID:         productID,
				VendorStoreID:     vendorID,
				Quantity:          1,
				UnitPriceCents:    2000,
				LineSubtotalCents: 2000,
				Status:            enums.CartItemStatusOK,
			},
		},
		VendorGroups: []models.CartVendorGroup{
			{
				VendorStoreID: vendorID,
				Status:        enums.VendorGroupStatusOK,
				SubtotalCents: 2000,
				TotalCents:    2000,
			},
		},
	}

	cartRepo := &stubCartRepo{record: cartRecord}
	storeSvc := &stubStoreService{
		records: map[uuid.UUID]*stores.StoreDTO{
			buyerID: {
				ID:        buyerID,
				Type:      enums.StoreTypeBuyer,
				KYCStatus: enums.KYCStatusVerified,
				Address:   types.Address{State: "OK"},
			},
			vendorID: {
				ID:                 vendorID,
				Type:               enums.StoreTypeVendor,
				KYCStatus:          enums.KYCStatusVerified,
				SubscriptionActive: true,
				Address:            types.Address{State: "OK"},
			},
		},
	}

	productLoader := stubProductLoader{
		products: map[uuid.UUID]*models.Product{
			productID: {
				ID:       productID,
				StoreID:  vendorID,
				SKU:      "SKU-ACH",
				Unit:     enums.ProductUnitUnit,
				Category: enums.ProductCategoryFlower,
			},
		},
	}

	reserver := stubReservationRunner{
		results: map[uuid.UUID]reservation.InventoryReservationResult{},
	}
	reserver.results[cartRecord.Items[0].ID] = reservation.InventoryReservationResult{
		CartItemID: cartRecord.Items[0].ID,
		ProductID:  cartRecord.Items[0].ProductID,
		Qty:        cartRecord.Items[0].Quantity,
		Reserved:   true,
	}

	orderRepo := newStubOrdersRepository()
	publisher := &stubOutboxPublisher{}

	service, err := NewService(
		stubTxRunner{},
		cartRepo,
		orderRepo,
		storeSvc,
		productLoader,
		reserver,
		publisher,
		true,
	)
	if err != nil {
		t.Fatalf("build service: %v", err)
	}

	result, err := service.Execute(context.Background(), buyerID, cartRecord.ID, CheckoutInput{
		IdempotencyKey:  "ach-key",
		ShippingAddress: &types.Address{Line1: "123 Market", City: "Tulsa", State: "OK", PostalCode: "74104", Country: "US"},
		PaymentMethod:   enums.PaymentMethodACH,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(result.VendorOrders) != 1 {
		t.Fatalf("expected 1 vendor order, got %d", len(result.VendorOrders))
	}

	order := result.VendorOrders[0]
	intent, ok := orderRepo.paymentIntents[order.ID]
	if !ok {
		t.Fatalf("payment intent missing for order %s", order.ID)
	}
	if intent.Method != enums.PaymentMethodACH {
		t.Fatalf("payment intent method mismatch: %s", intent.Method)
	}
	if intent.Status != enums.PaymentStatusPending {
		t.Fatalf("expected pending status for ACH intent, got %s", intent.Status)
	}
	if intent.AmountCents != order.TotalCents {
		t.Fatalf("payment intent amount mismatch (%d vs %d)", intent.AmountCents, order.TotalCents)
	}
}

func TestServiceRejectsACHWhenFlagDisabled(t *testing.T) {
	t.Parallel()

	buyerID := uuid.New()
	vendorID := uuid.New()
	productID := uuid.New()

	cartRecord := &models.CartRecord{
		ID:           uuid.New(),
		BuyerStoreID: buyerID,
		Status:       enums.CartStatusActive,
		Currency:     enums.CurrencyUSD,
		ValidUntil:   time.Now().Add(10 * time.Minute),
		Items: []models.CartItem{
			{
				ID:                uuid.New(),
				ProductID:         productID,
				VendorStoreID:     vendorID,
				Quantity:          1,
				UnitPriceCents:    1200,
				LineSubtotalCents: 1200,
				Status:            enums.CartItemStatusOK,
			},
		},
		VendorGroups: []models.CartVendorGroup{
			{
				VendorStoreID: vendorID,
				Status:        enums.VendorGroupStatusOK,
				SubtotalCents: 1200,
				TotalCents:    1200,
			},
		},
	}

	cartRepo := &stubCartRepo{record: cartRecord}
	storeSvc := &stubStoreService{
		records: map[uuid.UUID]*stores.StoreDTO{
			buyerID: {
				ID:        buyerID,
				Type:      enums.StoreTypeBuyer,
				KYCStatus: enums.KYCStatusVerified,
				Address:   types.Address{State: "OK"},
			},
			vendorID: {
				ID:                 vendorID,
				Type:               enums.StoreTypeVendor,
				KYCStatus:          enums.KYCStatusVerified,
				SubscriptionActive: true,
				Address:            types.Address{State: "OK"},
			},
		},
	}

	productLoader := stubProductLoader{
		products: map[uuid.UUID]*models.Product{
			productID: {
				ID:       productID,
				StoreID:  vendorID,
				SKU:      "SKU-ACH",
				Unit:     enums.ProductUnitUnit,
				Category: enums.ProductCategoryFlower,
			},
		},
	}

	reserver := stubReservationRunner{
		results: map[uuid.UUID]reservation.InventoryReservationResult{},
	}
	reserver.results[cartRecord.Items[0].ID] = reservation.InventoryReservationResult{
		CartItemID: cartRecord.Items[0].ID,
		ProductID:  cartRecord.Items[0].ProductID,
		Qty:        cartRecord.Items[0].Quantity,
		Reserved:   true,
	}

	orderRepo := newStubOrdersRepository()
	publisher := &stubOutboxPublisher{}

	service, err := NewService(
		stubTxRunner{},
		cartRepo,
		orderRepo,
		storeSvc,
		productLoader,
		reserver,
		publisher,
		false,
	)
	if err != nil {
		t.Fatalf("build service: %v", err)
	}

	if _, err := service.Execute(context.Background(), buyerID, cartRecord.ID, CheckoutInput{
		IdempotencyKey:  "ach-key",
		ShippingAddress: &types.Address{Line1: "123 Market", City: "Tulsa", State: "OK", PostalCode: "74104", Country: "US"},
		PaymentMethod:   enums.PaymentMethodACH,
	}); err == nil {
		t.Fatalf("expected error when ACH disabled")
	} else if typed := pkgerrors.As(err); typed == nil {
		t.Fatalf("unexpected error type: %v", err)
	} else if typed.Code() != pkgerrors.CodeValidation {
		t.Fatalf("expected validation error, got %s", typed.Code())
	} else if typed.Message() != "ach payments are disabled" {
		t.Fatalf("unexpected error message: %s", typed.Message())
	}
}

func TestServiceRejectsExpiredCartQuote(t *testing.T) {
	t.Parallel()

	buyerID := uuid.New()
	vendorID := uuid.New()
	productID := uuid.New()

	cartRecord := &models.CartRecord{
		ID:           uuid.New(),
		BuyerStoreID: buyerID,
		Status:       enums.CartStatusActive,
		Currency:     enums.CurrencyUSD,
		ValidUntil:   time.Now().Add(-1 * time.Minute),
		Items: []models.CartItem{
			{
				ID:                uuid.New(),
				ProductID:         productID,
				VendorStoreID:     vendorID,
				Quantity:          1,
				UnitPriceCents:    1000,
				LineSubtotalCents: 1000,
				Status:            enums.CartItemStatusOK,
			},
		},
		VendorGroups: []models.CartVendorGroup{
			{
				VendorStoreID: vendorID,
				Status:        enums.VendorGroupStatusOK,
				SubtotalCents: 1000,
				TotalCents:    1000,
			},
		},
	}

	cartRepo := &stubCartRepo{record: cartRecord}
	storeSvc := &stubStoreService{
		records: map[uuid.UUID]*stores.StoreDTO{
			buyerID: {
				ID:        buyerID,
				Type:      enums.StoreTypeBuyer,
				KYCStatus: enums.KYCStatusVerified,
				Address:   types.Address{State: "OK"},
			},
			vendorID: {
				ID:                 vendorID,
				Type:               enums.StoreTypeVendor,
				KYCStatus:          enums.KYCStatusVerified,
				SubscriptionActive: true,
				Address:            types.Address{State: "OK"},
			},
		},
	}

	productLoader := stubProductLoader{
		products: map[uuid.UUID]*models.Product{
			productID: {
				ID:       productID,
				StoreID:  vendorID,
				SKU:      "SKU123",
				Unit:     enums.ProductUnitUnit,
				Category: enums.ProductCategoryFlower,
			},
		},
	}

	reserver := stubReservationRunner{
		results: map[uuid.UUID]reservation.InventoryReservationResult{},
	}
	reserver.results[cartRecord.Items[0].ID] = reservation.InventoryReservationResult{
		CartItemID: cartRecord.Items[0].ID,
		ProductID:  cartRecord.Items[0].ProductID,
		Qty:        cartRecord.Items[0].Quantity,
		Reserved:   true,
	}

	orderRepo := newStubOrdersRepository()
	publisher := &stubOutboxPublisher{}

	service, err := NewService(
		stubTxRunner{},
		cartRepo,
		orderRepo,
		storeSvc,
		productLoader,
		reserver,
		publisher,
		false,
	)
	if err != nil {
		t.Fatalf("build service: %v", err)
	}

	if _, err := service.Execute(context.Background(), buyerID, cartRecord.ID, CheckoutInput{
		IdempotencyKey: "key",
	}); err == nil {
		t.Fatalf("expected error for expired cart")
	} else if typed := pkgerrors.As(err); typed == nil {
		t.Fatalf("unexpected error type: %v", err)
	} else if typed.Code() != pkgerrors.CodeConflict {
		t.Fatalf("expected conflict code, got %s", typed.Code())
	} else if typed.Message() != "cart quote expired" {
		t.Fatalf("unexpected error message: %s", typed.Message())
	}
}

func TestServiceReplaysConvertedCart(t *testing.T) {
	t.Parallel()

	buyerID := uuid.New()
	vendorID := uuid.New()
	productID := uuid.New()
	checkoutGroupID := uuid.New()

	cartRecord := &models.CartRecord{
		ID:              uuid.New(),
		BuyerStoreID:    buyerID,
		Status:          enums.CartStatusConverted,
		Currency:        enums.CurrencyUSD,
		CheckoutGroupID: &checkoutGroupID,
		ValidUntil:      time.Now().Add(15 * time.Minute),
		VendorGroups: []models.CartVendorGroup{
			{
				VendorStoreID: vendorID,
				Status:        enums.VendorGroupStatusOK,
				SubtotalCents: 1000,
				TotalCents:    1000,
			},
		},
	}

	order := &models.VendorOrder{
		ID:              uuid.New(),
		CheckoutGroupID: checkoutGroupID,
		BuyerStoreID:    buyerID,
		VendorStoreID:   vendorID,
		SubtotalCents:   1000,
		TotalCents:      1000,
		BalanceDueCents: 1000,
		Status:          enums.VendorOrderStatusCreatedPending,
	}

	cartRepo := &stubCartRepo{record: cartRecord}
	storeSvc := &stubStoreService{
		records: map[uuid.UUID]*stores.StoreDTO{
			buyerID: {
				ID:        buyerID,
				Type:      enums.StoreTypeBuyer,
				KYCStatus: enums.KYCStatusVerified,
				Address:   types.Address{State: "OK"},
			},
		},
	}

	orderRepo := newStubOrdersRepository()
	orderRepo.vendorOrders[order.ID] = order
	orderRepo.lineItems[order.ID] = []models.OrderLineItem{
		{
			ID:                uuid.New(),
			OrderID:           order.ID,
			ProductID:         &productID,
			Name:              "Sample",
			Category:          "Flower",
			Unit:              enums.ProductUnitUnit,
			MOQ:               1,
			UnitPriceCents:    1000,
			Qty:               1,
			LineSubtotalCents: 1000,
			TotalCents:        1000,
			Status:            enums.LineItemStatusPending,
			Warnings:          types.CartItemWarnings{},
		},
	}
	orderRepo.paymentIntents[order.ID] = &models.PaymentIntent{
		OrderID:     order.ID,
		Method:      enums.PaymentMethodCash,
		Status:      enums.PaymentStatusUnpaid,
		AmountCents: 1000,
	}

	publisher := &stubOutboxPublisher{}

	service, err := NewService(
		stubTxRunner{},
		cartRepo,
		orderRepo,
		storeSvc,
		stubProductLoader{products: map[uuid.UUID]*models.Product{}},
		stubReservationRunner{},
		publisher,
		false,
	)
	if err != nil {
		t.Fatalf("build service: %v", err)
	}

	result, err := service.Execute(context.Background(), buyerID, cartRecord.ID, CheckoutInput{
		IdempotencyKey: "key",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result == nil {
		t.Fatal("expected checkout group")
	}
	if result.ID != checkoutGroupID {
		t.Fatalf("expected checkout group %s, got %s", checkoutGroupID, result.ID)
	}
	if len(result.VendorOrders) != 1 {
		t.Fatalf("expected 1 vendor order, got %d", len(result.VendorOrders))
	}
	if cartRepo.updated != nil {
		t.Fatalf("expected no cart update on replay")
	}
}

func TestServiceRejectsVendorWhenAllReservationsFail(t *testing.T) {
	t.Parallel()

	buyerID := uuid.New()
	vendorID := uuid.New()
	productID := uuid.New()

	cartRecord := &models.CartRecord{
		ID:           uuid.New(),
		BuyerStoreID: buyerID,
		Status:       enums.CartStatusActive,
		Currency:     enums.CurrencyUSD,
		ValidUntil:   time.Now().Add(10 * time.Minute),
		Items: []models.CartItem{
			{
				ID:                uuid.New(),
				ProductID:         productID,
				VendorStoreID:     vendorID,
				Quantity:          2,
				UnitPriceCents:    1500,
				LineSubtotalCents: 3000,
				Status:            enums.CartItemStatusOK,
			},
		},
		VendorGroups: []models.CartVendorGroup{
			{
				VendorStoreID: vendorID,
				Status:        enums.VendorGroupStatusOK,
				SubtotalCents: 3000,
				TotalCents:    3000,
			},
		},
	}

	cartRepo := &stubCartRepo{record: cartRecord}

	storeSvc := &stubStoreService{
		records: map[uuid.UUID]*stores.StoreDTO{
			buyerID: {
				ID:          buyerID,
				Type:        enums.StoreTypeBuyer,
				KYCStatus:   enums.KYCStatusVerified,
				Address:     types.Address{State: "OK"},
				CompanyName: "Buyer",
			},
			vendorID: {
				ID:                 vendorID,
				Type:               enums.StoreTypeVendor,
				KYCStatus:          enums.KYCStatusVerified,
				SubscriptionActive: true,
				Address:            types.Address{State: "OK"},
				CompanyName:        "Vendor",
			},
		},
	}

	productLoader := stubProductLoader{
		products: map[uuid.UUID]*models.Product{
			productID: {
				ID:       productID,
				StoreID:  vendorID,
				SKU:      "SKU123",
				Title:    "Test Product",
				Category: enums.ProductCategoryFlower,
				Unit:     enums.ProductUnitGram,
			},
		},
	}

	reserver := stubReservationRunner{
		results: map[uuid.UUID]reservation.InventoryReservationResult{},
	}
	reserver.results[cartRecord.Items[0].ID] = reservation.InventoryReservationResult{
		CartItemID: cartRecord.Items[0].ID,
		ProductID:  cartRecord.Items[0].ProductID,
		Qty:        cartRecord.Items[0].Quantity,
		Reserved:   false,
		Reason:     "insufficient_inventory",
	}

	orderRepo := newStubOrdersRepository()
	publisher := &stubOutboxPublisher{}

	service, err := NewService(
		stubTxRunner{},
		cartRepo,
		orderRepo,
		storeSvc,
		productLoader,
		reserver,
		publisher,
		false,
	)
	if err != nil {
		t.Fatalf("build service: %v", err)
	}

	result, err := service.Execute(context.Background(), buyerID, cartRecord.ID, CheckoutInput{
		IdempotencyKey:  "key",
		ShippingAddress: &types.Address{Line1: "123 Market", City: "Tulsa", State: "OK", PostalCode: "74104", Country: "US"},
		PaymentMethod:   enums.PaymentMethodCash,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(result.VendorOrders) != 1 {
		t.Fatalf("expected 1 vendor order, got %d", len(result.VendorOrders))
	}

	order := result.VendorOrders[0]
	if order.Status != enums.VendorOrderStatusRejected {
		t.Fatalf("expected vendor order rejected, got %s", order.Status)
	}
	if order.BalanceDueCents != 0 {
		t.Fatalf("expected balance due 0 for rejected vendor, got %d", order.BalanceDueCents)
	}

	if len(order.Items) != 1 {
		t.Fatalf("expected 1 line item, got %d", len(order.Items))
	}
	if order.Items[0].Status != enums.LineItemStatusRejected {
		t.Fatalf("line item not rejected: %s", order.Items[0].Status)
	}

	if len(orderRepo.vendorOrders) != 1 {
		t.Fatalf("unexpected vendor order count in repo: %d", len(orderRepo.vendorOrders))
	}
	orderID := order.ID
	if payment, ok := orderRepo.paymentIntents[orderID]; ok {
		if payment.Status != enums.PaymentStatusUnpaid {
			t.Fatalf("payment intent status changed: %s", payment.Status)
		}
	}
}

func TestServiceAdjustsTotalsWhenSomeReservationsFail(t *testing.T) {
	t.Parallel()

	buyerID := uuid.New()
	vendorID := uuid.New()
	product1ID := uuid.New()
	product2ID := uuid.New()

	item1 := models.CartItem{
		ID:                    uuid.New(),
		ProductID:             product1ID,
		VendorStoreID:         vendorID,
		Quantity:              2,
		UnitPriceCents:        2000,
		LineSubtotalCents:     3500,
		Status:                enums.CartItemStatusOK,
		AppliedVolumeDiscount: &types.AppliedVolumeDiscount{Label: "tier 2", AmountCents: 500},
	}
	item2 := models.CartItem{
		ID:                uuid.New(),
		ProductID:         product2ID,
		VendorStoreID:     vendorID,
		Quantity:          1,
		UnitPriceCents:    1000,
		LineSubtotalCents: 1000,
		Status:            enums.CartItemStatusOK,
	}

	cartRecord := &models.CartRecord{
		ID:           uuid.New(),
		BuyerStoreID: buyerID,
		Status:       enums.CartStatusActive,
		Currency:     enums.CurrencyUSD,
		ValidUntil:   time.Now().Add(10 * time.Minute),
		Items:        []models.CartItem{item1, item2},
		VendorGroups: []models.CartVendorGroup{
			{
				VendorStoreID: vendorID,
				Status:        enums.VendorGroupStatusOK,
				SubtotalCents: 5000,
				TotalCents:    4500,
			},
		},
	}

	cartRepo := &stubCartRepo{record: cartRecord}

	storeSvc := &stubStoreService{
		records: map[uuid.UUID]*stores.StoreDTO{
			buyerID: {
				ID:          buyerID,
				Type:        enums.StoreTypeBuyer,
				KYCStatus:   enums.KYCStatusVerified,
				Address:     types.Address{State: "OK"},
				CompanyName: "Buyer",
			},
			vendorID: {
				ID:                 vendorID,
				Type:               enums.StoreTypeVendor,
				KYCStatus:          enums.KYCStatusVerified,
				SubscriptionActive: true,
				Address:            types.Address{State: "OK"},
				CompanyName:        "Vendor",
			},
		},
	}

	productLoader := stubProductLoader{
		products: map[uuid.UUID]*models.Product{
			product1ID: {
				ID:       product1ID,
				StoreID:  vendorID,
				SKU:      "SKU123",
				Title:    "Test Product 1",
				Category: enums.ProductCategoryFlower,
			},
			product2ID: {
				ID:       product2ID,
				StoreID:  vendorID,
				SKU:      "SKU321",
				Title:    "Test Product 2",
				Category: enums.ProductCategoryPreRoll,
			},
		},
	}

	reserver := stubReservationRunner{
		results: map[uuid.UUID]reservation.InventoryReservationResult{},
	}
	reserver.results[item1.ID] = reservation.InventoryReservationResult{
		CartItemID: item1.ID,
		ProductID:  item1.ProductID,
		Qty:        item1.Quantity,
		Reserved:   true,
	}
	reserver.results[item2.ID] = reservation.InventoryReservationResult{
		CartItemID: item2.ID,
		ProductID:  item2.ProductID,
		Qty:        item2.Quantity,
		Reserved:   false,
		Reason:     "insufficient_inventory",
	}

	orderRepo := newStubOrdersRepository()
	publisher := &stubOutboxPublisher{}

	service, err := NewService(
		stubTxRunner{},
		cartRepo,
		orderRepo,
		storeSvc,
		productLoader,
		reserver,
		publisher,
		false,
	)
	if err != nil {
		t.Fatalf("build service: %v", err)
	}

	result, err := service.Execute(context.Background(), buyerID, cartRecord.ID, CheckoutInput{
		IdempotencyKey:  "key",
		ShippingAddress: &types.Address{Line1: "123 Market", City: "Tulsa", State: "OK", PostalCode: "74104", Country: "US"},
		PaymentMethod:   enums.PaymentMethodCash,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if len(result.VendorOrders) != 1 {
		t.Fatalf("expected 1 vendor order, got %d", len(result.VendorOrders))
	}

	order := result.VendorOrders[0]
	if order.SubtotalCents != 4000 {
		t.Fatalf("subtotal mismatch: got %d", order.SubtotalCents)
	}
	if order.DiscountsCents != 500 {
		t.Fatalf("discount mismatch: got %d", order.DiscountsCents)
	}
	if order.TotalCents != 3500 {
		t.Fatalf("total mismatch: got %d", order.TotalCents)
	}
	if order.BalanceDueCents != 3500 {
		t.Fatalf("balance due mismatch: %d", order.BalanceDueCents)
	}

	if len(order.Items) != 2 {
		t.Fatalf("expected 2 line items, got %d", len(order.Items))
	}
	if order.Items[0].Status != enums.LineItemStatusPending {
		t.Fatalf("first line should be pending, got %s", order.Items[0].Status)
	}
	if order.Items[1].Status != enums.LineItemStatusRejected {
		t.Fatalf("second line should be rejected, got %s", order.Items[1].Status)
	}

	intent, ok := orderRepo.paymentIntents[order.ID]
	if !ok {
		t.Fatalf("payment intent missing for order %s", order.ID)
	}
	if intent.AmountCents != order.TotalCents {
		t.Fatalf("payment intent amount mismatch (%d vs %d)", intent.AmountCents, order.TotalCents)
	}
}

func ptrString(value string) *string {
	return &value
}

type stubTxRunner struct{}

func (stubTxRunner) WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return fn(nil)
}

type stubCartRepo struct {
	record  *models.CartRecord
	updated *models.CartRecord
}

func (s *stubCartRepo) WithTx(tx *gorm.DB) cart.CartRepository {
	return s
}

func (s *stubCartRepo) FindActiveByBuyerStore(ctx context.Context, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	return nil, gorm.ErrRecordNotFound
}

func (s *stubCartRepo) FindByIDAndBuyerStore(ctx context.Context, id, buyerStoreID uuid.UUID) (*models.CartRecord, error) {
	if s.record == nil || s.record.ID != id || s.record.BuyerStoreID != buyerStoreID {
		return nil, gorm.ErrRecordNotFound
	}
	return s.record, nil
}

func (s *stubCartRepo) Create(ctx context.Context, record *models.CartRecord) (*models.CartRecord, error) {
	return nil, errors.New("not implemented")
}

func (s *stubCartRepo) Update(ctx context.Context, record *models.CartRecord) (*models.CartRecord, error) {
	s.updated = record
	return record, nil
}

func (s *stubCartRepo) ReplaceItems(ctx context.Context, cartID uuid.UUID, items []models.CartItem) error {
	return errors.New("not implemented")
}

func (s *stubCartRepo) ReplaceVendorGroups(ctx context.Context, cartID uuid.UUID, groups []models.CartVendorGroup) error {
	return errors.New("not implemented")
}

func (s *stubCartRepo) UpdateStatus(ctx context.Context, id, buyerStoreID uuid.UUID, status enums.CartStatus) error {
	return errors.New("not implemented")
}

type stubStoreService struct {
	records map[uuid.UUID]*stores.StoreDTO
}

func (s *stubStoreService) GetByID(ctx context.Context, id uuid.UUID) (*stores.StoreDTO, error) {
	if store, ok := s.records[id]; ok {
		return store, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (*stubStoreService) Update(ctx context.Context, userID, storeID uuid.UUID, input stores.UpdateStoreInput) (*stores.StoreDTO, error) {
	return nil, errors.New("not implemented")
}

func (*stubStoreService) ListUsers(ctx context.Context, userID, storeID uuid.UUID) ([]memberships.StoreUserDTO, error) {
	return nil, errors.New("not implemented")
}

func (*stubStoreService) InviteUser(ctx context.Context, inviterID, storeID uuid.UUID, input stores.InviteUserInput) (*memberships.StoreUserDTO, string, error) {
	return nil, "", errors.New("not implemented")
}

func (*stubStoreService) RemoveUser(ctx context.Context, actorID, storeID, targetUserID uuid.UUID) error {
	return errors.New("not implemented")
}

type stubProductLoader struct {
	products map[uuid.UUID]*models.Product
}

func (s stubProductLoader) FindByID(ctx context.Context, id uuid.UUID) (*models.Product, error) {
	if product, ok := s.products[id]; ok {
		return product, nil
	}
	return nil, gorm.ErrRecordNotFound
}

type stubReservationRunner struct {
	results map[uuid.UUID]reservation.InventoryReservationResult
}

func (s stubReservationRunner) Reserve(ctx context.Context, tx *gorm.DB, requests []reservation.InventoryReservationRequest) ([]reservation.InventoryReservationResult, error) {
	results := make([]reservation.InventoryReservationResult, len(requests))
	for i, req := range requests {
		if res, ok := s.results[req.CartItemID]; ok {
			results[i] = res
			continue
		}
		results[i] = reservation.InventoryReservationResult{
			CartItemID: req.CartItemID,
			ProductID:  req.ProductID,
			Qty:        req.Qty,
			Reserved:   true,
		}
	}
	return results, nil
}

type stubOutboxPublisher struct {
	calls int
}

func (s *stubOutboxPublisher) Emit(ctx context.Context, tx *gorm.DB, event outbox.DomainEvent) error {
	s.calls++
	return nil
}

type stubOrdersRepository struct {
	vendorOrders   map[uuid.UUID]*models.VendorOrder
	lineItems      map[uuid.UUID][]models.OrderLineItem
	paymentIntents map[uuid.UUID]*models.PaymentIntent
}

func newStubOrdersRepository() *stubOrdersRepository {
	return &stubOrdersRepository{
		vendorOrders:   make(map[uuid.UUID]*models.VendorOrder),
		lineItems:      make(map[uuid.UUID][]models.OrderLineItem),
		paymentIntents: make(map[uuid.UUID]*models.PaymentIntent),
	}
}

func (s *stubOrdersRepository) WithTx(tx *gorm.DB) orders.Repository {
	return s
}

func (s *stubOrdersRepository) CreateVendorOrder(ctx context.Context, order *models.VendorOrder) (*models.VendorOrder, error) {
	if order.ID == uuid.Nil {
		order.ID = uuid.New()
	}
	s.vendorOrders[order.ID] = order
	return order, nil
}

func (s *stubOrdersRepository) CreateOrderLineItems(ctx context.Context, items []models.OrderLineItem) error {
	if len(items) == 0 {
		return nil
	}
	orderID := items[0].OrderID
	s.lineItems[orderID] = append(s.lineItems[orderID], items...)
	return nil
}

func (s *stubOrdersRepository) CreatePaymentIntent(ctx context.Context, intent *models.PaymentIntent) (*models.PaymentIntent, error) {
	s.paymentIntents[intent.OrderID] = intent
	return intent, nil
}

func (s *stubOrdersRepository) FindVendorOrdersByCheckoutGroup(ctx context.Context, checkoutGroupID uuid.UUID) ([]models.VendorOrder, error) {
	var orders []models.VendorOrder
	for _, order := range s.vendorOrders {
		copy := *order
		copy.Items = append([]models.OrderLineItem(nil), s.lineItems[order.ID]...)
		if intent, ok := s.paymentIntents[order.ID]; ok {
			copy.PaymentIntent = intent
		}
		orders = append(orders, copy)
	}
	return orders, nil
}

func (*stubOrdersRepository) FindOrderLineItemsByOrder(ctx context.Context, orderID uuid.UUID) ([]models.OrderLineItem, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) FindOrderLineItem(ctx context.Context, lineItemID uuid.UUID) (*models.OrderLineItem, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) FindPaymentIntentByOrder(ctx context.Context, orderID uuid.UUID) (*models.PaymentIntent, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) ListBuyerOrders(ctx context.Context, buyerStoreID uuid.UUID, params pagination.Params, filters orders.BuyerOrderFilters) (*orders.BuyerOrderList, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) ListVendorOrders(ctx context.Context, vendorStoreID uuid.UUID, params pagination.Params, filters orders.VendorOrderFilters) (*orders.VendorOrderList, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) ListUnassignedHoldOrders(ctx context.Context, params pagination.Params) (*orders.AgentOrderQueueList, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) ListAssignedOrders(ctx context.Context, agentID uuid.UUID, params pagination.Params) (*orders.AgentOrderQueueList, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) ListPayoutOrders(ctx context.Context, params pagination.Params) (*orders.PayoutOrderList, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) FindOrderDetail(ctx context.Context, orderID uuid.UUID) (*orders.OrderDetail, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) FindPendingOrdersBefore(ctx context.Context, cutoff time.Time) ([]models.VendorOrder, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) FindVendorOrder(ctx context.Context, orderID uuid.UUID) (*models.VendorOrder, error) {
	return nil, errors.New("not implemented")
}

func (*stubOrdersRepository) UpdateVendorOrderStatus(ctx context.Context, orderID uuid.UUID, status enums.VendorOrderStatus) error {
	return errors.New("not implemented")
}

func (*stubOrdersRepository) UpdateOrderLineItemStatus(ctx context.Context, lineItemID uuid.UUID, status enums.LineItemStatus, notes *string) error {
	return errors.New("not implemented")
}

func (s *stubOrdersRepository) UpdateVendorOrder(ctx context.Context, orderID uuid.UUID, updates map[string]any) error {
	order, ok := s.vendorOrders[orderID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	if status, ok := updates["status"].(enums.VendorOrderStatus); ok {
		order.Status = status
	}
	if balance, ok := updates["balance_due_cents"].(int); ok {
		order.BalanceDueCents = balance
	}
	return nil
}

func (*stubOrdersRepository) UpdatePaymentIntent(ctx context.Context, orderID uuid.UUID, updates map[string]any) error {
	return errors.New("not implemented")
}

func (*stubOrdersRepository) UpdateOrderAssignment(ctx context.Context, assignmentID uuid.UUID, updates map[string]any) error {
	return errors.New("not implemented")
}
