package paymentmethods

import (
	"context"
	"errors"

	"github.com/angelmondragon/packfinderz-backend/pkg/db/models"
	pkgerrors "github.com/angelmondragon/packfinderz-backend/pkg/errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UpdatePaymentMethodDefault toggles the default flag for a stored payment method.
func (s *service) UpdatePaymentMethodDefault(ctx context.Context, storeID uuid.UUID, paymentMethodID uuid.UUID, isDefault bool) (*models.PaymentMethod, error) {
	if storeID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "store id is required")
	}
	if paymentMethodID == uuid.Nil {
		return nil, pkgerrors.New(pkgerrors.CodeValidation, "payment method id is required")
	}

	var updated *models.PaymentMethod
	if err := s.txRunner.WithTx(ctx, func(tx *gorm.DB) error {
		txRepo := s.repo.WithTx(tx)
		if isDefault {
			if err := txRepo.ClearDefaultPaymentMethod(ctx, storeID); err != nil {
				return err
			}
		}

		if err := txRepo.UpdatePaymentMethodDefault(ctx, storeID, paymentMethodID, isDefault); err != nil {
			return err
		}

		methods, err := txRepo.ListPaymentMethodsByStore(ctx, storeID)
		if err != nil {
			return err
		}

		for i := range methods {
			if methods[i].ID == paymentMethodID {
				method := methods[i]
				updated = &method
				break
			}
		}
		if updated == nil {
			return gorm.ErrRecordNotFound
		}
		return nil
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.Wrap(pkgerrors.CodeNotFound, err, "payment method not found")
		}
		return nil, pkgerrors.Wrap(pkgerrors.CodeDependency, err, "update default payment method")
	}

	return updated, nil
}
