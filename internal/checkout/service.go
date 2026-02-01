package checkout

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/angelmondragon/packfinderz-backend/internal/cart"
	"github.com/angelmondragon/packfinderz-backend/internal/checkout/helpers"
	"github.com/angelmondragon/packfinderz-backend/internal/checkout/reservation"
	"github.com/angelmondragon/packfinderz-backend/internal/orders"
	"github.com/angelmondragon/packfinderz-backend/internal/stores"
	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	"github.com/angelmondragon/packfinderz-backend/pkg/enums"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox"
	"github.com/angelmondragon/packfinderz-backend/pkg/outbox/payloads"
	"github.com/angelmondragon/packfinderz-backend/pkg/types"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type txRunner interface {
	WithTx(ctx context.Context, fn func(tx *gorm.DB) error) error
}

type productLoader interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.Product, error)
}

type reservationRunner interface {
	Reserve(ctx context.Context, tx *gorm.DB, requests []reservation.InventoryReservationRequest) ([]reservation.InventoryReservationResult, error)
}

type outboxPublisher interface {
	Emit(ctx context.Context, tx *gorm.DB, event outbox.DomainEvent) error
}

type reservationEngine struct{}

func (reservationEngine) Reserve(ctx context.Context, tx *gorm.DB, requests []reservation.InventoryReservationRequest) ([]reservation.InventoryReservationResult, error) {
	return reservation.ReserveInventory(ctx, tx, requests)
}

// Service executes checkout orchestration.
type Service interface {
	Execute(ctx context.Context, buyerStoreID, cartID uuid.UUID, input CheckoutInput) (*models.CheckoutGroup, error)
}

// CheckoutInput captures optional data used during checkout.
type CheckoutInput struct {
	IdempotencyKey  string
	ShippingAddress *types.Address
	PaymentMethod   enums.PaymentMethod
	ShippingLine    *types.ShippingLine
}

type service struct {
	tx          txRunner
	cartRepo    cart.CartRepository
	ordersRepo  orders.Repository
	storeSvc    stores.Service
	productRepo productLoader
	reservation reservationRunner
	outbox      outboxPublisher
}

// NewService builds the checkout service.
func NewService(
	tx txRunner,
	cartRepo cart.CartRepository,
	ordersRepo orders.Repository,
	storeSvc stores.Service,
	productRepo productLoader,
	reservation reservationRunner,
	publisher outboxPublisher,
) (Service, error) {
	if tx == nil {
		return nil, fmt.Errorf("tx runner required")
	}
	if cartRepo == nil {
		return nil, fmt.Errorf("cart repository required")
	}
	if ordersRepo == nil {
		return nil, fmt.Errorf("orders repository required")
	}
	if storeSvc == nil {
		return nil, fmt.Errorf("store service required")
	}
	if productRepo == nil {
		return nil, fmt.Errorf("product loader required")
	}
	if reservation == nil {
		reservation = reservationEngine{}
	}
	if publisher == nil {
		return nil, fmt.Errorf("outbox publisher required")
	}
	return &service{
		tx:          tx,
		cartRepo:    cartRepo,
		ordersRepo:  ordersRepo,
		storeSvc:    storeSvc,
		productRepo: productRepo,
		reservation: reservation,
		outbox:      publisher,
	}, nil
}

func (s *service) Execute(ctx context.Context, buyerStoreID, cartID uuid.UUID, input CheckoutInput) (*models.CheckoutGroup, error) {
	if buyerStoreID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "buyer store id required")
	}
	if cartID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "cart id required")
	}

	var (
		result               *models.CheckoutGroup
		vendorGroupSnapshots []models.CartVendorGroup
	)
	err := s.tx.WithTx(ctx, func(tx *gorm.DB) error {
		cartRepo := s.cartRepo.WithTx(tx)
		ordersRepo := s.ordersRepo.WithTx(tx)

		record, err := cartRepo.FindByIDAndBuyerStore(ctx, cartID, buyerStoreID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return pkgerrors.New(pkgerrors.CodeNotFound, "cart not found")
			}
			return err
		}
		if err := validateCartForCheckout(record); err != nil {
			return err
		}

		buyerStore, err := s.storeSvc.GetByID(ctx, buyerStoreID)
		if err != nil {
			return err
		}
		buyerState, err := helpers.ValidateBuyerStore(buyerStore)
		if err != nil {
			return err
		}

		eligibleItems := make([]models.CartItem, 0, len(record.Items))
		for _, item := range record.Items {
			if item.Status == enums.CartItemStatusOK {
				eligibleItems = append(eligibleItems, item)
			}
		}
		if len(eligibleItems) == 0 {
			return pkgerrors.New(pkgerrors.CodeConflict, "cart contains no orderable items")
		}

		requests := make([]reservation.InventoryReservationRequest, len(eligibleItems))
		for i, item := range eligibleItems {
			requests[i] = reservation.InventoryReservationRequest{
				CartItemID: item.ID,
				ProductID:  item.ProductID,
				Qty:        item.Quantity,
			}
		}

		reservations, err := s.reservation.Reserve(ctx, tx, requests)
		if err != nil {
			return err
		}
		reservationMap := make(map[uuid.UUID]reservation.InventoryReservationResult, len(reservations))
		for _, res := range reservations {
			reservationMap[res.CartItemID] = res
		}

		productCache := map[uuid.UUID]*models.Product{}
		vendorCache := map[uuid.UUID]*stores.StoreDTO{}
		grouped := helpers.GroupCartItemsByVendor(eligibleItems)
		vendorGroups := map[uuid.UUID]models.CartVendorGroup{}
		for _, group := range record.VendorGroups {
			vendorGroups[group.VendorStoreID] = group
		}
		vendorOrderIDs := make([]uuid.UUID, 0, len(grouped))

		appliedShippingAddress := input.ShippingAddress
		if appliedShippingAddress == nil {
			appliedShippingAddress = record.ShippingAddress
		}
		appliedPaymentMethod := input.PaymentMethod
		if appliedPaymentMethod == "" {
			appliedPaymentMethod = enums.PaymentMethodCash
		}
		appliedShippingLine := input.ShippingLine

		checkoutGroupID := record.CheckoutGroupID
		if checkoutGroupID == nil {
			groupID := uuid.New()
			checkoutGroupID = &groupID
			record.CheckoutGroupID = checkoutGroupID
		}

		checkoutGroup := &models.CheckoutGroup{
			ID:           *checkoutGroupID,
			BuyerStoreID: buyerStoreID,
			CartID:       &record.ID,
		}
		createdGroup, err := ordersRepo.CreateCheckoutGroup(ctx, checkoutGroup)
		if err != nil {
			return err
		}

		vendorGroupSnapshots = append([]models.CartVendorGroup(nil), record.VendorGroups...)

		for vendorID, items := range grouped {
			if _, err := s.loadVendorStore(ctx, vendorID, buyerState, vendorCache); err != nil {
				return err
			}

			cartGroup, ok := vendorGroups[vendorID]
			if !ok {
				return pkgerrors.New(pkgerrors.CodeInternal, fmt.Sprintf("missing vendor group for vendor %s", vendorID))
			}
			totals := cartGroup.SubtotalCents
			discounts := cartGroup.SubtotalCents - cartGroup.TotalCents
			if discounts < 0 {
				discounts = 0
			}
			order := &models.VendorOrder{
				CartID:            record.ID,
				CheckoutGroupID:   createdGroup.ID,
				BuyerStoreID:      buyerStoreID,
				VendorStoreID:     vendorID,
				Currency:          record.Currency,
				ShippingAddress:   appliedShippingAddress,
				SubtotalCents:     totals,
				DiscountsCents:    discounts,
				TaxCents:          0,
				TransportFeeCents: 0,
				PaymentMethod:     appliedPaymentMethod,
				TotalCents:        cartGroup.TotalCents,
				BalanceDueCents:   cartGroup.TotalCents,
				Warnings:          cartGroup.Warnings,
				Promo:             cartGroup.Promo,
				ShippingLine:      appliedShippingLine,
			}
			createdOrder, err := ordersRepo.CreateVendorOrder(ctx, order)
			if err != nil {
				return err
			}

			lineItems := make([]models.OrderLineItem, 0, len(items))
			anyReserved := false
			for _, item := range items {
				product, err := s.loadProduct(ctx, item.ProductID, productCache)
				if err != nil {
					return err
				}
				result := reservationMap[item.ID]
				if result.Reserved {
					anyReserved = true
				}
				lineItems = append(lineItems, buildLineItem(createdOrder.ID, item, product, result))
			}

			if err := ordersRepo.CreateOrderLineItems(ctx, lineItems); err != nil {
				return err
			}
			if !anyReserved {
				updates := map[string]any{
					"status":            enums.VendorOrderStatusRejected,
					"balance_due_cents": 0,
				}
				if err := ordersRepo.UpdateVendorOrder(ctx, createdOrder.ID, updates); err != nil {
					return err
				}
				createdOrder.Status = enums.VendorOrderStatusRejected
				createdOrder.BalanceDueCents = 0
			}
			intent := &models.PaymentIntent{
				OrderID:     createdOrder.ID,
				Method:      appliedPaymentMethod,
				Status:      enums.PaymentStatusUnpaid,
				AmountCents: cartGroup.TotalCents,
			}
			if _, err := ordersRepo.CreatePaymentIntent(ctx, intent); err != nil {
				return err
			}
			vendorOrderIDs = append(vendorOrderIDs, createdOrder.ID)
		}

		finalizeCart(record, appliedShippingAddress, appliedPaymentMethod, appliedShippingLine)
		if _, err := cartRepo.Update(ctx, record); err != nil {
			return err
		}

		if err := s.emitOrderCreatedEvent(ctx, tx, createdGroup.ID, vendorOrderIDs); err != nil {
			return err
		}

		result, err = ordersRepo.FindCheckoutGroupByID(ctx, createdGroup.ID)
		if err != nil {
			return err
		}
		result.CartVendorGroups = vendorGroupSnapshots
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *service) loadVendorStore(ctx context.Context, vendorID uuid.UUID, buyerState string, cache map[uuid.UUID]*stores.StoreDTO) (*stores.StoreDTO, error) {
	if vendor, ok := cache[vendorID]; ok {
		return vendor, nil
	}
	vendor, err := s.storeSvc.GetByID(ctx, vendorID)
	if err != nil {
		return nil, err
	}
	if err := helpers.ValidateVendorStore(vendor, buyerState); err != nil {
		return nil, err
	}
	cache[vendorID] = vendor
	return vendor, nil
}

func (s *service) loadProduct(ctx context.Context, productID uuid.UUID, cache map[uuid.UUID]*models.Product) (*models.Product, error) {
	if product, ok := cache[productID]; ok {
		return product, nil
	}
	product, err := s.productRepo.FindByID(ctx, productID)
	if err != nil {
		return nil, err
	}
	cache[productID] = product
	return product, nil
}

func (s *service) emitOrderCreatedEvent(ctx context.Context, tx *gorm.DB, checkoutGroupID uuid.UUID, vendorOrderIDs []uuid.UUID) error {
	event := outbox.DomainEvent{
		EventType:     enums.EventOrderCreated,
		AggregateType: enums.AggregateCheckoutGroup,
		AggregateID:   checkoutGroupID,
		Data: payloads.OrderCreatedEvent{
			CheckoutGroupID: checkoutGroupID,
			VendorOrderIDs:  append([]uuid.UUID{}, vendorOrderIDs...),
		},
		Version: 1,
	}
	return s.outbox.Emit(ctx, tx, event)
}

func buildLineItem(orderID uuid.UUID, cartItem models.CartItem, product *models.Product, reservation reservation.InventoryReservationResult) models.OrderLineItem {
	total := cartItem.LineSubtotalCents
	if total == 0 {
		total = cartItem.UnitPriceCents * cartItem.Quantity
	}
	discount := 0
	if cartItem.AppliedVolumeDiscount != nil {
		discount = cartItem.AppliedVolumeDiscount.AmountCents
	}
	if discount < 0 {
		discount = 0
	}
	status := enums.LineItemStatusPending
	var notes *string
	if !reservation.Reserved {
		status = enums.LineItemStatusRejected
		reason := reservation.Reason
		notes = &reason
	}

	category := ""
	if product != nil {
		category = string(product.Category)
	}

	var classification *string
	if product != nil && product.Classification != nil {
		val := string(*product.Classification)
		classification = &val
	}

	name := ""
	if product != nil {
		name = product.SKU
		if product.Title != "" {
			name = product.Title
		}
	}
	if name == "" {
		name = cartItem.ProductID.String()
	}

	unit := enums.ProductUnit("")
	if product != nil {
		unit = product.Unit
	}

	return models.OrderLineItem{
		CartItemID:            &cartItem.ID,
		OrderID:               orderID,
		ProductID:             &cartItem.ProductID,
		Name:                  name,
		Category:              category,
		Strain:                product.Strain,
		Classification:        classification,
		Unit:                  unit,
		UnitPriceCents:        cartItem.UnitPriceCents,
		MOQ:                   cartItem.MOQ,
		MaxQty:                cartItem.MaxQty,
		Qty:                   cartItem.Quantity,
		DiscountCents:         discount,
		LineSubtotalCents:     cartItem.LineSubtotalCents,
		TotalCents:            total,
		Warnings:              cartItem.Warnings,
		AppliedVolumeDiscount: cartItem.AppliedVolumeDiscount,
		AttributedToken:       nil,
		Status:                status,
		Notes:                 notes,
	}
}

func validateCartForCheckout(record *models.CartRecord) error {
	if record == nil {
		return pkgerrors.New(pkgerrors.CodeValidation, "cart missing")
	}
	if record.Status != enums.CartStatusActive {
		return pkgerrors.New(pkgerrors.CodeConflict, "cart must be active")
	}
	hasOrderableItem := false
	for _, item := range record.Items {
		if item.Status == enums.CartItemStatusOK {
			hasOrderableItem = true
			break
		}
	}
	if !hasOrderableItem {
		return pkgerrors.New(pkgerrors.CodeConflict, "cart contains no orderable items")
	}
	return nil
}

func finalizeCart(record *models.CartRecord, shippingAddress *types.Address, paymentMethod enums.PaymentMethod, shippingLine *types.ShippingLine) {
	if record == nil {
		return
	}
	record.ShippingAddress = shippingAddress
	record.ShippingLine = shippingLine
	method := paymentMethod
	record.PaymentMethod = &method
	now := time.Now().UTC()
	record.ConvertedAt = &now
	record.Status = enums.CartStatusConverted
}
