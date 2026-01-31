package checkout

import (
	"context"
	"fmt"

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
	AttributedAdClickID *uuid.UUID
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

	var result *models.CheckoutGroup
	err := s.tx.WithTx(ctx, func(tx *gorm.DB) error {
		cartRepo := s.cartRepo.WithTx(tx)
		ordersRepo := s.ordersRepo.WithTx(tx)

		record, err := cartRepo.FindByIDAndBuyerStore(ctx, cartID, buyerStoreID)
		if err != nil {
			return err
		}
		if record.Status != enums.CartStatusActive {
			return pkgerrors.New(pkgerrors.CodeConflict, "cart already processed")
		}
		if len(record.Items) == 0 {
			return pkgerrors.New(pkgerrors.CodeValidation, "cart contains no items")
		}

		buyerStore, err := s.storeSvc.GetByID(ctx, buyerStoreID)
		if err != nil {
			return err
		}
		buyerState, err := helpers.ValidateBuyerStore(buyerStore)
		if err != nil {
			return err
		}

		requests := make([]reservation.InventoryReservationRequest, len(record.Items))
		for i, item := range record.Items {
			requests[i] = reservation.InventoryReservationRequest{
				CartItemID: item.ID,
				ProductID:  item.ProductID,
				Qty:        item.Qty,
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
		totalsByVendor := helpers.ComputeTotalsByVendor(record.Items)
		grouped := helpers.GroupCartItemsByVendor(record.Items)
		vendorOrderIDs := make([]uuid.UUID, 0, len(grouped))

		checkoutGroup := &models.CheckoutGroup{
			BuyerStoreID:        buyerStoreID,
			CartID:              &record.ID,
			AttributedAdClickID: input.AttributedAdClickID,
		}
		createdGroup, err := ordersRepo.CreateCheckoutGroup(ctx, checkoutGroup)
		if err != nil {
			return err
		}

		for vendorID, items := range grouped {
			if _, err := s.loadVendorStore(ctx, vendorID, buyerState, vendorCache); err != nil {
				return err
			}

			totals := totalsByVendor[vendorID]
			order := &models.VendorOrder{
				CheckoutGroupID:   createdGroup.ID,
				BuyerStoreID:      buyerStoreID,
				VendorStoreID:     vendorID,
				SubtotalCents:     totals.SubtotalCents,
				DiscountCents:     totals.DiscountCents,
				TaxCents:          0,
				TransportFeeCents: 0,
				TotalCents:        totals.TotalCents,
				BalanceDueCents:   totals.TotalCents,
			}
			createdOrder, err := ordersRepo.CreateVendorOrder(ctx, order)
			if err != nil {
				return err
			}

			lineItems := make([]models.OrderLineItem, 0, len(items))
			for _, item := range items {
				product, err := s.loadProduct(ctx, item.ProductID, productCache)
				if err != nil {
					return err
				}
				result := reservationMap[item.ID]
				lineItems = append(lineItems, buildLineItem(createdOrder.ID, item, product, result))
			}

			if err := ordersRepo.CreateOrderLineItems(ctx, lineItems); err != nil {
				return err
			}
			intent := &models.PaymentIntent{
				OrderID:     createdOrder.ID,
				AmountCents: totals.TotalCents,
			}
			if _, err := ordersRepo.CreatePaymentIntent(ctx, intent); err != nil {
				return err
			}
			vendorOrderIDs = append(vendorOrderIDs, createdOrder.ID)
		}

		if err := cartRepo.UpdateStatus(ctx, record.ID, buyerStoreID, enums.CartStatusConverted); err != nil {
			return err
		}

		if err := s.emitOrderCreatedEvent(ctx, tx, createdGroup.ID, vendorOrderIDs); err != nil {
			return err
		}

		result, err = ordersRepo.FindCheckoutGroupByID(ctx, createdGroup.ID)
		return err
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
	subtotal := cartItem.UnitPriceCents * cartItem.Qty
	total := subtotal
	if cartItem.SubTotalPrice != nil {
		total = *cartItem.SubTotalPrice
	} else if cartItem.DiscountedPrice != nil {
		total = (*cartItem.DiscountedPrice) * cartItem.Qty
	}
	discount := subtotal - total
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

	name := cartItem.ProductSKU
	if product != nil && product.Title != "" {
		name = product.Title
	}

	return models.OrderLineItem{
		OrderID:        orderID,
		ProductID:      &cartItem.ProductID,
		Name:           name,
		Category:       category,
		Strain:         product.Strain,
		Classification: classification,
		Unit:           cartItem.Unit,
		UnitPriceCents: cartItem.UnitPriceCents,
		Qty:            cartItem.Qty,
		DiscountCents:  discount,
		TotalCents:     total,
		Status:         status,
		Notes:          notes,
	}
}
