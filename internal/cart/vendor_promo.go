package cart

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// VendorPromo describes a vendor-scoped promo code.
type VendorPromo struct {
	VendorStoreID uuid.UUID
	Code          string
	AmountCents   int
	Active        bool
	ExpiresAt     time.Time
}

// IsValid reports whether the promo is active and not expired.
func (p VendorPromo) IsValid(now time.Time) bool {
	if !p.Active {
		return false
	}
	if !p.ExpiresAt.IsZero() && now.After(p.ExpiresAt) {
		return false
	}
	return true
}

type promoLoader interface {
	GetVendorPromo(ctx context.Context, vendorID uuid.UUID, code string) (*VendorPromo, error)
}

type promoLoaderFunc func(ctx context.Context, vendorID uuid.UUID, code string) (*VendorPromo, error)

func (fn promoLoaderFunc) GetVendorPromo(ctx context.Context, vendorID uuid.UUID, code string) (*VendorPromo, error) {
	return fn(ctx, vendorID, code)
}

func noopPromoLoader() promoLoader {
	return promoLoaderFunc(func(ctx context.Context, vendorID uuid.UUID, code string) (*VendorPromo, error) {
		return nil, nil
	})
}

// NoopPromoLoader returns a promo loader that never resolves any promo.
func NoopPromoLoader() promoLoader {
	return noopPromoLoader()
}
